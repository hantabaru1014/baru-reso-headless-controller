-- name: CreateRole :one
-- group_id が NULL ならグローバルカスタム、NOT NULL ならグループ内カスタム。
-- seedロール (is_builtin=true) はマイグレーション時のみ作成可能で、本クエリでは作らせない想定。
INSERT INTO roles (id, group_id, name, scope, is_builtin)
VALUES ($1, $2, $3, $4, FALSE)
RETURNING *;

-- name: GetRole :one
SELECT * FROM roles WHERE id = $1 LIMIT 1;

-- name: ListGlobalRoles :many
-- seed + グローバルカスタム
SELECT * FROM roles WHERE group_id IS NULL ORDER BY is_builtin DESC, name;

-- name: ListRolesByGroup :many
-- 指定グループ内のカスタムロールのみ (グローバルは含まない)
SELECT * FROM roles WHERE group_id = $1 ORDER BY name;

-- name: ListAssignableRoles :many
-- 指定グループに割り当て可能なロール (グローバル + 当該グループ内ロール) を scope で絞る
SELECT * FROM roles
WHERE (group_id IS NULL OR group_id = sqlc.narg('group_id')::text)
  AND scope = @scope::text
ORDER BY is_builtin DESC, group_id NULLS FIRST, name;

-- name: UpdateRoleName :exec
-- seedロール (is_builtin=true) は DB トリガーが阻止する。アプリ層でも事前にチェック。
UPDATE roles SET name = $2 WHERE id = $1;

-- name: DeleteRole :exec
-- seedロール (is_builtin=true) は DB トリガーが阻止する。
DELETE FROM roles WHERE id = $1;
