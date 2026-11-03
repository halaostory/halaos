-- Scoped API keys (Warp: "all gated by scoped API keys"). Empty = full access.

-- +goose Up
ALTER TABLE api_keys ADD COLUMN scopes TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE api_keys DROP COLUMN IF EXISTS scopes;
