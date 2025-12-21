-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, provider, provider_id, email_verified)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByProvider :one
SELECT * FROM users WHERE provider = $1 AND provider_id = $2;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE($2, name),
    avatar_url = COALESCE($3, avatar_url),
    email_verified = COALESCE($4, email_verified)
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
