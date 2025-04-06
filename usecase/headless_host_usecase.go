package usecase

import (
	"context"
	"errors"
	"os"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type HeadlessHostUsecase struct {
	hhrepo port.HeadlessHostRepository
	huc    *SessionUsecase
}

func NewHeadlessHostUsecase(hhrepo port.HeadlessHostRepository, huc *SessionUsecase) *HeadlessHostUsecase {
	return &HeadlessHostUsecase{
		hhrepo: hhrepo,
		huc:    huc,
	}
}

func (hhuc *HeadlessHostUsecase) stopRunningSessions(ctx context.Context, hostId string) (entity.SessionList, error) {
	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &hostId,
	})
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		err = hhuc.huc.StopSession(ctx, session.ID)
		if err != nil {
			return nil, err
		}
	}

	return sessions, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostStart(ctx context.Context, params port.HeadlessHostStartParams) (string, error) {
	if params.ContainerImageTag == nil {
		_, err := hhuc.PullLatestHostImage(ctx)
		if err != nil {
			return "", err
		}
		tags, err := hhuc.hhrepo.ListLocalAvailableContainerTags(ctx)
		if err != nil {
			return "", err
		}
		if len(tags) == 0 {
			return "", errors.New("no available container image tags")
		}
		params.ContainerImageTag = &tags[len(tags)-1]
	}

	return hhuc.hhrepo.Start(ctx, params)
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
	_, _ = hhuc.stopRunningSessions(ctx, id)
	var newImage *string
	if withUpdate {
		_, err := hhuc.PullLatestHostImage(ctx)
		if err != nil {
			return "", err
		}
		tags, err := hhuc.hhrepo.ListLocalAvailableContainerTags(ctx)
		if err != nil {
			return "", err
		}
		if len(tags) == 0 {
			return "", errors.New("no available container image tags")
		}
		image := os.Getenv("HEADLESS_IMAGE_NAME") + ":" + tags[len(tags)-1]
		newImage = &image
	}

	// TODO: うまい具合に非同期化する
	newId, err := hhuc.hhrepo.Restart(ctx, host, newImage)
	if err != nil {
		return "", err
	}

	// FIXME: ヘッドレスに起動Configを渡せるようにしたら修正する
	// for _, session := range stoppedSessions {
	// 	_, err = hhuc.huc.StartSession(ctx, newId, session.StartupParameters)
	// 	if err != nil {
	// 		return "", err
	// 	}
	// }

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

func (hhuc *HeadlessHostUsecase) PullLatestHostImage(ctx context.Context) (string, error) {
	localTags, err := hhuc.hhrepo.ListLocalAvailableContainerTags(ctx)
	if err != nil {
		return "", err
	}

	var latestLocalTag *string
	if len(localTags) > 0 {
		latestLocalTag = &localTags[len(localTags)-1]
	}
	remoteTags, err := hhuc.hhrepo.ListContainerTags(ctx, latestLocalTag)
	if err != nil {
		return "", err
	}
	if latestLocalTag == nil && len(remoteTags) == 0 {
		return "", errors.New("no available container image tags")
	}
	if len(remoteTags) == 0 {
		return "Already up to date", nil
	}

	logs, err := hhuc.hhrepo.PullContainerImage(ctx, remoteTags[len(remoteTags)-1])
	if err != nil {
		return "", err
	}

	return logs, nil
}
