-- +goose Up
-- HR Service Request system for employee self-service tickets

CREATE TABLE IF NOT EXISTS hr_requests (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    employee_id     BIGINT NOT NULL,
    request_type    VARCHAR(30) NOT NULL,  -- coe, salary_cert, id_replacement, equipment, schedule_change, general
    subject         VARCHAR(200) NOT NULL,
    description     TEXT,
    priority        VARCHAR(10) NOT NULL DEFAULT 'normal',  -- low, normal, high, urgent
    status          VARCHAR(20) NOT NULL DEFAULT 'open',    -- open, in_progress, resolved, closed, cancelled
    assigned_to     BIGINT,               -- HR staff user_id
    resolution_note TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);

CREATE INDEX idx_hr_requests_company ON hr_requests(company_id, status, created_at DESC);
CREATE INDEX idx_hr_requests_employee ON hr_requests(employee_id, created_at DESC);
CREATE INDEX idx_hr_requests_assigned ON hr_requests(assigned_to, status) WHERE assigned_to IS NOT NULL;

-- Add tool to general agent
UPDATE agents SET tools = tools || ARRAY['create_hr_request', 'query_hr_requests'] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(tools, 'create_hr_request'), 'query_hr_requests') WHERE slug = 'general';
DROP TABLE IF EXISTS hr_requests;
