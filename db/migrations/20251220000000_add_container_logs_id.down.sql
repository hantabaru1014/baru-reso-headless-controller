DROP INDEX IF EXISTS idx_container_logs_tag_id;
ALTER TABLE container_logs DROP COLUMN IF EXISTS id;
