package hostconnector

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type HostConnectString string

type HostConnector interface {
	GetStatus(ctx context.Context, connect_string HostConnectString) entity.HeadlessHostStatus
	GetRpcClient(ctx context.Context, connect_string HostConnectString) (headlessv1.HeadlessControlServiceClient, error)
	GetLogs(ctx context.Context, connect_string HostConnectString, limit int32, until, since string) (port.LogLineList, error)
	Start(ctx context.Context, params port.HeadlessHostStartParams) (HostConnectString, error)
	Stop(ctx context.Context, connect_string HostConnectString, timeoutSeconds int) error
	Kill(ctx context.Context, connect_string HostConnectString) error
}
