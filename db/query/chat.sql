-- name: CreateChatSession :one
INSERT INTO chat_sessions (company_id, user_id, agent_slug, title)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetChatSession :one
SELECT * FROM chat_sessions
WHERE id = $1 AND company_id = $2 AND user_id = $3;

-- name: UpdateChatSessionTitle :exec
UPDATE chat_sessions SET title = $2, updated_at = NOW()
WHERE id = $1 AND company_id = $3 AND user_id = $4;

-- name: TouchChatSession :exec
UPDATE chat_sessions SET updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: ListChatMessages :many
SELECT cm.* FROM chat_messages cm
JOIN chat_sessions cs ON cs.id = cm.session_id
WHERE cm.session_id = $1 AND cs.company_id = $2 AND cs.user_id = $3
ORDER BY cm.created_at ASC
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
WHERE id = $1 AND user_id = $2 AND company_id = $3;
