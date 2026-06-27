package worker

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type paramsApplyCall struct {
	id       string
	snapshot *headlessv1.Session
}

type worldSavedCall struct {
	id       string
	worldURL string
}

type statusCall struct {
	id     string
	status entity.SessionStatus
}

// syncFakeRepo records which repository method handled each write so a
// regression that re-introduces full Upsert is caught.
type syncFakeRepo struct {
	port.SessionRepository

	mu       sync.Mutex
	sessions map[string]*entity.Session

	paramsApplies []paramsApplyCall
	worldSaveds   []worldSavedCall
	downgrades    []string
	statusCalls   []statusCall
	upsertCalls   int
}

func newSyncFakeRepo() *syncFakeRepo {
	return &syncFakeRepo{sessions: make(map[string]*entity.Session)}
}

func (r *syncFakeRepo) Get(_ context.Context, id string) (*entity.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return s, nil
}

func (r *syncFakeRepo) Upsert(_ context.Context, _ *entity.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.upsertCalls++

	return nil
}

func (r *syncFakeRepo) UpdateStatus(_ context.Context, id string, status entity.SessionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.statusCalls = append(r.statusCalls, statusCall{id: id, status: status})

	return nil
}

func (r *syncFakeRepo) ApplySessionParametersChanged(_ context.Context, id string, snapshot *headlessv1.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.paramsApplies = append(r.paramsApplies, paramsApplyCall{id: id, snapshot: snapshot})

	return nil
}

func (r *syncFakeRepo) UpdateAfterWorldSaved(_ context.Context, id string, worldURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.worldSaveds = append(r.worldSaveds, worldSavedCall{id: id, worldURL: worldURL})

	return nil
}

func (r *syncFakeRepo) DowngradeToUnknownIfRunning(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.downgrades = append(r.downgrades, id)

	return nil
}

func (r *syncFakeRepo) ListByHostAndStatus(_ context.Context, hostID string, status entity.SessionStatus) (entity.SessionList, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(entity.SessionList, 0)

	for _, s := range r.sessions {
		if s.HostID == hostID && s.Status == status {
			out = append(out, s)
		}
	}

	return out, nil
}

func (r *syncFakeRepo) seed(s *entity.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[s.ID] = s
}

type syncSnapshot struct {
	ParamsApplies []paramsApplyCall
	WorldSaveds   []worldSavedCall
	Downgrades    []string
	StatusCalls   []statusCall
	Upserts       int
}

func (r *syncFakeRepo) snapshot() syncSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	return syncSnapshot{
		ParamsApplies: append([]paramsApplyCall(nil), r.paramsApplies...),
		WorldSaveds:   append([]worldSavedCall(nil), r.worldSaveds...),
		Downgrades:    append([]string(nil), r.downgrades...),
		StatusCalls:   append([]statusCall(nil), r.statusCalls...),
		Upserts:       r.upsertCalls,
	}
}

// fakeCache captures cache writes / deletes so tests can assert the handler
// routes events through both the DB and the in-memory cache.
type setCall struct {
	hostID    string
	sessionID string
}

type pruneCall struct {
	hostID  string
	liveIDs map[string]struct{}
}

type fakeCache struct {
	mu           sync.Mutex
	sessions     map[string]*headlessv1.Session
	sessionHosts map[string]string
	deletes      []string
	sets         []setCall
	prunes       []pruneCall
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		sessions:     make(map[string]*headlessv1.Session),
		sessionHosts: make(map[string]string),
	}
}

func (c *fakeCache) Get(id string) (*headlessv1.Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, ok := c.sessions[id]

	return s, ok
}

func (c *fakeCache) Set(hostID, id string, snapshot *headlessv1.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessions[id] = snapshot
	c.sessionHosts[id] = hostID
	c.sets = append(c.sets, setCall{hostID: hostID, sessionID: id})
}

