package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type SessionUsecase struct {
	sessionRepo port.SessionRepository
	hostRepo    port.HeadlessHostRepository
}

func NewSessionUsecase(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository) *SessionUsecase {
	return &SessionUsecase{
		sessionRepo: sessionRepo,
		hostRepo:    hostRepo,
	}
}

func (u *SessionUsecase) StartSession(ctx context.Context, hostId string, userId *string, params *headlessv1.WorldStartupParameters) (*entity.Session, error) {
	client, err := u.hostRepo.GetRpcClient(ctx, hostId)
	if err != nil {
		return nil, err
	}

	resp, err := client.StartWorld(ctx, &headlessv1.StartWorldRequest{
		Parameters: params,
	})
	if err != nil {
		return nil, err
	}

	startedAt := resp.OpenedSession.StartedAt.AsTime()
	session := &entity.Session{
		ID:                resp.OpenedSession.Id,
		Name:              resp.OpenedSession.Name,
		Status:            entity.SessionStatus_RUNNING,
		HostID:            hostId,
		StartedAt:         &startedAt,
		StartedBy:         userId,
		StartupParameters: params,
		CurrentState:      resp.OpenedSession,
	}
	err = u.sessionRepo.Upsert(ctx, session)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (u *SessionUsecase) StopSession(ctx context.Context, id string) error {
	s, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	client, err := u.hostRepo.GetRpcClient(ctx, s.HostID)
	if err != nil {
		return err
	}

	hdlSession, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if err != nil {
		return err
	}
	if hdlSession.Session.WorldUrl != "" {
		s.StartupParameters.LoadWorld = &headlessv1.WorldStartupParameters_LoadWorldUrl{
			LoadWorldUrl: hdlSession.Session.WorldUrl,
		}
	}

	_, err = client.StopSession(ctx, &headlessv1.StopSessionRequest{SessionId: id})
	if err != nil {
		return err
	}

	now := time.Now()
	s.EndedAt = &now
	s.Status = entity.SessionStatus_ENDED

	return u.sessionRepo.Upsert(ctx, s)
}

func (u *SessionUsecase) GetSession(ctx context.Context, id string) (*entity.Session, error) {
	dbSession, err := u.sessionRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	client, err := u.hostRepo.GetRpcClient(ctx, dbSession.HostID)
	if err != nil {
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
			return nil, err
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
		return nil, err
	}
	resp, err := client.ListSessions(ctx, &headlessv1.ListSessionsRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Sessions, nil
}

func (u *SessionUsecase) SearchSessions(ctx context.Context, filter SearchSessionsFilter) (entity.SessionList, error) {
	var dbSessions entity.SessionList
	if filter.Status != nil {
		s, err := u.sessionRepo.ListByStatus(ctx, *filter.Status)
		if err != nil {
			return nil, err
		}
		dbSessions = s
		if *filter.Status != entity.SessionStatus_RUNNING {
			return dbSessions, nil
		}
	} else {
		s, err := u.sessionRepo.ListAll(ctx)
		if err != nil {
			return nil, err
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
		hosts, err := u.hostRepo.ListAll(ctx)
		if err != nil {
			return nil, err
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
			delete(hdlSessionsMap, dbSession.ID)
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
		return err
	}
	client, err := u.hostRepo.GetRpcClient(ctx, s.HostID)
	if err != nil {
		return err
	}
	_, err = client.UpdateSessionParameters(ctx, params)
	if err != nil {
		return err
	}
	newSession, err := client.GetSession(ctx, &headlessv1.GetSessionRequest{SessionId: id})
	if err != nil {
		return err
	}
	if err := updateStartupParamsByUpdateRequest(s.StartupParameters, params); err != nil {
		return err
	}
	s.CurrentState = newSession.Session
	s.Name = newSession.Session.Name

	return u.sessionRepo.Upsert(ctx, s)
}

func (u *SessionUsecase) DeleteSession(ctx context.Context, id string) error {
	return u.sessionRepo.Delete(ctx, id)
}
