-- +goose Up
-- BYOK (Bring Your Own Key) — per-company / per-user LLM API keys

CREATE TABLE IF NOT EXISTS byok_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT REFERENCES users(id),  -- NULL = company-wide key
    provider VARCHAR(20) NOT NULL
        CHECK (provider IN ('anthropic', 'openai', 'gemini')),
    encrypted_key BYTEA NOT NULL,
    key_hint VARCHAR(20) NOT NULL DEFAULT '',  -- e.g. "sk-...ab3F"
    model_override VARCHAR(100) NOT NULL DEFAULT '',  -- optional custom model
    label VARCHAR(100) NOT NULL DEFAULT '',  -- user-friendly label
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_byok_keys_unique ON byok_keys(company_id, COALESCE(user_id, 0::bigint), provider);
CREATE INDEX idx_byok_keys_company ON byok_keys(company_id, is_active);
CREATE INDEX idx_byok_keys_user ON byok_keys(user_id, is_active) WHERE user_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS byok_keys;
