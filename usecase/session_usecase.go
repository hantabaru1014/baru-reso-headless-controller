package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/proto"
)

type SaveMode int32

const (
	SaveMode_OVERWRITE SaveMode = 1
	SaveMode_SAVE_AS   SaveMode = 2
	SaveMode_COPY      SaveMode = 3
)

type SessionUsecase struct {
	sessionRepo     port.SessionRepository
	hostRepo        port.HeadlessHostRepository
	hostDrainer     port.HostDrainer
	stateCache      port.SessionStateCache
	permUC          *PermissionUsecase
	forcePortMin    int
	forcePortMax    int
	resoniteLinkTTL time.Duration
	portMutex       sync.Mutex
}

func NewSessionUsecase(
	sessionRepo port.SessionRepository,
	hostRepo port.HeadlessHostRepository,
	hostDrainer port.HostDrainer,
	stateCache port.SessionStateCache,
	serverCfg *config.ServerConfig,
	linkCfg *config.ResoniteLinkConfig,
	permUC *PermissionUsecase,
) *SessionUsecase {
	return &SessionUsecase{
		sessionRepo:     sessionRepo,
		hostRepo:        hostRepo,
		hostDrainer:     hostDrainer,
		stateCache:      stateCache,
		permUC:          permUC,
		forcePortMin:    serverCfg.SessionPortMin,
		forcePortMax:    serverCfg.SessionPortMax,
		resoniteLinkTTL: linkCfg.TokenTTL,
	}
}

// ErrHostDraining is returned by StartSession when the target host is
// being drained for an in-flight auto-upgrade and therefore must not
// accept new sessions.
var ErrHostDraining = errors.New("host is draining for upgrade")

// Compile-time assertion that SessionUsecase satisfies port.SessionStopper —
// the upgrade orchestrator calls into us through this narrow interface.
var _ port.SessionStopper = (*SessionUsecase)(nil)

// IssueResoniteLinkToken は ResoniteLink WebSocket 接続用の短期 JWT を発行する.
// セッションが存在することを確認した上で、claims に session_id と userID を含める.
// TODO: owner-only enforcement - 現在は認証済みなら誰でも発行可能.
func (u *SessionUsecase) IssueResoniteLinkToken(ctx context.Context, sessionID, userID string) (string, time.Time, error) {
	if _, err := u.sessionRepo.Get(ctx, sessionID); err != nil {
		return "", time.Time{}, errors.Wrap(err, 0)
	}

	return auth.GenerateResoniteLinkToken(userID, sessionID, u.resoniteLinkTTL)
}

