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

// fakeSessionRepo is an in-memory port.SessionRepository for handler tests.
// It mimics the real adapter's behaviour as closely as practical:
//   - Upsert rejects sessions whose StartupParameters is nil (mirrors the
//     sessions.startup_parameters JSONB NOT NULL constraint).
//   - ApplySessionStarted / ApplySessionEnded enforce the same occurred_at
//     guard their SQL counterparts do.
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

	if session.StartupParameters == nil {
		return errors.New("startup_parameters must not be nil (would violate NOT NULL)")
	}

	clone := *session
	r.sessions[session.ID] = &clone

	return nil
}

func (r *fakeSessionRepo) UpdateStatus(_ context.Context, id string, status entity.SessionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return domain.ErrNotFound
	}

	s.Status = status

	return nil
}

func (r *fakeSessionRepo) ApplySessionStarted(_ context.Context, id, hostID, name string, occurredAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return false, nil
	}

	if s.StartedAt != nil && !occurredAt.After(*s.StartedAt) {
		return false, nil
	}

	s.Name = name
	s.Status = entity.SessionStatus_RUNNING
	s.StartedAt = &occurredAt
	s.EndedAt = nil
	s.HostID = hostID

	return true, nil
}

func (r *fakeSessionRepo) ApplySessionEnded(_ context.Context, id string, occurredAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return false, nil
	}

	if s.EndedAt != nil && !occurredAt.After(*s.EndedAt) {
		return false, nil
	}

	s.Status = entity.SessionStatus_ENDED
	s.EndedAt = &occurredAt

	return true, nil
}

