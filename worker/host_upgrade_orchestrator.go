package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// HostUpgradeOrchestrator rolls hosts that opted in
// (AutoUpdatePolicy = USERS_EMPTY) onto the latest container tag without
// disturbing active users.
//
// Lifecycle for one host:
//  1. ImageChecker observes a new tag and calls OnNewImage. The
//     orchestrator looks at every running host with USERS_EMPTY policy,
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
//     host leaves drainTargets; on failure we increment an attempt
//     counter and abort after maxRestartAttempts so a permanently broken
//     host doesn't block its session admission forever.
//
// State is purely in-memory; a controller restart loses it but the next
// ImageChecker poll will re-discover the upgrade candidate.
type HostUpgradeOrchestrator struct {
	hostRepo           port.HeadlessHostRepository
	sessionRepo        port.SessionRepository
	accountRepo        HeadlessAccountFetcher
	interval           time.Duration
	restartStopTimeout int
	maxRestartAttempts int

	mu            sync.Mutex
	drainTargets  map[string]*drainState      // hostID -> in-flight upgrade state
	sessionUsers  map[string]map[string]int32 // hostID -> sessionID -> user count
	reconcileLock map[string]*sync.Mutex      // hostID -> per-host reconcile lock

	// sessionStopper is wired AFTER construction (see SetSessionStopper)
	// to break the SessionUsecase ↔ orchestrator construction cycle.
	stopperMu      sync.RWMutex
	sessionStopper port.SessionStopper
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

// HeadlessAccountFetcher is the slice of HeadlessAccountUsecase the
// orchestrator needs. It is an interface so tests can substitute it
// without pulling in skyfrost.
type HeadlessAccountFetcher interface {
	GetHeadlessAccount(ctx context.Context, resoniteID string) (*entity.HeadlessAccount, error)
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
	cfg *config.WorkerConfig,
) *HostUpgradeOrchestrator {
	return &HostUpgradeOrchestrator{
		hostRepo:           hostRepo,
		sessionRepo:        sessionRepo,
		accountRepo:        accountRepo,
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
// upgrade candidates happens via the OnNewImage callback subscribed on
// ImageChecker; the periodic tick exists to retry restarts that failed
// (e.g. transient docker errors) and to catch sessions that became empty
// without the UserLeftSession event reaching us (cold start, stream
// reset).
func (o *HostUpgradeOrchestrator) Run(ctx context.Context) error {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			o.reconcileAllDraining(ctx)
		}
	}
}

// OnNewImage is the ImageChecker subscription. It enrolls each running
// USERS_EMPTY host whose AppVersion differs from the new tag, snapshots
// the startup config + account (BEFORE any sessions are stopped), seeds
// the per-host user-count cache from RPC, then triggers a reconcile.
func (o *HostUpgradeOrchestrator) OnNewImage(ctx context.Context, latest *port.ContainerImage) {
	if latest == nil || latest.IsPreRelease {
		return
	}

	hosts, err := o.hostRepo.ListAll(ctx, port.HeadlessHostFetchOptions{})
	if err != nil {
		slog.Error("upgrade-orchestrator: failed to list hosts", "error", err)

		return
	}

	for _, h := range hosts {
		if h.Status != entity.HeadlessHostStatus_RUNNING {
			continue
		}

		if h.AutoUpdatePolicy != entity.HostAutoUpdatePolicy_USERS_EMPTY {
			continue
		}

		if h.AppVersion == latest.AppVersion && h.ResoniteVersion == latest.ResoniteVersion {
			continue
		}

		if o.IsHostDraining(h.ID) {
			continue
		}

		o.enrollHost(ctx, h.ID, latest.Tag)
	}
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
// and adds the host to drainTargets. Failure to snapshot leaves the
// host unenrolled — better than enrolling with a bad snapshot that
// would restart the host with empty start_worlds.
func (o *HostUpgradeOrchestrator) enrollHost(ctx context.Context, hostID, targetTag string) {
	// CRITICAL: this Find must run with IncludeStartWorlds:true BEFORE
	// any sessions are stopped, because the underlying RPC reads
	// start_worlds from the running container's live state. Once
	// stopSession drains the sessions there is nothing left to capture.
	host, err := o.hostRepo.Find(ctx, hostID, port.HeadlessHostFetchOptions{
		IncludeStartWorlds: true,
	})
	if err != nil {
		slog.Error("upgrade-orchestrator: snapshot host failed; skipping enroll",
			"hostID", hostID, "error", err)

		return
	}

	account, err := o.accountRepo.GetHeadlessAccount(ctx, host.AccountId)
	if err != nil {
		slog.Error("upgrade-orchestrator: account fetch failed; skipping enroll",
			"hostID", hostID, "error", err)

		return
	}

	state := &drainState{
		targetTag: targetTag,
		restartParms: port.HeadlessHostStartParams{
			Name:              host.Name,
			ContainerImageTag: targetTag,
			StartupConfig:     converter.HeadlessHostSettingsToStartupConfigProto(&host.HostSettings),
			HeadlessAccount:   *account,
			AutoUpdatePolicy:  host.AutoUpdatePolicy,
			Memo:              host.Memo,
		},
	}

	o.mu.Lock()

	if _, already := o.drainTargets[hostID]; already {
		o.mu.Unlock()

		return
	}

	o.drainTargets[hostID] = state
	o.mu.Unlock()

	slog.Info("upgrade-orchestrator: enrolled host for drain",
		"hostID", hostID, "currentAppVersion", host.AppVersion, "targetTag", targetTag)

	// Reseed cache from live RPC so the very next reconcile has a
	// trustworthy baseline. Without this, controller-restart races
	// (a user-leave event arriving after a checkpoint with no matching
	// user-join replay) could leave a session falsely at 0.
	if err := o.seedSessionUserCache(ctx, hostID); err != nil {
		slog.Warn("upgrade-orchestrator: initial cache seed failed; reconcile will RPC-verify",
			"hostID", hostID, "error", err)
	}

	o.reconcileHost(ctx, hostID)
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

	if err := o.restartHost(ctx, hostID, state); err != nil {
		o.recordRestartFailure(hostID, err)

		return
	}

	o.mu.Lock()
	delete(o.drainTargets, hostID)
	delete(o.sessionUsers, hostID)
	o.mu.Unlock()

	slog.Info("upgrade-orchestrator: host upgraded", "hostID", hostID, "tag", state.targetTag)
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
// drain started.
func (o *HostUpgradeOrchestrator) restartHost(ctx context.Context, hostID string, state *drainState) error {
	return o.hostRepo.Restart(ctx, hostID, state.restartParms, o.restartStopTimeout)
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
