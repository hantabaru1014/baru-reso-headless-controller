-- name: ListHosts :many
SELECT * FROM hosts ORDER BY started_at DESC;

-- name: ListHostsByStatus :many
SELECT * FROM hosts WHERE status = $1 ORDER BY started_at DESC;

-- name: ListRunningHostsByAccount :many
SELECT * FROM hosts WHERE account_id = $1 AND status = 2 ORDER BY started_at DESC;

-- name: GetHost :one
SELECT * FROM hosts WHERE id = $1 LIMIT 1;

-- name: CreateHost :one
INSERT INTO hosts (
    id,
    name,
    status,
    account_id,
    owner_id,
    last_startup_config,
    last_startup_config_schema_version,
    connector_type,
    connect_string,
    started_at,
    auto_update_policy,
    memo,
    instance_count
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: UpdateHostStatus :exec
UPDATE hosts SET status = $2 WHERE id = $1;

-- name: UpdateHostName :exec
UPDATE hosts SET name = $2 WHERE id = $1;

-- name: UpdateHostLastStartupConfig :exec
UPDATE hosts SET last_startup_config = $2 WHERE id = $1;

-- name: UpdateHostStartedAt :exec
UPDATE hosts SET started_at = $2 WHERE id = $1;

-- name: UpdateHostMemo :exec
UPDATE hosts SET memo = $2 WHERE id = $1;

-- name: UpdateHostAutoUpdatePolicy :exec
UPDATE hosts SET auto_update_policy = $2 WHERE id = $1;

-- name: UpdateHostConnectString :exec
UPDATE hosts SET connect_string = $2 WHERE id = $1;

-- name: DeleteHost :exec
DELETE FROM hosts WHERE id = $1;

-- name: IncrementHostInstanceCount :one
UPDATE hosts SET instance_count = instance_count + 1 WHERE id = $1 RETURNING instance_count;
