CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE async_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type INTEGER NOT NULL, -- domain/entity/async_job.go の enum
    payload JSONB NOT NULL, -- 操作の引数 (protojson)
    status INTEGER NOT NULL DEFAULT 0, -- 0:PENDING / 1:RUNNING / 2:SUCCEEDED / 3:FAILED
    result_payload JSONB, -- 完了時の戻り値 (host_id / session_id 等). FAILED の場合は NULL
    last_error TEXT,
    claimed_by TEXT,
    claimed_at TIMESTAMP WITH TIME ZONE,
    executed_at TIMESTAMP WITH TIME ZONE,
    host_id TEXT, -- 一覧フィルタ用 (FK は貼らない: hosts 削除後も履歴を残せるように)
    session_id TEXT, -- StartSession 系では NULL (まだ ID 未確定)
    created_by TEXT, -- users.id. 完了通知の宛先にも使う
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_async_jobs_pending_created
    ON async_jobs (created_at) WHERE status = 0;
CREATE INDEX idx_async_jobs_running_claimed
    ON async_jobs (claimed_at) WHERE status = 1;
CREATE INDEX idx_async_jobs_host
    ON async_jobs (host_id) WHERE host_id IS NOT NULL;
CREATE INDEX idx_async_jobs_session
    ON async_jobs (session_id) WHERE session_id IS NOT NULL;

CREATE TRIGGER update_async_jobs_modtime
BEFORE UPDATE ON async_jobs
FOR EACH ROW
EXECUTE PROCEDURE update_timestamp();
