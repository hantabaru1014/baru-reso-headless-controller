-- 11/10/9. 既存テーブルの変更を巻き戻す
DROP INDEX IF EXISTS idx_headless_accounts_group_id;
ALTER TABLE headless_accounts DROP COLUMN IF EXISTS created_by;
ALTER TABLE headless_accounts DROP COLUMN IF EXISTS group_id;

DROP INDEX IF EXISTS idx_sessions_group_id;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_created_by_fkey;
ALTER TABLE sessions RENAME COLUMN created_by TO owner_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS group_id;

DROP INDEX IF EXISTS idx_hosts_group_id;
ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_created_by_fkey;
ALTER TABLE hosts RENAME COLUMN created_by TO owner_id;
ALTER TABLE hosts DROP COLUMN IF EXISTS group_id;

-- 12. seedロール保護トリガー / 関数
-- DROP TABLE で巻き取られるが、関数だけは残るので明示的に削除する
DROP TRIGGER IF EXISTS protect_builtin_role_permissions_trg ON role_permissions;
DROP TRIGGER IF EXISTS protect_builtin_roles_trg ON roles;

-- 4. group_members
DROP TABLE IF EXISTS group_members;

-- 3. role_permissions
DROP TABLE IF EXISTS role_permissions;
DROP FUNCTION IF EXISTS protect_builtin_role_permissions();

-- 2. roles
DROP TRIGGER IF EXISTS update_roles_modtime ON roles;
DROP TABLE IF EXISTS roles;
DROP FUNCTION IF EXISTS protect_builtin_roles();

-- 1. groups
DROP TRIGGER IF EXISTS update_groups_modtime ON groups;
DROP TABLE IF EXISTS groups;
