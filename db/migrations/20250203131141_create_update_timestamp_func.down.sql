DROP TRIGGER IF EXISTS update_users_modtime ON users;
DROP TRIGGER IF EXISTS update_headless_accounts_modtime ON headless_accounts;

DROP FUNCTION IF EXISTS update_timestamp();
