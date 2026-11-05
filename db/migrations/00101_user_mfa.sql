-- Two-factor sign-in with an authenticator app (TOTP, RFC 6238).

-- +goose Up
CREATE TABLE user_mfa (
    user_id         BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_enc      BYTEA NOT NULL,
    enabled_at      TIMESTAMPTZ,
    last_step       BIGINT NOT NULL DEFAULT 0,
    recovery_hashes TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS user_mfa;
