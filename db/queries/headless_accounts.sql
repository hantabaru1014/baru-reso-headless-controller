-- name: GetHeadlessAccount :one
SELECT * FROM headless_accounts WHERE resonite_id = $1;

-- name: ListHeadlessAccounts :many
SELECT * FROM headless_accounts ORDER BY resonite_id;

-- name: CreateHeadlessAccount :exec
INSERT INTO headless_accounts (resonite_id, credential, password, last_display_name, last_icon_url) VALUES ($1, $2, $3, $4, $5);

-- name: DeleteHeadlessAccount :exec
DELETE FROM headless_accounts WHERE resonite_id = $1;

-- name: UpdateAccountInfo :exec
UPDATE headless_accounts SET last_display_name = $2, last_icon_url = $3 WHERE resonite_id = $1;
