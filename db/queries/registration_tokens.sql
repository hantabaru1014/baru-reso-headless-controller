-- name: CreateRegistrationToken :one
INSERT INTO registration_tokens (token, resonite_id, expires_at, personal_role_id)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetRegistrationToken :one
SELECT * FROM registration_tokens WHERE token = $1;

-- name: GetValidRegistrationToken :one
SELECT * FROM registration_tokens
WHERE token = $1
  AND expires_at > NOW()
  AND used_at IS NULL;

-- name: MarkRegistrationTokenUsed :execrows
UPDATE registration_tokens SET used_at = NOW() WHERE token = $1 AND used_at IS NULL;

-- name: DeleteExpiredRegistrationTokens :exec
DELETE FROM registration_tokens WHERE expires_at < NOW();

-- name: ListPendingRegistrationTokens :many
-- 未使用の招待一覧 (期限切れを含む). 再発行 / 取消の対象.
SELECT * FROM registration_tokens WHERE used_at IS NULL ORDER BY created_at;

-- name: ReissueRegistrationToken :execrows
UPDATE registration_tokens SET token = $2, expires_at = $3
WHERE id = $1 AND used_at IS NULL;

-- name: DeletePendingRegistrationToken :execrows
DELETE FROM registration_tokens WHERE id = $1 AND used_at IS NULL;