func (u *SessionUsecase) StartSession(ctx context.Context, hostId string, groupID string, userId *string, params *headlessv1.WorldStartupParameters, memo *string) (*entity.Session, error) {
	if u.hostDrainer.IsHostDraining(hostId) {
		return nil, ErrHostDraining
	}

	// host:use + account:use + session:write を host の group に対して要求する.
	// 同一グループ制約: session.group_id == host.group_id (account.group_id は
	// StartHeadlessHost 時点で host.group_id に揃えてあるので透過的に満たされる).
	hostGroupID, err := u.hostRepo.GetGroupID(ctx, hostId)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// 引数 groupID が空ならホストの group に揃える (FK 違反防止).
	// 非空かつ不一致なら明示的に拒否する.
	if groupID == "" {
		groupID = hostGroupID
	} else if groupID != hostGroupID {
		return nil, errors.New("session group must equal host group")
	}

	if err := u.permUC.RequireAllPermissionsForGroup(ctx, hostGroupID, []string{
		entity.PermKey_HostUse,
		entity.PermKey_AccountUse,
		entity.PermKey_SessionWrite,
	}); err != nil {
		return nil, err
	}

	client, err := u.hostRepo.GetRpcClient(ctx, hostId)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// forcePortが指定されていない場合、環境変数が設定されていれば自動割り当て
	paramsForContainer := params

	if params.GetForcePort() == 0 {
		autoPort, err := u.getFreeSessionPort(ctx)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		if autoPort != 0 {
			// コンテナに渡すパラメータのコピーを作成してforcePortを設定
			cloned, ok := proto.Clone(params).(*headlessv1.WorldStartupParameters)
			if !ok {
				return nil, errors.New("failed to clone WorldStartupParameters")
			}

			paramsForContainer = cloned
			paramsForContainer.ForcePort = uint32(autoPort)
			slog.Info("Auto-assigned forcePort", "port", autoPort)
		}
	}

	resp, err := client.StartWorld(ctx, &headlessv1.StartWorldRequest{
		Parameters: paramsForContainer,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	startedAt := resp.GetOpenedSession().GetStartedAt().AsTime()
	openedSession := resp.GetOpenedSession()

	session := &entity.Session{
		ID:                openedSession.GetId(),
		Name:              openedSession.GetName(),
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostId,
		StartedAt:         &startedAt,
		CreatedBy:         userId,
		StartupParameters: params,
		CurrentState:      openedSession,
		GroupID:           groupID,
	}
	if memo != nil {
		session.Memo = *memo
	}

	if err := u.sessionRepo.Upsert(ctx, session); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// Upsert 後に cache.Set: 反転すると DB upsert 失敗時に cache に orphan が
	// 残って、SessionEnded event も来ないので controller 再起動まで漏れたままになる。
	u.stateCache.Set(hostId, openedSession.GetId(), openedSession)

	return session, nil
}

func (u *SessionUsecase) StopSession(ctx context.Context, id string) error {
	s, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, s.GroupID, entity.PermKey_SessionWrite); err != nil {
		return err
	}

	client, err := u.hostRepo.GetRpcClient(ctx, s.HostID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	// CurrentState は cache が権威。WorldSaved / SessionParametersChanged event で
	// 随時更新されているので、最後に観測した worldUrl があれば次回起動時に使う。
	if snapshot, ok := u.stateCache.Get(id); ok {
		if worldUrl := snapshot.GetWorldUrl(); worldUrl != "" && s.StartupParameters != nil {
			s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
				LoadWorldUrl: worldUrl,
			}
		}
	}

	_, err = client.StopSession(ctx, &headlessv1.StopSessionRequest{SessionId: id})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	now := time.Now()
	s.EndedAt = &now
	s.Status = entity.SessionStatus_ENDED

	if err := u.sessionRepo.Upsert(ctx, s); err != nil {
		return errors.Wrap(err, 0)
	}

	// Upsert 後に cache.Delete: 反転すると Upsert 失敗時に cache だけ消えて
	// 次の GetSession で container 問い合わせ → CRASHED 降格、の連鎖になりうる。
	u.stateCache.Delete(id)

	return nil
}

func (u *SessionUsecase) GetSession(ctx context.Context, id string) (*entity.Session, error) {
	dbSession, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// ENDED は終端なので container 問い合わせ不要。それ以外 (STARTING / RUNNING /
	// CRASHED / UNKNOWN) は cache 経路に乗せる: transient な RPC 失敗で CRASHED に
	// 落ちていた session も、container が応答するようになれば自動的に RUNNING に
	// 戻る (= self-healing)。
	if dbSession.Status == entity.SessionStatus_ENDED {
		return dbSession, nil
	}

	if snapshot, ok := u.stateCache.Get(id); ok {
		dbSession.CurrentState = snapshot
		// cache hit = event が届いている = container は session を抱えている。
		// CRASHED / UNKNOWN に降格していたら RUNNING に戻す。
		if dbSession.Status != entity.SessionStatus_RUNNING {
			_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_RUNNING)
			dbSession.Status = entity.SessionStatus_RUNNING
		}

		return dbSession, nil
	}

	// cache miss: controller 再起動直後 / event 未到達 / 過去に CRASHED 降格した
	// session の復旧試行 など。container から取り直す。
	client, clientErr := u.hostRepo.GetRpcClient(ctx, dbSession.HostID)
	if clientErr != nil {
		if dbSession.Status != entity.SessionStatus_CRASHED {
			_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
			dbSession.Status = entity.SessionStatus_CRASHED
		}

		return dbSession, nil
	}

	resp, rpcErr := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if rpcErr != nil || resp.GetSession() == nil {
		if dbSession.Status != entity.SessionStatus_CRASHED {
			_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
			dbSession.Status = entity.SessionStatus_CRASHED
		}

		return dbSession, nil
	}

	u.stateCache.Set(dbSession.HostID, id, resp.GetSession())
	dbSession.CurrentState = resp.GetSession()

	// container が応答した = session は生きている。CRASHED/UNKNOWN だったら戻す。
	if dbSession.Status != entity.SessionStatus_RUNNING {
		_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_RUNNING)
		dbSession.Status = entity.SessionStatus_RUNNING
	}

	return dbSession, nil
}

type SearchSessionsFilter struct {
	HostID *string
	Status *entity.SessionStatus
	// PageSize == 0 はページング無効 (全件取得) として扱う。RPC handler は常に >0 で渡し、
	// 内部呼び出し (HeadlessHostRestart/Shutdown/Kill の markSessionsAsEnded など) は
	// 0 を渡して全件取得する。
	PageIndex int32
	PageSize  int32
	// GroupIDs はグループフィルタ. semantics は port.SessionListPageOptions.GroupIDs と同一.
	// nil = 全件, 空 slice = 0 件, 非空 = ANY 絞り込み.
	// ページング無効 (PageSize == 0) 経路でも post-filter として作用する.
	GroupIDs []string
}

