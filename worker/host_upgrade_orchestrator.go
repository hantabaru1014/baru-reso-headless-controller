package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// HostUpgradeOrchestrator rolls hosts that opted in
// (AutoUpdatePolicy = USERS_EMPTY) onto the latest container tag without
// disturbing active users.
//
// Lifecycle for one host:
//  1. ResoniteVersionUsecase reports a successful build via OnNewImage.
//     Builds of anything other than the current latest release image
//     are ignored. The orchestrator looks at every running host with
//     USERS_EMPTY policy (already-draining hosts are retargeted),
//     and for those whose current AppVersion differs from the new tag it
//     (a) snapshots the host's StartupConfig + account (start_worlds in
//     particular MUST be captured here — once we stop the sessions the
//     live container has nothing to report) and (b) RPC-fetches each
//     session's current user count so the in-memory counter has a
//     trustworthy baseline.
//  2. While in drainTargets, StartSession on the host is rejected by
//     SessionUsecase (via the port.HostDrainer interface).
//  3. The periodic tick and per-host UserLeftSession events both call
//     reconcileHost: any session whose user count is 0 is stopped via
//     port.SessionStopper.
//  4. When no RUNNING session remains for the host, Restart is invoked
//     with the snapshotted StartupConfig + target tag. On success the
//     host leaves drainTargets (unless a newer image retargeted it while
//     the restart was in flight, in which case it keeps draining towards
//     that); on failure we increment an attempt counter and abort after
//     maxRestartAttempts so a permanently broken host doesn't block its
//     session admission forever.
//
// Discovery is driven only by build-success notifications. A
// notification whose handling did not complete (latest-release lookup
// failed, or some host's enroll was deferred) is kept as pendingImage and
// retried by the periodic tick.
//
// State is purely in-memory; a controller restart loses it and nothing
// re-discovers the upgrade until the next successful build.
type HostUpgradeOrchestrator struct {
	hostRepo           port.HeadlessHostRepository
	sessionRepo        port.SessionRepository
	accountRepo        HeadlessAccountFetcher
	releaseResolver    LatestReleaseResolver
	interval           time.Duration
	restartStopTimeout int
	maxRestartAttempts int

	mu            sync.Mutex
	drainTargets  map[string]*drainState      // hostID -> in-flight upgrade state
	sessionUsers  map[string]map[string]int32 // hostID -> sessionID -> user count
	reconcileLock map[string]*sync.Mutex      // hostID -> per-host reconcile lock

	// pendingImage is the most recent build notification whose handling
	// has not completed yet; the periodic tick retries it. Guarded by mu.
	pendingImage *port.ContainerImage

	// newImageMu serialises new-image handling. A run decides "is this
	// the latest release?" once up front and may then spend minutes in
	// enroll/restart, so letting runs interleave could have an older run
	// retarget a host back onto the image a newer run just moved it off.
	newImageMu sync.Mutex

	// sessionStopper is wired AFTER construction (see SetSessionStopper)
	// to break the SessionUsecase ↔ orchestrator construction cycle.
	stopperMu      sync.RWMutex
	sessionStopper port.SessionStopper

	// lifecycleCtx is the ctx the Manager passed to Run. OnNewImage
	// spawns its per-host work in a goroutine using this ctx (rather
	// than the build job's ctx it is notified with) so long restarts can
	// complete and so Manager-initiated shutdown still cancels the work.
	// Mirrors HostEventWatcher.
	lifecycleMu  sync.RWMutex
	lifecycleCtx context.Context //nolint:containedctx // see comment above
}

// drainState is everything the orchestrator captured at enroll time —
// the StartupConfig snapshot (in particular start_worlds), the resolved
// account credentials and the target tag — plus the running attempt
// counter so a permanently-failing restart eventually aborts.
type drainState struct {
	targetTag    string
	restartParms port.HeadlessHostStartParams
	attempts     int
}

// LatestReleaseResolver resolves the image tag hosts should currently be
// running. It is the slice of ResoniteVersionUsecase the orchestrator
// needs: ResolveForStart("latestRelease") returns the built tag of the
// newest `headless` branch version, or an error (NotBuiltError) when that
// version has not been built yet.
type LatestReleaseResolver interface {
	ResolveForStart(ctx context.Context, tagInput string) (string, error)
}

