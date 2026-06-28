-- 権限システム (RBAC) の DB 基盤を構築する。
-- 仕様: docs/permissions.md
--
-- 実施順序:
--   1. 新規テーブル (groups / roles / role_permissions / group_members) 作成
--   2. system グループ (singleton) 投入
--   3. seedロール + seedロール用 role_permissions 投入
--   4. 既存ユーザー全員分の personal グループ作成 + 本人を admin で登録
--   5. 移行前全体グループ (normal) を1つ作成 + 既存ユーザー全員を admin で登録
--   6. hosts / sessions / headless_accounts に group_id 追加・既存行を移行前全体グループへ紐付け
--   7. hosts.owner_id, sessions.owner_id を created_by へリネーム / headless_accounts.created_by 追加
--   8. seedロールガードのトリガーを作成 (※ seed投入後に作る。INSERT もガード対象に含める)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ============================================================
-- 1. groups
-- ============================================================
CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('personal', 'normal', 'system')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_groups_type ON groups(type);

CREATE TRIGGER update_groups_modtime
BEFORE UPDATE ON groups
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();

-- ============================================================
-- 2. roles
-- ============================================================
CREATE TABLE roles (
    id TEXT PRIMARY KEY,
    group_id TEXT REFERENCES groups(id) ON DELETE CASCADE, -- NULL = グローバル
    name TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('normal', 'system')),
    is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (group_id, name)
);

-- group_id IS NULL のグローバルロールについては UNIQUE (NULL, name) が機能しないため
-- 部分 UNIQUE インデックスでグローバル空間内の name 重複を防ぐ。
CREATE UNIQUE INDEX idx_roles_global_name ON roles(name) WHERE group_id IS NULL;
CREATE INDEX idx_roles_group_id ON roles(group_id) WHERE group_id IS NOT NULL;

CREATE TRIGGER update_roles_modtime
BEFORE UPDATE ON roles
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();

-- ============================================================
-- 3. role_permissions
-- ============================================================
CREATE TABLE role_permissions (
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_key TEXT NOT NULL,
    PRIMARY KEY (role_id, permission_key)
);

CREATE INDEX idx_role_permissions_permission_key ON role_permissions(permission_key);

-- ============================================================
-- 4. group_members
-- ============================================================
CREATE TABLE group_members (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id),
    added_by TEXT REFERENCES users(id) ON DELETE SET NULL, -- NULL: システム自動作成
    joined_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_group_members_user_id ON group_members(user_id);
CREATE INDEX idx_group_members_role_id ON group_members(role_id);

-- ============================================================
-- 5. system グループ singleton
-- ============================================================
INSERT INTO groups (id, name, type) VALUES ('system', 'system', 'system');

-- ============================================================
-- 6. seedロール (4種類) + role_permissions
-- ============================================================
INSERT INTO roles (id, group_id, name, scope, is_builtin) VALUES
    ('seed-admin',            NULL, 'admin',            'normal', TRUE),
    ('seed-user',             NULL, 'user',             'normal', TRUE),
    ('seed-session-operator', NULL, 'session-operator', 'normal', TRUE),
    ('seed-system-admin',     NULL, 'system-admin',     'system', TRUE);

-- normal scope のパーミッション
INSERT INTO role_permissions (role_id, permission_key) VALUES
    -- admin: host:*, session:*, account:*, group:members.manage, group:edit
    ('seed-admin', 'host:read'),
    ('seed-admin', 'host:write'),
    ('seed-admin', 'host:use'),
    ('seed-admin', 'session:read'),
    ('seed-admin', 'session:write'),
    ('seed-admin', 'account:read'),
    ('seed-admin', 'account:write'),
    ('seed-admin', 'account:use'),
    ('seed-admin', 'group:members.manage'),
    ('seed-admin', 'group:edit'),
    -- user: host:*, session:*, account:*
    ('seed-user', 'host:read'),
    ('seed-user', 'host:write'),
    ('seed-user', 'host:use'),
    ('seed-user', 'session:read'),
    ('seed-user', 'session:write'),
    ('seed-user', 'account:read'),
    ('seed-user', 'account:write'),
    ('seed-user', 'account:use'),
    -- session-operator: host:read, host:use, session:*, account:read, account:use
    ('seed-session-operator', 'host:read'),
    ('seed-session-operator', 'host:use'),
    ('seed-session-operator', 'session:read'),
    ('seed-session-operator', 'session:write'),
    ('seed-session-operator', 'account:read'),
    ('seed-session-operator', 'account:use'),
    -- system-admin: system:*
    ('seed-system-admin', 'system:user.create'),
    ('seed-system-admin', 'system:user.delete'),
    ('seed-system-admin', 'system:user.list'),
    ('seed-system-admin', 'system:group.list'),
    ('seed-system-admin', 'system:group.manage'),
    ('seed-system-admin', 'system:role.manage');

-- ============================================================
-- 7. 既存ユーザー分の personal グループ + admin メンバーシップ
-- id, name とも "<user-id>-personal" 形式 (spec 2.2)
-- ============================================================
INSERT INTO groups (id, name, type)
SELECT id || '-personal', id || '-personal', 'personal' FROM users;

INSERT INTO group_members (group_id, user_id, role_id, added_by)
SELECT id || '-personal', id, 'seed-admin', NULL FROM users;

