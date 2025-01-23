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

type HeadlessHostRepository interface {
	ListAll(ctx context.Context) (entity.HeadlessHostList, error)
	Find(ctx context.Context, id string) (*entity.HeadlessHost, error)
	GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error)
	GetLogs(ctx context.Context, id string, limit int32, until, since string) (LogLineList, error)
	Rename(ctx context.Context, id, newName string) error
	PullLatestContainerImage(ctx context.Context) (string, error)
}
