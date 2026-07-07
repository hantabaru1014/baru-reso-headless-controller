package hostconnector

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

type HostConnectString string

type HostStartParams struct {
	ID                string
	InstanceId        int32
	ContainerImageTag string
	HeadlessAccount   entity.HeadlessAccount
	StartupConfig     *headlessv1.StartupConfig
}

type HostConnector interface {
	GetStatus(ctx context.Context, connect_string HostConnectString) entity.HeadlessHostStatus
	GetRpcClient(ctx context.Context, connect_string HostConnectString) (headlessv1.HeadlessControlServiceClient, error)
	Start(ctx context.Context, params HostStartParams) (HostConnectString, error)
	Stop(ctx context.Context, connect_string HostConnectString, timeoutSeconds int) error
	Kill(ctx context.Context, connect_string HostConnectString) error
	// Remove removes the container. Returns nil if the container does not exist.
	Remove(ctx context.Context, connect_string HostConnectString) error
	// ListLocalImageTags はローカルに存在する headless image のタグ一覧を返す.
	// docker image はユーザー操作 (prune 等) で消えうるため、ビルド済み判定は
	// DB の記録を信頼せずこれで実在確認する.
	ListLocalImageTags(ctx context.Context) ([]string, error)
}
