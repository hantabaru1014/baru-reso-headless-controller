package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

type AsyncJobCreateParams struct {
	JobType   entity.AsyncJobType
	Payload   json.RawMessage
	HostID    *string
	SessionID *string
	CreatedBy *string
}

// AsyncJobRepository は非同期 job (ホスト/セッションの起動・停止等) の永続化を担う。
// ScheduledSessionOperationRepository と同様、ClaimDue / ReleaseStaleClaims で
// マルチインスタンス安全な claim を提供する。
type AsyncJobRepository interface {
	Create(ctx context.Context, params AsyncJobCreateParams) (*entity.AsyncJob, error)
	Get(ctx context.Context, id string) (*entity.AsyncJob, error)

	// ClaimDue は FOR UPDATE SKIP LOCKED で PENDING な行を最大 batchSize 件、
	// 古い順に原子的に RUNNING へ遷移しながら取得する。
	ClaimDue(ctx context.Context, instanceID string, batchSize int32) (entity.AsyncJobList, error)
	// ReleaseStaleClaims は実行中の instance が死んで RUNNING のまま残った行を PENDING に戻す。
	ReleaseStaleClaims(ctx context.Context, staleAfter time.Duration) (int64, error)

	// MarkSucceeded は RUNNING の行を SUCCEEDED に更新する。resultPayload は完了時の
	// 戻り値 (host_id / session_id 等) の protojson 表現。
	MarkSucceeded(ctx context.Context, id string, resultPayload json.RawMessage) error
	MarkFailed(ctx context.Context, id string, errMessage string) error
}
