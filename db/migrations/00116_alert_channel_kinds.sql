-- Team alert channels beyond Slack: Microsoft Teams, Google Chat, Lark/Feishu,
-- Telegram groups and LINE groups (Slack is little used in Southeast Asia).

-- +goose Up
ALTER TABLE alert_channels RENAME COLUMN slack_webhook_enc TO secret_enc;
ALTER TABLE alert_channels RENAME COLUMN slack_hint TO hint;
ALTER TABLE alert_channels ADD COLUMN kind VARCHAR(20) NOT NULL DEFAULT 'slack';
ALTER TABLE alert_channels ADD COLUMN target VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE alert_channels ADD CONSTRAINT alert_channels_kind_check
    CHECK (kind IN ('slack', 'teams', 'google_chat', 'lark', 'telegram', 'line'));

-- +goose Down
ALTER TABLE alert_channels DROP CONSTRAINT IF EXISTS alert_channels_kind_check;
ALTER TABLE alert_channels DROP COLUMN IF EXISTS target;
ALTER TABLE alert_channels DROP COLUMN IF EXISTS kind;
ALTER TABLE alert_channels RENAME COLUMN hint TO slack_hint;
ALTER TABLE alert_channels RENAME COLUMN secret_enc TO slack_webhook_enc;
