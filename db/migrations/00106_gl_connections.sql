-- Direct journal push to Xero / QuickBooks Online (OAuth 2.0).

-- +goose Up
CREATE TABLE gl_connections (
    company_id        BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider          VARCHAR(20) NOT NULL CHECK (provider IN ('xero', 'quickbooks')),
    tenant_id         TEXT NOT NULL,
    tenant_name       TEXT NOT NULL DEFAULT '',
    access_token_enc  BYTEA NOT NULL,
    refresh_token_enc BYTEA NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    connected_by      BIGINT REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, provider)
);

CREATE TABLE gl_oauth_states (
    state      TEXT PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id    BIGINT REFERENCES users(id) ON DELETE CASCADE,
    provider   VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE gl_pushes (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider    VARCHAR(20) NOT NULL,
    run_id      BIGINT NOT NULL REFERENCES payroll_runs(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    pushed_by   BIGINT REFERENCES users(id),
    pushed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, run_id)
);

-- +goose Down
DROP TABLE IF EXISTS gl_pushes;
DROP TABLE IF EXISTS gl_oauth_states;
DROP TABLE IF EXISTS gl_connections;
