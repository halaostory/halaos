-- +goose Up

-- Workflow Rules: configurable automation rules per company
CREATE TABLE workflow_rules (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    entity_type VARCHAR(50) NOT NULL, -- leave_request, overtime_request
    rule_type VARCHAR(30) NOT NULL DEFAULT 'auto_approve', -- auto_approve, auto_reject
    conditions JSONB NOT NULL DEFAULT '{}',
    priority INT NOT NULL DEFAULT 100, -- lower = higher priority, first match wins
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_rules_active ON workflow_rules(company_id, entity_type, is_active, priority)
    WHERE is_active = true;

-- Workflow Rule Executions: audit trail
CREATE TABLE workflow_rule_executions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    rule_id BIGINT NOT NULL REFERENCES workflow_rules(id),
    entity_type VARCHAR(50) NOT NULL,
    entity_id BIGINT NOT NULL,
    action VARCHAR(30) NOT NULL, -- auto_approved, auto_rejected, skipped
    reason TEXT,
    evaluated_conditions JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rule_executions_company ON workflow_rule_executions(company_id, created_at DESC);
CREATE INDEX idx_rule_executions_rule ON workflow_rule_executions(rule_id, created_at DESC);
CREATE INDEX idx_rule_executions_entity ON workflow_rule_executions(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS workflow_rule_executions;
DROP TABLE IF EXISTS workflow_rules;
