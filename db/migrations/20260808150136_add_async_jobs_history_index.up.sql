-- 既存の index は PENDING/RUNNING の worker 向け部分索引のみで、
-- 履歴一覧 (ORDER BY created_at DESC) には効かない。
CREATE INDEX IF NOT EXISTS idx_async_jobs_created_at
    ON async_jobs (created_at DESC);
-- 一般ユーザーの一覧は created_by で絞ってから created_at 降順で並べるため複合で張る。
CREATE INDEX IF NOT EXISTS idx_async_jobs_created_by_created_at
    ON async_jobs (created_by, created_at DESC);
