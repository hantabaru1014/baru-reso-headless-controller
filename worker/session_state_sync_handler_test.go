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

type partialUpdate struct {
	id           string
	currentState *headlessv1.Session
	name         string
}

type afterSavedUpdate struct {
	id       string
	worldURL string
}

type statusUpdate struct {
	id     string
	status entity.SessionStatus
}

// syncFakeRepo records which repository method handled each write so a
// regression that re-introduces full Upsert (= race window) is caught.
type syncFakeRepo struct {
	port.SessionRepository

	mu       sync.Mutex
	sessions map[string]*entity.Session

	partialUpdates []partialUpdate
	afterSaved     []afterSavedUpdate
	statusUpdates  []statusUpdate
	upsertCalls    int
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

	r.statusUpdates = append(r.statusUpdates, statusUpdate{id: id, status: status})
	if s, ok := r.sessions[id]; ok {
		s.Status = status
	}

	return nil
}

func (r *syncFakeRepo) UpdateCurrentStateAndName(_ context.Context, id string, currentState *headlessv1.Session, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.partialUpdates = append(r.partialUpdates, partialUpdate{id: id, currentState: currentState, name: name})
	if s, ok := r.sessions[id]; ok {
		s.CurrentState = currentState
		s.Name = name
	}

	return nil
}

func (r *syncFakeRepo) UpdateAfterWorldSaved(_ context.Context, id string, worldURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.afterSaved = append(r.afterSaved, afterSavedUpdate{id: id, worldURL: worldURL})

	return nil
}

func (r *syncFakeRepo) DowngradeToUnknownIfRunning(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.statusUpdates = append(r.statusUpdates, statusUpdate{id: id, status: entity.SessionStatus_UNKNOWN})
	if s, ok := r.sessions[id]; ok && s.Status == entity.SessionStatus_RUNNING {
		s.Status = entity.SessionStatus_UNKNOWN
	}

	return nil
}

func (r *syncFakeRepo) ListByStatus(_ context.Context, status entity.SessionStatus) (entity.SessionList, error) {
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

func (r *syncFakeRepo) seed(s *entity.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[s.ID] = s
}

type repoSnapshot struct {
	Partials []partialUpdate
	Afters   []afterSavedUpdate
	Statuses []statusUpdate
	Upserts  int
}

func (r *syncFakeRepo) snapshot() repoSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	return repoSnapshot{
		Partials: append([]partialUpdate(nil), r.partialUpdates...),
		Afters:   append([]afterSavedUpdate(nil), r.afterSaved...),
		Statuses: append([]statusUpdate(nil), r.statusUpdates...),
		Upserts:  r.upsertCalls,
	}
}

// stubHostRepo only implements GetRpcClient; other methods will panic if hit.
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

func TestSessionStateSyncHandler_SessionParametersChanged_UsesPartialUpdate(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s1", Name: "old", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	snapshot := &headlessv1.Session{Id: "s1", Name: "new", MaxUsers: 16}
	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{SessionId: "s1", Session: snapshot},
		},
	})

	snap := repo.snapshot()
	partials, upserts := snap.Partials, snap.Upserts
	require.Len(t, partials, 1, "must use partial UpdateCurrentStateAndName")
	assert.Equal(t, "s1", partials[0].id)
	assert.Equal(t, "new", partials[0].name)
	assert.Equal(t, int32(16), partials[0].currentState.GetMaxUsers())
	assert.Equal(t, 0, upserts, "must not invoke full Upsert (race window)")
}

func TestSessionStateSyncHandler_SessionParametersChanged_NilSnapshotIgnored(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{SessionId: "s1"},
		},
	})

	snap := repo.snapshot()
	partials := snap.Partials
	assert.Empty(t, partials)
}

