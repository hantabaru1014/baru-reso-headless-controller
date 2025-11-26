-- name: GetContainerLogsByTag :many
-- 特定のタグ（hostID + instanceID）のログを取得
SELECT tag, ts, data
FROM container_logs
WHERE tag = @tag
  AND (@until::timestamp IS NULL OR ts < @until)
  AND (@since::timestamp IS NULL OR ts >= @since)
ORDER BY ts DESC
LIMIT CASE WHEN @max_rows > 0 THEN @max_rows ELSE 1000 END;

-- name: InsertContainerLog :exec
-- テスト用
INSERT INTO container_logs (tag, ts, data) VALUES ($1, $2, $3);
