CREATE TABLE hosts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status INTEGER NOT NULL, -- domain/entity/headless_host.go のenum
    account_id TEXT NOT NULL, -- headless_accounts.resonite_id
    owner_id TEXT, -- users.id
    last_startup_config JSONB NOT NULL,
    last_startup_config_schema_version INTEGER NOT NULL,
    connector_type TEXT NOT NULL,
    connect_string TEXT NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE,
    memo TEXT,
    auto_update_policy INTEGER NOT NULL, -- domain/entity/headless_host.go のenum
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Apply update_timestamp trigger
CREATE TRIGGER update_hosts_modtime
BEFORE UPDATE ON hosts
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
