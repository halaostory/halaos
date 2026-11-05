-- Web Push: each device (browser / installed app) an employee turned
-- phone notifications on for, plus the server's VAPID keys when they are
-- not configured in the environment (generated once, kept here).

-- +goose Up
CREATE TABLE push_subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    user_agent VARCHAR(300) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);
CREATE INDEX idx_push_subscriptions_user ON push_subscriptions (user_id);

CREATE TABLE push_vapid_keys (
    id          SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    public_key  TEXT NOT NULL,
    private_key TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS push_vapid_keys;
DROP TABLE IF EXISTS push_subscriptions;
