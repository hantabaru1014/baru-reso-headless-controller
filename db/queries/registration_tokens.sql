-- name: CreateRegistrationToken :exec
INSERT INTO registration_tokens (token, resonite_id, expires_at) VALUES ($1, $2, $3);

-- name: GetRegistrationToken :one
SELECT * FROM registration_tokens WHERE token = $1;

-- name: GetValidRegistrationToken :one
SELECT * FROM registration_tokens
WHERE token = $1
  AND expires_at > NOW()
  AND used_at IS NULL;

-- name: MarkRegistrationTokenUsed :exec
UPDATE registration_tokens SET used_at = NOW() WHERE token = $1;

-- name: DeleteExpiredRegistrationTokens :exec
DELETE FROM registration_tokens WHERE expires_at < NOW();
