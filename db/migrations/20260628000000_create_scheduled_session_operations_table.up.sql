CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE scheduled_session_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_type INTEGER NOT NULL, -- domain/entity/scheduled_session_operation.go のenum
    operation_payload JSONB NOT NULL, -- 操作の引数 (protojson)
    trigger_type INTEGER NOT NULL, -- domain/entity/scheduled_session_operation.go のenum
    trigger_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    next_fire_at TIMESTAMP WITH TIME ZONE NOT NULL,
    host_id TEXT, -- 一覧フィルタ用 (FK は貼らない: hosts 削除でも履歴を残せるように)
    session_id TEXT, -- START では NULL
    status INTEGER NOT NULL DEFAULT 0, -- 0:PENDING / 1:RUNNING / 2:SUCCEEDED / 3:FAILED / 4:CANCELED
    last_error TEXT,
    claimed_by TEXT,
    claimed_at TIMESTAMP WITH TIME ZONE,
    executed_at TIMESTAMP WITH TIME ZONE,
    created_by TEXT, -- users.id (CLI/system なら NULL)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sched_ops_pending_next_fire
    ON scheduled_session_operations (next_fire_at) WHERE status = 0;
CREATE INDEX idx_sched_ops_running_claimed
    ON scheduled_session_operations (claimed_at) WHERE status = 1;
CREATE INDEX idx_sched_ops_session
    ON scheduled_session_operations (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_sched_ops_host
    ON scheduled_session_operations (host_id) WHERE host_id IS NOT NULL;

CREATE TRIGGER update_scheduled_session_operations_modtime
BEFORE UPDATE ON scheduled_session_operations
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
