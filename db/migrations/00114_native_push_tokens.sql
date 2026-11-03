-- Native app (iOS/Android) push: FCM registration tokens per device.

-- +goose Up
CREATE TABLE native_push_tokens (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        TEXT NOT NULL UNIQUE,
    platform     VARCHAR(10) NOT NULL DEFAULT 'android' CHECK (platform IN ('android', 'ios')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);
CREATE INDEX idx_native_push_tokens_user ON native_push_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS native_push_tokens;
