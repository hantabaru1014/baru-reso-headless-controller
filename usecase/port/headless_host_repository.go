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
}

type HeadlessHostRepository interface {
	ListAll(ctx context.Context) (entity.HeadlessHostList, error)
	Find(ctx context.Context, id string) (*entity.HeadlessHost, error)
	GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error)
	GetLogs(ctx context.Context, id string, limit int32, until, since string) (LogLineList, error)
	Rename(ctx context.Context, id, newName string) error
	PullContainerImage(ctx context.Context, tag string) error
	ListContainerTags(ctx context.Context, lastTag *string) ([]string, error)
	Restart(ctx context.Context, host *entity.HeadlessHost) (string, error)
	Start(ctx context.Context, params HeadlessHostStartParams) (string, error)
}
