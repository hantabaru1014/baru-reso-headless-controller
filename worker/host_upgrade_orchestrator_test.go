package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// orchestratorHostRepo is a port.HeadlessHostRepository fake covering only
// the methods the orchestrator exercises.
type orchestratorHostRepo struct {
	port.HeadlessHostRepository

	mu          sync.Mutex
	hosts       map[string]*entity.HeadlessHost
	listErr     error
	restarts    map[string][]port.HeadlessHostStartParams
	restartErrs map[string][]error // pop one per call
	clients     map[string]headlessv1.HeadlessControlServiceClient
	findOpts    map[string]port.HeadlessHostFetchOptions // last fetch options per host
}

func newOrchestratorHostRepo() *orchestratorHostRepo {
	return &orchestratorHostRepo{
		hosts:       make(map[string]*entity.HeadlessHost),
		restarts:    make(map[string][]port.HeadlessHostStartParams),
		restartErrs: make(map[string][]error),
		clients:     make(map[string]headlessv1.HeadlessControlServiceClient),
		findOpts:    make(map[string]port.HeadlessHostFetchOptions),
	}
}

func (r *orchestratorHostRepo) ListAll(_ context.Context, _ port.HeadlessHostFetchOptions) (entity.HeadlessHostList, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make(entity.HeadlessHostList, 0, len(r.hosts))
	for _, h := range r.hosts {
		out = append(out, h)
	}

	return out, nil
}

func (r *orchestratorHostRepo) Find(_ context.Context, id string, opts port.HeadlessHostFetchOptions) (*entity.HeadlessHost, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.findOpts[id] = opts

	h, ok := r.hosts[id]
	if !ok {
		return nil, errors.New("not found: " + id)
	}

	return h, nil
}

func (r *orchestratorHostRepo) Restart(_ context.Context, id string, params port.HeadlessHostStartParams, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if queued := r.restartErrs[id]; len(queued) > 0 {
		err := queued[0]
		r.restartErrs[id] = queued[1:]

		if err != nil {
			return err
		}
	}

	r.restarts[id] = append(r.restarts[id], params)

	if h, ok := r.hosts[id]; ok {
		h.AppVersion = params.ContainerImageTag
	}

	return nil
}

func (r *orchestratorHostRepo) GetRpcClient(_ context.Context, id string) (headlessv1.HeadlessControlServiceClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.clients[id]
	if !ok {
		return nil, errors.New("no client for host: " + id)
	}

	return c, nil
}

func (r *orchestratorHostRepo) restartCount(hostID string) int { //nolint:unparam // tests exercise a single host id
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.restarts[hostID])
}

func (r *orchestratorHostRepo) lastRestartParams(hostID string) (port.HeadlessHostStartParams, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	calls := r.restarts[hostID]
	if len(calls) == 0 {
		return port.HeadlessHostStartParams{}, false
	}

	return calls[len(calls)-1], true
}

func (r *orchestratorHostRepo) lastFindOpts(hostID string) (port.HeadlessHostFetchOptions, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	opts, ok := r.findOpts[hostID]

	return opts, ok
}

// orchestratorSessionRepo implements ListByStatus and Upsert.
type orchestratorSessionRepo struct {
	port.SessionRepository

	mu       sync.Mutex
	sessions []*entity.Session
	upserts  atomic.Int32
}

func (r *orchestratorSessionRepo) ListByStatus(_ context.Context, status entity.SessionStatus) (entity.SessionList, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(entity.SessionList, 0)

	for _, s := range r.sessions {
		if s.Status == status {
			out = append(out, s)
		}
	}

	return out, nil
}

func (r *orchestratorSessionRepo) Upsert(_ context.Context, s *entity.Session) error {
	r.upserts.Add(1)

	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.sessions {
		if existing.ID == s.ID {
			r.sessions[i] = s

			return nil
		}
	}

	r.sessions = append(r.sessions, s)

	return nil
}

type fakeAccountFetcher struct{}

func (fakeAccountFetcher) GetHeadlessAccount(_ context.Context, id string) (*entity.HeadlessAccount, error) {
	return &entity.HeadlessAccount{ResoniteID: id, Credential: "cred", Password: "pw"}, nil
}

// stubRPCClient is a tiny HeadlessControlServiceClient covering the RPCs
// the orchestrator uses: ListSessions (cache seed) and GetSession (cache
// fallback / verification).
type stubRPCClient struct {
	headlessv1.HeadlessControlServiceClient

	mu          sync.Mutex
	users       map[string]int32 // sessionID -> users count
	sessionList []*headlessv1.Session
	getCalls    int
	listCalls   int
}