// latestReleaseTagInput is the ResolveForStart input meaning "the newest
// version on the headless (release) branch".
const latestReleaseTagInput = "latestRelease"

// HeadlessAccountFetcher is the slice of HeadlessAccountUsecase the
// orchestrator needs. It is an interface so tests can substitute it
// without pulling in skyfrost.
type HeadlessAccountFetcher interface {
	GetHeadlessAccount(ctx context.Context, groupID, resoniteID string) (*entity.HeadlessAccount, error)
}

var (
	_ Runner           = (*HostUpgradeOrchestrator)(nil)
	_ HostEventHandler = (*HostUpgradeOrchestrator)(nil)
	_ port.HostDrainer = (*HostUpgradeOrchestrator)(nil)
)

const (
	// defaultRestartStopTimeoutSeconds is how long Restart waits for a
	// graceful container stop before forcing it down. Resonite headless
	// sometimes takes a while to flush state on shutdown, so we err on
	// the generous side.
	defaultRestartStopTimeoutSeconds = 180

	// defaultMaxRestartAttempts caps consecutive restart failures for a
	// host before the orchestrator gives up and removes it from the
	// drain set. Otherwise a permanently broken host would block all
	// new sessions on it forever.
	defaultMaxRestartAttempts = 5
)

func NewHostUpgradeOrchestrator(
	hostRepo port.HeadlessHostRepository,
	sessionRepo port.SessionRepository,
	accountRepo HeadlessAccountFetcher,
	releaseResolver LatestReleaseResolver,
	cfg *config.WorkerConfig,
) *HostUpgradeOrchestrator {
	return &HostUpgradeOrchestrator{
		hostRepo:           hostRepo,
		sessionRepo:        sessionRepo,
		accountRepo:        accountRepo,
		releaseResolver:    releaseResolver,
		interval:           cfg.UpgradeCheckInterval,
		restartStopTimeout: defaultRestartStopTimeoutSeconds,
		maxRestartAttempts: defaultMaxRestartAttempts,
		drainTargets:       make(map[string]*drainState),
		sessionUsers:       make(map[string]map[string]int32),
		reconcileLock:      make(map[string]*sync.Mutex),
	}
}

// SetSessionStopper installs the SessionStopper after construction.
// Required because SessionUsecase depends on HostDrainer (= orchestrator)
// which would otherwise form a wire-time cycle. The HTTP server's
// constructor performs this linkage once both ends are built.
func (o *HostUpgradeOrchestrator) SetSessionStopper(s port.SessionStopper) {
	o.stopperMu.Lock()
	defer o.stopperMu.Unlock()

	o.sessionStopper = s
}

func (o *HostUpgradeOrchestrator) Name() string { return "host-upgrade-orchestrator" }

// IsHostDraining implements port.HostDrainer.
func (o *HostUpgradeOrchestrator) IsHostDraining(hostID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	_, ok := o.drainTargets[hostID]

	return ok
}

// Run periodically reconciles already-enrolled hosts. Discovery of NEW
// upgrade candidates happens via the OnNewImage build-success callback;
// the periodic tick exists to retry new-image handling that did not
// complete, to retry restarts that failed (e.g. transient docker errors)
// and to catch sessions that became empty without the UserLeftSession
// event reaching us (cold start, stream reset).
func (o *HostUpgradeOrchestrator) Run(ctx context.Context) error {
	// orchestrator は system user として動作する. JWT を経由しないため
	// usecase の Require* チェックを通すために claims を ctx に詰める.
	ctx = auth.WithActAsUser(ctx, domain.SystemUserID)

	o.lifecycleMu.Lock()
	o.lifecycleCtx = ctx
	o.lifecycleMu.Unlock()

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			o.retryPendingImage(ctx)
			o.reconcileAllDraining(ctx)
		}
	}
}

