package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-errors/errors"
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
	sessionRepo  port.SessionRepository
	hostRepo     port.HeadlessHostRepository
	forcePortMin int
	forcePortMax int
}

func NewSessionUsecase(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository) *SessionUsecase {
	portMin, portMax := 0, 0
	portMinStr := os.Getenv("SESSION_PORT_MIN")
	portMaxStr := os.Getenv("SESSION_PORT_MAX")
	if portMinStr != "" && portMaxStr != "" {
		var err error
		portMin, err = strconv.Atoi(portMinStr)
		if err != nil {
			panic(fmt.Sprintf("invalid SESSION_PORT_MIN: %s", portMinStr))
		}
		portMax, err = strconv.Atoi(portMaxStr)
		if err != nil {
			panic(fmt.Sprintf("invalid SESSION_PORT_MAX: %s", portMaxStr))
		}
		if portMin > portMax {
			panic(fmt.Sprintf("invalid port range: SESSION_PORT_MIN(%d) > SESSION_PORT_MAX(%d)", portMin, portMax))
		}
	}

	return &SessionUsecase{
		sessionRepo:  sessionRepo,
		hostRepo:     hostRepo,
		forcePortMin: portMin,
		forcePortMax: portMax,
	}
}

func (u *SessionUsecase) StartSession(ctx context.Context, hostId string, userId *string, params *headlessv1.WorldStartupParameters, memo *string) (*entity.Session, error) {
	client, err := u.hostRepo.GetRpcClient(ctx, hostId)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	// forcePortが指定されていない場合、環境変数が設定されていれば自動割り当て
	paramsForContainer := params
	if params.ForcePort == 0 {
		autoPort, err := u.getFreeSessionPort()
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		if autoPort != 0 {
			// コンテナに渡すパラメータのコピーを作成してforcePortを設定
			paramsForContainer = proto.Clone(params).(*headlessv1.WorldStartupParameters)
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

	startedAt := resp.OpenedSession.StartedAt.AsTime()
	session := &entity.Session{
		ID:                resp.OpenedSession.Id,
		Name:              resp.OpenedSession.Name,
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostId,
		StartedAt:         &startedAt,
		OwnerID:           userId,
		StartupParameters: params,
		CurrentState:      resp.OpenedSession,
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
	if hdlSession.Session.WorldUrl != "" {
		s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
			LoadWorldUrl: hdlSession.Session.WorldUrl,
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
	if err != nil || resp.Session == nil {
		if dbSession.Status == entity.SessionStatus_STARTING || dbSession.Status == entity.SessionStatus_RUNNING {
			_ = u.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatus_CRASHED)
			dbSession.Status = entity.SessionStatus_CRASHED
		}
		return dbSession, nil
	}
	dbSession.CurrentState = resp.Session
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

func (u *SessionUsecase) getHostSessions(ctx context.Context, hostId string) ([]*headlessv1.Session, error) {
	client, err := u.hostRepo.GetRpcClient(ctx, hostId)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	resp, err := client.ListSessions(ctx, &headlessv1.ListSessionsRequest{})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return resp.Sessions, nil
}

func (u *SessionUsecase) SearchSessions(ctx context.Context, filter SearchSessionsFilter) (entity.SessionList, error) {
	var dbSessions entity.SessionList
	if filter.Status != nil {
		s, err := u.sessionRepo.ListByStatus(ctx, *filter.Status)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		dbSessions = s
		if *filter.Status != entity.SessionStatus_RUNNING {
			return dbSessions, nil
		}
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

	var hdlSessions []*headlessv1.Session
	sessionHostIdMap := make(map[string]string)
	if filter.HostID != nil {
		ss, err := u.getHostSessions(ctx, *filter.HostID)
		if err == nil {
			hdlSessions = ss
		}
		for _, s := range ss {
			sessionHostIdMap[s.Id] = *filter.HostID
		}
	} else {
		hosts, err := u.hostRepo.ListAll(ctx, port.HeadlessHostFetchOptions{})
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		for _, h := range hosts {
			ss, err := u.getHostSessions(ctx, h.ID)
			if err != nil {
				// hostIdの指定がないときはエラーになったやつは無視する
				continue
			}
			hdlSessions = append(hdlSessions, ss...)
			for _, s := range ss {
				sessionHostIdMap[s.Id] = h.ID
			}
		}
	}

	hdlSessionsMap := make(map[string]*headlessv1.Session)
	for _, s := range hdlSessions {
		hdlSessionsMap[s.Id] = s
	}
	sessions := make(entity.SessionList, 0, len(hdlSessions))
	for _, dbSession := range dbSessions {
		if s, ok := hdlSessionsMap[dbSession.ID]; ok {
			dbSession.CurrentState = s
			dbSession.Name = s.Name
			if dbSession.Status != entity.SessionStatus_RUNNING {
				_ = u.sessionRepo.UpdateStatus(ctx, dbSession.ID, entity.SessionStatus_RUNNING)
			}
			dbSession.Status = entity.SessionStatus_RUNNING
			if dbSession.HostID != sessionHostIdMap[dbSession.ID] {
				// たぶんCustomSessionIdが設定されているセッションが知らないうちに別のホストに移動している
				dbSession.HostID = sessionHostIdMap[dbSession.ID]
				startedAt := s.StartedAt.AsTime()
				dbSession.StartedAt = &startedAt
				dbSession.StartupParameters = s.StartupParameters
				if err := u.sessionRepo.Upsert(ctx, dbSession); err != nil {
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
		startedAt := s.StartedAt.AsTime()
		e := &entity.Session{
			ID:                s.Id,
			Name:              s.Name,
			HostID:            sessionHostIdMap[s.Id],
			Status:            entity.SessionStatus_RUNNING,
			StartedAt:         &startedAt,
			StartupParameters: s.StartupParameters,
			CurrentState:      s,
		}
		sessions = append(sessions, e)
		if err := u.sessionRepo.Upsert(ctx, e); err != nil {
			slog.Error("Failed to upsert session", "id", e.ID, "err", err)
		}
	}

	return sessions, nil
}

func updateStartupParamsByUpdateRequest(
	current *headlessv1.WorldStartupParameters,
	params *headlessv1.UpdateSessionParametersRequest,
) error {
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
		current.AccessLevel = *params.AccessLevel
	}
	if params.AwayKickMinutes != nil {
		current.AwayKickMinutes = *params.AwayKickMinutes
	}
	if params.IdleRestartIntervalSeconds != nil {
		current.IdleRestartIntervalSeconds = *params.IdleRestartIntervalSeconds
	}
	if params.SaveOnExit != nil {
		current.SaveOnExit = *params.SaveOnExit
	}
	if params.AutoSaveIntervalSeconds != nil {
		current.AutoSaveIntervalSeconds = *params.AutoSaveIntervalSeconds
	}
	if params.AutoSleep != nil {
		current.AutoSleep = *params.AutoSleep
	}
	if params.HideFromPublicListing != nil {
		current.HideFromPublicListing = *params.HideFromPublicListing
	}
	if params.UpdateTags {
		current.Tags = params.Tags
	}

	return nil
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
	if err := updateStartupParamsByUpdateRequest(s.StartupParameters, params); err != nil {
		return errors.Wrap(err, 0)
	}
	s.CurrentState = newSession.Session
	s.Name = newSession.Session.Name

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
		return s.CurrentState.WorldUrl, nil

	case SaveMode_SAVE_AS:
		saveAsResp, err := client.SaveAsSessionWorld(ctx, &headlessv1.SaveAsSessionWorldRequest{
			SessionId: id,
			Type:      headlessv1.SaveAsSessionWorldRequest_SAVE_AS_TYPE_SAVE_AS,
		})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		return saveAsResp.SavedRecordUrl, nil

	case SaveMode_COPY:
		saveAsResp, err := client.SaveAsSessionWorld(ctx, &headlessv1.SaveAsSessionWorldRequest{
			SessionId: id,
			Type:      headlessv1.SaveAsSessionWorldRequest_SAVE_AS_TYPE_COPY,
		})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		return saveAsResp.SavedRecordUrl, nil

	default:
		return "", errors.Errorf("unknown save mode: %d", saveMode)
	}
}

// getFreeSessionPort は環境変数で指定されたポート範囲から空きポートを探して返す
// 環境変数が設定されていない場合は0を返す
func (u *SessionUsecase) getFreeSessionPort() (int, error) {
	if u.forcePortMin == 0 && u.forcePortMax == 0 {
		return 0, nil
	}

	// ランダムな開始位置から探索（同じポートに偏らないように）
	offset := time.Now().UnixNano() % int64(u.forcePortMax-u.forcePortMin+1)
	for i := 0; i <= u.forcePortMax-u.forcePortMin; i++ {
		candidatePort := u.forcePortMin + int((offset+int64(i))%int64(u.forcePortMax-u.forcePortMin+1))
		if isPortAvailable(candidatePort) {
			return candidatePort, nil
		}
	}

	return 0, errors.Errorf("no free port found in range %d-%d", u.forcePortMin, u.forcePortMax)
}

func isPortAvailable(port int) bool {
	address := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