func newStubRPCClient() *stubRPCClient {
	return &stubRPCClient{users: make(map[string]int32)}
}

func (c *stubRPCClient) GetSession(_ context.Context, in *headlessv1.GetSessionRequest, _ ...grpc.CallOption) (*headlessv1.GetSessionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getCalls++

	return &headlessv1.GetSessionResponse{
		Session: &headlessv1.Session{
			Id:         in.GetSessionId(),
			UsersCount: c.users[in.GetSessionId()],
		},
	}, nil
}

func (c *stubRPCClient) ListSessions(_ context.Context, _ *headlessv1.ListSessionsRequest, _ ...grpc.CallOption) (*headlessv1.ListSessionsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.listCalls++

	return &headlessv1.ListSessionsResponse{Sessions: append([]*headlessv1.Session(nil), c.sessionList...)}, nil
}

func (c *stubRPCClient) setUsers(sessionID string, n int32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.users[sessionID] = n
}

func (c *stubRPCClient) setListedSessions(sessions ...*headlessv1.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessionList = sessions
}

// stubSessionStopper records every StopSession invocation and lets tests
// fail them on demand.
type stubSessionStopper struct {
	mu      sync.Mutex
	stopped []string
	err     error
}

func (s *stubSessionStopper) StopSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	s.stopped = append(s.stopped, sessionID)

	return nil
}

func (s *stubSessionStopper) stoppedIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.stopped...)
}

// orchestratorTestEnv bundles the wiring for a single test.
type orchestratorTestEnv struct {
	orch     *HostUpgradeOrchestrator
	hostRepo *orchestratorHostRepo
	sessRepo *orchestratorSessionRepo
	client   *stubRPCClient
	stopper  *stubSessionStopper
}

func newOrchestratorTestEnv(_ *testing.T) *orchestratorTestEnv {
	hr := newOrchestratorHostRepo()
	sr := &orchestratorSessionRepo{}
	c := newStubRPCClient()
	stop := &stubSessionStopper{}

	// Long interval so the background ticker doesn't race with tests
	// that drive reconcile/OnNewImage synchronously.
	o := NewHostUpgradeOrchestrator(hr, sr, fakeAccountFetcher{}, &config.WorkerConfig{
		UpgradeCheckInterval: time.Hour,
	})
	o.SetSessionStopper(stop)

	return &orchestratorTestEnv{
		orch:     o,
		hostRepo: hr,
		sessRepo: sr,
		client:   c,
		stopper:  stop,
	}
}

func (e *orchestratorTestEnv) addRunningHost(id, appVersion string, policy entity.HostAutoUpdatePolicy) {
	e.hostRepo.mu.Lock()
	defer e.hostRepo.mu.Unlock()

	e.hostRepo.hosts[id] = &entity.HeadlessHost{
		ID:               id,
		Name:             "host-" + id,
		AccountId:        "U-" + id,
		Status:           entity.HeadlessHostStatus_RUNNING,
		AppVersion:       appVersion,
		ResoniteVersion:  "2026.1.1",
		AutoUpdatePolicy: policy,
	}
	e.hostRepo.clients[id] = e.client
}

func (e *orchestratorTestEnv) addSession(id, hostID string) { //nolint:unparam // tests use a single host id today; helper kept general for future use
	e.sessRepo.mu.Lock()
	defer e.sessRepo.mu.Unlock()

	e.sessRepo.sessions = append(e.sessRepo.sessions, &entity.Session{
		ID:                id,
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostID,
		StartupParameters: &headlessv1.WorldStartupParameters{},
	})
}

func newImage(tag, appVersion string, isPrerelease bool) *port.ContainerImage {
	return &port.ContainerImage{
		Tag:             tag,
		AppVersion:      appVersion,
		ResoniteVersion: "2026.1.1",
		IsPreRelease:    isPrerelease,
	}
}

// --- discovery / enrollment ---------------------------------------------

func TestOrchestrator_OnNewImage_EnrollsRunningOptInHost(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	// Live session blocks immediate restart so we can assert enrolment.
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 3)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 3})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.True(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_OnNewImage_IgnoresHostsWithoutOptIn(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_NEVER)
	env.addRunningHost("h2", "v1.0.0", entity.HostAutoUpdatePolicy_UNSPECIFIED)
	env.client.setListedSessions()

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.False(t, env.orch.IsHostDraining("h1"))
	assert.False(t, env.orch.IsHostDraining("h2"))
}

