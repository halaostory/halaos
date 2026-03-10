-- +goose Up

-- Workflow triggers: configurable event-driven trigger-action pairs
CREATE TABLE workflow_triggers (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    trigger_type VARCHAR(30) NOT NULL, -- on_created, on_status_changed, on_sla_breach
    entity_type VARCHAR(50) NOT NULL,  -- leave_request, overtime_request, *
    trigger_config JSONB NOT NULL DEFAULT '{}',
    action_type VARCHAR(30) NOT NULL,  -- run_rules_then_agent, auto_approve, auto_reject, notify
    action_config JSONB NOT NULL DEFAULT '{}',
    priority INT NOT NULL DEFAULT 100,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_triggers_active ON workflow_triggers(company_id, entity_type, trigger_type, is_active, priority)
    WHERE is_active = true;

-- Workflow decisions: AI decision audit trail + learning feedback
CREATE TABLE workflow_decisions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    trigger_id BIGINT REFERENCES workflow_triggers(id),
    entity_type VARCHAR(50) NOT NULL,
    entity_id BIGINT NOT NULL,
    decision VARCHAR(30) NOT NULL, -- auto_approve, auto_reject, recommend_approve, recommend_reject, escalate, request_info
    confidence NUMERIC(4,3) NOT NULL DEFAULT 0.000,
    reasoning TEXT,
    context_snapshot JSONB,
    ai_agent_slug VARCHAR(50),
    tokens_used INT NOT NULL DEFAULT 0,
    executed BOOLEAN NOT NULL DEFAULT false,
    executed_at TIMESTAMPTZ,
    execution_result JSONB,
    overridden_by BIGINT REFERENCES users(id),
    override_action VARCHAR(30),
    override_reason TEXT,
    overridden_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_decisions_entity ON workflow_decisions(entity_type, entity_id);
CREATE INDEX idx_workflow_decisions_company ON workflow_decisions(company_id, created_at DESC);
CREATE INDEX idx_workflow_decisions_overridden ON workflow_decisions(company_id, overridden_at)
    WHERE overridden_at IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS workflow_decisions;
DROP TABLE IF EXISTS workflow_triggers;
