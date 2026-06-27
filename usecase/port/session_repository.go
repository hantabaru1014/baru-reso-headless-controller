package port

import (
	"context"

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
	ListPaged(ctx context.Context, opts SessionListPageOptions) (*SessionListPageResult, error)
	Delete(ctx context.Context, id string) error
}
