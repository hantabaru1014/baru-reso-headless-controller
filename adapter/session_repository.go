package adapter

import (
	"context"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/encoding/protojson"
)

var _ port.SessionRepository = (*SessionRepository)(nil)

type SessionRepository struct {
	q *db.Queries
}

func NewSessionRepository(q *db.Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

// Upsert implements port.SessionRepository.
func (r *SessionRepository) Upsert(ctx context.Context, session *entity.Session) error {
	var err error
	var startupParams []byte
	if session.StartupParameters != nil {
		startupParams, err = protojson.Marshal(session.StartupParameters)
		if err != nil {
			return err
		}
	}

	startedAt := pgtype.Timestamptz{
		Valid: session.StartedAt != nil,
	}
	if session.StartedAt != nil {
		startedAt.Time = *session.StartedAt
	}
	endedAt := pgtype.Timestamptz{
		Valid: session.EndedAt != nil,
	}
	if session.EndedAt != nil {
		endedAt.Time = *session.EndedAt
	}
	startedBy := pgtype.Text{
		Valid: session.StartedBy != nil,
	}
	if session.StartedBy != nil {
		startedBy.String = *session.StartedBy
	}
	memo := pgtype.Text{
		Valid: session.Memo != "",
	}
	if session.Memo != "" {
		memo.String = session.Memo
	}

	_, err = r.q.UpsertSession(ctx, db.UpsertSessionParams{
		ID:                             session.ID,
		Name:                           session.Name,
		Status:                         int32(session.Status),
		StartedAt:                      startedAt,
		StartedBy:                      startedBy,
		EndedAt:                        endedAt,
		HostID:                         session.HostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    session.AutoUpgrade,
		Memo:                           memo,
	})
	return err
}

func (r *SessionRepository) UpdateStatus(ctx context.Context, id string, status entity.SessionStatus) error {
	return r.q.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
		ID:     id,
		Status: int32(status),
	})
}

func (r *SessionRepository) Get(ctx context.Context, id string) (*entity.Session, error) {
	s, err := r.q.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return sessionToEntity(s)
}

func (r *SessionRepository) ListAll(ctx context.Context) (entity.SessionList, error) {
	sessions, err := r.q.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	result := make(entity.SessionList, 0, len(sessions))
	for _, s := range sessions {
		entity, err := sessionToEntity(s)
		if err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

func (r *SessionRepository) ListByStatus(ctx context.Context, status entity.SessionStatus) (entity.SessionList, error) {
	sessions, err := r.q.ListSessionsByStatus(ctx, int32(status))
	if err != nil {
		return nil, err
	}
	result := make(entity.SessionList, 0, len(sessions))
	for _, s := range sessions {
		entity, err := sessionToEntity(s)
		if err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

func sessionToEntity(s db.Session) (*entity.Session, error) {
	var startedAt *time.Time
	if s.StartedAt.Valid {
		startedAt = &s.StartedAt.Time
	}
	var endedAt *time.Time
	if s.EndedAt.Valid {
		endedAt = &s.EndedAt.Time
	}
	startupParams := headlessv1.WorldStartupParameters{}
	if err := protojson.Unmarshal(s.StartupParameters, &startupParams); err != nil {
		return nil, err
	}
	var startedBy *string
	if s.StartedBy.Valid {
		startedBy = &s.StartedBy.String
	}
	memo := ""
	if s.Memo.Valid {
		memo = s.Memo.String
	}

	return &entity.Session{
		ID:                s.ID,
		Name:              s.Name,
		Status:            entity.SessionStatus(s.Status),
		StartedAt:         startedAt,
		StartedBy:         startedBy,
		EndedAt:           endedAt,
		HostID:            s.HostID,
		StartupParameters: &startupParams,
		AutoUpgrade:       s.AutoUpgrade,
		Memo:              memo,
	}, nil
}

// Delete implements port.SessionRepository.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteSession(ctx, id)
}
