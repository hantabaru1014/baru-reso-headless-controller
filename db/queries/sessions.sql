-- name: UpsertSession :one
INSERT INTO sessions (
    id,
    name,
    status,
    started_at,
    ended_at,
    host_id,
    startup_parameters,
    auto_upgrade
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    started_at = EXCLUDED.started_at,
    ended_at = EXCLUDED.ended_at,
    host_id = EXCLUDED.host_id,
    startup_parameters = EXCLUDED.startup_parameters,
    auto_upgrade = EXCLUDED.auto_upgrade
RETURNING *;

-- name: UpdateSessionStatus :exec
UPDATE sessions SET status = $2 WHERE id = $1;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1 LIMIT 1;

-- name: ListSessions :many
SELECT * FROM sessions ORDER BY started_at DESC;

-- name: ListSessionsByStatus :many
SELECT * FROM sessions WHERE status = $1 ORDER BY started_at DESC;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
