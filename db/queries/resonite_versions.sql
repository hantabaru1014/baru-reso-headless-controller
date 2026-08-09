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
-- 各ブランチの「最新のビルド済みバージョン」のうち container repo の AppVersion と
-- 食い違うもの (= AppVersion bump 時の再ビルド対象).
--
-- 古いバージョンは対象にしない: container のコードは常に最新 Resonite の API に追従して
-- いるため、数世代前の Resonite を新しい AppVersion でビルドしてもコンパイルが通らない.
-- ホストの auto-update が乗り換える先も各ブランチの最新タグだけなので、最新1件で足りる.
--
-- 候補の選択は `build_status = 'built'` ではなく `built_at IS NOT NULL`
-- (= 一度でもビルドされた) で行う. 再ビルドが失敗すると SetFailed でその行は
-- 'built' でなくなるため、'built' で選ぶと次 tick で1つ前の版が繰り上がり、
-- 「1 tick に1本ずつ過去に遡って必ず失敗する」連鎖になる (かつ過去の行の
-- build_status を軒並み failed に潰してしまう). 最新版で選び続ければ、
-- 失敗した時点でその branch の再ビルドは止まる.
--
-- 対象ブランチは呼び出し側で絞る (`ANY (@branches)` で渡す).
WITH latest_ever_built AS (
    SELECT DISTINCT ON (branch) *
    FROM resonite_versions
    WHERE built_at IS NOT NULL
      AND branch = ANY (@branches::text[])
    ORDER BY branch, released_at DESC
)
SELECT * FROM latest_ever_built
WHERE build_status = 'built'
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
