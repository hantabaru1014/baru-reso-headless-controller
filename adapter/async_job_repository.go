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

func (r *AsyncJobRepository) List(ctx context.Context, filter port.AsyncJobListFilter) (*port.AsyncJobListResult, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	params := db.ListAsyncJobsParams{
		CreatedBy:  textFromPtr(filter.CreatedBy),
		PageSize:   pageSize,
		PageOffset: filter.PageIndex * pageSize,
	}
	if filter.Status != nil {
		params.Status = pgtype.Int4{Int32: int32(*filter.Status), Valid: true}
	}

	if filter.JobType != nil {
		params.JobType = pgtype.Int4{Int32: int32(*filter.JobType), Valid: true}
	}

	rows, err := r.q.ListAsyncJobs(ctx, params)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	result := &port.AsyncJobListResult{
		Items: make(entity.AsyncJobList, 0, len(rows)),
	}
	if len(rows) > 0 {
		result.TotalCount = int32(rows[0].TotalCount) //nolint:gosec // G115: テーブル件数なので int32 範囲を超えない
	}

	for _, row := range rows {
		e, err := asyncJobToEntity(row.AsyncJob)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		result.Items = append(result.Items, e)
	}

	return result, nil
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

func (r *AsyncJobRepository) MarkFailed(ctx context.Context, id string, errMessage string, errDetail *string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}

	rows, err := r.q.MarkAsyncJobFailed(ctx, db.MarkAsyncJobFailedParams{
		ID:        uid,
		LastError: errMessage,
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "async_job", 0)
	}

	// RUNNING 以外なら UPDATE は空振りしている (stale claim を他インスタンスが
	// 拾い直して既に完了させた等). その job に詳細ログだけ付くのは不整合なので書かない.
	if rows == 0 || errDetail == nil || *errDetail == "" {
		return nil
	}

	// 詳細の保存に失敗しても job の FAILED 化自体は済んでいるので、ここは best-effort に
	// せずエラーを返す (呼び出し側が warn ログに落とす).
	if err := r.q.UpsertAsyncJobLog(ctx, db.UpsertAsyncJobLogParams{
		JobID:   uid,
		Content: *errDetail,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "async_job_log", 0)
	}

	return nil
}

func (r *AsyncJobRepository) GetLog(ctx context.Context, id string) (string, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return "", err
	}

	content, err := r.q.GetAsyncJobLog(ctx, uid)
	if err != nil {
		return "", errors.WrapPrefix(convertDBErr(err), "async_job_log", 0)
	}

	return content, nil
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
