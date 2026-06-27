package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// SessionStateSyncHandler keeps the persisted Session.CurrentState in sync
// with what the container is broadcasting over the HostEventWatcher stream,
// removing the need for per-request polling in the session usecase.
type SessionStateSyncHandler struct {
	sessionRepo port.SessionRepository
	hostRepo    port.HeadlessHostRepository
}

func NewSessionStateSyncHandler(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository) *SessionStateSyncHandler {
	return &SessionStateSyncHandler{
		sessionRepo: sessionRepo,
		hostRepo:    hostRepo,
	}
}

var _ HostEventHandler = (*SessionStateSyncHandler)(nil)

func (h *SessionStateSyncHandler) HandleHostEvent(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	switch p := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_SessionParametersChanged:
		h.applySessionParametersChanged(ctx, hostID, p.SessionParametersChanged)
	case *headlessv1.HostEvent_WorldSaved:
		h.applyWorldSaved(ctx, hostID, p.WorldSaved)
	}
}

// HandleHostEventStreamReset re-pulls fresh state from the host so the DB
// catches up after the event buffer overflowed and we lost some updates.
func (h *SessionStateSyncHandler) HandleHostEventStreamReset(ctx context.Context, hostID string) {
	sessions, err := h.sessionRepo.ListByStatus(ctx, entity.SessionStatus_RUNNING)
	if err != nil {
		slog.Error("session-state-sync: failed to list running sessions on reset", "hostID", hostID, "error", err)

		return
	}

	client, err := h.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		slog.Warn("session-state-sync: failed to get rpc client on reset", "hostID", hostID, "error", err)

		return
	}

	for _, s := range sessions {
		if s.HostID != hostID {
			continue
		}

		resp, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: s.ID})
		if err != nil || resp.GetSession() == nil {
			slog.Warn("session-state-sync: GetSession failed during resync", "hostID", hostID, "sessionID", s.ID, "error", err)

			continue
		}

		s.CurrentState = resp.GetSession()
		if err := h.sessionRepo.Upsert(ctx, s); err != nil {
			slog.Warn("session-state-sync: Upsert failed during resync", "sessionID", s.ID, "error", err)
		}
	}
}

func (h *SessionStateSyncHandler) applySessionParametersChanged(ctx context.Context, hostID string, payload *headlessv1.SessionParametersChanged) {
	sessionID := payload.GetSessionId()

	s, err := h.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: Get failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}

		return
	}

	s.CurrentState = payload.GetSession()
	if name := payload.GetSession().GetName(); name != "" {
		s.Name = name
	}

	if err := h.sessionRepo.Upsert(ctx, s); err != nil {
		slog.Warn("session-state-sync: Upsert failed", "sessionID", sessionID, "error", err)
	}
}

func (h *SessionStateSyncHandler) applyWorldSaved(ctx context.Context, hostID string, payload *headlessv1.WorldSaved) {
	sessionID := payload.GetSessionId()

	worldURL := payload.GetWorldUrl()
	if worldURL == "" {
		return
	}

	s, err := h.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: Get failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}

		return
	}

	if s.StartupParameters != nil {
		s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
			LoadWorldUrl: worldURL,
		}
	}

	if s.CurrentState != nil {
		s.CurrentState.WorldUrl = worldURL
	}

	if err := h.sessionRepo.Upsert(ctx, s); err != nil {
		slog.Warn("session-state-sync: Upsert failed after WorldSaved", "sessionID", sessionID, "error", err)
	}
}
