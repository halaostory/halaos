-- name: CreateChatSession :one
INSERT INTO chat_sessions (company_id, user_id, agent_slug, title)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetChatSession :one
SELECT * FROM chat_sessions
WHERE id = $1 AND company_id = $2 AND user_id = $3;

-- name: UpdateChatSessionTitle :exec
UPDATE chat_sessions SET title = $2, updated_at = NOW()
WHERE id = $1;

-- name: TouchChatSession :exec
UPDATE chat_sessions SET updated_at = NOW()
WHERE id = $1;

-- name: ListChatMessages :many
SELECT * FROM chat_messages
WHERE session_id = $1
ORDER BY created_at ASC
LIMIT 50;

-- name: InsertChatMessage :one
INSERT INTO chat_messages (session_id, role, content, tokens_used)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListUserChatSessions :many
SELECT * FROM chat_sessions
WHERE user_id = $1 AND company_id = $2
ORDER BY updated_at DESC
LIMIT 20;

-- name: DeleteChatSession :exec
DELETE FROM chat_sessions
WHERE id = $1 AND user_id = $2;
