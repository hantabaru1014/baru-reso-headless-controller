package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/proto"
)

// SessionStateSyncHandler bridges HostEvent stream and the in-memory
// SessionStateCache plus the DB-side partial updates that persist what
// matters for restart (startup_parameters overlap fields, load_world_url).
//
//   - SessionParametersChanged → cache.Set + DB ApplySessionParametersChanged
//     so in-world edits survive a host restart via startup_parameters.
//   - WorldSaved → update the WorldUrl in cache + DB.startup_parameters.loadWorldUrl
//     so a restart loads the freshly saved record.
//   - SessionEnded → cache.Delete (lifecycle handler still owns DB row demotion).
//   - HandleHostEventStreamReset → ListSessions() to repopulate cache from
//     scratch for the affected host, demote DB rows that are gone.
type SessionStateSyncHandler struct {
	sessionRepo port.SessionRepository
	hostRepo    port.HeadlessHostRepository
	cache       port.SessionStateCache

	mu sync.Mutex
	// per-host index of which session IDs we've populated into cache, so we
	// can prune cache entries that disappear when the host's stream resets.
	hostSessions map[string]map[string]struct{}
}

func NewSessionStateSyncHandler(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository, cache port.SessionStateCache) *SessionStateSyncHandler {
	return &SessionStateSyncHandler{
		sessionRepo:  sessionRepo,
		hostRepo:     hostRepo,
		cache:        cache,
		hostSessions: make(map[string]map[string]struct{}),
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
		h.applySessionEnded(hostID, p.SessionEnded)
	}
}

// HandleHostEventStreamReset reconciles cache + DB after the event buffer
// overflowed. We ListSessions to authoritatively rebuild the cache for
// this host and DowngradeToUnknownIfRunning for any RUNNING row the host
// no longer reports (likely a missed SessionEnded).
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
	for _, s := range listResp.GetSessions() {
		live[s.GetId()] = s
	}

	// cache の host 別 index にあって live にない entry は削除 (cache cleanup)。
	for sessionID := range h.snapshotHostSessionIDs(hostID) {
		if _, ok := live[sessionID]; !ok {
			h.cache.Delete(sessionID)
			h.untrackSession(hostID, sessionID)
		}
	}

	// live snapshot を cache に投入し index も更新。
	for sessionID, snapshot := range live {
		h.cache.Set(sessionID, snapshot)
		h.trackSession(hostID, sessionID)
	}

	// DB の RUNNING で host snapshot に居ないものは UNKNOWN へ降格 (guarded)。
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

	h.cache.Set(sessionID, snapshot)
	h.trackSession(hostID, sessionID)

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

	// cache に既存 snapshot があれば WorldUrl だけ書き換えた clone を Set し直す
	// (snapshot を直接 mutate すると Get で受け取った reader に副作用が出る)
	if existing, ok := h.cache.Get(sessionID); ok {
		cloned, ok := proto.Clone(existing).(*headlessv1.Session)
		if !ok {
			slog.Warn("session-state-sync: failed to clone cached session", "sessionID", sessionID)
		} else {
			cloned.WorldUrl = worldURL
			h.cache.Set(sessionID, cloned)
			h.trackSession(hostID, sessionID)
		}
	}

	if err := h.sessionRepo.UpdateAfterWorldSaved(ctx, sessionID, worldURL); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Warn("session-state-sync: UpdateAfterWorldSaved failed", "hostID", hostID, "sessionID", sessionID, "error", err)
		}
	}
}

func (h *SessionStateSyncHandler) applySessionEnded(hostID string, payload *headlessv1.SessionEnded) {
	sessionID := payload.GetSessionId()
	// DB 側 status / ended_at の更新は SessionLifecycleHandler が担当。
	// ここは cache の cleanup のみ。
	h.cache.Delete(sessionID)
	h.untrackSession(hostID, sessionID)
}

func (h *SessionStateSyncHandler) trackSession(hostID, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	set, ok := h.hostSessions[hostID]
	if !ok {
		set = make(map[string]struct{})
		h.hostSessions[hostID] = set
	}

	set[sessionID] = struct{}{}
}

func (h *SessionStateSyncHandler) untrackSession(hostID, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if set, ok := h.hostSessions[hostID]; ok {
		delete(set, sessionID)

		if len(set) == 0 {
			delete(h.hostSessions, hostID)
		}
	}
}

func (h *SessionStateSyncHandler) snapshotHostSessionIDs(hostID string) map[string]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	set, ok := h.hostSessions[hostID]
	if !ok {
		return nil
	}

	out := make(map[string]struct{}, len(set))
	for k := range set {
		out[k] = struct{}{}
	}

	return out
}
