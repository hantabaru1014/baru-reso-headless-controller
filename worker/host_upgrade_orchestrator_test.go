package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// orchestratorHostRepo is a port.HeadlessHostRepository fake covering only
// the methods the orchestrator exercises. Counts how often Restart was
// invoked per host so tests can assert restart-once semantics.
type orchestratorHostRepo struct {
	port.HeadlessHostRepository

	mu       sync.Mutex
	hosts    map[string]*entity.HeadlessHost
	tags     port.ContainerImageList
	tagsErr  error
	listErr  error
	restarts map[string][]port.HeadlessHostStartParams
	clients  map[string]headlessv1.HeadlessControlServiceClient
}

func newOrchestratorHostRepo() *orchestratorHostRepo {
	return &orchestratorHostRepo{
		hosts:    make(map[string]*entity.HeadlessHost),
		restarts: make(map[string][]port.HeadlessHostStartParams),
		clients:  make(map[string]headlessv1.HeadlessControlServiceClient),
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

func (r *orchestratorHostRepo) Find(_ context.Context, id string, _ port.HeadlessHostFetchOptions) (*entity.HeadlessHost, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	h, ok := r.hosts[id]
	if !ok {
		return nil, errors.New("not found: " + id)
	}

	return h, nil
}

func (r *orchestratorHostRepo) ListContainerTags(_ context.Context, _ *string) (port.ContainerImageList, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tags, r.tagsErr
}

func (r *orchestratorHostRepo) Restart(_ context.Context, id string, params port.HeadlessHostStartParams, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

func (r *orchestratorHostRepo) restartCount(hostID string) int { //nolint:unparam // tests intentionally exercise one host
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.restarts[hostID])
}

// orchestratorSessionRepo only implements ListByStatus and Upsert.
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

// stubRPCClient is a tiny HeadlessControlServiceClient covering the two
// RPCs reconcile uses: GetSession (for user counts + worldUrl) and
// StopSession (for closing empty sessions).
type stubRPCClient struct {
	headlessv1.HeadlessControlServiceClient

	mu         sync.Mutex
	users      map[string]int32 // sessionID -> users count
	stopCalls  int
	getCalls   int
	worldURL   string
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
			WorldUrl:   c.worldURL,
		},
	}, nil
}

func (c *stubRPCClient) StopSession(_ context.Context, _ *headlessv1.StopSessionRequest, _ ...grpc.CallOption) (*headlessv1.StopSessionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopCalls++

	return &headlessv1.StopSessionResponse{}, nil
}

func (c *stubRPCClient) setUsers(sessionID string, n int32) { //nolint:unparam // tests intentionally exercise one session
	c.mu.Lock()
	defer c.mu.Unlock()

	c.users[sessionID] = n
}

func (c *stubRPCClient) stopInvocations() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.stopCalls
}

// orchestratorTestEnv bundles the wiring for a single test.
type orchestratorTestEnv struct {
	orch     *HostUpgradeOrchestrator
	hostRepo *orchestratorHostRepo
	sessRepo *orchestratorSessionRepo
	client   *stubRPCClient
}

