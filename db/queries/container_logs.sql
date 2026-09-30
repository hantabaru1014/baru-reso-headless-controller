-- name: GetContainerLogsBefore :many
-- 特定のタグ（hostID + instanceID）のログを、before_id より小さい ID の中から新しい順に取得
-- (古い方向へのページネーション / 最新からの初回取得)
-- カーソルなしで最新から取得する場合は before_id に bigint の最大値を渡す
SELECT tag, ts, data, id
FROM container_logs
WHERE tag = @tag
  AND id < @before_id::bigint
ORDER BY id DESC
LIMIT @max_rows;

-- name: GetContainerLogsAfter :many
-- 特定のタグ（hostID + instanceID）のログを、after_id より大きい ID の中から古い順に取得
-- (新しい方向へのページネーション)
SELECT tag, ts, data, id
FROM container_logs
WHERE tag = @tag
  AND id > @after_id::bigint
ORDER BY id ASC
LIMIT @max_rows;

-- name: SearchContainerLogBefore :one
-- before_id より小さい ID の中から、本文がパターンに一致する最も新しいログの ID を返す
-- カーソルなしで最新から検索する場合は before_id に bigint の最大値を渡す
SELECT id
FROM container_logs
WHERE tag = @tag
  AND id < @before_id::bigint
  AND data->>'log' ILIKE @pattern::text
ORDER BY id DESC
LIMIT 1;

-- name: SearchContainerLogAfter :one
-- after_id より大きい ID の中から、本文がパターンに一致する最も古いログの ID を返す
SELECT id
FROM container_logs
WHERE tag = @tag
  AND id > @after_id::bigint
  AND data->>'log' ILIKE @pattern::text
ORDER BY id ASC
LIMIT 1;

-- name: InsertContainerLog :exec
-- テスト用
INSERT INTO container_logs (tag, ts, data) VALUES ($1, $2, $3);

-- name: DeleteContainerLogsByHostID :exec
-- 特定のホストIDに関連するすべてのログを削除
-- タグは "headless-{hostID}-{instanceID}" の形式
DELETE FROM container_logs
WHERE tag LIKE 'headless-' || @host_id || '-%';

-- name: GetInstanceTimestamps :many
-- 各インスタンスのログの最初と最後のタイムスタンプを取得
SELECT
    CAST(SUBSTRING(tag FROM 'headless-[^-]+-(\d+)') AS INTEGER) AS instance_id,
    MIN(ts) AS first_log_at,
    MAX(ts) AS last_log_at,
    COUNT(*) AS log_count
FROM container_logs
WHERE tag LIKE 'headless-' || @host_id || '-%'
GROUP BY SUBSTRING(tag FROM 'headless-[^-]+-(\d+)')
ORDER BY instance_id DESC;
