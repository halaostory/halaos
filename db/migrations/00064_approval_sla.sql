-- +goose Up

-- SLA configuration per entity type per company
CREATE TABLE approval_sla_configs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    entity_type VARCHAR(50) NOT NULL,
    reminder_after_hours INT NOT NULL DEFAULT 12,
    second_reminder_hours INT NOT NULL DEFAULT 24,
    escalate_after_hours INT NOT NULL DEFAULT 48,
    auto_action_hours INT NOT NULL DEFAULT 72,
    auto_action VARCHAR(20) NOT NULL DEFAULT 'approve', -- approve, reject, none
    escalation_role VARCHAR(20) NOT NULL DEFAULT 'admin',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, entity_type)
);

-- SLA event log
CREATE TABLE approval_sla_events (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    entity_type VARCHAR(50) NOT NULL,
    entity_id BIGINT NOT NULL,
    event_type VARCHAR(30) NOT NULL, -- reminder_1, reminder_2, escalated, auto_approved, auto_rejected
    target_user_id BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sla_events_entity ON approval_sla_events(entity_type, entity_id);
CREATE INDEX idx_sla_events_company ON approval_sla_events(company_id, created_at DESC);

-- Add SLA tracking columns to approval_workflows
ALTER TABLE approval_workflows
    ADD COLUMN IF NOT EXISTS sla_deadline TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS escalated_to BIGINT REFERENCES employees(id),
    ADD COLUMN IF NOT EXISTS escalated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE approval_workflows
    DROP COLUMN IF EXISTS sla_deadline,
    DROP COLUMN IF EXISTS escalated_to,
    DROP COLUMN IF EXISTS escalated_at;
DROP TABLE IF EXISTS approval_sla_events;
DROP TABLE IF EXISTS approval_sla_configs;
