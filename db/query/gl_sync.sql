-- GL sync (Xero / QuickBooks)

-- name: ListGLConnections :many
SELECT company_id, provider, tenant_id, tenant_name, expires_at, created_at, updated_at FROM gl_connections WHERE company_id = $1;

-- name: GetGLConnection :one
SELECT * FROM gl_connections WHERE company_id = $1 AND provider = $2;

-- name: UpsertGLConnection :exec
INSERT INTO gl_connections (company_id, provider, tenant_id, tenant_name, access_token_enc, refresh_token_enc, expires_at, connected_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (company_id, provider) DO UPDATE SET tenant_id = EXCLUDED.tenant_id, tenant_name = EXCLUDED.tenant_name,
    access_token_enc = EXCLUDED.access_token_enc, refresh_token_enc = EXCLUDED.refresh_token_enc,
    expires_at = EXCLUDED.expires_at, connected_by = EXCLUDED.connected_by, updated_at = now();

-- name: UpdateGLTokens :exec
UPDATE gl_connections SET access_token_enc = $3, refresh_token_enc = $4, expires_at = $5, updated_at = now()
WHERE company_id = $1 AND provider = $2;

-- name: DeleteGLConnection :exec
DELETE FROM gl_connections WHERE company_id = $1 AND provider = $2;

-- name: CreateGLOAuthState :exec
INSERT INTO gl_oauth_states (state, company_id, user_id, provider) VALUES ($1, $2, $3, $4);

-- name: TakeGLOAuthState :one
DELETE FROM gl_oauth_states WHERE state = $1 AND created_at > now() - interval '15 minutes'
RETURNING company_id, user_id, provider;

-- name: GetGLPush :one
SELECT * FROM gl_pushes WHERE provider = $1 AND run_id = $2;

-- name: CreateGLPush :one
INSERT INTO gl_pushes (company_id, provider, run_id, external_id, pushed_by) VALUES ($1, $2, $3, $4, $5) RETURNING *;