func (c *fakeCache) Delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.sessions, id)
	delete(c.sessionHosts, id)
	c.deletes = append(c.deletes, id)
}

func (c *fakeCache) PruneHost(hostID string, liveIDs map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.prunes = append(c.prunes, pruneCall{hostID: hostID, liveIDs: liveIDs})

	for sid, h := range c.sessionHosts {
		if h != hostID {
			continue
		}

		if _, alive := liveIDs[sid]; alive {
			continue
		}

		delete(c.sessions, sid)
		delete(c.sessionHosts, sid)
	}
}

// stubHostRepo only implements GetRpcClient; other methods will panic.
type stubHostRepo struct {
	port.HeadlessHostRepository

	clients map[string]headlessv1.HeadlessControlServiceClient
}

func (r *stubHostRepo) GetRpcClient(_ context.Context, id string) (headlessv1.HeadlessControlServiceClient, error) {
	c, ok := r.clients[id]
	if !ok {
		return nil, errors.New("no client for host: " + id)
	}

	return c, nil
}

type stubControlClient struct {
	headlessv1.HeadlessControlServiceClient

	listSessionsResp *headlessv1.ListSessionsResponse
	listSessionsErr  error

	mu              sync.Mutex
	listSessionsHit int
}

func (c *stubControlClient) ListSessions(_ context.Context, _ *headlessv1.ListSessionsRequest, _ ...grpc.CallOption) (*headlessv1.ListSessionsResponse, error) {
	c.mu.Lock()
	c.listSessionsHit++
	c.mu.Unlock()

	if c.listSessionsErr != nil {
		return nil, c.listSessionsErr
	}

	if c.listSessionsResp != nil {
		return c.listSessionsResp, nil
	}

	return &headlessv1.ListSessionsResponse{}, nil
}

func TestSessionStateSyncHandler_SessionParametersChanged_WritesCacheAndPartialUpdate(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)

	snapshot := &headlessv1.Session{Id: "s1", Name: "new", MaxUsers: 16, Tags: []string{"a"}}
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{SessionId: "s1", Session: snapshot},
		},
	})

	snap := repo.snapshot()
	require.Len(t, snap.ParamsApplies, 1, "must call ApplySessionParametersChanged")
	assert.Equal(t, "s1", snap.ParamsApplies[0].id)
	assert.Equal(t, "new", snap.ParamsApplies[0].snapshot.GetName())
	assert.Equal(t, 0, snap.Upserts, "no full Upsert")

	got, ok := cache.Get("s1")
	require.True(t, ok, "cache must be populated")
	assert.Equal(t, "new", got.GetName())
}

func TestSessionStateSyncHandler_SessionParametersChanged_NilSnapshotIgnored(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{SessionId: "s1"},
		},
	})

	snap := repo.snapshot()
	assert.Empty(t, snap.ParamsApplies)

	_, ok := cache.Get("s1")
	assert.False(t, ok)
}

func TestSessionStateSyncHandler_WorldSaved_UpdatesCacheWorldUrlAndStartupParameters(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	cache.Set("h1", "s1", &headlessv1.Session{Id: "s1", Name: "n", WorldUrl: "old", MaxUsers: 16})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1", WorldUrl: "new"},
		},
	})

	snap := repo.snapshot()
	require.Len(t, snap.WorldSaveds, 1)
	assert.Equal(t, "new", snap.WorldSaveds[0].worldURL)
	assert.Equal(t, 0, snap.Upserts)

	got, ok := cache.Get("s1")
	require.True(t, ok)
	assert.Equal(t, "new", got.GetWorldUrl(), "cache WorldUrl must be replaced")
	assert.Equal(t, "n", got.GetName(), "other fields preserved")
	assert.Equal(t, int32(16), got.GetMaxUsers())
}

