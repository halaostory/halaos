-- +goose Up

-- Add contract_end_date for contractual employees
ALTER TABLE employees ADD COLUMN IF NOT EXISTS contract_end_date DATE;

-- Contract milestone alerts table
CREATE TABLE contract_milestones (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    milestone_type VARCHAR(30) NOT NULL, -- probation_ending, contract_expiring, anniversary, regularization_due
    milestone_date DATE NOT NULL,
    days_remaining INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, acknowledged, actioned
    acknowledged_by BIGINT REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_milestones_company ON contract_milestones(company_id, status);
CREATE INDEX idx_milestones_employee ON contract_milestones(employee_id);
CREATE INDEX idx_milestones_date ON contract_milestones(milestone_date);
CREATE UNIQUE INDEX idx_milestones_unique ON contract_milestones(company_id, employee_id, milestone_type, milestone_date);

-- +goose Down
DROP TABLE IF EXISTS contract_milestones;
ALTER TABLE employees DROP COLUMN IF EXISTS contract_end_date;
