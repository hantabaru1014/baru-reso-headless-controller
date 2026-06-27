package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/proto"
)

// SessionStateSyncHandler bridges the HostEvent stream and:
//   - the in-memory SessionStateCache (volatile CurrentState snapshot owned by container)
//   - the DB-side partial updates that persist what matters for restart
//     (startup_parameters overlap fields, load_world_url).
//
// Concrete event routing:
//   - SessionParametersChanged → cache.Set + DB ApplySessionParametersChanged
//     so in-world edits both stay visible in the UI and survive a host restart
//     via startup_parameters.
//   - WorldSaved → patch the cached snapshot's WorldUrl (via proto.Clone because
//     cache aliasing contract forbids mutation) + DB UpdateAfterWorldSaved so a
//     restart loads the freshly saved record.
//   - SessionEnded → cache.Delete. DB row demotion is SessionLifecycleHandler's job.
//   - HandleHostEventStreamReset → ListSessions on the host to authoritatively
//     rebuild cache for the host (PruneHost evicts entries the host no longer
//     reports), and DowngradeToUnknownIfRunning for RUNNING DB rows that the
//     host no longer reports (likely a missed SessionEnded).
type SessionStateSyncHandler struct {
	sessionRepo port.SessionRepository
	hostRepo    port.HeadlessHostRepository
	cache       port.SessionStateCache
}

func NewSessionStateSyncHandler(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository, cache port.SessionStateCache) *SessionStateSyncHandler {
	return &SessionStateSyncHandler{
		sessionRepo: sessionRepo,
		hostRepo:    hostRepo,
		cache:       cache,
	}
}

var _ HostEventHandler = (*SessionStateSyncHandler)(nil)

func (h *SessionStateSyncHandler) HandleHostEvent(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	switch p := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_SessionParametersChanged:
		h.applySessionParametersChanged(ctx, hostID, p.SessionParametersChanged)
	case *headlessv1.HostEvent_WorldSaved:
		h.applyWorldSaved(ctx, hostID, p.WorldSaved)
	case *headlessv1.HostEvent_SessionEnded:
		h.applySessionEnded(p.SessionEnded)
	}
}

// HandleHostEventStreamReset rebuilds the cache from the host snapshot and
// demotes DB rows the host no longer reports. SessionLifecycleHandler's
// HandleHostEventStreamReset is intentionally a no-op so it cannot clobber
// these per-session decisions.
func (h *SessionStateSyncHandler) HandleHostEventStreamReset(ctx context.Context, hostID string) {
	runningSessions, err := h.sessionRepo.ListByHostAndStatus(ctx, hostID, entity.SessionStatus_RUNNING)
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
	liveIDs := make(map[string]struct{}, len(listResp.GetSessions()))

	for _, s := range listResp.GetSessions() {
		live[s.GetId()] = s
		liveIDs[s.GetId()] = struct{}{}
	}

	// cache を host snapshot で再構築。live にあるものは Set し直し、無いものは
	// PruneHost でまとめて削除する。cache 自身が host_id を index しているので、
	// handler 経由で Set した entry も usecase 側経路 (StartSession / GetSession
	// cache-miss / UpdateSessionParameters) で Set した entry も同様に掃除される。
	for sessionID, snapshot := range live {
		h.cache.Set(hostID, sessionID, snapshot)
	}

	h.cache.PruneHost(hostID, liveIDs)

	// DB の RUNNING で host snapshot に居ないものは UNKNOWN へ降格 (guarded、
	// gRPC StopSession で並列に ENDED へ落ちたものは巻き戻さない)。
	for _, s := range runningSessions {
		if _, ok := live[s.ID]; ok {
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

	// 同じ snapshot を SessionUsecase.UpdateSessionParameters も cache.Set する
	// 経路があるが、a→d / d→a どちらの順序で観測されても最終 cache 状態は
	// convergent (どちらも container 由来の同じ snapshot)。
	h.cache.Set(hostID, sessionID, snapshot)

	if err := h.sessionRepo.ApplySessionParametersChanged(ctx, sessionID, snapshot); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: ApplySessionParametersChanged failed", "hostID", hostID, "sessionID", sessionID, "error", err)
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

	// cache aliasing contract: Get で受け取った snapshot を直接 mutate すると、
	// 別 caller が Get 済みの pointer にも副作用が出る (UI 表示中の race など)。
	// 必ず proto.Clone してから書き換え、Set し直す。
	if existing, ok := h.cache.Get(sessionID); ok {
		cloned, ok := proto.Clone(existing).(*headlessv1.Session)
		if !ok {
			slog.Warn("session-state-sync: failed to clone cached session", "sessionID", sessionID)
		} else {
			cloned.WorldUrl = worldURL
			h.cache.Set(hostID, sessionID, cloned)
		}
	}

	if err := h.sessionRepo.UpdateAfterWorldSaved(ctx, sessionID, worldURL); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: UpdateAfterWorldSaved failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}
	}
}

func (h *SessionStateSyncHandler) applySessionEnded(payload *headlessv1.SessionEnded) {
	sessionID := payload.GetSessionId()
	// DB 側 status / ended_at の更新は SessionLifecycleHandler が担当。
	// ここは cache の cleanup のみ。
	h.cache.Delete(sessionID)
}
