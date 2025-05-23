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
	Name                      string
	ContainerImageTag         *string
	HeadlessAccountCredential string
	HeadlessAccountPassword   string
	StartupConfig             *headlessv1.StartupConfig
}

type ContainerImage struct {
	Tag             string
	ResoniteVersion string
	IsPreRelease    bool
	AppVersion      string
}

type ContainerImageList []*ContainerImage

type HeadlessHostRepository interface {
	ListAll(ctx context.Context) (entity.HeadlessHostList, error)
	Find(ctx context.Context, id string) (*entity.HeadlessHost, error)
	GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error)
	GetLogs(ctx context.Context, id string, limit int32, until, since string) (LogLineList, error)
	Rename(ctx context.Context, id, newName string) error
	PullContainerImage(ctx context.Context, tag string) (string, error)
	// コンテナイメージのタグ一覧をリモートから取得する。一番新しいタグが最後。
	ListContainerTags(ctx context.Context, lastTag *string) (ContainerImageList, error)
	// ローカルにあるコンテナイメージのタグ一覧を取得する。一番新しいタグが最後。
	ListLocalAvailableContainerTags(ctx context.Context) (ContainerImageList, error)
	Restart(ctx context.Context, host *entity.HeadlessHost, newImage *string) (string, error)
	Start(ctx context.Context, params HeadlessHostStartParams) (string, error)
	GetStartParams(ctx context.Context, id string) (*HeadlessHostStartParams, error)
	// コンテナを終了する
	// timeoutSeconds:
	// - Use '-1' to wait indefinitely.
	// - Use '0' to not wait for the container to exit gracefully, and immediately proceeds to forcibly terminating the container.
	Stop(ctx context.Context, id string, timeoutSeconds int) error
}
