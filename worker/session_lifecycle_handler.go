package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// SessionLifecycleHandler keeps the sessions table in sync with the
// SessionStarted / SessionEnded events emitted by each headless host so
// that container-internal restarts (auto recover, world restart, ...)
// are reflected in the DB without going through a controller-side
// StartSession call.
type SessionLifecycleHandler struct {
	sessionRepo port.SessionRepository
}

func NewSessionLifecycleHandler(sessionRepo port.SessionRepository) *SessionLifecycleHandler {
	return &SessionLifecycleHandler{sessionRepo: sessionRepo}
}

var _ HostEventHandler = (*SessionLifecycleHandler)(nil)

func (h *SessionLifecycleHandler) HandleHostEvent(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	occurredAt := ev.GetOccurredAt().AsTime()

	switch payload := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_SessionStarted:
		h.handleSessionStarted(ctx, hostID, ev.GetId(), occurredAt, payload.SessionStarted)
	case *headlessv1.HostEvent_SessionEnded:
		h.handleSessionEnded(ctx, ev.GetId(), occurredAt, payload.SessionEnded)
	}
}

func (h *SessionLifecycleHandler) HandleHostEventStreamReset(_ context.Context, _ string) {}

func (h *SessionLifecycleHandler) handleSessionStarted(ctx context.Context, hostID, eventID string, occurredAt time.Time, payload *headlessv1.SessionStarted) {
	sessionID := payload.GetSessionId()
	logArgs := []any{"hostID", hostID, "eventID", eventID, "sessionID", sessionID}

	existing, err := h.sessionRepo.Get(ctx, sessionID)

	switch {
	case errors.Is(err, domain.ErrNotFound):
		existing = &entity.Session{
			ID:     sessionID,
			HostID: hostID,
		}
	case err != nil:
		slog.Error("session-lifecycle: failed to load session for SessionStarted",
			append(logArgs, "error", err)...)

		return
	case existing.StartedAt != nil && !occurredAt.After(*existing.StartedAt):
		slog.Debug("session-lifecycle: SessionStarted occurred_at is not newer than stored started_at; skipping",
			append(logArgs, "occurredAt", occurredAt, "storedStartedAt", *existing.StartedAt)...)

		return
	}

	existing.Name = payload.GetSessionName()
	existing.Status = entity.SessionStatus_RUNNING
	existing.StartedAt = &occurredAt
	existing.EndedAt = nil

	if err := h.sessionRepo.Upsert(ctx, existing); err != nil {
		slog.Error("session-lifecycle: failed to upsert session from SessionStarted",
			append(logArgs, "error", err)...)
	}
}

func (h *SessionLifecycleHandler) handleSessionEnded(ctx context.Context, eventID string, occurredAt time.Time, payload *headlessv1.SessionEnded) {
	sessionID := payload.GetSessionId()
	logArgs := []any{"eventID", eventID, "sessionID", sessionID}

	existing, err := h.sessionRepo.Get(ctx, sessionID)

	switch {
	case errors.Is(err, domain.ErrNotFound):
		slog.Warn("session-lifecycle: SessionEnded for unknown session; skipping", logArgs...)

		return
	case err != nil:
		slog.Error("session-lifecycle: failed to load session for SessionEnded",
			append(logArgs, "error", err)...)

		return
	case existing.EndedAt != nil && !occurredAt.After(*existing.EndedAt):
		slog.Debug("session-lifecycle: SessionEnded occurred_at is not newer than stored ended_at; skipping",
			append(logArgs, "occurredAt", occurredAt, "storedEndedAt", *existing.EndedAt)...)

		return
	}

	existing.Status = entity.SessionStatus_ENDED
	existing.EndedAt = &occurredAt

	if err := h.sessionRepo.Upsert(ctx, existing); err != nil {
		slog.Error("session-lifecycle: failed to upsert session from SessionEnded",
			append(logArgs, "error", err)...)
	}
}
