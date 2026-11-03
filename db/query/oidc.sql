-- OpenID Connect SSO

-- name: GetOIDCProvider :one
SELECT * FROM oidc_providers WHERE company_id = $1;

-- name: FindOIDCProviderByDomain :one
SELECT * FROM oidc_providers WHERE enabled AND sqlc.arg(domain)::text = ANY(email_domains) LIMIT 1;

-- name: UpsertOIDCProvider :one
INSERT INTO oidc_providers (company_id, issuer, client_id, client_secret_enc, email_domains, enabled, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company_id) DO UPDATE SET issuer = EXCLUDED.issuer, client_id = EXCLUDED.client_id,
    client_secret_enc = EXCLUDED.client_secret_enc, email_domains = EXCLUDED.email_domains,
    enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: DeleteOIDCProvider :exec
DELETE FROM oidc_providers WHERE company_id = $1;

-- name: DomainTakenByOtherCompany :one
SELECT EXISTS (
    SELECT 1 FROM oidc_providers WHERE company_id <> $1 AND email_domains && sqlc.arg(domains)::text[]
)::bool AS taken;

-- name: CreateOIDCLogin :exec
INSERT INTO oidc_logins (state, company_id, nonce, verifier) VALUES ($1, $2, $3, $4);

-- name: TakeOIDCLogin :one
-- Consumes a state once, within 10 minutes.
UPDATE oidc_logins SET completed_at = now()
WHERE state = $1 AND completed_at IS NULL AND created_at > now() - interval '10 minutes'
RETURNING *;

-- name: SetOIDCLoginResult :exec
UPDATE oidc_logins SET user_id = $2, exchange_hash = $3 WHERE state = $1;

-- name: RedeemOIDCExchange :one
-- Single-use, valid for 2 minutes after the callback.
UPDATE oidc_logins SET used_at = now()
WHERE exchange_hash = $1 AND used_at IS NULL AND completed_at > now() - interval '2 minutes'
RETURNING user_id;

-- name: PurgeOIDCLogins :exec
DELETE FROM oidc_logins WHERE created_at < now() - interval '1 day';
