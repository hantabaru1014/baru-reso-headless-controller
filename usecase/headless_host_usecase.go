package usecase

import (
	"context"
	"errors"
	"os"
	"slices"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type HeadlessHostUsecase struct {
	hhrepo port.HeadlessHostRepository
	srepo  port.SessionRepository
	huc    *SessionUsecase
}

func NewHeadlessHostUsecase(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, huc *SessionUsecase) *HeadlessHostUsecase {
	return &HeadlessHostUsecase{
		hhrepo: hhrepo,
		srepo:  srepo,
		huc:    huc,
	}
}

func (hhuc *HeadlessHostUsecase) HeadlessHostStart(ctx context.Context, params port.HeadlessHostStartParams) (string, error) {
	if params.ContainerImageTag == nil {
		tags, err := hhuc.hhrepo.ListContainerTags(ctx, nil)
		if err != nil {
			return "", err
		}
		var latestReleaseTag *string
		for _, tag := range slices.Backward(tags) {
			if !tag.IsPreRelease {
				latestReleaseTag = &tag.Tag
				break
			}
		}
		if latestReleaseTag == nil {
			return "", errors.New("no available container image tags (release version)")
		}
		params.ContainerImageTag = latestReleaseTag
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

func (hhuc *HeadlessHostUsecase) HeadlessHostGetSettings(ctx context.Context, id string) (*entity.HeadlessHostSettings, error) {
	cli, err := hhuc.hhrepo.GetRpcClient(ctx, id)
	if err != nil {
		return nil, err
	}
	resp, err := cli.GetHostSettings(ctx, &headlessv1.GetHostSettingsRequest{})
	if err != nil {
		return nil, err
	}
	settings := &entity.HeadlessHostSettings{
		UniverseID:                  resp.UniverseId,
		TickRate:                    resp.TickRate,
		MaxConcurrentAssetTransfers: resp.MaxConcurrentAssetTransfers,
		UsernameOverride:            resp.UsernameOverride,
		AllowedUrlHosts:             make([]entity.HostAllowedAccessEntry, 0, len(resp.AllowedUrlHosts)),
		AutoSpawnItems:              resp.AutoSpawnItems,
	}
	for _, entry := range resp.AllowedUrlHosts {
		types := make([]entity.HostAllowedAccessType, len(entry.AccessTypes))
		for _, t := range entry.AccessTypes {
			types = append(types, entity.HostAllowedAccessType(t))
		}
		settings.AllowedUrlHosts = append(settings.AllowedUrlHosts, entity.HostAllowedAccessEntry{
			Host:        entry.Host,
			Ports:       entry.Ports,
			AccessTypes: types,
		})
	}

	return settings, nil
}

func (hhuc *HeadlessHostUsecase) markSessionsAsEnded(ctx context.Context, sessions entity.SessionList) error {
	// FIXME: 仮の実装. session usecaseにまとめられるようにする
	now := time.Now()
	for _, s := range sessions {
		s.EndedAt = &now
		s.Status = entity.SessionStatus_ENDED
		if s.CurrentState != nil && s.CurrentState.WorldUrl != "" {
			s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
				LoadWorldUrl: s.CurrentState.WorldUrl,
			}
		}
		err := hhuc.srepo.Upsert(ctx, s)
		if err != nil {
			return err
		}
	}
	return nil
}

// HeadlessHostRestart restarts the headless host with the specified ID.
// If newTag is "latestRelease", it will use the latest release tag.
func (hhuc *HeadlessHostUsecase) HeadlessHostRestart(ctx context.Context, id string, newTag *string, withWorldRestart bool, timeoutSeconds int) (string, error) {
	host, err := hhuc.hhrepo.Find(ctx, id)
	if err != nil {
		return "", err
	}
	status := entity.SessionStatus_RUNNING
	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &host.ID,
		Status: &status,
	})
	if err != nil {
		return "", err
	}

	var tagToUse *string
	if newTag != nil && *newTag == "" {
		if *newTag == "latestRelease" {
			tags, err := hhuc.hhrepo.ListContainerTags(ctx, nil)
			if err != nil {
				return "", err
			}
			if len(tags) == 0 {
				return "", errors.New("no available container image tags")
			}
			for _, tag := range slices.Backward(tags) {
				if !tag.IsPreRelease {
					tagToUse = &tag.Tag
					break
				}
			}
			if tagToUse == nil {
				return "", errors.New("no available container image tags")
			}
		} else {
			tagToUse = newTag
		}
	}

	err = hhuc.markSessionsAsEnded(ctx, sessions)
	if err != nil {
		return "", err
	}

	if host.Status == entity.HeadlessHostStatus_RUNNING {
		startupConfig, err := hhuc.hhrepo.GetStartParams(ctx, id)
		if err != nil {
			return "", err
		}
		startupConfig.ContainerImageTag = tagToUse
		if !withWorldRestart && startupConfig.StartupConfig != nil {
			startupConfig.StartupConfig.StartWorlds = nil
		}
		err = hhuc.hhrepo.Stop(ctx, id, timeoutSeconds)
		if err != nil {
			return "", err
		}
		newId, err := hhuc.hhrepo.Start(ctx, *startupConfig)
		if err != nil {
			return "", err
		}

		return newId, nil
	} else {
		var newImage *string
		if tagToUse != nil {
			str := os.Getenv("HEADLESS_IMAGE_NAME") + ":" + *tagToUse
			newImage = &str
		}
		newId, err := hhuc.hhrepo.Restart(ctx, host, newImage)
		if err != nil {
			return "", err
		}

		return newId, nil
	}
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetLogs(ctx context.Context, id, until, since string, limit int32) (port.LogLineList, error) {
	return hhuc.hhrepo.GetLogs(ctx, id, limit, until, since)
}

func (hhuc *HeadlessHostUsecase) HeadlessHostShutdown(ctx context.Context, id string) error {
	conn, err := hhuc.hhrepo.GetRpcClient(ctx, id)
	if err != nil {
		return err
	}
	status := entity.SessionStatus_RUNNING
	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &id,
		Status: &status,
	})
	if err != nil {
		return err
	}

	_, err = conn.Shutdown(ctx, &headlessv1.ShutdownRequest{})
	if err != nil {
		return err
	}

	err = hhuc.markSessionsAsEnded(ctx, sessions)
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
	filteredTags := make(port.ContainerImageList, 0, len(localTags))
	for _, tag := range localTags {
		if !tag.IsPreRelease {
			filteredTags = append(filteredTags, tag)
		}
	}

	var latestLocalTag *string
	if len(filteredTags) > 0 {
		latestLocalTag = &filteredTags[len(filteredTags)-1].Tag
	}
	remoteTags, err := hhuc.hhrepo.ListContainerTags(ctx, latestLocalTag)
	if err != nil {
		return "", err
	}
	if latestLocalTag == nil && len(remoteTags) == 0 {
		return "", errors.New("no available container image tags")
	}
	filteredRemoteTags := make(port.ContainerImageList, 0, len(remoteTags))
	for _, tag := range remoteTags {
		if !tag.IsPreRelease {
			filteredRemoteTags = append(filteredRemoteTags, tag)
		}
	}
	if len(filteredRemoteTags) == 0 {
		return "Already up to date", nil
	}

	logs, err := hhuc.hhrepo.PullContainerImage(ctx, filteredRemoteTags[len(filteredRemoteTags)-1].Tag)
	if err != nil {
		return "", err
	}

	return logs, nil
}
