-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, company_id, prefix, key_hash, name)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, prefix, name, is_active, last_used_at, created_at;

-- name: GetAPIKeyByHash :one
SELECT ak.id, ak.user_id, ak.company_id, ak.prefix, ak.name,
       ak.is_active, ak.last_used_at, ak.created_at, u.role, u.email
FROM api_keys ak
JOIN users u ON u.id = ak.user_id
WHERE ak.key_hash = $1 AND ak.is_active = true;

-- name: ListAPIKeysByUser :many
SELECT id, prefix, name, is_active, last_used_at, created_at
FROM api_keys
WHERE user_id = $1 AND is_active = true
ORDER BY created_at DESC;

-- name: RevokeAPIKey :exec
UPDATE api_keys SET is_active = false WHERE id = $1 AND user_id = $2;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: RevokeAPIKeyByName :exec
UPDATE api_keys SET is_active = false
WHERE user_id = $1 AND name = $2 AND is_active = true;
