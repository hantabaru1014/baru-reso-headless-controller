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

// SessionLifecycleHandler は host から届く SessionStarted / SessionEnded を
// sessions テーブルに idempotent に反映する HostEventHandler。
// controller 経由の StartSession 由来か、container 内部の auto recover や
// world restart 由来か、host UI からの直接起動由来かに関係なく、同じ経路で
// started_at / ended_at / status を最新化する。
type SessionLifecycleHandler struct {
	sessionRepo port.SessionRepository
}

func NewSessionLifecycleHandler(sessionRepo port.SessionRepository) *SessionLifecycleHandler {
	return &SessionLifecycleHandler{sessionRepo: sessionRepo}
}

var _ HostEventHandler = (*SessionLifecycleHandler)(nil)

func (h *SessionLifecycleHandler) HandleHostEvent(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	if ev.GetOccurredAt() == nil {
		slog.Warn("session-lifecycle: dropping event with missing occurred_at",
			"hostID", hostID, "eventID", ev.GetId())

		return
	}

	occurredAt := ev.GetOccurredAt().AsTime()

	switch payload := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_SessionStarted:
		h.handleSessionStarted(ctx, hostID, ev.GetId(), occurredAt, payload.SessionStarted)
	case *headlessv1.HostEvent_SessionEnded:
		h.handleSessionEnded(ctx, hostID, ev.GetId(), occurredAt, payload.SessionEnded)
	}
}

// HandleHostEventStreamReset は OutOfRange でストリームが切れた直後に呼ばれる。
// 切れている間に SessionEnded を取りこぼした可能性があるため、当該 host で
// RUNNING になっている session を一旦 UNKNOWN に倒す。次の SessionStarted や
// 他の resync 経路で正しい状態に戻る想定の最弱の防衛。
func (h *SessionLifecycleHandler) HandleHostEventStreamReset(ctx context.Context, hostID string) {
	sessions, err := h.sessionRepo.ListByHostAndStatus(ctx, hostID, entity.SessionStatus_RUNNING)
	if err != nil {
		slog.Error("session-lifecycle: failed to list RUNNING sessions on stream reset",
			"hostID", hostID, "error", err)

		return
	}

	if len(sessions) == 0 {
		return
	}

	for _, s := range sessions {
		if err := h.sessionRepo.UpdateStatus(ctx, s.ID, entity.SessionStatus_UNKNOWN); err != nil {
			slog.Error("session-lifecycle: failed to demote session on stream reset",
				"hostID", hostID, "sessionID", s.ID, "error", err)
		}
	}

	slog.Warn("session-lifecycle: demoted RUNNING sessions to UNKNOWN due to host event stream reset",
		"hostID", hostID, "count", len(sessions))
}

func (h *SessionLifecycleHandler) handleSessionStarted(ctx context.Context, hostID, eventID string, occurredAt time.Time, payload *headlessv1.SessionStarted) {
	sessionID := payload.GetSessionId()
	logArgs := []any{"hostID", hostID, "eventID", eventID, "sessionID", sessionID}

	// 競合 path (SessionUsecase.StartSession 等) と race にならないよう、
	// 「未存在なら作る」と「部分更新」を分離して両方無条件に呼ぶ:
	//   1. InsertFromEvent は ON CONFLICT DO NOTHING なので、既に row があれば
	//      何もせず memo / owner_id / startup_parameters を壊さない。
	//   2. ApplySessionStarted は started_at < occurred_at 条件で部分 UPDATE。
	//      新しい event なら name/status/started_at/ended_at/host_id だけ反映、
	//      古い event なら no-op。
	// この順序なら「先に何が書かれたか問わず、最終状態は SessionStarted が
	// 反映され、かつ無関係なフィールドは保持される」が成立。
	newSession := &entity.Session{
		ID:                sessionID,
		Name:              payload.GetSessionName(),
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostID,
		StartedAt:         &occurredAt,
		StartupParameters: &headlessv1.WorldStartupParameters{},
	}
	if err := h.sessionRepo.InsertFromEvent(ctx, newSession); err != nil {
		slog.Error("session-lifecycle: failed to insert session from SessionStarted",
			append(logArgs, "error", err)...)

		return
	}

	applied, err := h.sessionRepo.ApplySessionStarted(ctx, sessionID, hostID, payload.GetSessionName(), occurredAt)
	if err != nil {
		slog.Error("session-lifecycle: failed to apply SessionStarted",
			append(logArgs, "error", err)...)

		return
	}

	if !applied {
		slog.Debug("session-lifecycle: SessionStarted occurred_at is not newer than stored started_at; skipped",
			append(logArgs, "occurredAt", occurredAt)...)
	}
}

func (h *SessionLifecycleHandler) handleSessionEnded(ctx context.Context, hostID, eventID string, occurredAt time.Time, payload *headlessv1.SessionEnded) {
	sessionID := payload.GetSessionId()
	logArgs := []any{"hostID", hostID, "eventID", eventID, "sessionID", sessionID}

	existing, err := h.sessionRepo.Get(ctx, sessionID)

	switch {
	case errors.Is(err, domain.ErrNotFound):
		slog.Warn("session-lifecycle: SessionEnded for unknown session; skipping", logArgs...)

		return
	case err != nil:
		slog.Error("session-lifecycle: failed to load session for SessionEnded",
			append(logArgs, "error", err)...)

		return
	}

	if existing.HostID != hostID {
		// 古い host から遅延配信された SessionEnded で現所有 host の session を
		// 倒さないようにスキップする (SQL の host_id 一致条件と二重防御)。
		slog.Warn("session-lifecycle: SessionEnded from non-owning host; skipping",
			append(logArgs, "ownerHostID", existing.HostID)...)

		return
	}

	if existing.StartedAt != nil && occurredAt.Before(*existing.StartedAt) {
		slog.Warn("session-lifecycle: SessionEnded occurred_at predates started_at; skipping",
			append(logArgs, "occurredAt", occurredAt, "storedStartedAt", *existing.StartedAt)...)

		return
	}

	applied, err := h.sessionRepo.ApplySessionEnded(ctx, sessionID, hostID, occurredAt)
	if err != nil {
		slog.Error("session-lifecycle: failed to apply SessionEnded",
			append(logArgs, "error", err)...)

		return
	}

	if !applied {
		slog.Debug("session-lifecycle: SessionEnded occurred_at is not newer than stored ended_at; skipped",
			append(logArgs, "occurredAt", occurredAt)...)
	}
}