-- ============================================================
-- 8. 移行前全体グループ (normal) + 既存ユーザー全員を admin で投入
-- 既存リソース (hosts / sessions / headless_accounts) はすべてここへ紐付ける
-- ============================================================
INSERT INTO groups (id, name, type)
VALUES ('migrated-pre-permission', '移行前全体グループ', 'normal');

INSERT INTO group_members (group_id, user_id, role_id, added_by)
SELECT 'migrated-pre-permission', id, 'seed-admin', NULL FROM users;

-- ============================================================
-- 9. hosts: group_id 追加 / owner_id → created_by リネーム
--   既存 owner_id は FK 制約が無かったため、リネーム後に明示 FK を追加
-- ============================================================
ALTER TABLE hosts ADD COLUMN group_id TEXT REFERENCES groups(id);
UPDATE hosts SET group_id = 'migrated-pre-permission';
ALTER TABLE hosts ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE hosts RENAME COLUMN owner_id TO created_by;
-- 既存 owner_id には FK が無かったので削除済みユーザーへの orphan が混じる可能性が
-- ある. ADD CONSTRAINT は FK 違反でトランザクション全体を abort するため、
-- 事前に orphan を NULL に倒しておく.
UPDATE hosts SET created_by = NULL
    WHERE created_by IS NOT NULL
      AND created_by NOT IN (SELECT id FROM users);
ALTER TABLE hosts ADD CONSTRAINT hosts_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_hosts_group_id ON hosts(group_id);

-- ============================================================
-- 10. sessions: group_id 追加 / owner_id → created_by リネーム
-- ============================================================
ALTER TABLE sessions ADD COLUMN group_id TEXT REFERENCES groups(id);
UPDATE sessions SET group_id = 'migrated-pre-permission';
ALTER TABLE sessions ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE sessions RENAME COLUMN owner_id TO created_by;
-- 既存 owner_id には FK が無かったので削除済みユーザーへの orphan が混じる可能性が
-- ある. hosts と同様に backfill しておく.
UPDATE sessions SET created_by = NULL
    WHERE created_by IS NOT NULL
      AND created_by NOT IN (SELECT id FROM users);
ALTER TABLE sessions ADD CONSTRAINT sessions_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_sessions_group_id ON sessions(group_id);

-- ============================================================
-- 11. headless_accounts: group_id 追加 (NOT NULL) / created_by 新規追加 (nullable)
-- ============================================================
ALTER TABLE headless_accounts ADD COLUMN group_id TEXT REFERENCES groups(id);
UPDATE headless_accounts SET group_id = 'migrated-pre-permission';
ALTER TABLE headless_accounts ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE headless_accounts ADD COLUMN created_by TEXT
    REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_headless_accounts_group_id ON headless_accounts(group_id);

-- ============================================================
-- 12. seedロール保護トリガー
--   - roles: builtin 行の UPDATE / DELETE を禁止
--   - role_permissions: builtin ロールに紐づく行の INSERT / UPDATE / DELETE を禁止
--   ※ seed投入後に作成しているため、初期投入は触れない
--   ※ アプリ層でも事前にチェックすべき (DBレベルは最後の防壁)
-- ============================================================
CREATE OR REPLACE FUNCTION protect_builtin_roles()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.is_builtin THEN
            RAISE EXCEPTION 'cannot delete builtin role: %', OLD.id
                USING ERRCODE = 'check_violation';
        END IF;
        RETURN OLD;
    END IF;
    -- UPDATE: 既存行が builtin なら name/scope/group_id/is_builtin の変更を禁止
    -- updated_at だけ動く更新は実害なしのため許容 (トリガー走るが何も弾かない)
    IF OLD.is_builtin THEN
        IF NEW.name IS DISTINCT FROM OLD.name
           OR NEW.scope IS DISTINCT FROM OLD.scope
           OR NEW.group_id IS DISTINCT FROM OLD.group_id
           OR NEW.is_builtin IS DISTINCT FROM OLD.is_builtin THEN
            RAISE EXCEPTION 'cannot update builtin role: %', OLD.id
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER protect_builtin_roles_trg
BEFORE UPDATE OR DELETE ON roles
FOR EACH ROW
EXECUTE PROCEDURE protect_builtin_roles();

CREATE OR REPLACE FUNCTION protect_builtin_role_permissions()
RETURNS TRIGGER AS $$
DECLARE
    role_is_builtin BOOLEAN;
    target_role_id TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_role_id := OLD.role_id;
    ELSE
        target_role_id := NEW.role_id;
    END IF;
    SELECT is_builtin INTO role_is_builtin FROM roles WHERE id = target_role_id;
    -- 親 role が既に消えているケース (CASCADE DELETE) では role_is_builtin が NULL
    -- → IS TRUE の判定により素通り
    IF role_is_builtin IS TRUE THEN
        RAISE EXCEPTION 'cannot modify permissions of builtin role: %', target_role_id
            USING ERRCODE = 'check_violation';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER protect_builtin_role_permissions_trg
BEFORE INSERT OR UPDATE OR DELETE ON role_permissions
FOR EACH ROW
EXECUTE PROCEDURE protect_builtin_role_permissions();
