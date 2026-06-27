package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeSessionRepo is a minimal in-memory stub of port.SessionRepository.
// Only Get and Upsert are implemented because that is all
// SessionLifecycleHandler touches; other methods panic so accidental
// usage is loud.
type fakeSessionRepo struct {
	port.SessionRepository

	mu       sync.Mutex
	sessions map[string]*entity.Session

	upsertErr error
	getErr    error
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: make(map[string]*entity.Session)}
}

func (r *fakeSessionRepo) Get(_ context.Context, id string) (*entity.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.getErr != nil {
		return nil, r.getErr
	}

	s, ok := r.sessions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	clone := *s

	return &clone, nil
}

func (r *fakeSessionRepo) Upsert(_ context.Context, session *entity.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.upsertErr != nil {
		return r.upsertErr
	}

	clone := *session
	r.sessions[session.ID] = &clone

	return nil
}

func (r *fakeSessionRepo) seed(s *entity.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	clone := *s
	r.sessions[s.ID] = &clone
}

func (r *fakeSessionRepo) snapshot(id string) *entity.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return nil
	}

	clone := *s

	return &clone
}

func sessionStartedEvent(t time.Time, sessionID, sessionName string) *headlessv1.HostEvent {
	return &headlessv1.HostEvent{
		Id:         "01EVENT",
		OccurredAt: timestamppb.New(t),
		Payload: &headlessv1.HostEvent_SessionStarted{
			SessionStarted: &headlessv1.SessionStarted{
				SessionId:   sessionID,
				SessionName: sessionName,
			},
		},
	}
}

func sessionEndedEvent(t time.Time, sessionID string) *headlessv1.HostEvent {
	return &headlessv1.HostEvent{
		Id:         "01EVENT",
		OccurredAt: timestamppb.New(t),
		Payload: &headlessv1.HostEvent_SessionEnded{
			SessionEnded: &headlessv1.SessionEnded{
				SessionId: sessionID,
			},
		},
	}
}

func TestSessionLifecycleHandler_SessionStarted(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("既存セッションを最新の occurred_at で更新する", func(t *testing.T) {
		t.Parallel()

		oldStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
		newStart := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:        "session-1",
			Name:      "old-name",
			Status:    entity.SessionStatus_ENDED,
			HostID:    "host-1",
			StartedAt: &oldStart,
			EndedAt:   &endedAt,
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionStartedEvent(newStart, "session-1", "new-name"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, "new-name", got.Name)
		assert.Equal(t, entity.SessionStatus_RUNNING, got.Status)
		require.NotNil(t, got.StartedAt)
		assert.True(t, got.StartedAt.Equal(newStart))
		assert.Nil(t, got.EndedAt)
	})

	t.Run("存在しないセッションは新規作成する", func(t *testing.T) {
		t.Parallel()

		occurredAt := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-7", sessionStartedEvent(occurredAt, "fresh-session", "fresh-name"))

		got := repo.snapshot("fresh-session")
		require.NotNil(t, got)
		assert.Equal(t, "fresh-name", got.Name)
		assert.Equal(t, "host-7", got.HostID)
		assert.Equal(t, entity.SessionStatus_RUNNING, got.Status)
		require.NotNil(t, got.StartedAt)
		assert.True(t, got.StartedAt.Equal(occurredAt))
		assert.Nil(t, got.EndedAt)
	})

	t.Run("古い occurred_at の SessionStarted は無視する", func(t *testing.T) {
		t.Parallel()

		current := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
		older := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:        "session-1",
			Name:      "current-name",
			Status:    entity.SessionStatus_RUNNING,
			HostID:    "host-1",
			StartedAt: &current,
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionStartedEvent(older, "session-1", "stale-name"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, "current-name", got.Name)
		require.NotNil(t, got.StartedAt)
		assert.True(t, got.StartedAt.Equal(current), "started_at must not be rewound by an older event")
	})

	t.Run("repository エラー時は state を変更しない", func(t *testing.T) {
		t.Parallel()

		repo := newFakeSessionRepo()
		repo.getErr = errors.New("db down")

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1",
			sessionStartedEvent(time.Now(), "session-1", "anything"))

		assert.Nil(t, repo.snapshot("session-1"),
			"transient errors must not silently insert a partial session")
	})
}

func TestSessionLifecycleHandler_SessionEnded(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("既存セッションの ended_at と status を更新する", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
		endedAt := time.Date(2025, 4, 1, 2, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:        "session-1",
			Name:      "name",
			Status:    entity.SessionStatus_RUNNING,
			HostID:    "host-1",
			StartedAt: &startedAt,
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionEndedEvent(endedAt, "session-1"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, entity.SessionStatus_ENDED, got.Status)
		require.NotNil(t, got.EndedAt)
		assert.True(t, got.EndedAt.Equal(endedAt))
	})

	t.Run("未知の session_id はスキップする", func(t *testing.T) {
		t.Parallel()

		repo := newFakeSessionRepo()
		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1",
			sessionEndedEvent(time.Now(), "ghost-session"))

		assert.Nil(t, repo.snapshot("ghost-session"),
			"unknown session ends must not create a ghost row")
	})

	t.Run("巻き戻し ended_at は無視する", func(t *testing.T) {
		t.Parallel()

		endedAt := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
		older := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:      "session-1",
			Status:  entity.SessionStatus_ENDED,
			HostID:  "host-1",
			EndedAt: &endedAt,
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionEndedEvent(older, "session-1"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		require.NotNil(t, got.EndedAt)
		assert.True(t, got.EndedAt.Equal(endedAt),
			"ended_at must not be rewound by an older event")
	})
}

func TestSessionLifecycleHandler_IgnoresUnrelatedPayloads(t *testing.T) {
	t.Parallel()

	repo := newFakeSessionRepo()
	h := NewSessionLifecycleHandler(repo)

	h.HandleHostEvent(t.Context(), "host-1", &headlessv1.HostEvent{
		Id:         "01EVENT",
		OccurredAt: timestamppb.New(time.Now()),
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "session-1"},
		},
	})

	assert.Empty(t, repo.sessions, "non-lifecycle payloads must not touch the repo")
}
