-- 管理者から利用者へのお知らせメッセージ (掲示板) テーブルを作成し、
-- 管理用 permission key (system:message.manage) を seed-system-admin に付与する。

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    -- NULL = 全員向け。指定時はそのグループのメンバーのみ閲覧可能。
    group_id TEXT REFERENCES groups(id) ON DELETE CASCADE,
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_updated_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_group_id ON messages(group_id) WHERE group_id IS NOT NULL;
CREATE INDEX idx_messages_updated_at ON messages(updated_at DESC);

CREATE TRIGGER update_messages_modtime
BEFORE UPDATE ON messages
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();

-- seed-system-admin へ system:message.manage を付与する。
-- role_permissions は builtin ロール保護トリガーが INSERT を阻止するため、
-- 一時的にトリガーを無効化して投入する。
ALTER TABLE role_permissions DISABLE TRIGGER protect_builtin_role_permissions_trg;

INSERT INTO role_permissions (role_id, permission_key)
VALUES ('seed-system-admin', 'system:message.manage')
ON CONFLICT DO NOTHING;

ALTER TABLE role_permissions ENABLE TRIGGER protect_builtin_role_permissions_trg;
