-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserWithPassword :one
SELECT * FROM users WHERE id = $1 AND password = $2;

-- name: ListUsers :many
SELECT * FROM users ORDER BY id;

-- name: CreateUser :exec
INSERT INTO users (id, password, resonite_id, icon_url) VALUES ($1, $2, $3, $4);

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
