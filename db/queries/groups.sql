-- name: CreateGroup :one
INSERT INTO groups (id, name, type) VALUES ($1, $2, $3) RETURNING *;

-- name: GetGroup :one
SELECT * FROM groups WHERE id = $1 LIMIT 1;

-- name: ListGroups :many
SELECT * FROM groups ORDER BY created_at DESC;

-- name: ListGroupsByType :many
SELECT * FROM groups WHERE type = $1 ORDER BY created_at DESC;

-- name: ListGroupsPaged :many
-- ページング付きグループ一覧。type は nullable フィルタ。total_count は全行同じ値が入る。
SELECT sqlc.embed(groups), COUNT(*) OVER() AS total_count
FROM groups
WHERE (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type')::text)
ORDER BY created_at DESC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: ListGroupsByUser :many
-- 指定ユーザーが所属しているグループ一覧 (ロール情報も同梱)
SELECT sqlc.embed(groups), gm.role_id, gm.joined_at
FROM groups
INNER JOIN group_members gm ON gm.group_id = groups.id
WHERE gm.user_id = $1
ORDER BY groups.created_at DESC;

-- name: UpdateGroupName :exec
-- name の更新は normal グループのみ許可される (アプリ層でチェック)
UPDATE groups SET name = $2 WHERE id = $1;

-- name: DeleteGroup :exec
-- personal / system グループの削除はアプリ層で禁止する
DELETE FROM groups WHERE id = $1;

-- name: GetPersonalGroupByUser :one
-- ユーザーの personal グループを取得 (1ユーザーにつき高々1つ)
SELECT g.* FROM groups g
INNER JOIN group_members gm ON gm.group_id = g.id
WHERE g.type = 'personal' AND gm.user_id = $1
LIMIT 1;
