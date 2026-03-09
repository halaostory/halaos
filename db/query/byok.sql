-- ========== byok_keys ==========

-- name: CreateByokKey :one
INSERT INTO byok_keys (company_id, user_id, provider, encrypted_key, key_hint, model_override, label, is_active, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetByokKey :one
SELECT * FROM byok_keys
WHERE id = $1 AND company_id = $2;

-- name: ListByokKeys :many
SELECT id, company_id, user_id, provider, key_hint, model_override, label, is_active, created_by, created_at, updated_at
FROM byok_keys
WHERE company_id = $1 AND is_active = true
ORDER BY provider, user_id NULLS FIRST;

-- name: ResolveByokKey :one
-- Priority: user-specific key > company-wide key (user_id IS NULL)
SELECT * FROM byok_keys
WHERE company_id = $1
  AND provider = $2
  AND is_active = true
  AND (user_id = $3 OR user_id IS NULL)
ORDER BY user_id NULLS LAST
LIMIT 1;

-- name: UpdateByokKey :one
UPDATE byok_keys SET
    encrypted_key = CASE WHEN @encrypted_key::bytea IS NULL THEN encrypted_key ELSE @encrypted_key END,
    key_hint = CASE WHEN @key_hint::text = '' THEN key_hint ELSE @key_hint END,
    model_override = @model_override,
    label = @label,
    is_active = @is_active,
    updated_at = NOW()
WHERE id = @id AND company_id = @company_id
RETURNING *;

-- name: DeleteByokKey :exec
DELETE FROM byok_keys
WHERE id = $1 AND company_id = $2;
