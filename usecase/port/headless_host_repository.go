package port

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

type LogLine struct {
	Timestamp int64
	IsError   bool
	Body      string
}

type LogLineList []*LogLine

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
	Find(ctx context.Context, id string, fetchOptions HeadlessHostFetchOptions) (*entity.HeadlessHost, error)
	GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error)
	GetLogs(ctx context.Context, id string, limit int32, until, since string) (LogLineList, error)
	Rename(ctx context.Context, id, newName string) error
	// コンテナイメージのタグ一覧をリモートから取得する。一番新しいタグが最後。
	ListContainerTags(ctx context.Context, lastTag *string) (ContainerImageList, error)
	Restart(ctx context.Context, id string, newStartupConfig HeadlessHostStartParams, timeoutSeconds int) error
	Start(ctx context.Context, connector HostConnectorType, params HeadlessHostStartParams, userId *string) (id string, err error)
	// コンテナを終了する
	// timeoutSeconds:
	// - Use '-1' to wait indefinitely.
	// - Use '0' to not wait for the container to exit gracefully, and immediately proceeds to forcibly terminating the container.
	Stop(ctx context.Context, id string, timeoutSeconds int) error
}
