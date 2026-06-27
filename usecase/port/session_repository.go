package port

import (
	"context"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
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
	// UpdateCurrentStateAndName は host 側 event 駆動同期で使う部分更新。
	// startup_parameters / status / ended_at など他カラムを書き換えないことで
	// 並行する UpdateSessionParameters / StartSession / StopSession の Upsert と
	// race しても上書き事故を起こさない
	UpdateCurrentStateAndName(ctx context.Context, id string, currentState *headlessv1.Session, name string) error
	// UpdateAfterWorldSaved は WorldSaved event 反映用の部分更新。world_url 1 値
	// だけを受け取り、startup_parameters の loadWorldUrl と current_state の
	// worldUrl を JSONB の in-place 書き換えで更新する。current_state が NULL
	// なら触らない (初期 startup 直後の race を救済)
	UpdateAfterWorldSaved(ctx context.Context, id string, worldURL string) error
	// DowngradeToUnknownIfRunning は StreamReset 時の lost session 救済用の
	// guarded update。RUNNING のときだけ UNKNOWN へ降ろし、StopSession で
	// ENDED に至った session を巻き戻さない
	DowngradeToUnknownIfRunning(ctx context.Context, id string) error
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