// OnNewImage is the ResoniteVersionUsecase build-success subscription.
// Fires when a new built image is confirmed (locally) after ContentPoller
// enqueues an auto-build and it completes. Returns immediately after
// dispatching the work to a goroutine so the notifier is not blocked by
// RPC fetches or Restart calls (which can take minutes).
func (o *HostUpgradeOrchestrator) OnNewImage(_ context.Context, latest *port.ContainerImage) {
	if latest == nil || latest.IsPreRelease {
		return
	}

	o.markPending(latest)

	// The notifier's ctx belongs to the build job and can end before a
	// Restart does, so we intentionally swap in the orchestrator's
	// lifecycle ctx for the goroutined work.
	go o.handleNewImage(o.lifecycle(), latest) //nolint:contextcheck // see comment above
}

// HandleHostEvent implements HostEventHandler. Cache updates happen
// unconditionally so a host enrolled later still sees up-to-date counts;
// reconcile is only triggered for hosts already in drainTargets.
func (o *HostUpgradeOrchestrator) HandleHostEvent(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	switch p := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_UserJoinedSession:
		o.bumpUserCount(hostID, p.UserJoinedSession.GetSessionId(), 1)
	case *headlessv1.HostEvent_UserLeftSession:
		o.bumpUserCount(hostID, p.UserLeftSession.GetSessionId(), -1)
	case *headlessv1.HostEvent_SessionEnded:
		o.forgetSession(hostID, p.SessionEnded.GetSessionId())
	default:
		return
	}

	if !o.IsHostDraining(hostID) {
		return
	}

	o.reconcileHost(ctx, hostID)
}

// HandleHostEventStreamReset implements HostEventHandler. The watcher
// has lost buffered events for this host, so any per-session cache we
// hold for it is suspect — wipe it (only this host's submap, not
// everyone's) and let the next reconcile re-fetch from RPC. We do this
// regardless of whether the host is currently draining: an enroll that
// happens later would otherwise pick up the stale counters.
func (o *HostUpgradeOrchestrator) HandleHostEventStreamReset(ctx context.Context, hostID string) {
	o.mu.Lock()
	delete(o.sessionUsers, hostID)
	o.mu.Unlock()

	if !o.IsHostDraining(hostID) {
		return
	}

	// Reseed from RPC so the upgrade decision keeps moving forward
	// even if no further events arrive on the new stream.
	if err := o.seedSessionUserCache(ctx, hostID); err != nil {
		slog.Warn("upgrade-orchestrator: reseed after stream reset failed",
			"hostID", hostID, "error", err)
	}

	o.reconcileHost(ctx, hostID)
}

// reconcileAllDraining is the periodic catch-up pass.
func (o *HostUpgradeOrchestrator) reconcileAllDraining(ctx context.Context) {
	for _, id := range o.drainingHostIDs() {
		o.reconcileHost(ctx, id)
	}
}

func (o *HostUpgradeOrchestrator) drainingHostIDs() []string {
	o.mu.Lock()
	defer o.mu.Unlock()

	ids := make([]string, 0, len(o.drainTargets))
	for id := range o.drainTargets {
		ids = append(ids, id)
	}

	return ids
}

