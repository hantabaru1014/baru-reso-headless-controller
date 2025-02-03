CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ language 'plpgsql';

-- すでにあるテーブルにトリガーを追加する
CREATE TRIGGER update_users_modtime
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();

CREATE TRIGGER update_headless_accounts_modtime
BEFORE UPDATE ON headless_accounts
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
