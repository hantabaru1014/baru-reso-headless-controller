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
// (UpdateCurrentStateAndName / UpdateAfterWorldSaved /
// DowngradeToUnknownIfRunning) so a concurrent UpdateSessionParameters /
// StartSession / StopSession can't have its startup_parameters / status /
// ended_at silently clobbered by a stale snapshot the handler had read a
// moment earlier. UpdateAfterWorldSaved takes only the new world_url and
// does the JSONB rewrite server-side, so there's no Get→mutate→Update
// round trip to lose updates against.
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
// Demotion uses DowngradeToUnknownIfRunning (guarded update) so a
// concurrent StopSession that successfully closed the session (status →
// ENDED) won't get clobbered back to UNKNOWN.
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

		// DB は RUNNING のままだが host から消えている = SessionEnded を取り
		// こぼした疑い。RUNNING のときだけ UNKNOWN に降ろす (StopSession で
		// すでに ENDED に至った session を巻き戻さないため guarded update)。
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
		// container 側 WorldSavedHook が観測ログを残しているのでここでは握りつぶす
		return
	}

	// world_url 1 値だけを SQL に渡し、JSONB の jsonb_set で server-side 書き換え
	// する。Get→mutate→Update を往復しないので、gRPC UpdateSessionParameters /
	// UpdateSessionExtraSettings の Upsert と race しても他フィールドを stale
	// snapshot で revert しない (preset case は SQL 側で除去、startup_parameters の
	// loadWorldUrl が次回起動時の load_world として使われる)。
	if err := h.sessionRepo.UpdateAfterWorldSaved(ctx, sessionID, worldURL); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: UpdateAfterWorldSaved failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}
	}
}