// enrollHost captures the snapshot data the eventual restart will need
// and adds the host to drainTargets. It reports false when the enroll
// was deferred: failure to snapshot leaves the host unenrolled, and the
// notification stays pending so the periodic tick retries it.
//
// `host` carries the row-level fields (Name, AccountId, AutoUpdatePolicy,
// Memo) which are populated from the DB and therefore reliable. We do
// NOT trust host.HostSettings here because the repository's Find path
// silently swallows RPC errors and leaves that field as a zero value —
// which on restart would produce an empty start_worlds and lose user
// data. Instead we fetch the startup config DIRECTLY via the RPC client
// so any error propagates (and aborts the enroll) rather than silently
// becoming a bad snapshot.
//
// NOTE: once a host is enrolled, edits to its startup_config via the UI
// will NOT be picked up by the eventual restart — the snapshot taken
// here is authoritative until the upgrade completes or aborts.
func (o *HostUpgradeOrchestrator) enrollHost(ctx context.Context, host *entity.HeadlessHost, targetTag string) bool {
	client, err := o.hostRepo.GetRpcClient(ctx, host.ID)
	if err != nil {
		slog.Warn("upgrade-orchestrator: cannot reach host RPC; deferring enroll",
			"hostID", host.ID, "error", err)

		return false
	}

	// CRITICAL: this must run BEFORE any sessions are stopped, because
	// GetStartupConfigToRestore reads start_worlds from the running
	// container's live state. Once stopSession drains the sessions
	// there is nothing left to capture.
	cfgResp, err := client.GetStartupConfigToRestore(ctx, &headlessv1.GetStartupConfigToRestoreRequest{
		IncludeStartWorlds: true,
	})
	if err != nil {
		slog.Warn("upgrade-orchestrator: GetStartupConfigToRestore failed; deferring enroll",
			"hostID", host.ID, "error", err)

		return false
	}

	startupConfig := cfgResp.GetStartupConfig()
	if startupConfig == nil {
		slog.Warn("upgrade-orchestrator: empty startup config response; deferring enroll",
			"hostID", host.ID)

		return false
	}

	account, err := o.accountRepo.GetHeadlessAccount(ctx, host.GroupID, host.AccountId)
	if err != nil {
		slog.Warn("upgrade-orchestrator: account fetch failed; deferring enroll",
			"hostID", host.ID, "error", err)

		return false
	}

	state := &drainState{
		targetTag: targetTag,
		restartParms: port.HeadlessHostStartParams{
			Name:              host.Name,
			ContainerImageTag: targetTag,
			StartupConfig:     startupConfig,
			HeadlessAccount:   *account,
			AutoUpdatePolicy:  host.AutoUpdatePolicy,
			Memo:              host.Memo,
		},
	}

	o.mu.Lock()

	if _, already := o.drainTargets[host.ID]; already {
		o.mu.Unlock()

		return true
	}

	o.drainTargets[host.ID] = state
	o.mu.Unlock()

	slog.Info("upgrade-orchestrator: enrolled host for drain",
		"hostID", host.ID, "currentAppVersion", host.AppVersion, "targetTag", targetTag)

	// Reseed cache from live RPC so the very next reconcile has a
	// trustworthy baseline. Without this, controller-restart races
	// (a user-leave event arriving after a checkpoint with no matching
	// user-join replay) could leave a session falsely at 0.
	if err := o.seedSessionUserCache(ctx, host.ID); err != nil {
		slog.Warn("upgrade-orchestrator: initial cache seed failed; reconcile will RPC-verify",
			"hostID", host.ID, "error", err)
	}

	o.reconcileHost(ctx, host.ID)

	return true
}

// seedSessionUserCache replaces the per-host user-count submap with the
// live counts reported by the container. Subsequent UserJoined/Left
// events are applied incrementally on top of this baseline.
func (o *HostUpgradeOrchestrator) seedSessionUserCache(ctx context.Context, hostID string) error {
	client, err := o.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	resp, err := client.ListSessions(ctx, &headlessv1.ListSessionsRequest{})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	seeded := make(map[string]int32, len(resp.GetSessions()))
	for _, s := range resp.GetSessions() {
		seeded[s.GetId()] = s.GetUsersCount()
	}

	o.mu.Lock()
	o.sessionUsers[hostID] = seeded
	o.mu.Unlock()

	return nil
}

