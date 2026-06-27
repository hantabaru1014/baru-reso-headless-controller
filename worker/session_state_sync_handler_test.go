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

// fakeSessionRepo is a minimal stub used to capture Upsert calls and seed
// Get responses. Methods not exercised by the handler stay panicking via
// the embedded interface to surface accidental usage.
type fakeSessionRepo struct {
	port.SessionRepository

	mu       sync.Mutex
	sessions map[string]*entity.Session
	upserts  []*entity.Session
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: make(map[string]*entity.Session)}
}

func (r *fakeSessionRepo) Get(_ context.Context, id string) (*entity.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return s, nil
}

func (r *fakeSessionRepo) Upsert(_ context.Context, s *entity.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[s.ID] = s
	r.upserts = append(r.upserts, s)

	return nil
}

func (r *fakeSessionRepo) ListByStatus(_ context.Context, status entity.SessionStatus) (entity.SessionList, error) {
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

func (r *fakeSessionRepo) seed(s *entity.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[s.ID] = s
}

func (r *fakeSessionRepo) upsertSnapshot() []*entity.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*entity.Session, len(r.upserts))
	copy(out, r.upserts)

	return out
}

// stubHostRepo only implements GetRpcClient. Returning the per-host client
// map matches how the real adapter resolves per-host RPC connections.
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

	getSessions    map[string]*headlessv1.Session
	getSessionErrs map[string]error

	mu        sync.Mutex
	getCalled []string
}

func (c *stubControlClient) GetSession(_ context.Context, in *headlessv1.GetSessionRequest, _ ...grpc.CallOption) (*headlessv1.GetSessionResponse, error) {
	c.mu.Lock()
	c.getCalled = append(c.getCalled, in.GetSessionId())
	c.mu.Unlock()

	if err, ok := c.getSessionErrs[in.GetSessionId()]; ok {
		return nil, err
	}

	s, ok := c.getSessions[in.GetSessionId()]
	if !ok {
		return nil, errors.New("no such session")
	}

	return &headlessv1.GetSessionResponse{Session: s}, nil
}

func (c *stubControlClient) getCalledSnapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]string, len(c.getCalled))
	copy(out, c.getCalled)

	return out
}

func TestSessionStateSyncHandler_SessionParametersChanged_UpsertsCurrentState(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	repo.seed(&entity.Session{
		ID:     "s1",
		Name:   "old-name",
		HostID: "h1",
		Status: entity.SessionStatus_RUNNING,
	})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	snapshot := &headlessv1.Session{Id: "s1", Name: "new-name", MaxUsers: 16}
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{
				SessionId: "s1",
				Session:   snapshot,
			},
		},
	})

	upserts := repo.upsertSnapshot()
	require.Len(t, upserts, 1)
	assert.Equal(t, "new-name", upserts[0].Name)
	require.NotNil(t, upserts[0].CurrentState)
	assert.Equal(t, int32(16), upserts[0].CurrentState.GetMaxUsers())
}

func TestSessionStateSyncHandler_SessionParametersChanged_UnknownSessionIgnored(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{
				SessionId: "unknown",
				Session:   &headlessv1.Session{Id: "unknown"},
			},
		},
	})

	assert.Empty(t, repo.upsertSnapshot(), "no upsert should happen for an unknown session id")
}

func TestSessionStateSyncHandler_WorldSaved_UpdatesStartupParametersAndCurrentState(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	repo.seed(&entity.Session{
		ID:     "s1",
		HostID: "h1",
		Status: entity.SessionStatus_RUNNING,
		StartupParameters: &headlessv1.WorldStartupParameters{
			LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: "old-url"},
		},
		CurrentState: &headlessv1.Session{Id: "s1", WorldUrl: "old-url"},
	})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1", WorldUrl: "new-url"},
		},
	})

	upserts := repo.upsertSnapshot()
	require.Len(t, upserts, 1)
	assert.Equal(t, "new-url", upserts[0].StartupParameters.GetLoadWorldUrl(),
		"StartupParameters.LoadWorldUrl must point to the latest world record")
	assert.Equal(t, "new-url", upserts[0].CurrentState.GetWorldUrl())
}

func TestSessionStateSyncHandler_WorldSaved_EmptyUrlSkipped(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1"},
		},
	})

	assert.Empty(t, repo.upsertSnapshot(), "empty WorldUrl should be a no-op")
}

func TestSessionStateSyncHandler_HandleHostEventStreamReset_RefetchesRunningSessions(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s2", HostID: "h2", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-ended", HostID: "h1", Status: entity.SessionStatus_ENDED})

	client := &stubControlClient{
		getSessions: map[string]*headlessv1.Session{
			"s1": {Id: "s1", Name: "from-resync", MaxUsers: 24},
		},
	}
	hosts := &stubHostRepo{clients: map[string]headlessv1.HeadlessControlServiceClient{"h1": client}}

	h := NewSessionStateSyncHandler(repo, hosts)
	h.HandleHostEventStreamReset(context.Background(), "h1")

	assert.Equal(t, []string{"s1"}, client.getCalledSnapshot(),
		"resync must only call GetSession for sessions belonging to the reset host that are still RUNNING")

	upserts := repo.upsertSnapshot()
	require.Len(t, upserts, 1)
	assert.Equal(t, "s1", upserts[0].ID)
	require.NotNil(t, upserts[0].CurrentState)
	assert.Equal(t, "from-resync", upserts[0].CurrentState.GetName())
}

func TestSessionStateSyncHandler_HandleHostEventStreamReset_RpcClientErrorSilenced(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{clients: nil})

	// Must not panic; should just no-op when the host RPC client is unavailable.
	h.HandleHostEventStreamReset(context.Background(), "h1")
	assert.Empty(t, repo.upsertSnapshot())
}
