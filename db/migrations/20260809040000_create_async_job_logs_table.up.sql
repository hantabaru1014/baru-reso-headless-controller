-- async_job_logs は job 失敗時の詳細ログ (現状は image build の builder ログ) を保持する。
-- async_jobs.last_error は一覧に載る一行サマリのままにし、数百KB になりうる本文は
-- こちらに分離する: 一覧クエリ (ListAsyncJobs) が巨大な text を引かずに済む。
CREATE TABLE async_job_logs (
    job_id uuid PRIMARY KEY REFERENCES async_jobs(id) ON DELETE CASCADE,
    content text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);
