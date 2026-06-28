package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

type ScheduledSessionOperationCreateParams struct {
	OperationType    entity.ScheduledOperationType
	OperationPayload json.RawMessage
	TriggerType      entity.ScheduledTriggerType
	TriggerConfig    json.RawMessage
	NextFireAt       time.Time
	HostID           *string
	SessionID        *string
	CreatedBy        *string
}

type ScheduledSessionOperationListFilter struct {
	SessionID *string
	HostID    *string
	Status    *entity.ScheduledOperationStatus
	PageIndex int32
	PageSize  int32
	// GroupID は in-app post-filter として usecase 層で処理される.
	// repo 層では使われない (scheduled_session_operations に group_id 列が無いため).
	GroupID *string
}

type ScheduledSessionOperationListResult struct {
	Items      entity.ScheduledSessionOperationList
	TotalCount int32
}

type ScheduledSessionOperationRepository interface {
	Create(ctx context.Context, params ScheduledSessionOperationCreateParams) (*entity.ScheduledSessionOperation, error)
	Get(ctx context.Context, id string) (*entity.ScheduledSessionOperation, error)
	List(ctx context.Context, filter ScheduledSessionOperationListFilter) (*ScheduledSessionOperationListResult, error)

	// ClaimDue は FOR UPDATE SKIP LOCKED で next_fire_at <= NOW() AND status = PENDING な行を
	// 原子的に RUNNING へ遷移しながら最大 batchSize 件取得する。複数の server インスタンスが
	// 同時にこのメソッドを呼んでも、同じ行を 2 回返さない。
	ClaimDue(ctx context.Context, instanceID string, batchSize int32) (entity.ScheduledSessionOperationList, error)
	// ReleaseStaleClaims は実行中の instance が死んで RUNNING のまま残った行を PENDING に戻す。
	// startup と定期実行の両方で呼ぶ。
	ReleaseStaleClaims(ctx context.Context, staleAfter time.Duration) (int64, error)

	MarkSucceeded(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMessage string) error
	// Requeue は trigger.Evaluate が "未だ ready ではない" を返した場合に呼ぶ。
	// RUNNING の行を PENDING に戻し、次回再評価時刻を設定する。後続 PR の condition 系で使用。
	Requeue(ctx context.Context, id string, nextFireAt time.Time) error
	// Cancel は PENDING の行のみを CANCELED にする。RUNNING / SUCCEEDED / FAILED / CANCELED で
	// 呼ばれた場合は ok=false を返す。
	Cancel(ctx context.Context, id string) (ok bool, err error)
}