// reconcileHost stops empty sessions on a draining host and, once no
// RUNNING sessions remain, restarts the host on its target tag.
func (o *HostUpgradeOrchestrator) reconcileHost(ctx context.Context, hostID string) {
	// orchestrator は system user として動作する. HandleHostEvent 経由など
	// 呼び出し元 ctx に claims が無い経路にも備えてここでも被せる (idempotent).
	ctx = auth.WithActAsUser(ctx, domain.SystemUserID)

	// Per-host serialisation so the periodic tick and event-driven
	// triggers cannot double-restart the same host, but reconciles for
	// different hosts can still proceed in parallel.
	o.hostReconcileMutex(hostID).Lock()
	defer o.hostReconcileMutex(hostID).Unlock()

	o.mu.Lock()
	state, draining := o.drainTargets[hostID]
	o.mu.Unlock()

	if !draining {
		return
	}

	sessions, err := o.runningSessionsForHost(ctx, hostID)
	if err != nil {
		slog.Error("upgrade-orchestrator: failed to list sessions", "hostID", hostID, "error", err)

		return
	}

	remaining := 0

	for _, s := range sessions {
		count, err := o.userCountForSession(ctx, hostID, s.ID)
		if err != nil {
			slog.Warn("upgrade-orchestrator: failed to fetch session user count",
				"hostID", hostID, "sessionID", s.ID, "error", err)

			remaining++

			continue
		}

		if count > 0 {
			remaining++

			continue
		}

		if err := o.stopSession(ctx, s.ID); err != nil {
			slog.Error("upgrade-orchestrator: failed to stop empty session",
				"hostID", hostID, "sessionID", s.ID, "error", err)

			remaining++

			continue
		}

		o.forgetSession(hostID, s.ID)
	}

	if remaining > 0 {
		return
	}

	usedTag, err := o.restartHost(ctx, hostID, state)
	if err != nil {
		o.recordRestartFailure(hostID, err)

		return
	}

	// handleNewImage may have retargeted the host while Restart was in
	// flight. The host is now on usedTag, so keep it draining towards the
	// newer target; the snapshot is still the config it was just
	// restarted with.
	o.mu.Lock()
	newerTarget := state.targetTag
	retargeted := newerTarget != usedTag

	if retargeted {
		state.attempts = 0
	} else {
		delete(o.drainTargets, hostID)
	}

	delete(o.sessionUsers, hostID)
	o.mu.Unlock()

	if retargeted {
		slog.Info("upgrade-orchestrator: host restarted but a newer image arrived meanwhile; keeping it draining",
			"hostID", hostID, "tag", usedTag, "targetTag", newerTarget)

		return
	}

	slog.Info("upgrade-orchestrator: host upgraded", "hostID", hostID, "tag", usedTag)
}

func (o *HostUpgradeOrchestrator) recordRestartFailure(hostID string, restartErr error) {
	o.mu.Lock()

	state, ok := o.drainTargets[hostID]
	if !ok {
		o.mu.Unlock()

		return
	}

	state.attempts++
	attempts := state.attempts
	target := state.targetTag

	if attempts >= o.maxRestartAttempts {
		delete(o.drainTargets, hostID)
		delete(o.sessionUsers, hostID)
	}

	o.mu.Unlock()

	if attempts >= o.maxRestartAttempts {
		slog.Error("upgrade-orchestrator: giving up after repeated restart failures",
			"hostID", hostID, "tag", target, "attempts", attempts, "lastError", restartErr)
	} else {
		slog.Error("upgrade-orchestrator: restart failed; will retry",
			"hostID", hostID, "tag", target, "attempts", attempts, "error", restartErr)
	}
}

// runningSessionsForHost returns RUNNING sessions on this host based on
// the DB (which the HostEventWatcher keeps up to date).
func (o *HostUpgradeOrchestrator) runningSessionsForHost(ctx context.Context, hostID string) (entity.SessionList, error) {
	all, err := o.sessionRepo.ListByStatus(ctx, entity.SessionStatus_RUNNING)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	out := make(entity.SessionList, 0)

	for _, s := range all {
		if s.HostID == hostID {
			out = append(out, s)
		}
	}

	return out, nil
}

// userCountForSession returns the latest user count for the session.
// The per-host cache is treated as authoritative when it reports a
// strictly positive value (we wouldn't stop the session anyway), but a
// cached 0 OR a cache miss falls back to an RPC verification. This
// guards against a stale "0" left over from a partial event replay
// incorrectly tearing down a session that actually has users.
func (o *HostUpgradeOrchestrator) userCountForSession(ctx context.Context, hostID, sessionID string) (int32, error) {
	o.mu.Lock()

	var (
		cached int32
		hit    bool
	)

	if hostMap, ok := o.sessionUsers[hostID]; ok {
		cached, hit = hostMap[sessionID]
	}

	o.mu.Unlock()

	if hit && cached > 0 {
		return cached, nil
	}

	return o.fetchUserCountRPC(ctx, hostID, sessionID)
}

func (o *HostUpgradeOrchestrator) fetchUserCountRPC(ctx context.Context, hostID, sessionID string) (int32, error) {
	client, err := o.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		return 0, errors.Wrap(err, 0)
	}

	resp, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: sessionID})
	if err != nil {
		return 0, errors.Wrap(err, 0)
	}

	count := resp.GetSession().GetUsersCount()

	o.mu.Lock()

	if o.sessionUsers[hostID] == nil {
		o.sessionUsers[hostID] = make(map[string]int32)
	}

	o.sessionUsers[hostID][sessionID] = count
	o.mu.Unlock()

	return count, nil
}

