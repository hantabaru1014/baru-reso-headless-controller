package port

import (
	"context"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

type SessionListPageOptions struct {
	HostID    *string
	Status    *entity.SessionStatus
	PageIndex int32
	PageSize  int32
}

type SessionListPageResult struct {
	Sessions   entity.SessionList
	TotalCount int32
}

type SessionRepository interface {
	Upsert(ctx context.Context, session *entity.Session) error
	UpdateStatus(ctx context.Context, id string, status entity.SessionStatus) error
	Get(ctx context.Context, id string) (*entity.Session, error)
	ListAll(ctx context.Context) (entity.SessionList, error)
	ListByStatus(ctx context.Context, status entity.SessionStatus) (entity.SessionList, error)
	ListByHostAndStatus(ctx context.Context, hostID string, status entity.SessionStatus) (entity.SessionList, error)
	ListPaged(ctx context.Context, opts SessionListPageOptions) (*SessionListPageResult, error)
	Delete(ctx context.Context, id string) error

	// InsertFromEvent は host 由来の SessionStarted で「DB に存在しない session」を
	// 作るための ON CONFLICT DO NOTHING な INSERT。競合 path (SessionUsecase.StartSession 等)
	// が先に row を作っていた場合は何もしない (handler 側で続けて ApplySessionStarted を
	// 呼ぶ前提)。
	InsertFromEvent(ctx context.Context, session *entity.Session) error
	// ApplySessionStarted は host 由来の SessionStarted を反映する部分更新。
	// occurred_at が現在の started_at より新しい場合のみ status/started_at/ended_at/name/host_id を更新する。
	// 更新が実際に行われた場合は true を返す (skip された場合は false)。
	// memo / auto_upgrade / startup_parameters / owner_id 等他フィールドには触れない。
	ApplySessionStarted(ctx context.Context, id, hostID, name string, occurredAt time.Time) (bool, error)
	// ApplySessionEnded は host 由来の SessionEnded を反映する部分更新。
	// occurred_at が現在の ended_at より新しい場合のみ status/ended_at を更新する。
	// hostID が現所有 host と一致する場合のみ反映 (旧 host から遅延配信された
	// SessionEnded で現所有 host の session を倒さないようにする)。
	ApplySessionEnded(ctx context.Context, id, hostID string, occurredAt time.Time) (bool, error)
}