func TestSessionStateSyncHandler_WorldSaved_DelegatesToPartialUpdate(t *testing.T) {
	t.Parallel()

	// The handler must NOT fetch the entity via Get and reassemble it before
	// writing — that's the Get→mutate→Update race the partial-update query
	// was introduced to remove. Don't seed anything; the SQL handles
	// non-existence on its own (UPDATE WHERE id matches nothing is a no-op).
	repo := newSyncFakeRepo()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1", WorldUrl: "new"},
		},
	})

	snap := repo.snapshot()
	require.Len(t, snap.Afters, 1, "must dispatch through UpdateAfterWorldSaved")
	assert.Equal(t, "s1", snap.Afters[0].id)
	assert.Equal(t, "new", snap.Afters[0].worldURL)
	assert.Equal(t, 0, snap.Upserts, "no full Upsert (race window)")
}

func TestSessionStateSyncHandler_WorldSaved_EmptyUrlSkipped(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})

	h.HandleHostEvent(context.Background(), "h1", &headlessv1.HostEvent{
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "s1"},
		},
	})

	snap := repo.snapshot()
	assert.Empty(t, snap.Afters)
}

func TestSessionStateSyncHandler_StreamReset_RefreshesLiveAndDemotesLost(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s-alive", HostID: "h1", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-lost", HostID: "h1", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-other-host", HostID: "h2", Status: entity.SessionStatus_RUNNING})
	repo.seed(&entity.Session{ID: "s-ended", HostID: "h1", Status: entity.SessionStatus_ENDED})

	client := &stubControlClient{
		listSessionsResp: &headlessv1.ListSessionsResponse{
			Sessions: []*headlessv1.Session{
				{Id: "s-alive", Name: "refreshed", MaxUsers: 24},
			},
		},
	}
	hosts := &stubHostRepo{clients: map[string]headlessv1.HeadlessControlServiceClient{"h1": client}}

	h := NewSessionStateSyncHandler(repo, hosts)
	h.HandleHostEventStreamReset(context.Background(), "h1")

	snap := repo.snapshot()
	partials, statuses, upserts := snap.Partials, snap.Statuses, snap.Upserts

	require.Len(t, partials, 1, "alive session is refreshed via partial update")
	assert.Equal(t, "s-alive", partials[0].id)
	assert.Equal(t, "refreshed", partials[0].name)

	require.Len(t, statuses, 1, "session missing from ListSessions response gets demoted to UNKNOWN")
	assert.Equal(t, "s-lost", statuses[0].id)
	assert.Equal(t, entity.SessionStatus_UNKNOWN, statuses[0].status)

	assert.Equal(t, 0, upserts, "reset must not use full Upsert")
	assert.Equal(t, 1, client.listSessionsHit, "single ListSessions covers the whole host")
}

func TestSessionStateSyncHandler_StreamReset_RpcClientErrorSilenced(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	h := NewSessionStateSyncHandler(repo, &stubHostRepo{})
	h.HandleHostEventStreamReset(context.Background(), "h1")

	snap := repo.snapshot()
	partials, statuses := snap.Partials, snap.Statuses
	assert.Empty(t, partials)
	assert.Empty(t, statuses, "host unreachable -> can't know whether sessions are lost; leave status alone")
}

func TestSessionStateSyncHandler_StreamReset_ListSessionsErrorSilenced(t *testing.T) {
	t.Parallel()

	repo := newSyncFakeRepo()
	repo.seed(&entity.Session{ID: "s1", HostID: "h1", Status: entity.SessionStatus_RUNNING})

	client := &stubControlClient{listSessionsErr: errors.New("transient")}
	hosts := &stubHostRepo{clients: map[string]headlessv1.HeadlessControlServiceClient{"h1": client}}

	h := NewSessionStateSyncHandler(repo, hosts)
	h.HandleHostEventStreamReset(context.Background(), "h1")

	snap := repo.snapshot()
	partials, statuses := snap.Partials, snap.Statuses
	assert.Empty(t, partials)
	assert.Empty(t, statuses, "ListSessions failure -> no demotion (avoid false positives)")
}