// stopSession delegates to the wired SessionStopper (SessionUsecase) so
// that LoadWorld URL preservation, status updates and any future
// stop-time logic stay in one place. Returns an error if the stopper
// has not been wired (programmer error).
func (o *HostUpgradeOrchestrator) stopSession(ctx context.Context, sessionID string) error {
	o.stopperMu.RLock()
	stopper := o.sessionStopper
	o.stopperMu.RUnlock()

	if stopper == nil {
		return errors.New("upgrade-orchestrator: SessionStopper not wired")
	}

	return stopper.StopSession(ctx, sessionID)
}

// restartHost uses the snapshot captured at enroll time (NOT a live
// Find) so start_worlds reflects the sessions that were running BEFORE
// drain started. Returns the image tag the host was restarted onto.
func (o *HostUpgradeOrchestrator) restartHost(ctx context.Context, hostID string, state *drainState) (string, error) {
	// handleNewImage may retarget the state concurrently, so read it
	// under the lock.
	o.mu.Lock()
	params := state.restartParms
	o.mu.Unlock()

	if err := o.hostRepo.Restart(ctx, hostID, params, o.restartStopTimeout); err != nil {
		return "", err
	}

	return params.ContainerImageTag, nil
}

func (o *HostUpgradeOrchestrator) bumpUserCount(hostID, sessionID string, delta int32) {
	if hostID == "" || sessionID == "" {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	hostMap, ok := o.sessionUsers[hostID]
	if !ok {
		hostMap = make(map[string]int32)
		o.sessionUsers[hostID] = hostMap
	}

	hostMap[sessionID] = max(hostMap[sessionID]+delta, 0)
}

func (o *HostUpgradeOrchestrator) forgetSession(hostID, sessionID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if hostMap, ok := o.sessionUsers[hostID]; ok {
		delete(hostMap, sessionID)

		if len(hostMap) == 0 {
			delete(o.sessionUsers, hostID)
		}
	}
}

// hostReconcileMutex returns the per-host reconcile lock, allocating it
// on first use. Per-host (rather than global) so unrelated draining
// hosts can reconcile in parallel.
// TODO: reconcileLock entries are never deleted, so a long-running
// controller that has churned through many distinct host IDs will leak
// one mutex per ever-drained host. Deleting on restart-success risks
// "in-flight reconcile holds a stale lock; new reconcile acquires a
// fresh one and the two run in parallel"; we'll need a more careful
// scheme (e.g. refcounting) to GC them safely. Not urgent for current
// deployments.
func (o *HostUpgradeOrchestrator) hostReconcileMutex(hostID string) *sync.Mutex {
	o.mu.Lock()
	defer o.mu.Unlock()

	m, ok := o.reconcileLock[hostID]
	if !ok {
		m = &sync.Mutex{}
		o.reconcileLock[hostID] = m
	}

	return m
}

// lifecycle returns the Manager-supplied ctx if Run is currently
// executing, falling back to context.Background otherwise (only matters
// in tests that drive OnNewImage without spinning up the runner loop).
func (o *HostUpgradeOrchestrator) lifecycle() context.Context {
	o.lifecycleMu.RLock()
	defer o.lifecycleMu.RUnlock()

	if o.lifecycleCtx == nil {
		return context.Background()
	}

	return o.lifecycleCtx
}

// handleNewImage is the synchronous body of OnNewImage. Split out so
// tests can drive it deterministically without depending on goroutine
// scheduling.
func (o *HostUpgradeOrchestrator) handleNewImage(ctx context.Context, latest *port.ContainerImage) {
	o.newImageMu.Lock()
	defer o.newImageMu.Unlock()

	o.processNewImage(ctx, latest)
}

// retryPendingImage re-runs a notification whose handling did not
// complete. Skips when a run is already in progress: that run (or the
// next tick) covers it.
func (o *HostUpgradeOrchestrator) retryPendingImage(ctx context.Context) {
	o.mu.Lock()
	pending := o.pendingImage
	o.mu.Unlock()

	if pending == nil {
		return
	}

	if !o.newImageMu.TryLock() {
		return
	}
	defer o.newImageMu.Unlock()

	o.processNewImage(ctx, pending)
}

// processNewImage must be called with newImageMu held.
func (o *HostUpgradeOrchestrator) processNewImage(ctx context.Context, latest *port.ContainerImage) {
	if !o.applyNewImage(ctx, latest) {
		return
	}

	o.mu.Lock()
	if o.pendingImage == latest {
		o.pendingImage = nil
	}
	o.mu.Unlock()
}

// applyNewImage enrolls / retargets hosts for latest and reports whether
// handling completed. false means it should be retried (lookup failure
// or deferred enroll).
func (o *HostUpgradeOrchestrator) applyNewImage(ctx context.Context, latest *port.ContainerImage) bool {
	if latest == nil || latest.IsPreRelease {
		return true
	}

	// Build success fires for ANY version, not only the newest one: e.g.
	// when a new Resonite version and a builder AppVersion bump land
	// together, ContentPoller enqueues both the new version's build and a
	// rebuild of the previous version, and the rebuild may finish first.
	// Rolling hosts onto such an image would restart them on the old
	// version while the real update is still unbuilt, so only act when
	// the image is what "latestRelease" resolves to right now.
	isLatest, err := o.isLatestRelease(ctx, latest)
	if err != nil {
		slog.Warn("upgrade-orchestrator: failed to resolve latest release; will retry",
			"tag", latest.Tag, "error", err)

		return false
	}

	if !isLatest {
		return true
	}

	hosts, err := o.hostRepo.ListAll(ctx, port.HeadlessHostFetchOptions{})
	if err != nil {
		slog.Error("upgrade-orchestrator: failed to list hosts", "error", err)

		return false
	}

	complete := true

	for _, h := range hosts {
		if h.AutoUpdatePolicy != entity.HostAutoUpdatePolicy_USERS_EMPTY {
			continue
		}

		// Checked before the status filter: a host the orchestrator is
		// restarting right now is STOPPING/STARTING but must still follow
		// the newer target.
		if o.retargetDraining(h.ID, latest.Tag) {
			continue
		}

		if h.Status != entity.HeadlessHostStatus_RUNNING {
			continue
		}

		if h.AppVersion == latest.AppVersion && h.ResoniteVersion == latest.ResoniteVersion {
			continue
		}

		if !o.enrollHost(ctx, h, latest.Tag) {
			complete = false
		}
	}

	return complete
}

// markPending records latest as the notification to retry until its
// handling completes. A newer notification replaces an older one.
func (o *HostUpgradeOrchestrator) markPending(latest *port.ContainerImage) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.pendingImage = latest
}

