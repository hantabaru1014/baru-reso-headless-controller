package usecase

import (
	"context"
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
	permUC *PermissionUsecase
	rvuc   *ResoniteVersionUsecase
}

func NewHeadlessHostUsecase(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, huc *SessionUsecase, hauc *HeadlessAccountUsecase, permUC *PermissionUsecase, rvuc *ResoniteVersionUsecase) *HeadlessHostUsecase {
	return &HeadlessHostUsecase{
		hhrepo: hhrepo,
		srepo:  srepo,
		huc:    huc,
		hauc:   hauc,
		permUC: permUC,
		rvuc:   rvuc,
	}
}

// HeadlessHostStart は host:write + account:use を params.GroupID に対して要求する.
// RPC interceptor (checkStartHeadlessHost) と同等のチェックを usecase 層で最終ガードする.
func (hhuc *HeadlessHostUsecase) HeadlessHostStart(ctx context.Context, params port.HeadlessHostStartParams, userId *string) (string, error) {
	if err := hhuc.permUC.RequirePermissionForGroup(ctx, params.GroupID, entity.PermKey_HostWrite); err != nil {
		return "", err
	}

	if err := hhuc.permUC.RequirePermissionForGroup(ctx, params.GroupID, entity.PermKey_AccountUse); err != nil {
		return "", err
	}

	// 同一グループ制約: account.group_id == params.group_id を usecase 側でも検証.
	if params.HeadlessAccount.GroupID != "" && params.HeadlessAccount.GroupID != params.GroupID {
		return "", errors.New("account group does not match host group")
	}

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

// HeadlessHostListPagedOptions は HeadlessHostListPaged の引数.
// GroupIDs の semantics は port.HostListPageOptions.GroupIDs と同一.
type HeadlessHostListPagedOptions struct {
	PageIndex int32
	PageSize  int32
	GroupIDs  []string
}

func (hhuc *HeadlessHostUsecase) HeadlessHostListPaged(ctx context.Context, opts HeadlessHostListPagedOptions) (*port.HostListPageResult, error) {
	result, err := hhuc.hhrepo.ListPaged(ctx, port.HostListPageOptions{
		PageIndex:    opts.PageIndex,
		PageSize:     opts.PageSize,
		FetchOptions: port.HeadlessHostFetchOptions{},
		GroupIDs:     opts.GroupIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return result, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGet(ctx context.Context, id string) (*entity.HeadlessHost, error) {
	if err := hhuc.requireHostRead(ctx, id); err != nil {
		return nil, err
	}

	host, err := hhuc.hhrepo.Find(ctx, id, port.HeadlessHostFetchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return host, nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostDelete(ctx context.Context, id string) error {
	if err := hhuc.requireHostWrite(ctx, id); err != nil {
		return err
	}

	return hhuc.hhrepo.Delete(ctx, id)
}

// HeadlessHostRestart restarts the headless host with the specified ID.
// If newTag is "latestRelease", it will use the latest release tag.
func (hhuc *HeadlessHostUsecase) HeadlessHostRestart(ctx context.Context, id string, newTag *string, withWorldRestart bool, timeoutSeconds int) error {
	if err := hhuc.requireHostWrite(ctx, id); err != nil {
		return err
	}

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

	err = hhuc.markSessionsAsEnded(ctx, sessions.Sessions)
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

// HostLogsCursor はログ取得の起点の指定方法.
type HostLogsCursor int

const (
	HostLogsCursor_LATEST HostLogsCursor = iota // 最新のログから取得 (CursorID は使わない)
	HostLogsCursor_BEFORE                       // CursorID より古いログ (古い方向へのページネーション)
	HostLogsCursor_AFTER                        // CursorID より新しいログ (新しい方向へのページネーション)
	HostLogsCursor_AROUND                       // CursorID のログを中心に前後のログ (検索結果へのジャンプ)
)

type HeadlessHostGetLogsParams struct {
	HostID     string
	InstanceID int32
	Limit      int32
	Cursor     HostLogsCursor
	CursorID   int64
}

type HeadlessHostGetLogsResult struct {
	Logs          port.LogLineList
	HasMoreBefore bool
	HasMoreAfter  bool
}

const (
	defaultLogsLimit = 100
	// maxLogsLimit は 1 リクエストで返すログ件数の上限 (proto の GetHeadlessHostLogsRequest.limit に記載の最大値).
	maxLogsLimit = 1000
)

func (hhuc *HeadlessHostUsecase) HeadlessHostGetLogs(ctx context.Context, params HeadlessHostGetLogsParams) (*HeadlessHostGetLogsResult, error) {
	if err := hhuc.requireHostRead(ctx, params.HostID); err != nil {
		return nil, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultLogsLimit
	}

	limit = min(limit, maxLogsLimit)

	older := port.GetLogsParams{
		HostID:     params.HostID,
		InstanceID: params.InstanceID,
		Limit:      limit,
		CursorID:   port.LatestLogCursorID,
	}
	newer := older
	newer.CursorID = params.CursorID
	newer.Newer = true

	result := &HeadlessHostGetLogsResult{}

	var err error

	switch params.Cursor {
	case HostLogsCursor_AROUND:
		// CursorID 以下を前半、CursorID より大きいログを後半として取得する.
		// limit が奇数の場合は対象ログを含む前半を 1 件多くする (limit = 1 でも対象ログを返す).
		newer.Limit = limit / 2 //nolint:mnd // 前後で半分ずつ
		older.Limit = limit - newer.Limit

		// 「CursorID 以下」=「CursorID + 1 より古い」. オーバーフローする場合は最新からと同義.
		if params.CursorID < port.LatestLogCursorID {
			older.CursorID = params.CursorID + 1
		}

		var after port.LogLineList

		result.Logs, result.HasMoreBefore, err = hhuc.hhrepo.GetLogs(ctx, older)
		if err == nil {
			after, result.HasMoreAfter, err = hhuc.hhrepo.GetLogs(ctx, newer)
			result.Logs = append(result.Logs, after...)
		}
	case HostLogsCursor_AFTER:
		result.Logs, result.HasMoreAfter, err = hhuc.hhrepo.GetLogs(ctx, newer)
	case HostLogsCursor_BEFORE:
		older.CursorID = params.CursorID
		result.Logs, result.HasMoreBefore, err = hhuc.hhrepo.GetLogs(ctx, older)
	case HostLogsCursor_LATEST:
		result.Logs, result.HasMoreBefore, err = hhuc.hhrepo.GetLogs(ctx, older)
	}

	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return result, nil
}

// HeadlessHostSearchLogs は本文が Query を含むログのうち、カーソルから指定方向で最も近い 1 件の ID を返す.
// 見つからない場合は found = false.
func (hhuc *HeadlessHostUsecase) HeadlessHostSearchLogs(ctx context.Context, params port.SearchLogParams) (int64, bool, error) {
	if err := hhuc.requireHostRead(ctx, params.HostID); err != nil {
		return 0, false, err
	}

	logID, found, err := hhuc.hhrepo.SearchLog(ctx, params)
	if err != nil {
		return 0, false, errors.Wrap(err, 0)
	}

	return logID, found, nil
}

type HeadlessHostInstance struct {
	InstanceID int32
	FirstLogAt *int64 // UnixTime (秒), nil = データなし
	LastLogAt  *int64 // UnixTime (秒), nil = データなし
	LogCount   int64
	IsCurrent  bool
}

func (hhuc *HeadlessHostUsecase) HeadlessHostGetInstances(ctx context.Context, hostID string) ([]*HeadlessHostInstance, error) {
	if err := hhuc.requireHostRead(ctx, hostID); err != nil {
		return nil, err
	}

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
	if err := hhuc.requireHostWrite(ctx, id); err != nil {
		return err
	}

	status := entity.SessionStatus_RUNNING

	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &id,
		Status: &status,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = hhuc.markSessionsAsEnded(ctx, sessions.Sessions)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	// TODO: さすがにタイムアウト設定すべき？
	err = hhuc.hhrepo.Stop(ctx, id, -1)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (hhuc *HeadlessHostUsecase) HeadlessHostKill(ctx context.Context, id string) error {
	if err := hhuc.requireHostWrite(ctx, id); err != nil {
		return err
	}

	status := entity.SessionStatus_RUNNING

	sessions, err := hhuc.huc.SearchSessions(ctx, SearchSessionsFilter{
		HostID: &id,
		Status: &status,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = hhuc.markSessionsAsEnded(ctx, sessions.Sessions)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = hhuc.hhrepo.Kill(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

// requireHostWrite は hostID の group を引いて host:write を要求する.
// host が存在しなければ repo 由来のエラー (典型的に domain.ErrNotFound) を返す.
func (hhuc *HeadlessHostUsecase) requireHostWrite(ctx context.Context, hostID string) error {
	groupID, err := hhuc.hhrepo.GetGroupID(ctx, hostID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return hhuc.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_HostWrite)
}

// requireHostRead は hostID の group に対する host:read (または host:write) を要求する.
// 読み取り系 usecase の最終ガード. host:write 保持者は更新 UI 経由で詳細を読む
// 必要があるため read 相当として扱う.
//
// OR 判定は RequireAnyPermissionForGroup に委譲する. 個別に read→write と
// 呼び分けると read チェックの内部エラーを握り潰して write の denial にすり替えて
// しまうため.
func (hhuc *HeadlessHostUsecase) requireHostRead(ctx context.Context, hostID string) error {
	groupID, err := hhuc.hhrepo.GetGroupID(ctx, hostID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return hhuc.permUC.RequireAnyPermissionForGroup(ctx, groupID,
		[]string{entity.PermKey_HostRead, entity.PermKey_HostWrite})
}

func (hhuc *HeadlessHostUsecase) resolveTagToUse(ctx context.Context, tagInput *string) (string, error) {
	input := ""
	if tagInput != nil {
		input = *tagInput
	}

	return hhuc.rvuc.ResolveForStart(ctx, input)
}

func (hhuc *HeadlessHostUsecase) markSessionsAsEnded(ctx context.Context, sessions entity.SessionList) error {
	// FIXME: 仮の実装. session usecaseにまとめられるようにする
	now := time.Now()
	for _, s := range sessions {
		s.EndedAt = &now

		s.Status = entity.SessionStatus_ENDED

		if s.CurrentState != nil && s.CurrentState.GetWorldUrl() != "" {
			s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
				LoadWorldUrl: s.CurrentState.GetWorldUrl(),
			}
		}

		err := hhuc.srepo.Upsert(ctx, s)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return nil
}
