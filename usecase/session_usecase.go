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
	stateCache      port.SessionStateCache
	forcePortMin    int
	forcePortMax    int
	resoniteLinkTTL time.Duration
	portMutex       sync.Mutex
}

func NewSessionUsecase(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository, stateCache port.SessionStateCache, serverCfg *config.ServerConfig, linkCfg *config.ResoniteLinkConfig) *SessionUsecase {
	return &SessionUsecase{
		sessionRepo:     sessionRepo,
		hostRepo:        hostRepo,
		stateCache:      stateCache,
		forcePortMin:    serverCfg.SessionPortMin,
		forcePortMax:    serverCfg.SessionPortMax,
		resoniteLinkTTL: linkCfg.TokenTTL,
	}
}

// IssueResoniteLinkToken は ResoniteLink WebSocket 接続用の短期 JWT を発行する.
// セッションが存在することを確認した上で、claims に session_id と userID を含める.
// TODO: owner-only enforcement - 現在は認証済みなら誰でも発行可能.
func (u *SessionUsecase) IssueResoniteLinkToken(ctx context.Context, sessionID, userID string) (string, time.Time, error) {
	if _, err := u.sessionRepo.Get(ctx, sessionID); err != nil {
		return "", time.Time{}, errors.Wrap(err, 0)
	}

	return auth.GenerateResoniteLinkToken(userID, sessionID, u.resoniteLinkTTL)
}

func (u *SessionUsecase) StartSession(ctx context.Context, hostId string, userId *string, params *headlessv1.WorldStartupParameters, memo *string) (*entity.Session, error) {
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

	// 起動直後 SessionParametersChanged event より先に GetSession 等が来ても
	// CurrentState を返せるよう、cache に最初の snapshot を入れておく。
	u.stateCache.Set(openedSession.GetId(), openedSession)

	session := &entity.Session{
		ID:                openedSession.GetId(),
		Name:              openedSession.GetName(),
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostId,
		StartedAt:         &startedAt,
		OwnerID:           userId,
		StartupParameters: params,
		CurrentState:      openedSession,
	}
	if memo != nil {
		session.Memo = *memo
	}

	err = u.sessionRepo.Upsert(ctx, session)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return session, nil
}

func (u *SessionUsecase) StopSession(ctx context.Context, id string) error {
	s, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, 0)
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

	u.stateCache.Delete(id)

	return u.sessionRepo.Upsert(ctx, s)
}

func (u *SessionUsecase) GetSession(ctx context.Context, id string) (*entity.Session, error) {
	dbSession, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if dbSession.Status != entity.SessionStatus_STARTING && dbSession.Status != entity.SessionStatus_RUNNING {
		return dbSession, nil
	}

	// cache hit ならそのまま返す。
	if snapshot, ok := u.stateCache.Get(id); ok {
		dbSession.CurrentState = snapshot

		return dbSession, nil
	}

	// cache miss は controller 再起動直後や、まだ event が届いていないケース。
	// container から取り直し cache を populate する。RPC が引けない / 失敗したら
	// CRASHED に降ろして DB session を返す (UI 側は CurrentState=nil でも optional
	// chain で扱える)。
	client, clientErr := u.hostRepo.GetRpcClient(ctx, dbSession.HostID)
	if clientErr != nil {
		_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
		dbSession.Status = entity.SessionStatus_CRASHED

		return dbSession, nil //nolint:nilerr // CRASHED 降格として吸収する設計
	}

	resp, rpcErr := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if rpcErr != nil || resp.GetSession() == nil {
		_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
		dbSession.Status = entity.SessionStatus_CRASHED

		return dbSession, nil //nolint:nilerr // CRASHED 降格として吸収する設計
	}

	u.stateCache.Set(id, resp.GetSession())
	dbSession.CurrentState = resp.GetSession()

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

	// container の最新 snapshot で cache を更新しておく。SessionParametersChanged
	// event 経由でも同じ snapshot が流れてくるが、handler の到達を待たずに
	// 即時に最新 state を見せる。
	if newSession.GetSession() != nil {
		u.stateCache.Set(id, newSession.GetSession())
	}

	s.Name = newSession.GetSession().GetName()

	return u.sessionRepo.Upsert(ctx, s)
}

func (u *SessionUsecase) DeleteSession(ctx context.Context, id string) error {
	u.stateCache.Delete(id)

	return u.sessionRepo.Delete(ctx, id)
}

func (u *SessionUsecase) SaveSessionWorld(ctx context.Context, id string, saveMode SaveMode) (string, error) {
	s, err := u.GetSession(ctx, id)
	if err != nil {
		return "", errors.Wrap(err, 0)
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