type SearchSessionsResult struct {
	Sessions   entity.SessionList
	TotalCount int32
}

func (u *SessionUsecase) SearchSessions(ctx context.Context, filter SearchSessionsFilter) (*SearchSessionsResult, error) {
	// CurrentState は cache から hydrate する。cache miss は許容 (list view の
	// 表示劣化のみで、詳細画面の GetSession で取り直される)。
	if filter.PageSize > 0 {
		pageResult, err := u.sessionRepo.ListPaged(ctx, port.SessionListPageOptions{
			HostID:    filter.HostID,
			Status:    filter.Status,
			PageIndex: filter.PageIndex,
			PageSize:  filter.PageSize,
			GroupIDs:  filter.GroupIDs,
		})
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		u.hydrateCurrentState(pageResult.Sessions)

		return &SearchSessionsResult{Sessions: pageResult.Sessions, TotalCount: pageResult.TotalCount}, nil
	}

	var dbSessions entity.SessionList

	if filter.Status != nil {
		s, err := u.sessionRepo.ListByStatus(ctx, *filter.Status)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		dbSessions = s
	} else {
		s, err := u.sessionRepo.ListAll(ctx)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		dbSessions = s
	}

	if filter.HostID != nil {
		filtered := make(entity.SessionList, 0, len(dbSessions))

		for _, s := range dbSessions {
			if s.HostID == *filter.HostID {
				filtered = append(filtered, s)
			}
		}

		dbSessions = filtered
	}

	// GroupIDs フィルタは ListAll/ListByStatus 経路では SQL に組み込まれていないため、
	// メモリ上で post-filter する. ページング経路と同一の semantics.
	//   nil → no-op (全件)
	//   非 nil → set 包含チェック (空 set も含む)
	if filter.GroupIDs != nil {
		allow := make(map[string]struct{}, len(filter.GroupIDs))
		for _, gid := range filter.GroupIDs {
			allow[gid] = struct{}{}
		}

		filtered := make(entity.SessionList, 0, len(dbSessions))

		for _, s := range dbSessions {
			if _, ok := allow[s.GroupID]; ok {
				filtered = append(filtered, s)
			}
		}

		dbSessions = filtered
	}

	u.hydrateCurrentState(dbSessions)

	return &SearchSessionsResult{
		Sessions:   dbSessions,
		TotalCount: int32(len(dbSessions)), //nolint:gosec // G115: セッション件数は int32 範囲を超えない
	}, nil
}

func updateStartupParamsByUpdateRequest(
	current *headlessv1.WorldStartupParameters,
	params *headlessv1.UpdateSessionParametersRequest,
) {
	if params.Name != nil {
		current.Name = params.Name
	}

	if params.Description != nil {
		current.Description = params.Description
	}

	if params.MaxUsers != nil {
		current.MaxUsers = params.MaxUsers
	}

	if params.AccessLevel != nil {
		current.AccessLevel = params.GetAccessLevel()
	}

	if params.AwayKickMinutes != nil {
		current.AwayKickMinutes = params.GetAwayKickMinutes()
	}

	if params.IdleRestartIntervalSeconds != nil {
		current.IdleRestartIntervalSeconds = params.GetIdleRestartIntervalSeconds()
	}

	if params.SaveOnExit != nil {
		current.SaveOnExit = params.GetSaveOnExit()
	}

	if params.AutoSaveIntervalSeconds != nil {
		current.AutoSaveIntervalSeconds = params.GetAutoSaveIntervalSeconds()
	}

	if params.AutoSleep != nil {
		current.AutoSleep = params.GetAutoSleep()
	}

	if params.HideFromPublicListing != nil {
		current.HideFromPublicListing = params.GetHideFromPublicListing()
	}

	if params.GetUpdateTags() {
		current.Tags = params.GetTags()
	}
}

func (u *SessionUsecase) UpdateSessionParameters(ctx context.Context, id string, params *headlessv1.UpdateSessionParametersRequest) error {
	s, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, s.GroupID, entity.PermKey_SessionWrite); err != nil {
		return err
	}

	client, err := u.hostRepo.GetRpcClient(ctx, s.HostID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	_, err = client.UpdateSessionParameters(ctx, params)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	newSession, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	updateStartupParamsByUpdateRequest(s.StartupParameters, params)
	s.Name = newSession.GetSession().GetName()

	if err := u.sessionRepo.Upsert(ctx, s); err != nil {
		return errors.Wrap(err, 0)
	}

	// Upsert 成功後に cache を最新 snapshot で更新。SessionParametersChanged event
	// 経由でも同じ snapshot が流れてくるが (どちらが先でも最終状態は convergent)、
	// handler の到達を待たずに即時に最新 state を見せたい。
	if newSession.GetSession() != nil {
		u.stateCache.Set(s.HostID, id, newSession.GetSession())
	}

	return nil
}

