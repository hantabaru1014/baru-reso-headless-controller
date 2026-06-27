package adapter

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ port.ScheduledSessionOperationRepository = (*ScheduledSessionOperationRepository)(nil)

type ScheduledSessionOperationRepository struct {
	q *db.Queries
}

func NewScheduledSessionOperationRepository(q *db.Queries) *ScheduledSessionOperationRepository {
	return &ScheduledSessionOperationRepository{q: q}
}

func (r *ScheduledSessionOperationRepository) Create(ctx context.Context, params port.ScheduledSessionOperationCreateParams) (*entity.ScheduledSessionOperation, error) {
	row, err := r.q.CreateScheduledSessionOperation(ctx, db.CreateScheduledSessionOperationParams{
		OperationType:    int32(params.OperationType),
		OperationPayload: params.OperationPayload,
		TriggerType:      int32(params.TriggerType),
		TriggerConfig:    params.TriggerConfig,
		NextFireAt:       pgtype.Timestamptz{Time: params.NextFireAt, Valid: true},
		HostID:           textFromPtr(params.HostID),
		SessionID:        textFromPtr(params.SessionID),
		CreatedBy:        textFromPtr(params.CreatedBy),
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return scheduledSessionOperationToEntity(row)
}

func (r *ScheduledSessionOperationRepository) Get(ctx context.Context, id string) (*entity.ScheduledSessionOperation, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}

	row, err := r.q.GetScheduledSessionOperation(ctx, uid)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return scheduledSessionOperationToEntity(row)
}

func (r *ScheduledSessionOperationRepository) List(ctx context.Context, filter port.ScheduledSessionOperationListFilter) (*port.ScheduledSessionOperationListResult, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	params := db.ListScheduledSessionOperationsParams{
		SessionID:  textFromPtr(filter.SessionID),
		HostID:     textFromPtr(filter.HostID),
		PageSize:   pageSize,
		PageOffset: filter.PageIndex * pageSize,
	}
	if filter.Status != nil {
		params.Status = pgtype.Int4{Int32: int32(*filter.Status), Valid: true}
	}

	rows, err := r.q.ListScheduledSessionOperations(ctx, params)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	result := &port.ScheduledSessionOperationListResult{
		Items: make(entity.ScheduledSessionOperationList, 0, len(rows)),
	}
	if len(rows) > 0 {
		result.TotalCount = int32(rows[0].TotalCount) //nolint:gosec // G115: テーブル件数なので int32 範囲を超えない
	}

	for _, row := range rows {
		e, err := scheduledSessionOperationToEntity(row.ScheduledSessionOperation)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result.Items = append(result.Items, e)
	}

	return result, nil
}

func (r *ScheduledSessionOperationRepository) ClaimDue(ctx context.Context, instanceID string, batchSize int32) (entity.ScheduledSessionOperationList, error) {
	rows, err := r.q.ClaimDueScheduledSessionOperations(ctx, db.ClaimDueScheduledSessionOperationsParams{
		InstanceID: instanceID,
		BatchSize:  batchSize,
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	result := make(entity.ScheduledSessionOperationList, 0, len(rows))

	for _, row := range rows {
		e, err := scheduledSessionOperationToEntity(row)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result = append(result, e)
	}

	return result, nil
}

func (r *ScheduledSessionOperationRepository) ReleaseStaleClaims(ctx context.Context, staleAfter time.Duration) (int64, error) {
	seconds := int32(staleAfter / time.Second) //nolint:gosec // G115: 設定値で int32 範囲を超えない

	rows, err := r.q.ReleaseStaleScheduledSessionOperationClaims(ctx, seconds)
	if err != nil {
		return 0, errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return rows, nil
}

func (r *ScheduledSessionOperationRepository) MarkSucceeded(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}

	if _, err := r.q.MarkScheduledSessionOperationSucceeded(ctx, uid); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return nil
}

func (r *ScheduledSessionOperationRepository) MarkFailed(ctx context.Context, id string, errMessage string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}

	if _, err := r.q.MarkScheduledSessionOperationFailed(ctx, db.MarkScheduledSessionOperationFailedParams{
		ID:        uid,
		LastError: errMessage,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return nil
}

func (r *ScheduledSessionOperationRepository) Requeue(ctx context.Context, id string, nextFireAt time.Time) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}

	if _, err := r.q.RequeueScheduledSessionOperation(ctx, db.RequeueScheduledSessionOperationParams{
		ID:         uid,
		NextFireAt: pgtype.Timestamptz{Time: nextFireAt, Valid: true},
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return nil
}

func (r *ScheduledSessionOperationRepository) Cancel(ctx context.Context, id string) (bool, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return false, err
	}

	rows, err := r.q.CancelScheduledSessionOperation(ctx, uid)
	if err != nil {
		return false, errors.WrapPrefix(convertDBErr(err), "scheduled_session_operation", 0)
	}

	return rows > 0, nil
}

func scheduledSessionOperationToEntity(s db.ScheduledSessionOperation) (*entity.ScheduledSessionOperation, error) {
	id, err := formatUUID(s.ID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var lastError *string

	if s.LastError.Valid {
		v := s.LastError.String
		lastError = &v
	}

	var claimedBy *string

	if s.ClaimedBy.Valid {
		v := s.ClaimedBy.String
		claimedBy = &v
	}

	var claimedAt *time.Time

	if s.ClaimedAt.Valid {
		v := s.ClaimedAt.Time
		claimedAt = &v
	}

	var executedAt *time.Time

	if s.ExecutedAt.Valid {
		v := s.ExecutedAt.Time
		executedAt = &v
	}

	var createdBy *string

	if s.CreatedBy.Valid {
		v := s.CreatedBy.String
		createdBy = &v
	}

	var hostID *string

	if s.HostID.Valid {
		v := s.HostID.String
		hostID = &v
	}

	var sessionID *string

	if s.SessionID.Valid {
		v := s.SessionID.String
		sessionID = &v
	}

	payload := json.RawMessage(s.OperationPayload)
	cfg := json.RawMessage(s.TriggerConfig)

	return &entity.ScheduledSessionOperation{
		ID:               id,
		OperationType:    entity.ScheduledOperationType(s.OperationType),
		OperationPayload: payload,
		TriggerType:      entity.ScheduledTriggerType(s.TriggerType),
		TriggerConfig:    cfg,
		NextFireAt:       s.NextFireAt.Time,
		HostID:           hostID,
		SessionID:        sessionID,
		Status:           entity.ScheduledOperationStatus(s.Status),
		LastError:        lastError,
		ClaimedBy:        claimedBy,
		ClaimedAt:        claimedAt,
		ExecutedAt:       executedAt,
		CreatedBy:        createdBy,
		CreatedAt:        s.CreatedAt.Time,
		UpdatedAt:        s.UpdatedAt.Time,
	}, nil
}
