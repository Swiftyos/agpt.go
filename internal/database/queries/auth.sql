-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1 AND revoked = FALSE AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1;

-- name: RevokeAllUserTokens :exec
UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1;

-- name: CleanExpiredTokens :execrows
DELETE FROM refresh_tokens WHERE expires_at < NOW() OR revoked = TRUE;
