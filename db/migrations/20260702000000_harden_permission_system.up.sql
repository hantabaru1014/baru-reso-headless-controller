-- system グループの singleton をスキーマレベルで強制する.
-- アプリ層の規約 (SystemGroupID='system' の固定 ID) に加えた多層防御.
CREATE UNIQUE INDEX groups_singleton_system_idx ON groups (type) WHERE type = 'system';

-- 登録トークンを平文保存から SHA-256 ハッシュ保存に切り替える.
-- 既存の平文トークンはハッシュへ変換できないため破棄する (TTL 24h の招待
-- トークンのみで、無効化しても再発行すれば良い).
-- NOTE: ローリングデプロイ中は旧バイナリが本 migration 適用後も平文トークンを
-- 発行しうる. token 列に SHA-256 hex 形式の CHECK 制約を課すことで、旧バイナリの
-- 平文 INSERT を「静かに検証不能な招待」ではなく即エラー (fail loud) にする.
-- デプロイ前に配布済みの招待リンクは無効化されるため、運用側で再発行が必要.
DELETE FROM registration_tokens;

ALTER TABLE registration_tokens
  ADD CONSTRAINT registration_tokens_token_is_hash
  CHECK (token ~ '^[0-9a-f]{64}$');
