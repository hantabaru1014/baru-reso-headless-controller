package worker

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// HostUpgradeOrchestrator watches for newly published headless container
// images and rolls hosts that opted in (AutoUpdatePolicy = USERS_EMPTY)
// onto the latest tag without disturbing active users.
//
// The lifecycle for one host is:
//  1. A periodic tick discovers that the host is running an older release
//     tag than what the registry advertises and adds the host to drainSet.
//  2. While in drainSet, StartSession on this host is rejected (via the
//     HostDrainer interface SessionUsecase consults).
//  3. Whenever a session on a draining host reports an empty user count
//     (either from a UserLeftSession event or from the periodic poll), it
//     is stopped.
//  4. Once the host has zero RUNNING sessions, it is restarted on the
//     target tag and removed from drainSet.
//
// State (drainSet, targets, sessionUsers) is kept in memory; a controller
// restart loses it but the next tick reconstructs an equivalent view from
// the registry + live RPC state, so no upgrade is permanently lost.
type HostUpgradeOrchestrator struct {
	hostRepo           port.HeadlessHostRepository
	sessionRepo        port.SessionRepository
	accountRepo        HeadlessAccountFetcher
	interval           time.Duration
	restartStopTimeout int

	mu           sync.Mutex
	drainTargets map[string]string // hostID -> target image tag
	sessionUsers map[string]int32  // sessionID -> last-known user count

	// reconcileOnce serialises reconcile passes so the periodic ticker
	// and event-driven triggers cannot race on the host/session RPCs or
	// double-fire a restart for the same host.
	reconcileOnce sync.Mutex
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

// defaultRestartStopTimeoutSeconds is how long Restart waits for a
// graceful container stop before forcing it down. Resonite headless
// sometimes takes a while to flush state on shutdown, so we err on the
// generous side.
const defaultRestartStopTimeoutSeconds = 180

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
		drainTargets:       make(map[string]string),
		sessionUsers:       make(map[string]int32),
	}
}

func (o *HostUpgradeOrchestrator) Name() string { return "host-upgrade-orchestrator" }

// IsHostDraining implements port.HostDrainer.
func (o *HostUpgradeOrchestrator) IsHostDraining(hostID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	_, ok := o.drainTargets[hostID]

	return ok
}

func (o *HostUpgradeOrchestrator) Run(ctx context.Context) error {
	o.tick(ctx)

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			o.tick(ctx)
		}
	}
}

// HandleHostEvent implements HostEventHandler. UserLeftSession and
// SessionEnded events on a draining host trigger an immediate reconcile
// so the upgrade fires the moment the last user disconnects rather than
// waiting up to UpgradeCheckInterval.
func (o *HostUpgradeOrchestrator) HandleHostEvent(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	switch p := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_UserJoinedSession:
		o.bumpUserCount(p.UserJoinedSession.GetSessionId(), 1)
	case *headlessv1.HostEvent_UserLeftSession:
		o.bumpUserCount(p.UserLeftSession.GetSessionId(), -1)
	case *headlessv1.HostEvent_SessionEnded:
		o.forgetSession(p.SessionEnded.GetSessionId())
	default:
		return
	}

	if !o.IsHostDraining(hostID) {
		return
	}

	o.reconcileHost(ctx, hostID)
}

// HandleHostEventStreamReset implements HostEventHandler. We don't know
// which sessions belong to the reset host (the counter is keyed on
// sessionID alone), so the conservative move is to drop every cached
// count and let the next reconcile re-fetch via RPC.
func (o *HostUpgradeOrchestrator) HandleHostEventStreamReset(_ context.Context, hostID string) {
	if !o.IsHostDraining(hostID) {
		return
	}

	o.mu.Lock()
	o.sessionUsers = make(map[string]int32)
	o.mu.Unlock()
}

