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
	grpcCallTimeout time.Duration
	forcePortMin    int
	forcePortMax    int
	portMutex       sync.Mutex
}

func NewSessionUsecase(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository, grpcCfg *config.GRPCConfig, serverCfg *config.ServerConfig) *SessionUsecase {
	return &SessionUsecase{
		sessionRepo:     sessionRepo,
		hostRepo:        hostRepo,
		grpcCallTimeout: grpcCfg.CallTimeout,
		forcePortMin:    serverCfg.SessionPortMin,
		forcePortMax:    serverCfg.SessionPortMax,
	}
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

	session := &entity.Session{
		ID:                resp.GetOpenedSession().GetId(),
		Name:              resp.GetOpenedSession().GetName(),
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostId,
		StartedAt:         &startedAt,
		OwnerID:           userId,
		StartupParameters: params,
		CurrentState:      resp.GetOpenedSession(),
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

	hdlSession, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if hdlSession.GetSession().GetWorldUrl() != "" {
		s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
			LoadWorldUrl: hdlSession.GetSession().GetWorldUrl(),
		}
	}

	_, err = client.StopSession(ctx, &headlessv1.StopSessionRequest{SessionId: id})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	now := time.Now()
	s.EndedAt = &now
	s.Status = entity.SessionStatus_ENDED

	return u.sessionRepo.Upsert(ctx, s)
}