func TestOrchestrator_OnNewImage_IgnoresUpToDateHosts(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v2.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.False(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_OnNewImage_IgnoresPrereleaseTags(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0-pre", "v2.0.0-pre", true))

	assert.False(t, env.orch.IsHostDraining("h1"))
}

// --- snapshot at enroll (MAJOR #1) --------------------------------------

func TestOrchestrator_EnrollSnapshotsStartupConfigBeforeStop(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)

	// Configure host with start_worlds that must survive the upgrade.
	// After all sessions are stopped, the LIVE container would report
	// empty start_worlds — the orchestrator must use the snapshot
	// captured at enroll time instead.
	env.hostRepo.mu.Lock()
	env.hostRepo.hosts["h1"].HostSettings = entity.HeadlessHostSettings{
		StartWorlds: []*headlessv1.WorldStartupParameters{
			{Name: stringPtr("preserved-world")},
		},
	}
	env.hostRepo.mu.Unlock()

	env.addSession("s1", "h1")
	env.client.setUsers("s1", 0)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 0})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	// Find at enroll time must request start_worlds.
	opts, ok := env.hostRepo.lastFindOpts("h1")
	require.True(t, ok, "Find must have been invoked at enroll")
	assert.True(t, opts.IncludeStartWorlds, "snapshot Find must include start_worlds")

	// Restart should fire (session was empty) using the snapshotted
	// start_worlds.
	require.Equal(t, 1, env.hostRepo.restartCount("h1"))

	last, ok := env.hostRepo.lastRestartParams("h1")
	require.True(t, ok)

	require.Len(t, last.StartupConfig.GetStartWorlds(), 1,
		"start_worlds captured at enroll time must be preserved across restart")
	assert.Equal(t, "preserved-world", last.StartupConfig.GetStartWorlds()[0].GetName())
}

func TestOrchestrator_RestartUsesLatestTagAndPolicy(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.client.setListedSessions()

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	last, ok := env.hostRepo.lastRestartParams("h1")
	require.True(t, ok)
	assert.Equal(t, "v2.0.0", last.ContainerImageTag, "restart must use the newest tag")
	assert.Equal(t, entity.HostAutoUpdatePolicy_USERS_EMPTY, last.AutoUpdatePolicy,
		"restart must preserve the AutoUpdatePolicy so future upgrades continue working")
}

// --- session stopping + restart -----------------------------------------

func TestOrchestrator_RestartsHostWhenSessionsAreEmpty(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 0)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 0})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.Equal(t, []string{"s1"}, env.stopper.stoppedIDs(),
		"empty session should be stopped via SessionStopper")
	assert.Equal(t, 1, env.hostRepo.restartCount("h1"))
	assert.False(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_DoesNotRestartWhileUsersPresent(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 3)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 3})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.Empty(t, env.stopper.stoppedIDs(), "session with users must not be stopped")
	assert.Equal(t, 0, env.hostRepo.restartCount("h1"), "host with users must not be restarted")
	assert.True(t, env.orch.IsHostDraining("h1"), "host should remain in drain set")
}

func TestOrchestrator_UserLeftEventTriggersRestartWhenLastUserLeaves(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 1)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 1})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))
	require.True(t, env.orch.IsHostDraining("h1"))
	require.Equal(t, 0, env.hostRepo.restartCount("h1"))

	// Last user leaves — the event handler must immediately reconcile.
	env.client.setUsers("s1", 0)
	env.orch.HandleHostEvent(t.Context(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserLeftSession{
			UserLeftSession: &headlessv1.UserLeftSession{SessionId: "s1"},
		},
	})

	assert.Equal(t, []string{"s1"}, env.stopper.stoppedIDs())
	assert.Equal(t, 1, env.hostRepo.restartCount("h1"))
	assert.False(t, env.orch.IsHostDraining("h1"))
}

// --- cold-start cache race (MAJOR #2) -----------------------------------