// tick runs one full sweep: discover upgrade candidates, then reconcile
// every host currently in drainSet.
func (o *HostUpgradeOrchestrator) tick(ctx context.Context) {
	o.discoverUpgradeCandidates(ctx)

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

// discoverUpgradeCandidates compares the latest non-prerelease registry
// tag against each RUNNING host with AutoUpdatePolicy == USERS_EMPTY. A
// host whose AppVersion differs from the newest tag (and isn't already
// draining) is enrolled.
func (o *HostUpgradeOrchestrator) discoverUpgradeCandidates(ctx context.Context) {
	tags, err := o.hostRepo.ListContainerTags(ctx, nil)
	if err != nil {
		slog.Error("upgrade-orchestrator: failed to list tags", "error", err)

		return
	}

	latest := newestReleaseTag(tags)
	if latest == nil {
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

		if o.enrollHost(h.ID, latest.Tag) {
			slog.Info("upgrade-orchestrator: enrolled host for drain",
				"hostID", h.ID, "currentAppVersion", h.AppVersion, "targetTag", latest.Tag)
		}
	}
}

// enrollHost atomically adds the host to drainTargets unless it was
// already present. Returns true if this call actually enrolled it.
func (o *HostUpgradeOrchestrator) enrollHost(hostID, targetTag string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, already := o.drainTargets[hostID]; already {
		return false
	}

	o.drainTargets[hostID] = targetTag

	return true
}

// reconcileHost stops empty sessions on a draining host and, once no
// RUNNING sessions remain, restarts the host on its target tag.
func (o *HostUpgradeOrchestrator) reconcileHost(ctx context.Context, hostID string) {
	// Serialise reconcile passes so the ticker and event-driven calls
	// don't double-restart the same host.
	o.reconcileOnce.Lock()
	defer o.reconcileOnce.Unlock()

	o.mu.Lock()
	targetTag, draining := o.drainTargets[hostID]
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
		count, err := o.refreshUserCount(ctx, hostID, s)
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

		if err := o.stopSession(ctx, hostID, s); err != nil {
			slog.Error("upgrade-orchestrator: failed to stop empty session",
				"hostID", hostID, "sessionID", s.ID, "error", err)

			remaining++
		}
	}

	if remaining > 0 {
		return
	}

	if err := o.restartHost(ctx, hostID, targetTag); err != nil {
		slog.Error("upgrade-orchestrator: restart failed", "hostID", hostID, "tag", targetTag, "error", err)

		return
	}

	o.mu.Lock()
	delete(o.drainTargets, hostID)
	o.mu.Unlock()

	slog.Info("upgrade-orchestrator: host upgraded", "hostID", hostID, "tag", targetTag)
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

// refreshUserCount returns the latest user count for the session,
// preferring the in-memory counter (kept up to date by host events) and
// falling back to an RPC call on cache miss.
func (o *HostUpgradeOrchestrator) refreshUserCount(ctx context.Context, hostID string, s *entity.Session) (int32, error) {
	o.mu.Lock()
	cached, ok := o.sessionUsers[s.ID]
	o.mu.Unlock()

	if ok {
		return cached, nil
	}

	client, err := o.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		return 0, errors.Wrap(err, 0)
	}

	resp, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: s.ID})
	if err != nil {
		return 0, errors.Wrap(err, 0)
	}

	count := resp.GetSession().GetUsersCount()

	o.mu.Lock()
	o.sessionUsers[s.ID] = count
	o.mu.Unlock()

	return count, nil
}

// stopSession mirrors SessionUsecase.StopSession but is inlined here to
// avoid a dependency cycle between worker and usecase (SessionUsecase
// already depends on this orchestrator via port.HostDrainer).
func (o *HostUpgradeOrchestrator) stopSession(ctx context.Context, hostID string, s *entity.Session) error {
	client, err := o.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	hdlSession, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: s.ID})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if url := hdlSession.GetSession().GetWorldUrl(); url != "" {
		s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
			LoadWorldUrl: url,
		}
	}

	_, err = client.StopSession(ctx, &headlessv1.StopSessionRequest{SessionId: s.ID})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	now := time.Now()
	s.EndedAt = &now
	s.Status = entity.SessionStatus_ENDED

	if err := o.sessionRepo.Upsert(ctx, s); err != nil {
		return errors.Wrap(err, 0)
	}

	o.forgetSession(s.ID)

	return nil
}

// restartHost builds the StartupConfig from the host's persisted state
// and calls into the repository directly. Done this way (rather than via
// HeadlessHostUsecase.HeadlessHostRestart) to avoid the dependency cycle
// orchestrator → SessionUsecase → HostDrainer → orchestrator.
func (o *HostUpgradeOrchestrator) restartHost(ctx context.Context, hostID, targetTag string) error {
	host, err := o.hostRepo.Find(ctx, hostID, port.HeadlessHostFetchOptions{
		IncludeStartWorlds: true,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	account, err := o.accountRepo.GetHeadlessAccount(ctx, host.AccountId)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	params := port.HeadlessHostStartParams{
		Name:              host.Name,
		ContainerImageTag: targetTag,
		StartupConfig:     converter.HeadlessHostSettingsToStartupConfigProto(&host.HostSettings),
		HeadlessAccount:   *account,
		AutoUpdatePolicy:  host.AutoUpdatePolicy,
		Memo:              host.Memo,
	}

	return o.hostRepo.Restart(ctx, host.ID, params, o.restartStopTimeout)
}

func (o *HostUpgradeOrchestrator) bumpUserCount(sessionID string, delta int32) {
	if sessionID == "" {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.sessionUsers[sessionID] = max(o.sessionUsers[sessionID]+delta, 0)
}

func (o *HostUpgradeOrchestrator) forgetSession(sessionID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.sessionUsers, sessionID)
}

// newestReleaseTag returns the latest non-prerelease tag, or nil if the
// list is empty. ContainerTagList is ordered oldest-to-newest by the
// repository.
func newestReleaseTag(tags port.ContainerImageList) *port.ContainerImage {
	for _, t := range slices.Backward(tags) {
		if t.IsPreRelease {
			continue
		}

		return t
	}

	return nil
}
