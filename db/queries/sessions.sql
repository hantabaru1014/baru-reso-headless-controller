-- name: UpsertSession :one
INSERT INTO sessions (
    id,
    name,
    status,
    started_at,
    owner_id,
    ended_at,
    host_id,
    startup_parameters,
    startup_parameters_schema_version,
    auto_upgrade,
    memo,
    current_state
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    started_at = EXCLUDED.started_at,
    owner_id = EXCLUDED.owner_id,
    ended_at = EXCLUDED.ended_at,
    host_id = EXCLUDED.host_id,
    startup_parameters = EXCLUDED.startup_parameters,
    startup_parameters_schema_version = EXCLUDED.startup_parameters_schema_version,
    auto_upgrade = EXCLUDED.auto_upgrade,
    memo = EXCLUDED.memo,
    current_state = EXCLUDED.current_state
RETURNING *;

-- name: UpdateSessionStatus :exec
UPDATE sessions SET status = $2 WHERE id = $1;

-- name: DowngradeSessionToUnknownIfRunning :exec
-- StreamReset reconcile 期間中に gRPC StopSession 等で正常終端した session を、
-- host snapshot に居ないという根拠だけで UNKNOWN へ降格しないようガードする。
-- 2 = SessionStatus_RUNNING, 0 = SessionStatus_UNKNOWN
UPDATE sessions SET status = 0 WHERE id = $1 AND status = 2;

-- name: UpdateSessionCurrentStateAndName :exec
-- event 駆動の SessionParametersChanged 反映用。current_state と name の
-- 部分更新にして、並行する UpdateSessionParameters / StartSession など
-- の Upsert と他フィールドが衝突しないようにする
UPDATE sessions SET current_state = $2, name = $3 WHERE id = $1;

-- name: UpdateSessionAfterWorldSaved :exec
-- event 駆動の WorldSaved 反映用。world_url 1 値だけ受け取り、protojson の
-- JSONB に対して jsonb_set で in-place 書き換えする。handler 側で Get→mutate
-- →Update を往復しないので、gRPC UpdateSessionParameters / Upsert と
-- race しても他フィールドを stale snapshot で revert しない。
--
-- protojson は oneof を active case のキーだけ出力するため、preset case が
-- 残っていると `loadWorld` で 2 つのキーが現れて parse 不能になる。
-- 先に `loadWorldPresetName` を `-` で削ってから `loadWorldUrl` を書き込む。
-- current_state は NULL のままなら触らない (初期 startup 直後の race で
-- 空オブジェクトを書いて UI を 0/0 表示にしないため)。
UPDATE sessions
SET startup_parameters = jsonb_set(
        COALESCE(startup_parameters, '{}'::jsonb) - 'loadWorldPresetName',
        '{loadWorldUrl}',
        to_jsonb(@world_url::text)
    ),
    current_state = CASE
        WHEN current_state IS NULL THEN current_state
        ELSE jsonb_set(current_state, '{worldUrl}', to_jsonb(@world_url::text))
    END
WHERE id = @id;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1 LIMIT 1;

-- name: ListSessions :many
SELECT * FROM sessions ORDER BY started_at DESC;

-- name: ListSessionsByStatus :many
SELECT * FROM sessions WHERE status = $1 ORDER BY started_at DESC;

-- name: ListSessionsPaged :many
-- ページング付きセッション一覧。
-- status / host_id は nullable パラメータ (sqlc.narg)。NULL なら未指定として扱う。
-- total_count は全行同じ値が入る。
SELECT sqlc.embed(sessions), COUNT(*) OVER() AS total_count
FROM sessions
WHERE (sqlc.narg('status')::int IS NULL OR status = sqlc.narg('status')::int)
  AND (sqlc.narg('host_id')::text IS NULL OR host_id = sqlc.narg('host_id')::text)
ORDER BY started_at DESC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;
