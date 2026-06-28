-- name: GetHeadlessAccount :one
SELECT * FROM headless_accounts WHERE resonite_id = $1;

-- name: ListHeadlessAccounts :many
SELECT * FROM headless_accounts ORDER BY resonite_id;

-- name: ListHeadlessAccountsPaged :many
-- ページング付きアカウント一覧。total_count は全行同じ値が入る。
-- group_ids は nullable パラメータ (sqlc.narg)。NULL の場合は全グループ対象。
-- 空配列を渡すと結果ゼロ件 (= 所属グループが無いユーザーに対する自動絞り込み)。
SELECT sqlc.embed(headless_accounts), COUNT(*) OVER() AS total_count
FROM headless_accounts
WHERE (sqlc.narg('group_ids')::text[] IS NULL OR group_id = ANY(sqlc.narg('group_ids')::text[]))
ORDER BY resonite_id
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: CreateHeadlessAccount :exec
INSERT INTO headless_accounts (resonite_id, credential, password, last_display_name, last_icon_url, group_id, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: DeleteHeadlessAccount :exec
DELETE FROM headless_accounts WHERE resonite_id = $1;

-- name: UpdateAccountInfo :exec
UPDATE headless_accounts SET last_display_name = $2, last_icon_url = $3 WHERE resonite_id = $1;

-- name: UpdateHeadlessAccountCredentials :exec
UPDATE headless_accounts SET credential = $2, password = $3 WHERE resonite_id = $1;

-- name: UpdateAccountIconUrl :exec
UPDATE headless_accounts SET last_icon_url = $2 WHERE resonite_id = $1;
