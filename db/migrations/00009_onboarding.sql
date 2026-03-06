-- +goose Up

-- Onboarding/Offboarding checklist templates (company-configurable)
CREATE TABLE onboarding_templates (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    workflow_type VARCHAR(20) NOT NULL DEFAULT 'onboarding', -- onboarding, offboarding
    title VARCHAR(255) NOT NULL,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    is_required BOOLEAN NOT NULL DEFAULT true,
    assignee_role VARCHAR(50), -- hr, manager, employee, it
    due_days INT NOT NULL DEFAULT 0, -- days from hire/separation date
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_onboarding_templates_company ON onboarding_templates(company_id, workflow_type, is_active);

-- Per-employee onboarding/offboarding task instances
CREATE TABLE onboarding_tasks (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    template_id BIGINT REFERENCES onboarding_templates(id),
    workflow_type VARCHAR(20) NOT NULL DEFAULT 'onboarding',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    is_required BOOLEAN NOT NULL DEFAULT true,
    assignee_role VARCHAR(50),
    assigned_to BIGINT REFERENCES users(id),
    due_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, in_progress, completed, skipped
    completed_by BIGINT REFERENCES users(id),
    completed_at TIMESTAMPTZ,
    notes TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_onboarding_tasks_employee ON onboarding_tasks(employee_id, workflow_type);
CREATE INDEX idx_onboarding_tasks_status ON onboarding_tasks(company_id, status, workflow_type);
CREATE INDEX idx_onboarding_tasks_assigned ON onboarding_tasks(assigned_to, status) WHERE status IN ('pending', 'in_progress');

-- +goose Down
DROP TABLE IF EXISTS onboarding_tasks;
DROP TABLE IF EXISTS onboarding_templates;
