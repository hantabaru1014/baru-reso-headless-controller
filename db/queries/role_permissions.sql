-- name: AddRolePermission :exec
-- seedロールへの追加は DB トリガーが阻止する。
INSERT INTO role_permissions (role_id, permission_key) VALUES ($1, $2)
ON CONFLICT (role_id, permission_key) DO NOTHING;

-- name: RemoveRolePermission :exec
-- seedロールからの削除は DB トリガーが阻止する。
DELETE FROM role_permissions WHERE role_id = $1 AND permission_key = $2;

-- name: ListRolePermissions :many
SELECT permission_key FROM role_permissions WHERE role_id = $1 ORDER BY permission_key;

-- name: ListRolePermissionsForRoles :many
-- 複数ロール分まとめて取得。permission インターセプタで使う想定。
SELECT role_id, permission_key
FROM role_permissions
WHERE role_id = ANY(@role_ids::text[])
ORDER BY role_id, permission_key;

-- name: GetUserPermissionsForGroup :many
-- 指定 (user, group) に対する permission_key を引く。
-- group に所属していない場合は空集合が返る。
SELECT DISTINCT rp.permission_key
FROM group_members gm
INNER JOIN role_permissions rp ON rp.role_id = gm.role_id
WHERE gm.user_id = @user_id::text
  AND gm.group_id = @group_id::text;

-- name: ListUserSystemPermissions :many
-- 指定ユーザーが system グループ経由で保持する permission_key 一覧。
-- (system:* の判定や、system:group.manage による personal/normal の代行権限判定に使う)
SELECT DISTINCT rp.permission_key
FROM group_members gm
INNER JOIN role_permissions rp ON rp.role_id = gm.role_id
WHERE gm.user_id = @user_id::text
  AND gm.group_id = 'system';
