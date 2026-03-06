-- +goose Up

-- Benefit plans
CREATE TABLE benefit_plans (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(255) NOT NULL,
    category VARCHAR(30) NOT NULL, -- medical, dental, life_insurance, retirement, allowance, other
    description TEXT,
    provider VARCHAR(255), -- insurance company or provider name
    employer_share NUMERIC(12,2) NOT NULL DEFAULT 0,
    employee_share NUMERIC(12,2) NOT NULL DEFAULT 0,
    coverage_amount NUMERIC(12,2),
    eligibility_type VARCHAR(20) NOT NULL DEFAULT 'all', -- all, regular, after_probation, by_grade
    eligibility_months INT NOT NULL DEFAULT 0, -- min months of service
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benefit_plans_company ON benefit_plans(company_id, is_active);

-- Employee benefit enrollments
CREATE TABLE benefit_enrollments (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    plan_id BIGINT NOT NULL REFERENCES benefit_plans(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active, pending, cancelled, expired
    enrollment_date DATE NOT NULL DEFAULT CURRENT_DATE,
    effective_date DATE NOT NULL,
    end_date DATE,
    employer_share NUMERIC(12,2) NOT NULL DEFAULT 0,
    employee_share NUMERIC(12,2) NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id, plan_id)
);

CREATE INDEX idx_benefit_enrollments_company ON benefit_enrollments(company_id, status);
CREATE INDEX idx_benefit_enrollments_employee ON benefit_enrollments(employee_id);

-- Dependents for benefit coverage
CREATE TABLE benefit_dependents (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    enrollment_id BIGINT NOT NULL REFERENCES benefit_enrollments(id),
    name VARCHAR(255) NOT NULL,
    relationship VARCHAR(30) NOT NULL, -- spouse, child, parent, sibling
    birth_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benefit_dependents_enrollment ON benefit_dependents(enrollment_id);

-- Benefit claims
CREATE TABLE benefit_claims (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    enrollment_id BIGINT NOT NULL REFERENCES benefit_enrollments(id),
    claim_date DATE NOT NULL DEFAULT CURRENT_DATE,
    amount NUMERIC(12,2) NOT NULL,
    description TEXT NOT NULL,
    receipt_path TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected, paid
    approved_by BIGINT REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benefit_claims_company ON benefit_claims(company_id, status);
CREATE INDEX idx_benefit_claims_employee ON benefit_claims(employee_id);

-- +goose Down
DROP TABLE IF EXISTS benefit_claims;
DROP TABLE IF EXISTS benefit_dependents;
DROP TABLE IF EXISTS benefit_enrollments;
DROP TABLE IF EXISTS benefit_plans;
