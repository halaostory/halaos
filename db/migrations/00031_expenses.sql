-- +goose Up

-- Expense Categories
CREATE TABLE expense_categories (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    max_amount NUMERIC(12, 2),
    requires_receipt BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, name)
);

-- Expense Claims
CREATE TABLE expense_claims (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    claim_number VARCHAR(30) NOT NULL,
    category_id BIGINT NOT NULL REFERENCES expense_categories(id),
    description TEXT NOT NULL,
    amount NUMERIC(12, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'PHP',
    expense_date DATE NOT NULL,
    receipt_path VARCHAR(500),
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, submitted, approved, rejected, paid
    submitted_at TIMESTAMPTZ,
    approver_id BIGINT REFERENCES employees(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    paid_at TIMESTAMPTZ,
    paid_reference VARCHAR(200),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_expense_claims_company ON expense_claims(company_id, status);
CREATE INDEX idx_expense_claims_employee ON expense_claims(employee_id, created_at DESC);
CREATE UNIQUE INDEX idx_expense_claims_number ON expense_claims(company_id, claim_number);

-- +goose Down
DROP TABLE IF EXISTS expense_claims;
DROP TABLE IF EXISTS expense_categories;
