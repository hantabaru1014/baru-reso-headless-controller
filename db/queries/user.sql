-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY id;

-- name: CreateUser :exec
INSERT INTO users (id, password, resonite_id, icon_url) VALUES ($1, $2, $3, $4);

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password = $2, updated_at = current_timestamp WHERE id = $1;
