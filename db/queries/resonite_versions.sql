-- name: UpsertResoniteVersion :one
-- versions.json の 1 エントリを upsert する。build 状態や image_tag は保持する。
INSERT INTO resonite_versions (
    manifest_id, branch, game_version, released_at
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (manifest_id, branch) DO UPDATE
SET game_version = EXCLUDED.game_version,
    released_at = EXCLUDED.released_at
RETURNING *;

-- name: GetResoniteVersion :one
SELECT * FROM resonite_versions
WHERE manifest_id = $1 AND branch = $2 LIMIT 1;

-- name: GetResoniteVersionByImageTag :one
SELECT * FROM resonite_versions
WHERE image_tag = $1 LIMIT 1;

-- name: ListResoniteVersions :many
SELECT * FROM resonite_versions
WHERE (sqlc.narg('branch')::text IS NULL OR branch = sqlc.narg('branch')::text)
ORDER BY released_at DESC;

-- name: GetLatestBuiltResoniteVersionByBranch :one
SELECT * FROM resonite_versions
WHERE branch = $1
  AND build_status = 'built'
  AND game_version IS NOT NULL
ORDER BY released_at DESC
LIMIT 1;

-- name: ListStaleBuiltResoniteVersions :many
-- built 済みだが container repo の AppVersion と食い違う (再ビルド対象) 行.
-- 対象ブランチは呼び出し側で絞る (`ANY (@branches)` で渡す).
SELECT * FROM resonite_versions
WHERE build_status = 'built'
  AND branch = ANY (@branches::text[])
  AND (built_with_app_version IS NULL OR built_with_app_version <> @current_app_version::text)
ORDER BY released_at DESC;

-- name: SetResoniteVersionBuilding :execrows
UPDATE resonite_versions
SET build_status = 'building',
    build_error = NULL
WHERE manifest_id = $1 AND branch = $2;

-- name: SetResoniteVersionBuilt :execrows
UPDATE resonite_versions
SET build_status = 'built',
    image_tag = @image_tag::text,
    built_with_app_version = @built_with_app_version::text,
    built_at = NOW(),
    build_error = NULL
WHERE manifest_id = $1 AND branch = $2;

-- name: SetResoniteVersionFailed :execrows
UPDATE resonite_versions
SET build_status = 'failed',
    build_error = @build_error::text
WHERE manifest_id = $1 AND branch = $2;
