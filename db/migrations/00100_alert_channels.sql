-- Slack alerts (Warp: payroll notifications, tax alerts and deadline reminders in Slack).

-- +goose Up
CREATE TABLE alert_channels (
    company_id        BIGINT PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    slack_webhook_enc BYTEA,
    slack_hint        VARCHAR(40) NOT NULL DEFAULT '',
    events            TEXT[] NOT NULL DEFAULT ARRAY['agent_attention', 'notice', 'filing_due', 'payroll_ready'],
    enabled           BOOLEAN NOT NULL DEFAULT true,
    last_digest_on    DATE,
    updated_by        BIGINT REFERENCES users(id),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS alert_channels;
