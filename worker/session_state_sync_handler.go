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
// with what the container broadcasts over the HostEventWatcher stream.
// All writes go through partial-update repository methods so concurrent
// UpdateSessionParameters / StartSession / StopSession are not clobbered.
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

// HandleHostEventStreamReset reconciles DB state with the host after the
// event buffer overflowed. For sessions the host still reports we refresh
// CurrentState; for RUNNING DB rows the host no longer reports we demote
// to UNKNOWN via guarded update so a concurrent ENDED is not clobbered.
func (h *SessionStateSyncHandler) HandleHostEventStreamReset(ctx context.Context, hostID string) {
	runningSessions, err := h.sessionRepo.ListByStatus(ctx, entity.SessionStatus_RUNNING)
	if err != nil {
		slog.Error("session-state-sync: failed to list running sessions on reset", "hostID", hostID, "error", err)

		return
	}

	client, err := h.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		slog.Warn("session-state-sync: failed to get rpc client on reset", "hostID", hostID, "error", err)

		return
	}

	listResp, err := client.ListSessions(ctx, &headlessv1.ListSessionsRequest{})
	if err != nil {
		slog.Warn("session-state-sync: ListSessions failed during resync", "hostID", hostID, "error", err)

		return
	}

	live := make(map[string]*headlessv1.Session, len(listResp.GetSessions()))
	for _, s := range listResp.GetSessions() {
		live[s.GetId()] = s
	}

	for _, s := range runningSessions {
		if s.HostID != hostID {
			continue
		}

		if snapshot, ok := live[s.ID]; ok {
			if err := h.sessionRepo.UpdateCurrentStateAndName(ctx, s.ID, snapshot, snapshot.GetName()); err != nil {
				slog.Warn("session-state-sync: UpdateCurrentStateAndName failed during resync", "sessionID", s.ID, "error", err)
			}

			continue
		}

		if err := h.sessionRepo.DowngradeToUnknownIfRunning(ctx, s.ID); err != nil {
			slog.Warn("session-state-sync: DowngradeToUnknownIfRunning failed during resync", "sessionID", s.ID, "error", err)
		}
	}
}

func (h *SessionStateSyncHandler) applySessionParametersChanged(ctx context.Context, hostID string, payload *headlessv1.SessionParametersChanged) {
	sessionID := payload.GetSessionId()
	snapshot := payload.GetSession()

	if snapshot == nil {
		return
	}

	if err := h.sessionRepo.UpdateCurrentStateAndName(ctx, sessionID, snapshot, snapshot.GetName()); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: UpdateCurrentStateAndName failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}
	}
}

func (h *SessionStateSyncHandler) applyWorldSaved(ctx context.Context, hostID string, payload *headlessv1.WorldSaved) {
	sessionID := payload.GetSessionId()
	worldURL := payload.GetWorldUrl()

	if worldURL == "" {
		return
	}

	if err := h.sessionRepo.UpdateAfterWorldSaved(ctx, sessionID, worldURL); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: UpdateAfterWorldSaved failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}
	}
}
