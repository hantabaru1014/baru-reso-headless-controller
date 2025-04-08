CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status INTEGER NOT NULL, -- domain/entity/session.go のenum
    started_at TIMESTAMP WITH TIME ZONE,
    owner_id TEXT, -- users.id
    ended_at TIMESTAMP WITH TIME ZONE,
    host_id TEXT NOT NULL,
    startup_parameters JSONB NOT NULL,
    startup_parameters_schema_version INTEGER NOT NULL,
    auto_upgrade BOOLEAN NOT NULL DEFAULT FALSE,
    memo TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Apply update_timestamp trigger
CREATE TRIGGER update_sessions_modtime
BEFORE UPDATE ON sessions
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
