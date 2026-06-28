-- name: AddGroupMember :one
INSERT INTO group_members (group_id, user_id, role_id, added_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RemoveGroupMember :exec
DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;

-- name: UpdateGroupMemberRole :exec
-- personal グループのロール変更は system:group.manage が必要 (アプリ層で検査)
UPDATE group_members SET role_id = $3 WHERE group_id = $1 AND user_id = $2;

-- name: GetGroupMember :one
SELECT * FROM group_members WHERE group_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListGroupMembers :many
SELECT * FROM group_members WHERE group_id = $1 ORDER BY joined_at;

-- name: ListGroupMembersPaged :many
-- ページング付きメンバー一覧。total_count は全行同じ値が入る。
SELECT sqlc.embed(group_members), COUNT(*) OVER() AS total_count
FROM group_members
WHERE group_id = @group_id::text
ORDER BY joined_at
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: ListUserGroupMemberships :many
-- 指定ユーザーが所属する (group, role) ペア一覧
SELECT * FROM group_members WHERE user_id = $1 ORDER BY joined_at;

-- name: CountGroupMembersByRole :one
-- 「system グループの最後の system-admin を削除しようとしているか」等の判定用
SELECT COUNT(*) FROM group_members
WHERE group_id = $1 AND role_id = $2;
