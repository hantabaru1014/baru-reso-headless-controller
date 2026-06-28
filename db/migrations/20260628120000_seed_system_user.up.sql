-- システムユーザー (id='system') を作成し、system グループに seed-system-admin
-- ロールで登録する.
--
-- CLI / 内部 worker など「特定の利用者を持たない」操作はこのユーザーとして実行する.
-- 直接ログインは usecase 層でブロックする (password は空文字、id='system' を弾く).
INSERT INTO users (id, password, resonite_id, icon_url, created_at, updated_at)
VALUES ('system', '', NULL, NULL, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO group_members (group_id, user_id, role_id, added_by, joined_at)
VALUES ('system', 'system', 'seed-system-admin', NULL, NOW())
ON CONFLICT (group_id, user_id) DO NOTHING;
