-- name: GetCache :one
SELECT * FROM cache
WHERE key = $1 AND (expires_at IS NULL OR expires_at > NOW());

-- name: SetCache :exec
INSERT INTO cache (key, value, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, created_at = NOW();

-- name: DeleteCache :exec
DELETE FROM cache WHERE key = $1;

-- name: DeleteCacheByPrefix :execrows
DELETE FROM cache WHERE key LIKE $1 || '%';

-- name: CleanExpiredCache :execrows
DELETE FROM cache WHERE expires_at < NOW();
