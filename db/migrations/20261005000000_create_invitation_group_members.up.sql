-- 招待 (registration_tokens) を参照するための不変 ID を追加する.
-- token 列はハッシュで、再発行時に差し替わるため参照キーには使えない.
ALTER TABLE registration_tokens
    ADD COLUMN id TEXT NOT NULL DEFAULT gen_random_uuid()::text;
ALTER TABLE registration_tokens ADD CONSTRAINT registration_tokens_id_key UNIQUE (id);

-- 招待中 (未登録) ユーザーのグループ参加予定.
-- RegisterWithToken 時に group_members へ移し、招待の削除に追随して消える.
CREATE TABLE invitation_group_members (
    invitation_id TEXT NOT NULL REFERENCES registration_tokens(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id),
    added_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    added_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, invitation_id)
);

CREATE INDEX idx_invitation_group_members_invitation_id ON invitation_group_members(invitation_id);
CREATE INDEX idx_invitation_group_members_role_id ON invitation_group_members(role_id);
