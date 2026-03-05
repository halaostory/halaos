-- +goose Up

-- HR Events (event sourcing lite / outbox pattern)
CREATE TABLE hr_events (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    aggregate_type VARCHAR(50) NOT NULL, -- employee, attendance, leave, payroll, etc.
    aggregate_id BIGINT NOT NULL,
    event_type VARCHAR(100) NOT NULL, -- employee.hired, attendance.clocked_in, leave.approved, etc.
    event_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL DEFAULT '{}',
    actor_user_id BIGINT REFERENCES users(id),
    idempotency_key VARCHAR(100),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    retries INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, processing, processed, failed, dead
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hr_events_pending ON hr_events(status, created_at) WHERE status IN ('pending', 'failed');
CREATE INDEX idx_hr_events_aggregate ON hr_events(aggregate_type, aggregate_id, occurred_at DESC);
CREATE INDEX idx_hr_events_company ON hr_events(company_id, occurred_at DESC);
CREATE UNIQUE INDEX idx_hr_events_idempotency ON hr_events(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- AI Audit Log (immutable AI decision records)
CREATE TABLE ai_audit_log (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    request_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36),
    intent VARCHAR(100) NOT NULL, -- leave.query, payroll.simulate, policy.ask, etc.
    model VARCHAR(50) NOT NULL, -- claude-sonnet-4, gpt-4o, etc.
    prompt_hash VARCHAR(64), -- SHA-256 of prompt for dedup
    tool_calls JSONB, -- [{name, params, result_summary}]
    decision VARCHAR(500), -- what the AI decided/recommended
    risk_level VARCHAR(10) NOT NULL DEFAULT 'low', -- low, medium, high, critical
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    redacted_input TEXT, -- PII-stripped input
    redacted_output TEXT, -- PII-stripped output
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_audit_company ON ai_audit_log(company_id, created_at DESC);
CREATE INDEX idx_ai_audit_user ON ai_audit_log(user_id, created_at DESC);
CREATE INDEX idx_ai_audit_intent ON ai_audit_log(intent, created_at DESC);

-- Approval Workflows (generic approval engine)
CREATE TABLE approval_workflows (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    entity_type VARCHAR(50) NOT NULL, -- leave_request, overtime_request, payroll_run
    entity_id BIGINT NOT NULL,
    step INT NOT NULL DEFAULT 1,
    approver_id BIGINT NOT NULL REFERENCES employees(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected
    comments TEXT,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approvals_pending ON approval_workflows(approver_id, status) WHERE status = 'pending';
CREATE INDEX idx_approvals_entity ON approval_workflows(entity_type, entity_id);

-- Announcements
CREATE TABLE announcements (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    priority VARCHAR(10) NOT NULL DEFAULT 'normal', -- low, normal, high, urgent
    target_roles TEXT[], -- null = all, or specific roles
    target_departments BIGINT[], -- null = all
    published_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_announcements_company ON announcements(company_id, published_at DESC);

-- Audit Logs (system-wide change tracking)
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT REFERENCES users(id),
    action VARCHAR(50) NOT NULL, -- create, update, delete, login, approve, etc.
    entity_type VARCHAR(50) NOT NULL,
    entity_id BIGINT,
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    user_agent VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_company ON audit_logs(company_id, created_at DESC);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS approval_workflows;
DROP TABLE IF EXISTS ai_audit_log;
DROP TABLE IF EXISTS hr_events;
