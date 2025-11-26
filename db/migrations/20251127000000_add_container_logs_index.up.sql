CREATE INDEX IF NOT EXISTS idx_container_logs_tag_ts ON container_logs(tag, ts DESC);