// isLatestRelease reports whether image is the current latest built
// release image. The latest version not being built yet is a definite
// "no": its own build will notify once it succeeds. Any other resolve
// failure is returned so the caller can retry.
func (o *HostUpgradeOrchestrator) isLatestRelease(ctx context.Context, image *port.ContainerImage) (bool, error) {
	latestTag, err := o.releaseResolver.ResolveForStart(ctx, latestReleaseTagInput)
	if err != nil {
		var nbe *usecase.NotBuiltError
		if errors.As(err, &nbe) {
			slog.Info("upgrade-orchestrator: latest release is not built yet; ignoring built image",
				"tag", image.Tag, "latestManifest", nbe.ManifestID)

			return false, nil
		}

		return false, err
	}

	if latestTag != image.Tag {
		slog.Info("upgrade-orchestrator: built image is not the latest release; ignoring",
			"tag", image.Tag, "latestTag", latestTag)

		return false, nil
	}

	return true, nil
}

// retargetDraining points an already-enrolled host at tag and reports
// whether the host was draining. Without this, a host enrolled for an
// earlier image would be restarted onto it even after a newer image
// became available. The snapshot (start_worlds etc.) is kept as is.
func (o *HostUpgradeOrchestrator) retargetDraining(hostID, tag string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	state, ok := o.drainTargets[hostID]
	if !ok {
		return false
	}

	if state.targetTag != tag {
		slog.Info("upgrade-orchestrator: retargeting draining host to newer image",
			"hostID", hostID, "fromTag", state.targetTag, "toTag", tag)

		state.targetTag = tag
		state.restartParms.ContainerImageTag = tag
		// Failures restarting onto the old image say nothing about the new one.
		state.attempts = 0
	}

	return true
}
