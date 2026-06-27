CREATE TABLE host_event_checkpoints (
    host_id TEXT PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
    last_event_id TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_host_event_checkpoints_modtime
BEFORE UPDATE ON host_event_checkpoints
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
