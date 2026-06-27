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
	// UpdateAfterWorldSaved は WorldSaved event 反映用の部分更新。
	// startup_parameters と current_state を 1 トランザクションで書き換える
	UpdateAfterWorldSaved(ctx context.Context, id string, startupParameters *headlessv1.WorldStartupParameters, currentState *headlessv1.Session) error
	Get(ctx context.Context, id string) (*entity.Session, error)
	ListAll(ctx context.Context) (entity.SessionList, error)
	ListByStatus(ctx context.Context, status entity.SessionStatus) (entity.SessionList, error)
	ListPaged(ctx context.Context, opts SessionListPageOptions) (*SessionListPageResult, error)
	Delete(ctx context.Context, id string) error
}
