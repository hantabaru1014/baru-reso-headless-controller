-- name: GetHeadlessAccount :one
-- アカウントは (group_id, resonite_id) で一意. 同一 resonite_id を複数グループに登録できる.
SELECT * FROM headless_accounts WHERE group_id = $1 AND resonite_id = $2;

-- name: ListHeadlessAccountsByResoniteID :many
-- group_id 未指定のリクエストを解決するための逆引き (登録先グループが 1 つに定まるか調べる).
SELECT * FROM headless_accounts WHERE resonite_id = $1 ORDER BY group_id;

-- name: ListHeadlessAccounts :many
SELECT * FROM headless_accounts ORDER BY resonite_id, group_id;

-- name: ListHeadlessAccountsPaged :many
-- ページング付きアカウント一覧。total_count は全行同じ値が入る。
-- group_ids は nullable パラメータ (sqlc.narg)。NULL の場合は全グループ対象。
-- 空配列を渡すと結果ゼロ件 (= 所属グループが無いユーザーに対する自動絞り込み)。
SELECT sqlc.embed(headless_accounts), COUNT(*) OVER() AS total_count
FROM headless_accounts
WHERE (sqlc.narg('group_ids')::text[] IS NULL OR group_id = ANY(sqlc.narg('group_ids')::text[]))
ORDER BY resonite_id, group_id
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: CreateHeadlessAccount :exec
INSERT INTO headless_accounts (resonite_id, credential, password, last_display_name, last_icon_url, group_id, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: DeleteHeadlessAccount :exec
DELETE FROM headless_accounts WHERE group_id = $1 AND resonite_id = $2;

-- name: UpdateAccountInfo :exec
UPDATE headless_accounts SET last_display_name = $3, last_icon_url = $4 WHERE group_id = $1 AND resonite_id = $2;

-- name: UpdateHeadlessAccountCredentials :exec
UPDATE headless_accounts SET credential = $3, password = $4 WHERE group_id = $1 AND resonite_id = $2;

-- name: UpdateAccountIconUrl :exec
UPDATE headless_accounts SET last_icon_url = $3 WHERE group_id = $1 AND resonite_id = $2;

-- name: UpdateHeadlessAccountGroup :exec
-- グループ間移管用. 移管先に同一 resonite_id が既にある場合は PK 違反になるので、
-- 呼び出し側で事前に確認してマージ (移管元の行を削除) すること.
UPDATE headless_accounts SET group_id = @new_group_id::text WHERE group_id = @group_id::text AND resonite_id = @resonite_id::text;
