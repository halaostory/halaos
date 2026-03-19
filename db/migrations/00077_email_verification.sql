-- +goose Up
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN verification_token VARCHAR(100);
ALTER TABLE users ADD COLUMN verification_token_expires_at TIMESTAMPTZ;

CREATE INDEX idx_users_verification_token ON users(verification_token) WHERE verification_token IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_verification_token;
ALTER TABLE users DROP COLUMN IF EXISTS verification_token_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS verification_token;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
