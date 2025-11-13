CREATE TABLE IF NOT EXISTS container_logs (
    tag TEXT,
    ts TIMESTAMP WITHOUT TIME ZONE,
    data JSONB
);

CREATE INDEX IF NOT EXISTS idx_container_logs_ts ON container_logs(ts);
