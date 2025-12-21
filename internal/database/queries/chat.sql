-- name: CreateChatSession :one
INSERT INTO chat_sessions (user_id, title, model, system_prompt)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetChatSession :one
SELECT * FROM chat_sessions WHERE id = $1;

-- name: GetChatSessionByUser :one
SELECT * FROM chat_sessions WHERE id = $1 AND user_id = $2;

-- name: ListChatSessions :many
SELECT * FROM chat_sessions
WHERE user_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateChatSession :one
UPDATE chat_sessions
SET title = COALESCE($2, title),
    system_prompt = COALESCE($3, system_prompt)
WHERE id = $1
RETURNING *;

-- name: DeleteChatSession :exec
DELETE FROM chat_sessions WHERE id = $1 AND user_id = $2;

-- name: CreateChatMessage :one
INSERT INTO chat_messages (session_id, role, content, tokens_used)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetChatMessages :many
SELECT * FROM chat_messages
WHERE session_id = $1
ORDER BY created_at ASC;

-- name: GetRecentChatMessages :many
SELECT * FROM chat_messages
WHERE session_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: DeleteChatMessage :exec
DELETE FROM chat_messages WHERE id = $1;

-- name: CountSessionMessages :one
SELECT COUNT(*) FROM chat_messages WHERE session_id = $1;

-- name: GetSessionTokenCount :one
SELECT COALESCE(SUM(tokens_used), 0)::INTEGER as total_tokens
FROM chat_messages WHERE session_id = $1;