func (u *SessionUsecase) GetSession(ctx context.Context, id string) (*entity.Session, error) {
	dbSession, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	client, err := u.hostRepo.GetRpcClient(ctx, dbSession.HostID)
	if err != nil {
		// TODO: 詳細画面に来る前に SearchSessions を叩いていればhostIdが違うということはないはずだが、
		// 直接来た & customSessionIdが設定されている場合は、hostIdが違うことがある
		if dbSession.Status == entity.SessionStatus_STARTING || dbSession.Status == entity.SessionStatus_RUNNING {
			_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
			dbSession.Status = entity.SessionStatus_CRASHED
		}

		return dbSession, nil
	}

	resp, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if err != nil || resp.GetSession() == nil {
		if dbSession.Status == entity.SessionStatus_STARTING || dbSession.Status == entity.SessionStatus_RUNNING {
			_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
			dbSession.Status = entity.SessionStatus_CRASHED
		}

		return dbSession, nil
	}

	dbSession.CurrentState = resp.GetSession()
	if dbSession.Status != entity.SessionStatus_RUNNING {
		dbSession.Status = entity.SessionStatus_RUNNING

		err = u.sessionRepo.UpdateStatus(ctx, id, dbSession.Status)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	return dbSession, nil
}

type SearchSessionsFilter struct {
	HostID *string
	Status *entity.SessionStatus
}

func (u *SessionUsecase) SearchSessions(ctx context.Context, filter SearchSessionsFilter) (entity.SessionList, error) {
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
		var filteredSessions entity.SessionList

		for _, s := range dbSessions {
			if s.HostID == *filter.HostID {
				filteredSessions = append(filteredSessions, s)
			}
		}

		dbSessions = filteredSessions
	}

	// RUNNING以外のステータスでフィルタする場合は、headlessからのリアルタイム情報は不要
	if filter.Status != nil && *filter.Status != entity.SessionStatus_RUNNING {
		return dbSessions, nil
	}

	var hdlSessions []*headlessv1.Session

	sessionHostIdMap := make(map[string]string)

	if filter.HostID != nil {
		timeoutCtx, cancel := context.WithTimeout(ctx, u.grpcCallTimeout)
		defer cancel()

		ss, err := u.getHostSessions(timeoutCtx, *filter.HostID)
		if err == nil {
			hdlSessions = ss
		}

		for _, s := range ss {
			sessionHostIdMap[s.GetId()] = *filter.HostID
		}
	} else {
		hosts, err := u.hostRepo.ListAll(ctx, port.HeadlessHostFetchOptions{})
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		// Parallel fetch sessions from all hosts
		type hostSessionResult struct {
			hostID   string
			sessions []*headlessv1.Session
			err      error
		}

		resultChan := make(chan hostSessionResult, len(hosts))

		var wg sync.WaitGroup

		for _, h := range hosts {
			wg.Add(1)

			go func(hostID string) {
				defer wg.Done()

				timeoutCtx, cancel := context.WithTimeout(ctx, u.grpcCallTimeout)
				defer cancel()

				ss, err := u.getHostSessions(timeoutCtx, hostID)
				resultChan <- hostSessionResult{hostID: hostID, sessions: ss, err: err}
			}(h.ID)
		}

		go func() {
			wg.Wait()
			close(resultChan)
		}()

		for result := range resultChan {
			if result.err != nil {
				// hostIdの指定がないときはエラーになったやつは無視する
				slog.Warn("Failed to get sessions from host", "host_id", result.hostID, "error", result.err)

				continue
			}

			hdlSessions = append(hdlSessions, result.sessions...)

			for _, s := range result.sessions {
				sessionHostIdMap[s.GetId()] = result.hostID
			}
		}
	}

	hdlSessionsMap := make(map[string]*headlessv1.Session)
	for _, s := range hdlSessions {
		hdlSessionsMap[s.GetId()] = s
	}

	sessions := make(entity.SessionList, 0, len(hdlSessions))

	for _, dbSession := range dbSessions {
		if s, ok := hdlSessionsMap[dbSession.ID]; ok {
			dbSession.CurrentState = s

			dbSession.Name = s.GetName()

			if dbSession.Status != entity.SessionStatus_RUNNING {
				_ = u.sessionRepo.UpdateStatus(ctx, dbSession.ID, entity.SessionStatus_RUNNING)
			}

			dbSession.Status = entity.SessionStatus_RUNNING
			if dbSession.HostID != sessionHostIdMap[dbSession.ID] {
				// たぶんCustomSessionIdが設定されているセッションが知らないうちに別のホストに移動している
				dbSession.HostID = sessionHostIdMap[dbSession.ID]
				startedAt := s.GetStartedAt().AsTime()
				dbSession.StartedAt = &startedAt

				dbSession.StartupParameters = s.GetStartupParameters()

				err := u.sessionRepo.Upsert(ctx, dbSession)
				if err != nil {
					slog.Error("Failed to upsert session", "id", dbSession.ID, "err", err)
				}
			}

			delete(hdlSessionsMap, dbSession.ID)
		} else if dbSession.Status == entity.SessionStatus_RUNNING {
			dbSession.Status = entity.SessionStatus_UNKNOWN
			_ = u.sessionRepo.UpdateStatus(ctx, dbSession.ID, entity.SessionStatus_UNKNOWN)
		}

		sessions = append(sessions, dbSession)
	}

	for _, s := range hdlSessionsMap {
		startedAt := s.GetStartedAt().AsTime()
		e := &entity.Session{
			ID:                s.GetId(),
			Name:              s.GetName(),
			HostID:            sessionHostIdMap[s.GetId()],
			Status:            entity.SessionStatus_RUNNING,
			StartedAt:         &startedAt,
			StartupParameters: s.GetStartupParameters(),
			CurrentState:      s,
		}

		sessions = append(sessions, e)

		err := u.sessionRepo.Upsert(ctx, e)
		if err != nil {
			slog.Error("Failed to upsert session", "id", e.ID, "err", err)
		}
	}

	return sessions, nil
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

	s.CurrentState = newSession.GetSession()
	s.Name = newSession.GetSession().GetName()

	return u.sessionRepo.Upsert(ctx, s)
}

func (u *SessionUsecase) DeleteSession(ctx context.Context, id string) error {
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
		_, err = client.SaveSessionWorld(ctx, &headlessv1.SaveSessionWorldRequest{SessionId: id})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return s.CurrentState.GetWorldUrl(), nil

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

func (u *SessionUsecase) getHostSessions(ctx context.Context, hostId string) ([]*headlessv1.Session, error) {
	client, err := u.hostRepo.GetRpcClient(ctx, hostId)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	resp, err := client.ListSessions(ctx, &headlessv1.ListSessionsRequest{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return resp.GetSessions(), nil
}
