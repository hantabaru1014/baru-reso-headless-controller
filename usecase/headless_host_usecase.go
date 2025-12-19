package usecase

import (
	"context"
	"slices"
	"time"

	"github.com/go-errors/errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
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
	tag, err := hhuc.resolveTagToUse(ctx, &params.ContainerImageTag)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	params.ContainerImageTag = tag
	return hhuc.hhrepo.Start(ctx, port.HostConnectorType_DOCKER, params, userId)
}

func (hhuc *HeadlessHostUsecase) HeadlessHostList(ctx context.Context) (entity.HeadlessHostList, error) {
	hosts, err := hhuc.hhrepo.ListAll(ctx, port.HeadlessHostFetchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return hosts, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGet(ctx context.Context, id string) (*entity.HeadlessHost, error) {
	host, err := hhuc.hhrepo.Find(ctx, id, port.HeadlessHostFetchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return host, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostDelete(ctx context.Context, id string) error {
	return hhuc.hhrepo.Delete(ctx, id)
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
	host, err := hhuc.hhrepo.Find(ctx, id, port.HeadlessHostFetchOptions{
		IncludeStartWorlds: withWorldRestart,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	tagToUse, err := hhuc.resolveTagToUse(ctx, newTag)
	if err != nil {
		return errors.Wrap(err, 0)
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
		StartupConfig:     converter.HeadlessHostSettingsToStartupConfigProto(&host.HostSettings),
		HeadlessAccount:   *account,
	}
	err = hhuc.hhrepo.Restart(ctx, host.ID, startupConfig, timeoutSeconds)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

type HeadlessHostGetLogsParams struct {
	HostID     string
	InstanceID int32
	Limit      int32
	BeforeID   int64 // このIDより小さいログ (古い方向へのページネーション)
	AfterID    int64 // このIDより大きいログ (新しい方向へのページネーション)
}

type HeadlessHostGetLogsResult struct {
	Logs          port.LogLineList
	HasMoreBefore bool
	HasMoreAfter  bool
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetLogs(ctx context.Context, params HeadlessHostGetLogsParams) (*HeadlessHostGetLogsResult, error) {
	// limit+1 件取得して has_more を判定
	fetchLimit := params.Limit
	if fetchLimit <= 0 {
		fetchLimit = 100
	}
	fetchLimit++ // 1件多く取得して has_more 判定

	logs, err := hhuc.hhrepo.GetLogs(ctx, port.GetLogsParams{
		HostID:     params.HostID,
		InstanceID: params.InstanceID,
		Limit:      fetchLimit,
		BeforeID:   params.BeforeID,
		AfterID:    params.AfterID,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	hasMore := len(logs) >= int(fetchLimit)
	if hasMore {
		logs = logs[:len(logs)-1] // 余分な1件を削除
	}

	// before_id 指定時は has_more_before、after_id 指定時は has_more_after
	result := &HeadlessHostGetLogsResult{
		Logs:          logs,
		HasMoreBefore: false,
		HasMoreAfter:  false,
	}

	if params.BeforeID > 0 {
		// before_id 指定 = 古いログを取得中 → hasMore は「さらに古いログがある」
		result.HasMoreBefore = hasMore
	} else if params.AfterID > 0 {
		// after_id 指定 = 新しいログを取得中 → hasMore は「さらに新しいログがある」
		result.HasMoreAfter = hasMore
	} else {
		// カーソルなし = 最新から取得 → hasMore は「古いログがある」
		result.HasMoreBefore = hasMore
	}

	return result, nil
}

type HeadlessHostInstance struct {
	InstanceID int32
	FirstLogAt *int64 // UnixTime (秒), nil = データなし
	LastLogAt  *int64 // UnixTime (秒), nil = データなし
	LogCount   int64
	IsCurrent  bool
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetInstances(ctx context.Context, hostID string) ([]*HeadlessHostInstance, error) {
	// ホストを取得して現在のinstance_idを確認
	host, err := hhuc.hhrepo.Find(ctx, hostID, port.HeadlessHostFetchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// リポジトリからインスタンスタイムスタンプを取得
	timestamps, err := hhuc.hhrepo.GetInstanceTimestamps(ctx, hostID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// 現在のインスタンスかどうかを判定
	result := make([]*HeadlessHostInstance, 0, len(timestamps))
	for _, ts := range timestamps {
		result = append(result, &HeadlessHostInstance{
			InstanceID: ts.InstanceID,
			FirstLogAt: ts.FirstLogAt,
			LastLogAt:  ts.LastLogAt,
			LogCount:   ts.LogCount,
			IsCurrent:  ts.InstanceID == host.InstanceId,
		})
	}
	return result, nil
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

func (hhuc *HeadlessHostUsecase) HeadlessHostKill(ctx context.Context, id string) error {
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

	err = hhuc.hhrepo.Kill(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (hhuc *HeadlessHostUsecase) resolveTagToUse(ctx context.Context, tagInput *string) (string, error) {
	if tagInput == nil || *tagInput == "" || *tagInput == "latestRelease" || *tagInput == "latestPreRelease" {
		tags, err := hhuc.hhrepo.ListContainerTags(ctx, nil)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		if len(tags) == 0 {
			return "", errors.New("no available container image tags")
		}
		wantPreRelease := tagInput != nil && *tagInput == "latestPreRelease"
		for _, tag := range slices.Backward(tags) {
			if tag.IsPreRelease == wantPreRelease {
				return tag.Tag, nil
			}
		}
		return "", errors.New("no available container image tags")
	}
	return *tagInput, nil
}
