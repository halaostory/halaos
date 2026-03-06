-- +goose Up

CREATE TABLE leave_encashments (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    leave_type_id BIGINT NOT NULL REFERENCES leave_types(id),
    year INT NOT NULL,
    days NUMERIC(5, 1) NOT NULL,
    daily_rate NUMERIC(12, 2) NOT NULL,
    total_amount NUMERIC(12, 2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected, paid
    remarks TEXT,
    approved_by BIGINT REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_leave_encashments_employee ON leave_encashments(employee_id, year);
CREATE INDEX idx_leave_encashments_company ON leave_encashments(company_id, status);

-- +goose Down

DROP TABLE IF EXISTS leave_encashments;
