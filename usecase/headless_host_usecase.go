package usecase

import (
	"context"
	"slices"
	"time"

	"github.com/go-errors/errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type HeadlessHostUsecase struct {
	hhrepo port.HeadlessHostRepository
	srepo  port.SessionRepository
	huc    *SessionUsecase
	hauc   *HeadlessAccountUsecase
}

func NewHeadlessHostUsecase(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, huc *SessionUsecase, hauc *HeadlessAccountUsecase) *HeadlessHostUsecase {
	return &HeadlessHostUsecase{
		hhrepo: hhrepo,
		srepo:  srepo,
		huc:    huc,
		hauc:   hauc,
	}
}

func (hhuc *HeadlessHostUsecase) HeadlessHostStart(ctx context.Context, params port.HeadlessHostStartParams, userId *string) (string, error) {
	return hhuc.hhrepo.Start(ctx, port.HostConnectorType_DOCKER, params, userId)
}

func (hhuc *HeadlessHostUsecase) HeadlessHostList(ctx context.Context) (entity.HeadlessHostList, error) {
	hosts, err := hhuc.hhrepo.ListAll(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return hosts, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGet(ctx context.Context, id string) (*entity.HeadlessHost, error) {
	host, err := hhuc.hhrepo.Find(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return host, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetSettings(ctx context.Context, id string) (*entity.HeadlessHostSettings, error) {
	cli, err := hhuc.hhrepo.GetRpcClient(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	resp, err := cli.GetHostSettings(ctx, &headlessv1.GetHostSettingsRequest{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
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
			return errors.Wrap(err, 0)
		}
	}
	return nil
}

// HeadlessHostRestart restarts the headless host with the specified ID.
// If newTag is "latestRelease", it will use the latest release tag.
func (hhuc *HeadlessHostUsecase) HeadlessHostRestart(ctx context.Context, id string, newTag *string, withWorldRestart bool, timeoutSeconds int) error {
	host, err := hhuc.hhrepo.Find(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	tagToUse := ""
	if newTag == nil || *newTag == "" || *newTag == "latestRelease" {
		tags, err := hhuc.hhrepo.ListContainerTags(ctx, nil)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if len(tags) == 0 {
			return errors.New("no available container image tags")
		}
		for _, tag := range slices.Backward(tags) {
			if !tag.IsPreRelease {
				tagToUse = tag.Tag
				break
			}
		}
		if tagToUse == "" {
			return errors.New("no available container image tags")
		}
	} else {
		tagToUse = *newTag
	}

	account, err := hhuc.hauc.GetHeadlessAccount(ctx, host.AccountId)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	status := entity.SessionStatus_RUNNING
	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &host.ID,
		Status: &status,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = hhuc.markSessionsAsEnded(ctx, sessions)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	startupConfig := port.HeadlessHostStartParams{
		Name:              host.Name,
		ContainerImageTag: tagToUse,
		StartupConfig:     host.StartupConfig,
		HeadlessAccount:   *account,
	}
	if !withWorldRestart && startupConfig.StartupConfig != nil {
		startupConfig.StartupConfig.StartWorlds = nil
	}
	err = hhuc.hhrepo.Restart(ctx, host.ID, startupConfig, timeoutSeconds)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetLogs(ctx context.Context, id, until, since string, limit int32) (port.LogLineList, error) {
	return hhuc.hhrepo.GetLogs(ctx, id, limit, until, since)
}

func (hhuc *HeadlessHostUsecase) HeadlessHostShutdown(ctx context.Context, id string) error {
	status := entity.SessionStatus_RUNNING
	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &id,
		Status: &status,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = hhuc.markSessionsAsEnded(ctx, sessions)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	// TODO: さすがにタイムアウト設定すべき？
	err = hhuc.hhrepo.Stop(context.Background(), id, -1)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}
