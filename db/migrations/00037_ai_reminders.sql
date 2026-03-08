-- AI proactive reminder tracking
CREATE TABLE ai_reminders (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    user_id         BIGINT NOT NULL,
    reminder_type   VARCHAR(50) NOT NULL,
    entity_type     VARCHAR(50),
    entity_id       BIGINT,
    scheduled_date  DATE NOT NULL,
    sent_at         TIMESTAMPTZ,
    notification_id BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_ai_reminders_unique
    ON ai_reminders(company_id, reminder_type, entity_type, COALESCE(entity_id, 0), scheduled_date);
CREATE INDEX idx_ai_reminders_lookup ON ai_reminders(company_id, scheduled_date);
