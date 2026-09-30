-- name: ListHosts :many
SELECT * FROM hosts ORDER BY started_at DESC;

-- name: ListHostsPaged :many
-- ページング付きホスト一覧。total_count は全行同じ値が入る。
-- group_ids は nullable パラメータ (sqlc.narg)。NULL の場合は全グループ対象。
-- 空配列を渡すと結果ゼロ件 (= 所属グループが無いユーザーに対する自動絞り込み)。
SELECT sqlc.embed(hosts), COUNT(*) OVER() AS total_count
FROM hosts
WHERE (sqlc.narg('group_ids')::text[] IS NULL OR group_id = ANY(sqlc.narg('group_ids')::text[]))
ORDER BY started_at DESC NULLS LAST, id ASC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: ListHostsByStatus :many
SELECT * FROM hosts WHERE status = $1 ORDER BY started_at DESC;

-- name: ListRunningHostsByAccount :many
-- アカウントは (group_id, resonite_id) で一意なので、ホストも group_id 込みで引く.
SELECT * FROM hosts WHERE group_id = $1 AND account_id = $2 AND status = 2 ORDER BY started_at DESC;

-- name: ListHostsByAccount :many
SELECT * FROM hosts WHERE group_id = $1 AND account_id = $2 ORDER BY started_at DESC NULLS LAST, id ASC;

-- name: UpdateHostsGroupByAccount :exec
-- グループ間移管用. 指定アカウントを使う全ホストをまとめて別グループへ移す.
UPDATE hosts SET group_id = @new_group_id::text WHERE group_id = @group_id::text AND account_id = @account_id::text;

-- name: GetHost :one
SELECT * FROM hosts WHERE id = $1 LIMIT 1;

-- name: CreateHost :one
INSERT INTO hosts (
    id,
    name,
    status,
    account_id,
    created_by,
    last_startup_config,
    last_startup_config_schema_version,
    connector_type,
    connect_string,
    started_at,
    auto_update_policy,
    memo,
    instance_count,
    group_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
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

-- name: GetHostByContainerID :one
SELECT * FROM hosts WHERE connect_string LIKE $1 || ':%' LIMIT 1;