func (u *SessionUsecase) DeleteSession(ctx context.Context, id string) error {
	s, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, s.GroupID, entity.PermKey_SessionWrite); err != nil {
		return err
	}

	if err := u.sessionRepo.Delete(ctx, id); err != nil {
		return err
	}

	u.stateCache.Delete(id)

	return nil
}

func (u *SessionUsecase) SaveSessionWorld(ctx context.Context, id string, saveMode SaveMode) (string, error) {
	s, err := u.GetSession(ctx, id)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, s.GroupID, entity.PermKey_SessionWrite); err != nil {
		return "", err
	}

	client, err := u.hostRepo.GetRpcClient(ctx, s.HostID)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	switch saveMode {
	case SaveMode_OVERWRITE:
		// preset 由来の初回 save では record が新規発番されるので、保存直後の
		// URL は response から同期的に取る。
		resp, err := client.SaveSessionWorld(ctx, &headlessv1.SaveSessionWorldRequest{SessionId: id})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		if url := resp.GetSavedWorldUrl(); url != "" {
			return url, nil
		}

		// saved_world_url を埋めない container との互換用 fallback (cache から取る)
		if snapshot, ok := u.stateCache.Get(id); ok {
			return snapshot.GetWorldUrl(), nil
		}

		return "", nil

	case SaveMode_SAVE_AS:
		saveAsResp, err := client.SaveAsSessionWorld(ctx, &headlessv1.SaveAsSessionWorldRequest{
			SessionId: id,
			Type:      headlessv1.SaveAsSessionWorldRequest_SAVE_AS_TYPE_SAVE_AS,
		})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return saveAsResp.GetSavedRecordUrl(), nil

	case SaveMode_COPY:
		saveAsResp, err := client.SaveAsSessionWorld(ctx, &headlessv1.SaveAsSessionWorldRequest{
			SessionId: id,
			Type:      headlessv1.SaveAsSessionWorldRequest_SAVE_AS_TYPE_COPY,
		})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return saveAsResp.GetSavedRecordUrl(), nil

	default:
		return "", errors.Errorf("unknown save mode: %d", saveMode)
	}
}

// UpdateSessionExtraSettings は memo / auto_upgrade を更新する.
// 内部的に GetSession (cache hydrate 込み) → Upsert で書き戻す.
func (u *SessionUsecase) UpdateSessionExtraSettings(ctx context.Context, sessionID string, autoUpgrade *bool, memo *string) error {
	s, err := u.GetSession(ctx, sessionID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, s.GroupID, entity.PermKey_SessionWrite); err != nil {
		return err
	}

	if autoUpgrade != nil {
		s.AutoUpgrade = *autoUpgrade
	}

	if memo != nil {
		s.Memo = *memo
	}

	if err := u.sessionRepo.Upsert(ctx, s); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (u *SessionUsecase) hydrateCurrentState(sessions entity.SessionList) {
	for _, s := range sessions {
		if snapshot, ok := u.stateCache.Get(s.ID); ok {
			s.CurrentState = snapshot
		}
	}
}

// getFreeSessionPort は環境変数で指定されたポート範囲から空きポートを探して返す
// 環境変数が設定されていない場合は0を返す.
func (u *SessionUsecase) getFreeSessionPort(ctx context.Context) (int, error) {
	if u.forcePortMin == 0 && u.forcePortMax == 0 {
		return 0, nil
	}

	u.portMutex.Lock()
	defer u.portMutex.Unlock()

	// ランダムな開始位置から探索（同じポートに偏らないように）
	offset := time.Now().UnixNano() % int64(u.forcePortMax-u.forcePortMin+1)
	for i := 0; i <= u.forcePortMax-u.forcePortMin; i++ {
		candidatePort := u.forcePortMin + int((offset+int64(i))%int64(u.forcePortMax-u.forcePortMin+1))
		if isPortAvailable(ctx, candidatePort) {
			return candidatePort, nil
		}
	}

	return 0, errors.Errorf("no free port found in range %d-%d", u.forcePortMin, u.forcePortMax)
}

func isPortAvailable(ctx context.Context, port int) bool {
	address := fmt.Sprintf(":%d", port)

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", address)
	if err != nil {
		return false
	}

	if err := listener.Close(); err != nil {
		slog.Warn("failed to close listener when checking port availability", "port", port, "error", err)
	}

	return true
}