func (r *fakeSessionRepo) ListByHostAndStatus(_ context.Context, hostID string, status entity.SessionStatus) (entity.SessionList, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := entity.SessionList{}

	for _, s := range r.sessions {
		if s.HostID == hostID && s.Status == status {
			clone := *s
			out = append(out, &clone)
		}
	}

	return out, nil
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

func ptr[T any](v T) *T { return &v }

func TestSessionLifecycleHandler_SessionStarted(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("既存セッションを最新の occurred_at で更新する (他フィールドは保持)", func(t *testing.T) {
		t.Parallel()

		oldStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
		newStart := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Name:              "old-name",
			Status:            entity.SessionStatus_ENDED,
			HostID:            "host-1",
			StartedAt:         &oldStart,
			EndedAt:           &endedAt,
			OwnerID:           ptr("owner-1"),
			Memo:              "important note",
			AutoUpgrade:       true,
			StartupParameters: &headlessv1.WorldStartupParameters{},
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
		// 部分更新により他フィールドは保持される
		assert.Equal(t, ptr("owner-1"), got.OwnerID)
		assert.Equal(t, "important note", got.Memo)
		assert.True(t, got.AutoUpgrade)
	})

	t.Run("存在しないセッションは新規作成する (StartupParameters は non-nil)", func(t *testing.T) {
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
		require.NotNil(t, got.StartupParameters,
			"new session must have non-nil StartupParameters to satisfy DB NOT NULL")
	})

	t.Run("古い occurred_at の SessionStarted は SQL ガードで skip される", func(t *testing.T) {
		t.Parallel()

		current := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
		older := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Name:              "current-name",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-1",
			StartedAt:         &current,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionStartedEvent(older, "session-1", "stale-name"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, "current-name", got.Name)
		require.NotNil(t, got.StartedAt)
		assert.True(t, got.StartedAt.Equal(current), "started_at must not be rewound by an older event")
	})

	t.Run("occurred_at == 既存 started_at の場合も skip される", func(t *testing.T) {
		t.Parallel()

		ts := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Name:              "current-name",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-1",
			StartedAt:         &ts,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionStartedEvent(ts, "session-1", "same-time"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, "current-name", got.Name, "equal occurred_at must not overwrite the name")
	})

	t.Run("別 host から来た SessionStarted は host_id を更新する", func(t *testing.T) {
		t.Parallel()

		oldStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		newStart := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Name:              "name",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-A",
			StartedAt:         &oldStart,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-B", sessionStartedEvent(newStart, "session-1", "name"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, "host-B", got.HostID, "SessionStarted from a new host must update host_id")
		require.NotNil(t, got.StartedAt)
		assert.True(t, got.StartedAt.Equal(newStart))
	})

	t.Run("Get エラー時は state を変更しない", func(t *testing.T) {
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

	t.Run("既存セッションの ended_at と status を更新する (他フィールドは保持)", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
		endedAt := time.Date(2025, 4, 1, 2, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Name:              "name",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-1",
			StartedAt:         &startedAt,
			OwnerID:           ptr("owner-1"),
			Memo:              "important note",
			AutoUpgrade:       true,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionEndedEvent(endedAt, "session-1"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, entity.SessionStatus_ENDED, got.Status)
		require.NotNil(t, got.EndedAt)
		assert.True(t, got.EndedAt.Equal(endedAt))
		assert.Equal(t, "name", got.Name)
		assert.Equal(t, ptr("owner-1"), got.OwnerID)
		assert.Equal(t, "important note", got.Memo)
		assert.True(t, got.AutoUpgrade)
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
			ID:                "session-1",
			Status:            entity.SessionStatus_ENDED,
			HostID:            "host-1",
			EndedAt:           &endedAt,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionEndedEvent(older, "session-1"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		require.NotNil(t, got.EndedAt)
		assert.True(t, got.EndedAt.Equal(endedAt),
			"ended_at must not be rewound by an older event")
	})

	t.Run("別 host からの SessionEnded はスキップする", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Name:              "name",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-current",
			StartedAt:         &startedAt,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-stale",
			sessionEndedEvent(time.Date(2025, 4, 1, 2, 0, 0, 0, time.UTC), "session-1"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, entity.SessionStatus_RUNNING, got.Status,
			"SessionEnded from a non-owning host must not flip status to ENDED")
		assert.Nil(t, got.EndedAt)
	})

	t.Run("started_at より前の occurred_at はスキップする", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
		bogusEnd := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "session-1",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-1",
			StartedAt:         &startedAt,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEvent(ctx, "host-1", sessionEndedEvent(bogusEnd, "session-1"))

		got := repo.snapshot("session-1")
		require.NotNil(t, got)
		assert.Equal(t, entity.SessionStatus_RUNNING, got.Status)
		assert.Nil(t, got.EndedAt)
	})
}

func TestSessionLifecycleHandler_HandleHostEventStreamReset(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("hostID 配下の RUNNING セッションのみ UNKNOWN に倒す", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

		repo := newFakeSessionRepo()
		repo.seed(&entity.Session{
			ID:                "running-1",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-1",
			StartedAt:         &startedAt,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})
		repo.seed(&entity.Session{
			ID:                "ended-1",
			Status:            entity.SessionStatus_ENDED,
			HostID:            "host-1",
			StartedAt:         &startedAt,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})
		repo.seed(&entity.Session{
			ID:                "other-host-running",
			Status:            entity.SessionStatus_RUNNING,
			HostID:            "host-2",
			StartedAt:         &startedAt,
			StartupParameters: &headlessv1.WorldStartupParameters{},
		})

		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEventStreamReset(ctx, "host-1")

		assert.Equal(t, entity.SessionStatus_UNKNOWN, repo.snapshot("running-1").Status,
			"host-1 RUNNING session must be demoted to UNKNOWN")
		assert.Equal(t, entity.SessionStatus_ENDED, repo.snapshot("ended-1").Status,
			"non-RUNNING sessions on the same host must be left alone")
		assert.Equal(t, entity.SessionStatus_RUNNING, repo.snapshot("other-host-running").Status,
			"sessions on a different host must be left alone")
	})

	t.Run("対象が無い場合は no-op", func(t *testing.T) {
		t.Parallel()

		repo := newFakeSessionRepo()
		h := NewSessionLifecycleHandler(repo)
		h.HandleHostEventStreamReset(ctx, "host-empty")
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