func TestSessionStateSyncHandler_WorldSaved_NoCacheEntryStillWritesDB(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1", WorldUrl: "new"},
		},
	})

	snap := repo.snapshot()
	require.Len(t, snap.WorldSaveds, 1, "DB partial update fires even with no cache hit")

	_, ok := cache.Get("s1")
	assert.False(t, ok, "no cache entry -> do not invent one")
}

func TestSessionStateSyncHandler_WorldSaved_EmptyUrlSkipped(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1"},
		},
	})

	snap := repo.snapshot()
	assert.Empty(t, snap.WorldSaveds)
}

func TestSessionStateSyncHandler_UserJoinedSession_BumpsCacheUsersCount(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	cache.Set("h1", "s1", &headlessv1.Session{Id: "s1", UsersCount: 2})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserJoinedSession{
			UserJoinedSession: &headlessv1.UserJoinedSession{SessionId: "s1"},
		},
	})

	got, ok := cache.Get("s1")
	require.True(t, ok)
	assert.Equal(t, int32(3), got.GetUsersCount())
}

func TestSessionStateSyncHandler_UserLeftSession_DecrementsCacheUsersCount(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	cache.Set("h1", "s1", &headlessv1.Session{Id: "s1", UsersCount: 2})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserLeftSession{
			UserLeftSession: &headlessv1.UserLeftSession{SessionId: "s1"},
		},
	})

	got, ok := cache.Get("s1")
	require.True(t, ok)
	assert.Equal(t, int32(1), got.GetUsersCount())
}

func TestSessionStateSyncHandler_UserLeftSession_ClampsAtZero(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	cache.Set("h1", "s1", &headlessv1.Session{Id: "s1", UsersCount: 0})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserLeftSession{
			UserLeftSession: &headlessv1.UserLeftSession{SessionId: "s1"},
		},
	})

	got, ok := cache.Get("s1")
	require.True(t, ok)
	assert.Equal(t, int32(0), got.GetUsersCount(), "UsersCount は 0 未満にならない")
}

func TestSessionStateSyncHandler_UserJoinedSession_NoCacheEntryIgnored(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)

	// cache miss は no-op (event 順序 / 起動前). 後続の SessionParametersChanged や
	// StreamReset で正しい count が seed される.
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_UserJoinedSession{
			UserJoinedSession: &headlessv1.UserJoinedSession{SessionId: "s-missing"},
		},
	})

	_, ok := cache.Get("s-missing")
	assert.False(t, ok)
}

func TestSessionStateSyncHandler_SessionEnded_DeletesCache(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	cache := newFakeCache()
	cache.Set("h1", "s1", &headlessv1.Session{Id: "s1"})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionEnded{
			SessionEnded: &headlessv1.SessionEnded{SessionId: "s1"},
		},
	})

	_, ok := cache.Get("s1")
	assert.False(t, ok, "cache entry must be evicted")

	snap := repo.snapshot()
	// DB demotion is SessionLifecycleHandler's job; this handler must not touch DB.
	assert.Empty(t, snap.ParamsApplies)
	assert.Empty(t, snap.WorldSaveds)
	assert.Empty(t, snap.Downgrades)
	assert.Empty(t, snap.StatusCalls)
	assert.Equal(t, 0, snap.Upserts)
}

