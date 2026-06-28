package adapter

import (
	"context"
	"encoding/json"
	"log/slog"
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
	)

	if session.StartupParameters != nil {
		startupParams, err = protojson.Marshal(session.StartupParameters)
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

	createdBy := pgtype.Text{
		Valid: session.CreatedBy != nil,
	}
	if session.CreatedBy != nil {
		createdBy.String = *session.CreatedBy
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
		CreatedBy:                      createdBy,
		GroupID:                        session.GroupID,
		EndedAt:                        endedAt,
		HostID:                         session.HostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    session.AutoUpgrade,
		Memo:                           memo,
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

func (r *SessionRepository) ApplySessionParametersChanged(ctx context.Context, id string, snapshot *headlessv1.Session) error {
	if snapshot == nil {
		return nil
	}

	// tags は JSON array として渡す。空配列で「タグ全消し」を表現できる。
	// 必ず非 nil の []string を marshal して "[]" を保証する (nil だと "null" に
	// なって JSONB || 演算で型不整合になる)。
	tagSlice := snapshot.GetTags()
	if tagSlice == nil {
		tagSlice = []string{}
	}

	tagsJSON, err := json.Marshal(tagSlice)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return r.q.ApplySessionParametersChanged(ctx, db.ApplySessionParametersChangedParams{
		ID:                         id,
		Name:                       snapshot.GetName(),
		Description:                snapshot.GetDescription(),
		MaxUsers:                   snapshot.GetMaxUsers(),
		AccessLevel:                snapshot.GetAccessLevel().String(),
		HideFromPublicListing:      snapshot.GetHideFromPublicListing(),
		AwayKickMinutes:            snapshot.GetAwayKickMinutes(),
		IdleRestartIntervalSeconds: snapshot.GetIdleRestartIntervalSeconds(),
		SaveOnExit:                 snapshot.GetSaveOnExit(),
		AutoSaveIntervalSeconds:    snapshot.GetAutoSaveIntervalSeconds(),
		AutoSleep:                  snapshot.GetAutoSleep(),
		Tags:                       tagsJSON,
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

	// group_id は SQL 内で host から自動導出するため、引数からは外している.
	rows, err := r.q.InsertSessionFromEvent(ctx, db.InsertSessionFromEventParams{
		ID:                             session.ID,
		Name:                           session.Name,
		Status:                         int32(session.Status),
		StartedAt:                      startedAt,
		HostID:                         session.HostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "session", 0)
	}

	// rows=0 は (a) ON CONFLICT で既存 row を温存した = 正常 と (b) host_id に
	// 一致する hosts row が無い = SessionStarted event が孤立 (host 削除との race
	// 等) を区別できない. (b) はデータ損失なので警告として残す.
	if rows == 0 {
		slog.Warn("session_repository: InsertSessionFromEvent inserted 0 rows; either ON CONFLICT skipped or host is missing",
			"session_id", session.ID, "host_id", session.HostID)
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
	// GroupIDs == nil → sqlc.narg('group_ids') を NULL にして全件対象.
	// GroupIDs == [] or [...] → ANY 絞り込み. 空配列の場合は結果ゼロ件.
	params := db.ListSessionsPagedParams{
		PageOffset: opts.PageIndex * opts.PageSize,
		PageSize:   opts.PageSize,
		GroupIds:   opts.GroupIDs,
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

	var createdBy *string
	if s.CreatedBy.Valid {
		createdBy = &s.CreatedBy.String
	}

	memo := ""
	if s.Memo.Valid {
		memo = s.Memo.String
	}

	// CurrentState は揮発状態のため DB に持たず in-memory cache (port.SessionStateCache)
	// が権威。Caller 側で cache から populate される。
	return &entity.Session{
		ID:                s.ID,
		Name:              s.Name,
		Status:            entity.SessionStatus(s.Status),
		StartedAt:         startedAt,
		CreatedBy:         createdBy,
		EndedAt:           endedAt,
		HostID:            s.HostID,
		StartupParameters: &startupParams,
		AutoUpgrade:       s.AutoUpgrade,
		Memo:              memo,
		GroupID:           s.GroupID,
	}, nil
}

// Delete implements port.SessionRepository.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteSession(ctx, id)
}
