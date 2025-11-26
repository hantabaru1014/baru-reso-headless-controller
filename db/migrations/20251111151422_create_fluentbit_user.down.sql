-- Revoke all privileges from fluentbit user
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM fluentbit;
REVOKE ALL PRIVILEGES ON SCHEMA public FROM fluentbit;
DO $$
BEGIN
  EXECUTE 'REVOKE CONNECT ON DATABASE ' || current_database() || ' FROM fluentbit';
END
$$;

-- Drop fluentbit user
DROP USER IF EXISTS fluentbit;
