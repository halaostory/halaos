-- +goose Up

CREATE TABLE grievance_cases (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    case_number VARCHAR(30) NOT NULL,
    category VARCHAR(50) NOT NULL, -- workplace_safety, harassment, discrimination, policy_violation, compensation, working_conditions, other
    subject VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium', -- low, medium, high, critical
    status VARCHAR(20) NOT NULL DEFAULT 'open', -- open, under_review, in_mediation, resolved, closed, withdrawn
    assigned_to BIGINT REFERENCES users(id),
    resolution TEXT,
    resolution_date DATE,
    is_anonymous BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_grievance_company ON grievance_cases(company_id, status);
CREATE INDEX idx_grievance_employee ON grievance_cases(employee_id);

CREATE TABLE grievance_comments (
    id BIGSERIAL PRIMARY KEY,
    grievance_id BIGINT NOT NULL REFERENCES grievance_cases(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    comment TEXT NOT NULL,
    is_internal BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_grievance_comments ON grievance_comments(grievance_id);

-- +goose Down
DROP TABLE IF EXISTS grievance_comments;
DROP TABLE IF EXISTS grievance_cases;
