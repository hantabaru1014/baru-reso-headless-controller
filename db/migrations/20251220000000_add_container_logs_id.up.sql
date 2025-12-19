-- Add serial id column to container_logs for reliable cursor-based pagination
-- Note: Multiple containers write logs to this table, so id order within a tag matches insertion order
ALTER TABLE container_logs ADD COLUMN id BIGSERIAL;

-- Create index for efficient id-based queries filtered by tag
CREATE INDEX IF NOT EXISTS idx_container_logs_tag_id ON container_logs(tag, id DESC);

-- Grant usage on the sequence to fluentbit user (required for BIGSERIAL INSERT)
GRANT USAGE, SELECT ON SEQUENCE container_logs_id_seq TO fluentbit;
