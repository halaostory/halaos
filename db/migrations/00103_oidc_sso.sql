-- Single sign-on with any OpenID Connect provider (Google Workspace, Okta,
-- Microsoft Entra ID, Keycloak…), one provider per company.

-- +goose Up
CREATE TABLE oidc_providers (
    company_id        BIGINT PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    issuer            TEXT NOT NULL,
    client_id         TEXT NOT NULL,
    client_secret_enc BYTEA NOT NULL,
    email_domains     TEXT[] NOT NULL DEFAULT '{}',
    enabled           BOOLEAN NOT NULL DEFAULT true,
    updated_by        BIGINT REFERENCES users(id),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oidc_providers_domains ON oidc_providers USING GIN (email_domains);

CREATE TABLE oidc_logins (
    state         TEXT PRIMARY KEY,
    company_id    BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    nonce         TEXT NOT NULL,
    verifier      TEXT NOT NULL,
    user_id       BIGINT REFERENCES users(id) ON DELETE CASCADE,
    exchange_hash TEXT UNIQUE,
    completed_at  TIMESTAMPTZ,
    used_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS oidc_logins;
DROP TABLE IF EXISTS oidc_providers;
