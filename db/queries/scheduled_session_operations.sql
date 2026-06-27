-- name: CreateScheduledSessionOperation :one
INSERT INTO scheduled_session_operations (
    operation_type,
    operation_payload,
    trigger_type,
    trigger_config,
    next_fire_at,
    host_id,
    session_id,
    created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetScheduledSessionOperation :one
SELECT * FROM scheduled_session_operations WHERE id = $1 LIMIT 1;

-- name: ListScheduledSessionOperations :many
-- session_id / host_id / status は nullable パラメータ。NULL なら未指定として扱う。
-- total_count は全行同じ値が入る (COUNT(*) OVER())。
SELECT sqlc.embed(scheduled_session_operations), COUNT(*) OVER() AS total_count
FROM scheduled_session_operations
WHERE (sqlc.narg('session_id')::text IS NULL OR session_id = sqlc.narg('session_id')::text)
  AND (sqlc.narg('host_id')::text    IS NULL OR host_id    = sqlc.narg('host_id')::text)
  AND (sqlc.narg('status')::int       IS NULL OR status    = sqlc.narg('status')::int)
ORDER BY next_fire_at ASC, id ASC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: ClaimDueScheduledSessionOperations :many
-- 1つのtxで原子的にclaim。FOR UPDATE SKIP LOCKED で他インスタンスとの競合を回避。
UPDATE scheduled_session_operations
SET status = 1, claimed_by = @instance_id::text, claimed_at = NOW()
WHERE id IN (
    SELECT id FROM scheduled_session_operations
    WHERE status = 0 AND next_fire_at <= NOW()
    ORDER BY next_fire_at
    LIMIT @batch_size::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: ReleaseStaleScheduledSessionOperationClaims :execrows
-- 落ちた instance が残した RUNNING 行を PENDING に戻す (クラッシュ救済)。
UPDATE scheduled_session_operations
SET status = 0, claimed_by = NULL, claimed_at = NULL
WHERE status = 1
  AND claimed_at IS NOT NULL
  AND claimed_at < NOW() - make_interval(secs => @stale_after_seconds::int);

-- name: MarkScheduledSessionOperationSucceeded :execrows
-- RUNNING の行のみ SUCCEEDED にする (cancel と race しないため status guard)。
UPDATE scheduled_session_operations
SET status = 2, executed_at = NOW(), last_error = NULL, claimed_by = NULL, claimed_at = NULL
WHERE id = $1 AND status = 1;

-- name: MarkScheduledSessionOperationFailed :execrows
UPDATE scheduled_session_operations
SET status = 3, executed_at = NOW(), last_error = @last_error::text, claimed_by = NULL, claimed_at = NULL
WHERE id = $1 AND status = 1;

-- name: RequeueScheduledSessionOperation :execrows
-- Condition trigger の "未だ ready ではない" 経路。RUNNING の行を PENDING に戻し、次回再評価時刻を更新する。
UPDATE scheduled_session_operations
SET status = 0, next_fire_at = @next_fire_at::timestamptz, claimed_by = NULL, claimed_at = NULL
WHERE id = $1 AND status = 1;

-- name: CancelScheduledSessionOperation :execrows
-- PENDING のみキャンセル可能。RUNNING / SUCCEEDED / FAILED / CANCELED は呼び出し側で FailedPrecondition。
UPDATE scheduled_session_operations
SET status = 4
WHERE id = $1 AND status = 0;
