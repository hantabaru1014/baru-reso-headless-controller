-- Revoke all privileges from fluentbit user
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM fluentbit;
REVOKE ALL PRIVILEGES ON SCHEMA public FROM fluentbit;
REVOKE CONNECT ON DATABASE brhcdb FROM fluentbit;

-- Drop fluentbit user
DROP USER IF EXISTS fluentbit;