func TestOrchestrator_StaleCacheZeroIsRPCVerifiedBeforeStop(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")

	// Simulate a controller-restart race: a UserLeftSession event came
	// through without its prior UserJoinedSession (the checkpoint sat
	// between them), so the bump clamped to 0 in the cache. This is
	// before any enrolment / RPC-seed so the stale value persists.
	env.orch.HandleHostEvent(t.Context(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserLeftSession{
			UserLeftSession: &headlessv1.UserLeftSession{SessionId: "s1"},
		},
	})

	// Reality: the session actually has 4 users. The RPC seed at
	// enroll time should replace the stale 0 with the truth, AND the
	// userCountForSession path also RPC-verifies on cached zeroes for
	// belt-and-braces.
	env.client.setUsers("s1", 4)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 4})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.Empty(t, env.stopper.stoppedIDs(),
		"orchestrator must NOT stop a session whose true user count is non-zero")
	assert.Equal(t, 0, env.hostRepo.restartCount("h1"))
	assert.True(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_EnrollSeedsCacheFromRPC(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)

	// Park a session with users present so the reconcile triggered at
	// the end of enroll doesn't restart-and-wipe the host's cache
	// before we can inspect it.
	env.addSession("s1", "h1")
	env.addSession("s2", "h1")
	env.client.setUsers("s1", 2)
	env.client.setUsers("s2", 1)
	env.client.setListedSessions(
		&headlessv1.Session{Id: "s1", UsersCount: 2},
		&headlessv1.Session{Id: "s2", UsersCount: 1},
	)

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	env.client.mu.Lock()
	calls := env.client.listCalls
	env.client.mu.Unlock()

	assert.GreaterOrEqual(t, calls, 1, "enroll must call ListSessions to seed the cache")

	env.orch.mu.Lock()
	host := env.orch.sessionUsers["h1"]
	env.orch.mu.Unlock()

	require.NotNil(t, host, "host cache must exist after enroll seed")
	assert.Equal(t, int32(2), host["s1"])
	assert.Equal(t, int32(1), host["s2"])
}

// --- HandleHostEventStreamReset (MAJOR #3) ------------------------------

func TestOrchestrator_StreamResetClearsOnlyAffectedHostsCache(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	// Seed cache for two unrelated hosts via UserJoined events.
	env.orch.HandleHostEvent(t.Context(), "host-a", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserJoinedSession{
			UserJoinedSession: &headlessv1.UserJoinedSession{SessionId: "sa"},
		},
	})
	env.orch.HandleHostEvent(t.Context(), "host-b", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserJoinedSession{
			UserJoinedSession: &headlessv1.UserJoinedSession{SessionId: "sb"},
		},
	})

	env.orch.HandleHostEventStreamReset(t.Context(), "host-a")

	env.orch.mu.Lock()
	_, hadA := env.orch.sessionUsers["host-a"]
	_, hadB := env.orch.sessionUsers["host-b"]
	env.orch.mu.Unlock()

	assert.False(t, hadA, "host-a's cache must be cleared on reset")
	assert.True(t, hadB, "host-b's cache must NOT be affected by host-a's reset")
}

// --- restart retry abort (MINOR #2) -------------------------------------

func TestOrchestrator_AbortsRestartAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.client.setListedSessions() // no sessions → reconcile goes straight to restart

	// Queue up enough failures to trip the abort limit (5).
	env.hostRepo.mu.Lock()

	for range 6 {
		env.hostRepo.restartErrs["h1"] = append(env.hostRepo.restartErrs["h1"], errors.New("docker daemon angry"))
	}

	env.hostRepo.mu.Unlock()

	// First OnNewImage enrols + tries once.
	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	// Drive 4 more reconciles via the periodic path.
	for range 4 {
		env.orch.reconcileAllDraining(t.Context())
	}

	assert.False(t, env.orch.IsHostDraining("h1"), "host should have been dropped after exhausting attempts")
	assert.Equal(t, 0, env.hostRepo.restartCount("h1"), "all restarts failed; count must remain 0")
}

// --- SessionStopper integration -----------------------------------------

func TestOrchestrator_DelegatesStopThroughSessionStopper(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 0)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 0})

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.Equal(t, []string{"s1"}, env.stopper.stoppedIDs(),
		"orchestrator must route session stop through the wired SessionStopper")
}

func TestOrchestrator_StopFailureLeavesHostDraining(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 0)
	env.client.setListedSessions(&headlessv1.Session{Id: "s1", UsersCount: 0})
	env.stopper.err = errors.New("RPC: container unreachable")

	env.orch.OnNewImage(t.Context(), newImage("v2.0.0", "v2.0.0", false))

	assert.Equal(t, 0, env.hostRepo.restartCount("h1"),
		"restart must not fire while sessions remain (stop failed)")
	assert.True(t, env.orch.IsHostDraining("h1"))
}

// --- NoopHostDrainer ----------------------------------------------------

func TestOrchestrator_NoopHostDrainer(t *testing.T) {
	t.Parallel()

	var d port.HostDrainer = port.NoopHostDrainer{}

	assert.False(t, d.IsHostDraining("anything"))
}

// --- helpers ------------------------------------------------------------

func stringPtr(s string) *string { return &s }
