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
-- session_id / host_id / status / group_ids は nullable パラメータ。NULL なら未指定として扱う。
-- group_ids は対象 host (session 対象の op は session の host) の group_id で ANY 絞り込み。
-- 空配列なら結果ゼロ件。対象 host が削除済みの op は group_ids 指定時にはヒットしない。
-- total_count は全行同じ値が入る (COUNT(*) OVER())。
-- 並び順:
--   1. 未完了 (PENDING / RUNNING) を先頭に、未実行のものを next_fire_at 昇順 → 実行済みのものを executed_at 降順。
--   2. 完了 (SUCCEEDED / FAILED / CANCELED) を最後のアクティビティ (updated_at) 降順。
--      終了状態の行は以降 UPDATE されない (全 UPDATE が status 0/1 を guard している) ため、
--      updated_at = 完了・キャンセルした時刻になる。
SELECT sqlc.embed(o), COUNT(*) OVER() AS total_count
FROM scheduled_session_operations o
LEFT JOIN sessions s ON s.id = o.session_id
LEFT JOIN hosts h ON h.id = COALESCE(o.host_id, s.host_id)
WHERE (sqlc.narg('session_id')::text IS NULL OR o.session_id = sqlc.narg('session_id')::text)
  AND (sqlc.narg('host_id')::text    IS NULL OR o.host_id    = sqlc.narg('host_id')::text)
  AND (sqlc.narg('status')::int       IS NULL OR o.status    = sqlc.narg('status')::int)
  AND (sqlc.narg('group_ids')::text[] IS NULL OR h.group_id = ANY(sqlc.narg('group_ids')::text[]))
ORDER BY (o.status IN (2, 3, 4)) ASC,
         CASE WHEN o.status IN (2, 3, 4) THEN o.updated_at END DESC,
         o.executed_at DESC NULLS FIRST, o.next_fire_at ASC, o.id ASC
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

-- name: RescheduleScheduledSessionOperation :execrows
-- 繰り返し trigger の実行後に、実行結果を記録して次回の発火時刻で PENDING に戻す。
-- last_error は成功時 NULL、失敗時はそのエラー (次回の実行結果で上書きされる)。
UPDATE scheduled_session_operations
SET status = 0, next_fire_at = @next_fire_at::timestamptz, executed_at = NOW(),
    last_error = sqlc.narg('last_error')::text, claimed_by = NULL, claimed_at = NULL
WHERE id = $1 AND status = 1;

-- name: CancelScheduledSessionOperation :execrows
-- PENDING のみキャンセル可能。RUNNING / SUCCEEDED / FAILED / CANCELED は呼び出し側で FailedPrecondition。
UPDATE scheduled_session_operations
SET status = 4
WHERE id = $1 AND status = 0;
