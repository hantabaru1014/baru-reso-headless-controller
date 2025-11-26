-- name: GetContainerLogsByTag :many
-- 特定のタグ（hostID + instanceID）のログを取得
SELECT tag, ts, data
FROM container_logs
WHERE tag = $1
  AND ($2::timestamp IS NULL OR ts < $2)
  AND ($3::timestamp IS NULL OR ts >= $3)
ORDER BY ts DESC
LIMIT CASE WHEN $4 > 0 THEN $4 ELSE 1000 END;

-- name: InsertContainerLog :exec
-- テスト用
INSERT INTO container_logs (tag, ts, data) VALUES ($1, $2, $3);
