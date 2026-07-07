-- name: CreateMessage :one
INSERT INTO messages (id, title, body, group_id, created_by, last_updated_by)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING *;

-- name: GetMessage :one
SELECT sqlc.embed(messages), g.name AS group_name
FROM messages
LEFT JOIN groups g ON g.id = messages.group_id
WHERE messages.id = $1
LIMIT 1;

-- name: ListAllMessages :many
-- system:message.manage 保持者向け: 全メッセージを updated_at 降順で返す。
SELECT sqlc.embed(messages), g.name AS group_name
FROM messages
LEFT JOIN groups g ON g.id = messages.group_id
ORDER BY messages.updated_at DESC;

-- name: ListMessagesVisibleToUser :many
-- 全員向け (group_id IS NULL) + ユーザーが所属するグループ向けを updated_at 降順で返す。
SELECT sqlc.embed(messages), g.name AS group_name
FROM messages
LEFT JOIN groups g ON g.id = messages.group_id
WHERE messages.group_id IS NULL
   OR messages.group_id IN (SELECT gm.group_id FROM group_members gm WHERE gm.user_id = $1)
ORDER BY messages.updated_at DESC;

-- name: UpdateMessage :exec
-- title / body / group_id を完全置換する。updated_at はトリガーで更新される。
UPDATE messages
SET title = $2, body = $3, group_id = $4, last_updated_by = $5
WHERE id = $1;

-- name: DeleteMessage :exec
DELETE FROM messages WHERE id = $1;
