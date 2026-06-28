package adapter

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var _ port.AsyncJobRepository = (*AsyncJobRepository)(nil)

type AsyncJobRepository struct {
	q *db.Queries
}

func NewAsyncJobRepository(q *db.Queries) *AsyncJobRepository {
	return &AsyncJobRepository{q: q}
}

func (r *AsyncJobRepository) Create(ctx context.Context, params port.AsyncJobCreateParams) (*entity.AsyncJob, error) {
	row, err := r.q.CreateAsyncJob(ctx, db.CreateAsyncJobParams{
		JobType:   int32(params.JobType),
		Payload:   params.Payload,
		HostID:    textFromPtr(params.HostID),
		SessionID: textFromPtr(params.SessionID),
		CreatedBy: textFromPtr(params.CreatedBy),
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	return asyncJobToEntity(row)
}

func (r *AsyncJobRepository) Get(ctx context.Context, id string) (*entity.AsyncJob, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}

	row, err := r.q.GetAsyncJob(ctx, uid)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	return asyncJobToEntity(row)
}

func (r *AsyncJobRepository) ClaimDue(ctx context.Context, instanceID string, batchSize int32) (entity.AsyncJobList, error) {
	rows, err := r.q.ClaimDueAsyncJobs(ctx, db.ClaimDueAsyncJobsParams{
		InstanceID: instanceID,
		BatchSize:  batchSize,
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	result := make(entity.AsyncJobList, 0, len(rows))

	for _, row := range rows {
		e, err := asyncJobToEntity(row)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result = append(result, e)
	}

	return result, nil
}

func (r *AsyncJobRepository) ReleaseStaleClaims(ctx context.Context, staleAfter time.Duration) (int64, error) {
	seconds := int32(staleAfter / time.Second) //nolint:gosec // G115: 設定値で int32 範囲を超えない

	rows, err := r.q.ReleaseStaleAsyncJobClaims(ctx, seconds)
	if err != nil {
		return 0, errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	return rows, nil
}

func (r *AsyncJobRepository) MarkSucceeded(ctx context.Context, id string, resultPayload json.RawMessage) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}

	// nil の場合は SQL NULL ではなく JSON null を入れたくないので、空であれば JSON null リテラルを入れる。
	if len(resultPayload) == 0 {
		resultPayload = json.RawMessage("null")
	}

	if _, err := r.q.MarkAsyncJobSucceeded(ctx, db.MarkAsyncJobSucceededParams{
		ID:            uid,
		ResultPayload: resultPayload,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	return nil
}

func (r *AsyncJobRepository) MarkFailed(ctx context.Context, id string, errMessage string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}

	if _, err := r.q.MarkAsyncJobFailed(ctx, db.MarkAsyncJobFailedParams{
		ID:        uid,
		LastError: errMessage,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	return nil
}

func asyncJobToEntity(s db.AsyncJob) (*entity.AsyncJob, error) {
	id, err := formatUUID(s.ID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return &entity.AsyncJob{
		ID:            id,
		JobType:       entity.AsyncJobType(s.JobType),
		Payload:       json.RawMessage(s.Payload),
		Status:        entity.AsyncJobStatus(s.Status),
		ResultPayload: json.RawMessage(s.ResultPayload),
		LastError:     ptrFromText(s.LastError),
		ClaimedBy:     ptrFromText(s.ClaimedBy),
		ClaimedAt:     ptrFromTimestamptz(s.ClaimedAt),
		ExecutedAt:    ptrFromTimestamptz(s.ExecutedAt),
		HostID:        ptrFromText(s.HostID),
		SessionID:     ptrFromText(s.SessionID),
		CreatedBy:     ptrFromText(s.CreatedBy),
		CreatedAt:     s.CreatedAt.Time,
		UpdatedAt:     s.UpdatedAt.Time,
	}, nil
}
