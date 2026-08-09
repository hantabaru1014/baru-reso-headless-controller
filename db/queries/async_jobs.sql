-- name: CreateAsyncJob :one
INSERT INTO async_jobs (
    job_type,
    payload,
    host_id,
    session_id,
    created_by
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetAsyncJob :one
SELECT * FROM async_jobs WHERE id = $1 LIMIT 1;

-- name: ListAsyncJobs :many
-- status / job_type / created_by は nullable パラメータ。NULL なら未指定として扱う。
-- created_by は「誰の job を見せるか」の認可結果が入る (usecase 層が決める)。
-- total_count は全行同じ値が入る (COUNT(*) OVER())。
SELECT sqlc.embed(async_jobs), COUNT(*) OVER() AS total_count
FROM async_jobs
WHERE (sqlc.narg('status')::int      IS NULL OR status     = sqlc.narg('status')::int)
  AND (sqlc.narg('job_type')::int    IS NULL OR job_type   = sqlc.narg('job_type')::int)
  AND (sqlc.narg('created_by')::text IS NULL OR created_by = sqlc.narg('created_by')::text)
ORDER BY created_at DESC, id DESC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: ClaimDueAsyncJobs :many
-- 1つのtxで原子的にclaim。FOR UPDATE SKIP LOCKED で他インスタンスとの競合を回避。
-- 古い PENDING ジョブから順に最大 batch_size 件を RUNNING に遷移して返す。
UPDATE async_jobs
SET status = 1, claimed_by = @instance_id::text, claimed_at = NOW()
WHERE id IN (
    SELECT id FROM async_jobs
    WHERE status = 0
    ORDER BY created_at
    LIMIT @batch_size::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: ReleaseStaleAsyncJobClaims :execrows
-- 落ちた instance が残した RUNNING 行を PENDING に戻す (クラッシュ救済)。
UPDATE async_jobs
SET status = 0, claimed_by = NULL, claimed_at = NULL
WHERE status = 1
  AND claimed_at IS NOT NULL
  AND claimed_at < NOW() - make_interval(secs => @stale_after_seconds::int);

-- name: MarkAsyncJobSucceeded :execrows
-- RUNNING の行のみ SUCCEEDED にする。result_payload は呼び出し側で必要なら詰める。
UPDATE async_jobs
SET status = 2, executed_at = NOW(), result_payload = @result_payload::jsonb, last_error = NULL, claimed_by = NULL, claimed_at = NULL
WHERE id = $1 AND status = 1;

-- name: MarkAsyncJobFailed :execrows
UPDATE async_jobs
SET status = 3, executed_at = NOW(), last_error = @last_error::text, claimed_by = NULL, claimed_at = NULL
WHERE id = $1 AND status = 1;

-- name: UpsertAsyncJobLog :exec
-- 失敗時の詳細ログ (builder のフルログ等)。job あたり1本。
INSERT INTO async_job_logs (job_id, content)
VALUES ($1, @content::text)
ON CONFLICT (job_id) DO UPDATE SET content = EXCLUDED.content;

-- name: GetAsyncJobLog :one
SELECT content FROM async_job_logs WHERE job_id = $1;
