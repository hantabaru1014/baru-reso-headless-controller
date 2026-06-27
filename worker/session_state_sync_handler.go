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
//
// All writes go through partial-update repository methods
// (UpdateCurrentStateAndName / UpdateAfterWorldSaved) instead of Upsert
// so a concurrent UpdateSessionParameters / StartSession / StopSession
// can't have its startup_parameters / status / ended_at silently
// clobbered by a stale snapshot that the handler had Get'd moments earlier.
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
// event buffer overflowed (some events were definitely lost).
//
// Two things must happen:
//  1. For each session the host still reports, refresh CurrentState so
//     any missed SessionParametersChanged catches up.
//  2. For sessions the DB believes are RUNNING on this host but the host
//     no longer reports, demote status to UNKNOWN — we likely missed a
//     SessionEnded and would otherwise leave the row RUNNING forever.
//
// A separate SessionLifecycleHandler (sibling branch) also touches status
// on reset by indiscriminately marking every RUNNING session UNKNOWN.
// Both handlers can coexist: the lifecycle handler's blanket demotion is
// harmless because the live event stream that resumes after the reset,
// together with this handler's per-host ListSessions resync, will promote
// surviving sessions back to RUNNING with fresh CurrentState.
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

		// DB は RUNNING のままだが host から消えている = SessionEnded を
		// 取りこぼした疑い。確証はないので一旦 UNKNOWN へ落として、後続の
		// reconciler / 手動オペが正しい終端 status に降ろせるようにする
		if err := h.sessionRepo.UpdateStatus(ctx, s.ID, entity.SessionStatus_UNKNOWN); err != nil {
			slog.Warn("session-state-sync: UpdateStatus(UNKNOWN) failed during resync", "sessionID", s.ID, "error", err)
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
		// container 側 WorldSavedHook が観測ログを残しているのでここでは握りつぶす
		return
	}

	s, err := h.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: Get failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}

		return
	}

	if s.StartupParameters == nil && s.CurrentState == nil {
		return
	}

	// LoadWorldPresetName を意図的に load_world_url で置き換える: preset 由来で
	// 保存された world は次回起動時 preset ではなく保存済み URL でロードしたい
	if s.StartupParameters != nil {
		s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
			LoadWorldUrl: worldURL,
		}
	}

	if s.CurrentState != nil {
		s.CurrentState.WorldUrl = worldURL
	}

	if err := h.sessionRepo.UpdateAfterWorldSaved(ctx, sessionID, s.StartupParameters, s.CurrentState); err != nil {
		slog.Warn("session-state-sync: UpdateAfterWorldSaved failed", "sessionID", sessionID, "error", err)
	}
}
