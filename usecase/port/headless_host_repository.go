package port

import (
	"context"

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

type GetLogsParams struct {
	HostID     string
	InstanceID int32
	Limit      int32
	BeforeID   int64 // このIDより小さいログ (古い方向へのページネーション)
	AfterID    int64 // このIDより大きいログ (新しい方向へのページネーション)
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
}

type HeadlessHostFetchOptions struct {
	IncludeStartWorlds bool
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
	ListRunningByAccount(ctx context.Context, accountId string) (entity.HeadlessHostList, error)
	Find(ctx context.Context, id string, fetchOptions HeadlessHostFetchOptions) (*entity.HeadlessHost, error)
	GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error)
	GetLogs(ctx context.Context, params GetLogsParams) (LogLineList, error)
	GetInstanceTimestamps(ctx context.Context, hostID string) (InstanceTimestampList, error)
	Rename(ctx context.Context, id, newName string) error
	UpdateHostSettings(ctx context.Context, id string, settings *entity.HeadlessHostSettings) error
	// コンテナイメージのタグ一覧をリモートから取得する。一番新しいタグが最後。
	ListContainerTags(ctx context.Context, lastTag *string) (ContainerImageList, error)
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