func newOrchestratorTestEnv(_ *testing.T) *orchestratorTestEnv {
	hr := newOrchestratorHostRepo()
	sr := &orchestratorSessionRepo{}
	c := newStubRPCClient()

	o := NewHostUpgradeOrchestrator(hr, sr, fakeAccountFetcher{}, &config.WorkerConfig{
		UpgradeCheckInterval: 1, // value irrelevant; we drive tick manually
	})

	return &orchestratorTestEnv{
		orch:     o,
		hostRepo: hr,
		sessRepo: sr,
		client:   c,
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

func (e *orchestratorTestEnv) setLatestTag(appVersion string) { //nolint:unparam // tests vary appVersion via separate setups
	e.hostRepo.mu.Lock()
	defer e.hostRepo.mu.Unlock()

	e.hostRepo.tags = port.ContainerImageList{
		{Tag: "old", ResoniteVersion: "2026.1.1", AppVersion: "v0.9.0"},
		{Tag: "v" + appVersion, ResoniteVersion: "2026.1.1", AppVersion: appVersion},
	}
}

func (e *orchestratorTestEnv) addSession(id, hostID string) {
	e.sessRepo.mu.Lock()
	defer e.sessRepo.mu.Unlock()

	e.sessRepo.sessions = append(e.sessRepo.sessions, &entity.Session{
		ID:                id,
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostID,
		StartupParameters: &headlessv1.WorldStartupParameters{},
	})
}

func TestOrchestrator_EnrollsHostWhenNewerTagAvailable(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.setLatestTag("v2.0.0")

	env.orch.discoverUpgradeCandidates(t.Context())

	assert.True(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_IgnoresHostsWithoutOptIn(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_NEVER)
	env.addRunningHost("h2", "v1.0.0", entity.HostAutoUpdatePolicy_UNSPECIFIED)
	env.setLatestTag("v2.0.0")

	env.orch.discoverUpgradeCandidates(t.Context())

	assert.False(t, env.orch.IsHostDraining("h1"))
	assert.False(t, env.orch.IsHostDraining("h2"))
}

func TestOrchestrator_IgnoresUpToDateHosts(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v2.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.setLatestTag("v2.0.0")

	env.orch.discoverUpgradeCandidates(t.Context())

	assert.False(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_RestartsHostWhenSessionsAreEmpty(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 0)
	env.setLatestTag("v2.0.0")

	env.orch.tick(t.Context())

	assert.Equal(t, 1, env.client.stopInvocations(), "empty session should be stopped")
	assert.Equal(t, 1, env.hostRepo.restartCount("h1"), "host should be restarted once its sessions drained")
	assert.False(t, env.orch.IsHostDraining("h1"), "host should leave drain set after restart")
}

func TestOrchestrator_DoesNotRestartWhileUsersPresent(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 3)
	env.setLatestTag("v2.0.0")

	env.orch.tick(t.Context())

	assert.Equal(t, 0, env.client.stopInvocations(), "session with users must not be stopped")
	assert.Equal(t, 0, env.hostRepo.restartCount("h1"), "host with users must not be restarted")
	assert.True(t, env.orch.IsHostDraining("h1"), "host should remain in drain set")
}

func TestOrchestrator_UserLeftEventTriggersRestartWhenLastUserLeaves(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.addSession("s1", "h1")
	env.client.setUsers("s1", 1)
	env.setLatestTag("v2.0.0")

	// Seed the in-memory counter (as the watcher would after a join).
	env.orch.bumpUserCount("s1", 1)

	// First tick records the host into drain set; users are still present.
	env.orch.tick(t.Context())
	require.True(t, env.orch.IsHostDraining("h1"))
	require.Equal(t, 0, env.hostRepo.restartCount("h1"))

	// Now the last user leaves: this should immediately reconcile.
	env.client.setUsers("s1", 0) // RPC also reports 0 (defensive fallback)
	env.orch.HandleHostEvent(t.Context(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserLeftSession{
			UserLeftSession: &headlessv1.UserLeftSession{SessionId: "s1"},
		},
	})

	assert.Equal(t, 1, env.client.stopInvocations(), "empty session should be stopped on user-left")
	assert.Equal(t, 1, env.hostRepo.restartCount("h1"), "host should restart immediately on event")
	assert.False(t, env.orch.IsHostDraining("h1"))
}

func TestOrchestrator_RestartUsesLatestTagAndPolicy(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)
	env.setLatestTag("v2.0.0") // -> Tag = "vv2.0.0" per the helper

	env.orch.tick(t.Context())

	env.hostRepo.mu.Lock()
	calls := env.hostRepo.restarts["h1"]
	env.hostRepo.mu.Unlock()

	require.Len(t, calls, 1)
	assert.Equal(t, "vv2.0.0", calls[0].ContainerImageTag, "restart must use the newest tag")
	assert.Equal(t, entity.HostAutoUpdatePolicy_USERS_EMPTY, calls[0].AutoUpdatePolicy,
		"restart must preserve the AutoUpdatePolicy so future upgrades continue working")
}

func TestOrchestrator_PreReleaseTagsAreIgnored(t *testing.T) {
	t.Parallel()

	env := newOrchestratorTestEnv(t)
	env.addRunningHost("h1", "v1.0.0", entity.HostAutoUpdatePolicy_USERS_EMPTY)

	// Newest tag is a prerelease; the orchestrator should still see the
	// earlier release as latest-stable.
	env.hostRepo.mu.Lock()
	env.hostRepo.tags = port.ContainerImageList{
		{Tag: "v1.0.0", ResoniteVersion: "2026.1.1", AppVersion: "v1.0.0"},
		{Tag: "v1.0.0-pre", ResoniteVersion: "2026.1.1", AppVersion: "v1.0.0-pre", IsPreRelease: true},
	}
	env.hostRepo.mu.Unlock()

	env.orch.discoverUpgradeCandidates(t.Context())

	assert.False(t, env.orch.IsHostDraining("h1"), "host is already on latest stable")
}

func TestOrchestrator_NoopHostDrainer(t *testing.T) {
	t.Parallel()

	var d port.HostDrainer = port.NoopHostDrainer{}

	assert.False(t, d.IsHostDraining("anything"))
}
