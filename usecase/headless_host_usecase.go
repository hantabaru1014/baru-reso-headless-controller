package usecase

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

const (
	// 起動したホストが RPC に応答できる (engine 初期化 + ログイン完了) までの待ち時間の上限.
	defaultHostReadyTimeout      = 3 * time.Minute
	defaultHostReadyPollInterval = 3 * time.Second
	// ready 判定の RPC 1 回あたりの上限. 起動途中のコンテナは応答が返らないことがある.
	hostReadyProbeTimeout = 5 * time.Second
	// HeadlessHostEnsureRunning が Restart に渡す停止猶予. 対象は停止済みなので通常は
	// 使われないが、判定後に他経路で起動されていた場合に即 kill しないための値.
	ensureRunningStopTimeoutSeconds = 10 * 60
)

type HeadlessHostUsecase struct {
	hhrepo port.HeadlessHostRepository
	srepo  port.SessionRepository
	huc    *SessionUsecase
	hauc   *HeadlessAccountUsecase
	permUC *PermissionUsecase
	rvuc   *ResoniteVersionUsecase

	hostReadyTimeout      time.Duration
	hostReadyPollInterval time.Duration

	// startLocks は HeadlessHostEnsureRunning をホスト単位で直列化する (値は容量 1 の semaphore).
	startLocksMu sync.Mutex
	startLocks   map[string]chan struct{}
}

func NewHeadlessHostUsecase(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, huc *SessionUsecase, hauc *HeadlessAccountUsecase, permUC *PermissionUsecase, rvuc *ResoniteVersionUsecase) *HeadlessHostUsecase {
	return &HeadlessHostUsecase{
		hhrepo:                hhrepo,
		srepo:                 srepo,
		huc:                   huc,
		hauc:                  hauc,
		permUC:                permUC,
		rvuc:                  rvuc,
		hostReadyTimeout:      defaultHostReadyTimeout,
		hostReadyPollInterval: defaultHostReadyPollInterval,
		startLocks:            make(map[string]chan struct{}),
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

	account, err := hhuc.hauc.GetHeadlessAccount(ctx, host.GroupID, host.AccountId)
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

	// repo.Restart は memo と自動アップデート設定を渡された値で上書きするので、
	// 現在の値を引き継ぐ.
	startupConfig := port.HeadlessHostStartParams{
		Name:              host.Name,
		ContainerImageTag: tagToUse,
		StartupConfig:     converter.HeadlessHostSettingsToStartupConfigProto(&host.HostSettings),
		HeadlessAccount:   *account,
		AutoUpdatePolicy:  host.AutoUpdatePolicy,
		Memo:              host.Memo,
	}

	err = hhuc.hhrepo.Restart(ctx, host.ID, startupConfig, timeoutSeconds)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

// HeadlessHostEnsureRunning は停止中 (EXITED / CRASHED) のホストを起動し、RPC を
// 受けられる状態になるまで待つ. 起動した場合は true を返す.
// 停止中以外 (RUNNING や遷移中) のホストには何もしない.
// 起動は HeadlessHostRestart と同じ扱い (最新イメージ + 停止時の world を復元) で、
// host:write を要求する. イメージが未 built なら *NotBuiltError を返す.
func (hhuc *HeadlessHostUsecase) HeadlessHostEnsureRunning(ctx context.Context, id string) (bool, error) {
	// 同じホストを指定した呼び出しが並行しても起動は 1 回にする. 後続は先行の
	// ready 待ちが終わってから RUNNING を観測して素通りする (プロセス内の best-effort).
	unlock, err := hhuc.lockHostStart(ctx, id)
	if err != nil {
		return false, err
	}
	defer unlock()

	status, err := hhuc.hhrepo.GetStatus(ctx, id)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if !status.IsStopped() {
		return false, nil
	}

	slog.Info("starting stopped host on demand", "hostID", id)

	if err := hhuc.HeadlessHostRestart(ctx, id, nil, true, ensureRunningStopTimeoutSeconds); err != nil {
		return false, err
	}

	if err := hhuc.waitUntilReady(ctx, id); err != nil {
		return false, err
	}

	return true, nil
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
	if err := hhuc.requireHostRead(ctx, params.HostID); err != nil {
		return nil, err
	}

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

	switch {
	case params.BeforeID > 0:
		// before_id 指定 = 古いログを取得中 → hasMore は「さらに古いログがある」
		result.HasMoreBefore = hasMore
	case params.AfterID > 0:
		// after_id 指定 = 新しいログを取得中 → hasMore は「さらに新しいログがある」
		result.HasMoreAfter = hasMore
	default:
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

func (hhuc *HeadlessHostUsecase) lockHostStart(ctx context.Context, id string) (func(), error) {
	hhuc.startLocksMu.Lock()

	lock, ok := hhuc.startLocks[id]
	if !ok {
		lock = make(chan struct{}, 1)
		hhuc.startLocks[id] = lock
	}

	hhuc.startLocksMu.Unlock()

	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), 0)
	}
}

// waitUntilReady は起動直後のホストが RPC を受けられるようになるまで待つ.
// コンテナは engine 初期化やログインより先に gRPC を listen し始めるので、
// ログイン完了後にしか成功しない GetAccountInfo が通ることを ready の目印にする.
func (hhuc *HeadlessHostUsecase) waitUntilReady(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, hhuc.hostReadyTimeout)
	defer cancel()

	ticker := time.NewTicker(hhuc.hostReadyPollInterval)
	defer ticker.Stop()

	var client headlessv1.HeadlessControlServiceClient

	for {
		// 起動に失敗してコンテナが落ちた場合は docker event watcher が status を
		// 書き換えるので、timeout を待たずに諦める.
		status, err := hhuc.hhrepo.GetStatus(ctx, id)
		if err == nil && status.IsStopped() {
			return errors.New("host exited before it became ready")
		}

		// 毎回取り直すと接続が増え続けるので、取得できた client を使い回す.
		if client == nil {
			client, _ = hhuc.hhrepo.GetRpcClient(ctx, id)
		}

		if client != nil && hhuc.probeReady(ctx, client) {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.WrapPrefix(ctx.Err(), "host did not become ready", 0)
		case <-ticker.C:
		}
	}
}

func (hhuc *HeadlessHostUsecase) probeReady(ctx context.Context, client headlessv1.HeadlessControlServiceClient) bool {
	ctx, cancel := context.WithTimeout(ctx, hostReadyProbeTimeout)
	defer cancel()

	_, err := client.GetAccountInfo(ctx, &headlessv1.GetAccountInfoRequest{})

	return err == nil
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
