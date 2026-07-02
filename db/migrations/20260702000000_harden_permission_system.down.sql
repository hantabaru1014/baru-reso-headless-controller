DROP INDEX IF EXISTS groups_singleton_system_idx;

ALTER TABLE registration_tokens
  DROP CONSTRAINT IF EXISTS registration_tokens_token_is_hash;

-- ハッシュ保存に切り替えたトークンは平文に戻せないため破棄する.
DELETE FROM registration_tokens;
