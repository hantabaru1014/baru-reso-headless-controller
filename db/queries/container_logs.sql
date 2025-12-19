-- name: GetContainerLogsByTag :many
-- 特定のタグ（hostID + instanceID）のログを取得
-- before_id: このIDより小さいログ (古い方向へのページネーション)
-- after_id: このIDより大きいログ (新しい方向へのページネーション)
SELECT id, tag, ts, data
FROM container_logs
WHERE tag = @tag
  AND (@before_id::bigint IS NULL OR @before_id = 0 OR id < @before_id)
  AND (@after_id::bigint IS NULL OR @after_id = 0 OR id > @after_id)
ORDER BY id DESC
LIMIT CASE WHEN @max_rows > 0 THEN @max_rows ELSE 100 END;

-- name: InsertContainerLog :exec
-- テスト用
INSERT INTO container_logs (tag, ts, data) VALUES ($1, $2, $3);
