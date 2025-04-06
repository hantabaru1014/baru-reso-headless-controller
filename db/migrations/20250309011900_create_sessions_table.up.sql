CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status INTEGER NOT NULL, -- 0: ended, 1: running
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    host_id TEXT NOT NULL,
    startup_parameters JSONB NOT NULL,
    auto_upgrade BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Apply update_timestamp trigger
CREATE TRIGGER update_sessions_modtime
BEFORE UPDATE ON sessions
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
