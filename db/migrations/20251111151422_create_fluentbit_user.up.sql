-- Create fluentbit user if not exists (password will be set by setup script)
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_catalog.pg_user WHERE usename = 'fluentbit') THEN
    CREATE USER fluentbit;
  END IF;
END
$$;

-- Grant connect privilege to the database
DO $$
BEGIN
  EXECUTE 'GRANT CONNECT ON DATABASE ' || current_database() || ' TO fluentbit';
END
$$;

-- Grant usage and create privileges on public schema
GRANT USAGE, CREATE ON SCHEMA public TO fluentbit;

-- Grant insert and select privileges on container_logs table
GRANT INSERT, SELECT ON TABLE container_logs TO fluentbit;
