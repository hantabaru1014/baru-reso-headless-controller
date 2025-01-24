package usecase

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type HeadlessHostUsecase struct {
	hhrepo port.HeadlessHostRepository
}

func NewHeadlessHostUsecase(hhrepo port.HeadlessHostRepository) *HeadlessHostUsecase {
	return &HeadlessHostUsecase{
		hhrepo: hhrepo,
	}
}

func (hhuc *HeadlessHostUsecase) HeadlessHostList(ctx context.Context) (entity.HeadlessHostList, error) {
	hosts, err := hhuc.hhrepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGet(ctx context.Context, id string) (*entity.HeadlessHost, error) {
	host, err := hhuc.hhrepo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	return host, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostRestart(ctx context.Context, id string, withUpdate bool) (string, error) {
	host, err := hhuc.hhrepo.Find(ctx, id)
	if err != nil {
		return "", err
	}
	if withUpdate {
		_, err := hhuc.hhrepo.PullLatestContainerImage(ctx)
		if err != nil {
			return "", err
		}
	}
	// TODO: うまい具合に非同期化する
	newId, err := hhuc.hhrepo.Restart(ctx, host)
	if err != nil {
		return "", err
	}

	return newId, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetLogs(ctx context.Context, id, until, since string, limit int32) (port.LogLineList, error) {
	return hhuc.hhrepo.GetLogs(ctx, id, limit, until, since)
}

func (hhuc *HeadlessHostUsecase) HeadlessHostShutdown(ctx context.Context, id string) error {
	conn, err := hhuc.hhrepo.GetRpcClient(ctx, id)
	if err != nil {
		return err
	}
	_, err = conn.Shutdown(ctx, &headlessv1.ShutdownRequest{})
	if err != nil {
		return err
	}

	return nil
}
