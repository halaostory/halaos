-- +goose Up
ALTER TABLE users ADD COLUMN reset_token VARCHAR(100);
ALTER TABLE users ADD COLUMN reset_token_expires_at TIMESTAMPTZ;

CREATE INDEX idx_users_reset_token ON users(reset_token) WHERE reset_token IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_reset_token;
ALTER TABLE users DROP COLUMN IF EXISTS reset_token_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS reset_token;
