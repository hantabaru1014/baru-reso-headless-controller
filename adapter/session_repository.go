package adapter

import (
	"context"
	"time"

	"github.com/go-errors/errors"
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
	var (
		err           error
		startupParams []byte
		currentState  []byte
	)

	if session.StartupParameters != nil {
		startupParams, err = protojson.Marshal(session.StartupParameters)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	if session.CurrentState != nil {
		currentState, err = protojson.Marshal(session.CurrentState)
		if err != nil {
			return errors.Wrap(err, 0)
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

	ownerId := pgtype.Text{
		Valid: session.OwnerID != nil,
	}
	if session.OwnerID != nil {
		ownerId.String = *session.OwnerID
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
		OwnerID:                        ownerId,
		EndedAt:                        endedAt,
		HostID:                         session.HostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    session.AutoUpgrade,
		Memo:                           memo,
		CurrentState:                   currentState,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (r *SessionRepository) UpdateStatus(ctx context.Context, id string, status entity.SessionStatus) error {
	return r.q.UpdateSessionStatus(ctx, db.UpdateSessionStatusParams{
		ID:     id,
		Status: int32(status),
	})
}

func (r *SessionRepository) UpdateCurrentStateAndName(ctx context.Context, id string, currentState *headlessv1.Session, name string) error {
	cs, err := protojson.Marshal(currentState)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return r.q.UpdateSessionCurrentStateAndName(ctx, db.UpdateSessionCurrentStateAndNameParams{
		ID:           id,
		CurrentState: cs,
		Name:         name,
	})
}

func (r *SessionRepository) UpdateAfterWorldSaved(ctx context.Context, id string, worldURL string) error {
	return r.q.UpdateSessionAfterWorldSaved(ctx, db.UpdateSessionAfterWorldSavedParams{
		ID:       id,
		WorldUrl: worldURL,
	})
}

func (r *SessionRepository) DowngradeToUnknownIfRunning(ctx context.Context, id string) error {
	return r.q.DowngradeSessionToUnknownIfRunning(ctx, id)
}

func (r *SessionRepository) Get(ctx context.Context, id string) (*entity.Session, error) {
	s, err := r.q.GetSession(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	return sessionToEntity(s)
}

func (r *SessionRepository) ListAll(ctx context.Context) (entity.SessionList, error) {
	sessions, err := r.q.ListSessions(ctx)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	result := make(entity.SessionList, 0, len(sessions))

	for _, s := range sessions {
		entity, err := sessionToEntity(s)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result = append(result, entity)
	}

	return result, nil
}

func (r *SessionRepository) ListByStatus(ctx context.Context, status entity.SessionStatus) (entity.SessionList, error) {
	sessions, err := r.q.ListSessionsByStatus(ctx, int32(status))
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	result := make(entity.SessionList, 0, len(sessions))

	for _, s := range sessions {
		entity, err := sessionToEntity(s)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result = append(result, entity)
	}

	return result, nil
}

func (r *SessionRepository) ListByHostAndStatus(ctx context.Context, hostID string, status entity.SessionStatus) (entity.SessionList, error) {
	sessions, err := r.q.ListSessionsByHostAndStatus(ctx, db.ListSessionsByHostAndStatusParams{
		HostID: hostID,
		Status: int32(status),
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	result := make(entity.SessionList, 0, len(sessions))

	for _, s := range sessions {
		entity, err := sessionToEntity(s)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result = append(result, entity)
	}

	return result, nil
}

func (r *SessionRepository) InsertFromEvent(ctx context.Context, session *entity.Session) error {
	// event 由来の session は startup_parameters を知らないため、nil の場合は
	// 空 proto で埋める (sessions.startup_parameters の NOT NULL 制約を満たす)。
	startupSource := session.StartupParameters
	if startupSource == nil {
		startupSource = &headlessv1.WorldStartupParameters{}
	}

	startupParams, err := protojson.Marshal(startupSource)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	startedAt := pgtype.Timestamptz{Valid: session.StartedAt != nil}
	if session.StartedAt != nil {
		startedAt.Time = *session.StartedAt
	}

	if err := r.q.InsertSessionFromEvent(ctx, db.InsertSessionFromEventParams{
		ID:                             session.ID,
		Name:                           session.Name,
		Status:                         int32(session.Status),
		StartedAt:                      startedAt,
		HostID:                         session.HostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	return nil
}

func (r *SessionRepository) ApplySessionStarted(ctx context.Context, id, hostID, name string, occurredAt time.Time) (bool, error) {
	rows, err := r.q.ApplySessionStarted(ctx, db.ApplySessionStartedParams{
		ID:        id,
		Name:      name,
		Status:    int32(entity.SessionStatus_RUNNING),
		StartedAt: pgtype.Timestamptz{Time: occurredAt, Valid: true},
		HostID:    hostID,
	})
	if err != nil {
		return false, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	return rows > 0, nil
}

func (r *SessionRepository) ApplySessionEnded(ctx context.Context, id, hostID string, occurredAt time.Time) (bool, error) {
	rows, err := r.q.ApplySessionEnded(ctx, db.ApplySessionEndedParams{
		ID:      id,
		Status:  int32(entity.SessionStatus_ENDED),
		EndedAt: pgtype.Timestamptz{Time: occurredAt, Valid: true},
		HostID:  hostID,
	})
	if err != nil {
		return false, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	return rows > 0, nil
}

func (r *SessionRepository) ListPaged(ctx context.Context, opts port.SessionListPageOptions) (*port.SessionListPageResult, error) {
	params := db.ListSessionsPagedParams{
		PageOffset: opts.PageIndex * opts.PageSize,
		PageSize:   opts.PageSize,
	}
	if opts.Status != nil {
		params.Status = pgtype.Int4{Int32: int32(*opts.Status), Valid: true}
	}

	if opts.HostID != nil {
		params.HostID = pgtype.Text{String: *opts.HostID, Valid: true}
	}

	rows, err := r.q.ListSessionsPaged(ctx, params)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	result := &port.SessionListPageResult{
		Sessions: make(entity.SessionList, 0, len(rows)),
	}
	if len(rows) > 0 {
		result.TotalCount = int32(rows[0].TotalCount) //nolint:gosec // G115: total_count はテーブル件数で int32 範囲を超えない
	}

	for _, row := range rows {
		s, err := sessionToEntity(row.Session)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result.Sessions = append(result.Sessions, s)
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

	err := protojson.Unmarshal(s.StartupParameters, &startupParams)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var currentState *headlessv1.Session

	if len(s.CurrentState) > 0 {
		cs := &headlessv1.Session{}
		if err := protojson.Unmarshal(s.CurrentState, cs); err != nil {
			return nil, errors.Wrap(err, 0)
		}

		currentState = cs
	}

	var ownerId *string
	if s.OwnerID.Valid {
		ownerId = &s.OwnerID.String
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
		OwnerID:           ownerId,
		EndedAt:           endedAt,
		HostID:            s.HostID,
		StartupParameters: &startupParams,
		AutoUpgrade:       s.AutoUpgrade,
		Memo:              memo,
		CurrentState:      currentState,
	}, nil
}

// Delete implements port.SessionRepository.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteSession(ctx, id)
}