func TestSessionStateSyncHandler_StreamReset_RebuildsCacheAndDemotesLost(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s-alive", HostID: "h1", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-lost", HostID: "h1", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-other-host", HostID: "h2", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-ended", HostID: "h1", Status: entity.SessionStatus_ENDED})

	cache := newFakeCache()
	// host h1 で過去に観測された (= cache に entry がある) が stream reset 期間中に
	// container 側からは消えてしまった stale entry。PruneHost で掃除される。
	cache.Set("h1", "s-stale", &headlessv1.Session{Id: "s-stale"})

	client := &stubControlClient{
		listSessionsResp: &headlessv1.ListSessionsResponse{
			Sessions: []*headlessv1.Session{
				{Id: "s-alive", Name: "refreshed", MaxUsers: 24},
			},
		},
	}
	hosts := &stubHostRepo{clients: map[string]headlessv1.HeadlessControlServiceClient{"h1": client}}

	h := NewSessionStateSyncHandler(repo, hosts, cache)
	h.HandleHostEventStreamReset(context.Background(), "h1")

	got, ok := cache.Get("s-alive")
	require.True(t, ok, "live snapshot must populate cache")
	assert.Equal(t, "refreshed", got.GetName())

	_, ok = cache.Get("s-stale")
	assert.False(t, ok, "stale cache entry must be evicted during resync")

	snap := repo.snapshot()
	require.Len(t, snap.Downgrades, 1, "lost RUNNING session demoted to UNKNOWN")
	assert.Equal(t, "s-lost", snap.Downgrades[0])
	assert.Equal(t, 0, snap.Upserts)
	assert.Equal(t, 1, client.listSessionsHit)
}

func TestSessionStateSyncHandler_StreamReset_RpcClientErrorSilenced(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	cache := newFakeCache()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{}, cache)

	h.HandleHostEventStreamReset(context.Background(), "h1")

	snap := repo.snapshot()
	assert.Empty(t, snap.Downgrades, "host unreachable -> leave DB and cache untouched")
}

func TestSessionStateSyncHandler_StreamReset_ListSessionsErrorSilenced(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	client := &stubControlClient{listSessionsErr: errors.New("transient")}
	hosts := &stubHostRepo{clients: map[string]headlessv1.HeadlessControlServiceClient{"h1": client}}
	cache := newFakeCache()
	h := NewSessionStateSyncHandler(repo, hosts, cache)

	h.HandleHostEventStreamReset(context.Background(), "h1")

	snap := repo.snapshot()
	assert.Empty(t, snap.Downgrades, "ListSessions failure -> no demotion (avoid false positives)")
}

// TestSessionStateSyncHandler_StreamReset_LifecycleHandlerDoesNotShadow verifies
// the regression that the previous SessionLifecycleHandler.HandleHostEventStreamReset
// blanket-demoted everything to UNKNOWN, clobbering the per-session reconciliation
// this handler had just performed. HostEventWatcher.notifyReset dispatches all
// handlers in sequence; we simulate that here by calling both in the wired order
// and asserting the live session stays RUNNING.
func TestSessionStateSyncHandler_StreamReset_LifecycleHandlerDoesNotShadow(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s-alive", HostID: "h1", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-lost", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	cache := newFakeCache()
	client := &stubControlClient{
		listSessionsResp: &headlessv1.ListSessionsResponse{
			Sessions: []*headlessv1.Session{
				{Id: "s-alive", Name: "alive"},
			},
		},
	}
	hosts := &stubHostRepo{clients: map[string]headlessv1.HeadlessControlServiceClient{"h1": client}}

	sync := NewSessionStateSyncHandler(repo, hosts, cache)
	lifecycle := NewSessionLifecycleHandler(repo)

	// HostEventWatcher.notifyReset と同じ順序で呼ぶ
	sync.HandleHostEventStreamReset(context.Background(), "h1")
	lifecycle.HandleHostEventStreamReset(context.Background(), "h1")

	snap := repo.snapshot()

	// s-alive は live snapshot にいるので RUNNING のまま残るべき
	// (lifecycle の blanket UNKNOWN 化が復活していたら UNKNOWN になる)
	var aliveStatusUpdates int

	for _, st := range snap.StatusCalls {
		if st.id == "s-alive" {
			aliveStatusUpdates++
		}
	}

	assert.Equal(t, 0, aliveStatusUpdates,
		"live session must not have its status changed by either handler")

	// s-lost は sync 側 (guarded downgrade) で 1 回だけ降格される
	require.Len(t, snap.Downgrades, 1)
	assert.Equal(t, "s-lost", snap.Downgrades[0])
}
