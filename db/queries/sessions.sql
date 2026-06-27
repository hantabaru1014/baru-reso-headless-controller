-- name: UpsertSession :one
INSERT INTO sessions (
    id,
    name,
    status,
    started_at,
    owner_id,
    ended_at,
    host_id,
    startup_parameters,
    startup_parameters_schema_version,
    auto_upgrade,
    memo
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    started_at = EXCLUDED.started_at,
    owner_id = EXCLUDED.owner_id,
    ended_at = EXCLUDED.ended_at,
    host_id = EXCLUDED.host_id,
    startup_parameters = EXCLUDED.startup_parameters,
    startup_parameters_schema_version = EXCLUDED.startup_parameters_schema_version,
    auto_upgrade = EXCLUDED.auto_upgrade,
    memo = EXCLUDED.memo
RETURNING *;

-- name: UpdateSessionStatus :exec
UPDATE sessions SET status = $2 WHERE id = $1;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1 LIMIT 1;

-- name: ListSessions :many
SELECT * FROM sessions ORDER BY started_at DESC;

-- name: ListSessionsByStatus :many
SELECT * FROM sessions WHERE status = $1 ORDER BY started_at DESC;

-- name: ListSessionsByHostAndStatus :many
SELECT * FROM sessions WHERE host_id = $1 AND status = $2 ORDER BY started_at DESC;

-- name: ApplySessionStarted :execrows
-- host から届く SessionStarted を反映する部分更新。
-- occurred_at が現在の started_at より新しい場合のみ更新する (idempotent / 巻き戻し防止)。
-- 他フィールド (memo, auto_upgrade, startup_parameters, owner_id 等) には触れない。
UPDATE sessions
SET
    name = $2,
    status = $3,
    started_at = $4,
    ended_at = NULL,
    host_id = $5
WHERE id = $1
  AND (started_at IS NULL OR started_at < $4);

-- name: ApplySessionEnded :execrows
-- host から届く SessionEnded を反映する部分更新。
-- occurred_at が現在の ended_at より新しい場合のみ更新する。
UPDATE sessions
SET
    status = $2,
    ended_at = $3
WHERE id = $1
  AND (ended_at IS NULL OR ended_at < $3);

-- name: ListSessionsPaged :many
-- ページング付きセッション一覧。
-- status / host_id は nullable パラメータ (sqlc.narg)。NULL なら未指定として扱う。
-- total_count は全行同じ値が入る。
SELECT sqlc.embed(sessions), COUNT(*) OVER() AS total_count
FROM sessions
WHERE (sqlc.narg('status')::int IS NULL OR status = sqlc.narg('status')::int)
  AND (sqlc.narg('host_id')::text IS NULL OR host_id = sqlc.narg('host_id')::text)
ORDER BY started_at DESC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
