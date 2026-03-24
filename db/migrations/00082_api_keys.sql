-- +goose Up
CREATE TABLE api_keys (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    prefix       VARCHAR(20) NOT NULL,
    key_hash     VARCHAR(64) NOT NULL,
    name         VARCHAR(100) NOT NULL DEFAULT 'default',
    is_active    BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash) WHERE is_active = true;
CREATE INDEX idx_api_keys_user ON api_keys(user_id) WHERE is_active = true;

-- +goose Down
DROP TABLE IF EXISTS api_keys;
