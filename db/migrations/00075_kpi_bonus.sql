-- +goose Up

-- bonus_structures: KPI bonus scheme configurations
CREATE TABLE bonus_structures (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    bonus_type VARCHAR(30) NOT NULL DEFAULT 'kpi',        -- kpi, fixed, percentage
    base_amount NUMERIC(12,2) NOT NULL DEFAULT 0,          -- fixed amount or base value
    base_type VARCHAR(20) NOT NULL DEFAULT 'fixed',        -- fixed, basic_salary_pct
    rating_map JSONB NOT NULL DEFAULT '{}',                 -- {"5":1.5,"4":1.2,"3":1.0,"2":0.5,"1":0}
    review_cycle_id BIGINT REFERENCES review_cycles(id),   -- linked performance cycle
    is_taxable BOOLEAN NOT NULL DEFAULT true,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',            -- draft, active, closed
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_bonus_structures_company ON bonus_structures(company_id, status);

-- bonus_allocations: per-employee bonus allocation records
CREATE TABLE bonus_allocations (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    structure_id BIGINT NOT NULL REFERENCES bonus_structures(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    performance_review_id BIGINT REFERENCES performance_reviews(id),
    rating INT,                                             -- performance rating snapshot
    multiplier NUMERIC(5,2) NOT NULL DEFAULT 1.0,           -- from rating_map lookup
    base_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    final_amount NUMERIC(12,2) NOT NULL DEFAULT 0,          -- base_amount * multiplier
    manual_override NUMERIC(12,2),                          -- optional manual adjustment
    status VARCHAR(20) NOT NULL DEFAULT 'pending',          -- pending, approved, paid, cancelled
    payroll_cycle_id BIGINT REFERENCES payroll_cycles(id),  -- linked payroll cycle
    approved_by BIGINT REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(structure_id, employee_id)
);
CREATE INDEX idx_bonus_allocations_employee ON bonus_allocations(employee_id);
CREATE INDEX idx_bonus_allocations_cycle ON bonus_allocations(payroll_cycle_id, status);

-- Add bonus_pay column to payroll_items
ALTER TABLE payroll_items ADD COLUMN bonus_pay NUMERIC(10,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE payroll_items DROP COLUMN IF EXISTS bonus_pay;
DROP TABLE IF EXISTS bonus_allocations;
DROP TABLE IF EXISTS bonus_structures;
