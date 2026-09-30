package port

import (
	"context"
	"math"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

type LogLine struct {
	ID        int64
	Timestamp int64
	IsError   bool
	Body      string
}

type LogLineList []*LogLine

// LatestLogCursorID は「最新のログから古い方向へ」を表すカーソル ID (どのログ ID よりも大きい).
const LatestLogCursorID int64 = math.MaxInt64

// GetLogsParams はログ取得の条件.
type GetLogsParams struct {
	HostID     string
	InstanceID int32
	Limit      int32
	// 起点となるログID (このID自体は対象外).
	CursorID int64
	// true なら CursorID より新しいログを、false なら古いログを、カーソルに近い方から Limit 件取得する.
	Newer bool
}

// SearchLogParams はログ検索の条件.
type SearchLogParams struct {
	HostID     string
	InstanceID int32
	Query      string // 部分一致・大文字小文字を区別しない
	// 検索の起点となるログID (このID自体は対象外).
	CursorID int64
	// true なら CursorID より新しい方向へ、false なら古い方向へ検索する.
	Newer bool
}

type InstanceTimestamp struct {
	InstanceID int32
	FirstLogAt *int64 // UnixTime (秒), nil = データなし
	LastLogAt  *int64 // UnixTime (秒), nil = データなし
	LogCount   int64
}

type InstanceTimestampList []*InstanceTimestamp

type HeadlessHostStartParams struct {
	Name              string
	ContainerImageTag string
	HeadlessAccount   entity.HeadlessAccount
	StartupConfig     *headlessv1.StartupConfig
	AutoUpdatePolicy  entity.HostAutoUpdatePolicy
	Memo              string
	GroupID           string // 起動するホストの所属グループ ID. 必須.
}

type HeadlessHostFetchOptions struct {
	IncludeStartWorlds bool
}

type HostListPageOptions struct {
	PageIndex    int32
	PageSize     int32
	FetchOptions HeadlessHostFetchOptions
	// GroupIDs はグループフィルタ.
	//   - nil: 全グループ対象 (system:group.list 保持者 / 認可レイヤで判断済み)
	//   - 空 slice: マッチゼロ件 (= 所属グループが無いユーザーの自動絞り込み結果)
	//   - 非空 slice: 指定 group_id 群でのみ絞り込み
	GroupIDs []string
}

type HostListPageResult struct {
	Hosts      entity.HeadlessHostList
	TotalCount int32
}

type ContainerImage struct {
	Tag             string
	ResoniteVersion string
	IsPreRelease    bool
	AppVersion      string
}

type ContainerImageList []*ContainerImage

type HostConnectorType string

const HostConnectorType_DOCKER HostConnectorType = "docker"

type HeadlessHostRepository interface {
	ListAll(ctx context.Context, fetchOptions HeadlessHostFetchOptions) (entity.HeadlessHostList, error)
	ListPaged(ctx context.Context, opts HostListPageOptions) (*HostListPageResult, error)
	ListRunningByAccount(ctx context.Context, accountId string) (entity.HeadlessHostList, error)
	Find(ctx context.Context, id string, fetchOptions HeadlessHostFetchOptions) (*entity.HeadlessHost, error)
	// GetGroupID は host の group_id だけを DB のみで返す軽量メソッド.
	// permission interceptor が container RPC を起こさないようにするための専用 API.
	GetGroupID(ctx context.Context, id string) (string, error)
	GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error)
	// GetLogs はログを時系列順 (ID 昇順) で返す. hasMore は取得方向にさらにログが存在するかどうか.
	GetLogs(ctx context.Context, params GetLogsParams) (logs LogLineList, hasMore bool, err error)
	// SearchLog は本文が Query を含むログのうち、カーソルから指定方向で最も近い 1 件の ID を返す.
	// 見つからない場合は found = false (エラーではない).
	SearchLog(ctx context.Context, params SearchLogParams) (logID int64, found bool, err error)
	GetInstanceTimestamps(ctx context.Context, hostID string) (InstanceTimestampList, error)
	Rename(ctx context.Context, id, newName string) error
	UpdateHostSettings(ctx context.Context, id string, settings *entity.HeadlessHostSettings) error
	UpdateAutoUpdatePolicy(ctx context.Context, id string, policy entity.HostAutoUpdatePolicy) error
	Restart(ctx context.Context, id string, newStartupConfig HeadlessHostStartParams, timeoutSeconds int) error
	Start(ctx context.Context, connector HostConnectorType, params HeadlessHostStartParams, userId *string) (id string, err error)
	// コンテナを終了する
	// timeoutSeconds:
	// - Use '-1' to wait indefinitely.
	// - Use '0' to not wait for the container to exit gracefully, and immediately proceeds to forcibly terminating the container.
	Stop(ctx context.Context, id string, timeoutSeconds int) error
	// コンテナを強制停止する
	Kill(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}
