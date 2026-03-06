-- +goose Up

-- Loan types (SSS salary loan, Pag-IBIG MPL, company cash advance, etc.)
CREATE TABLE loan_types (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    code VARCHAR(30) NOT NULL, -- sss_salary, pagibig_mpl, cash_advance, etc.
    provider VARCHAR(30) NOT NULL DEFAULT 'company', -- government, company
    max_term_months INT NOT NULL DEFAULT 24,
    interest_rate NUMERIC(5,4) NOT NULL DEFAULT 0, -- monthly rate, e.g. 0.01 = 1%
    max_amount NUMERIC(12,2),
    requires_approval BOOLEAN NOT NULL DEFAULT true,
    auto_deduct BOOLEAN NOT NULL DEFAULT true, -- auto-deduct from payroll
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_loan_types_company_code ON loan_types(company_id, code);

-- Seed default PH loan types for all companies (company_id = 0 means system default)
-- These will be copied when a company is created; for now we add them inline per company

-- Employee loans
CREATE TABLE loans (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    loan_type_id BIGINT NOT NULL REFERENCES loan_types(id),
    reference_no VARCHAR(50), -- external reference (SSS/Pag-IBIG loan number)
    principal_amount NUMERIC(12,2) NOT NULL,
    interest_rate NUMERIC(5,4) NOT NULL DEFAULT 0,
    term_months INT NOT NULL,
    monthly_amortization NUMERIC(12,2) NOT NULL,
    total_amount NUMERIC(12,2) NOT NULL, -- principal + total interest
    total_paid NUMERIC(12,2) NOT NULL DEFAULT 0,
    remaining_balance NUMERIC(12,2) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, active, completed, cancelled
    approved_by BIGINT REFERENCES employees(id),
    approved_at TIMESTAMPTZ,
    remarks TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loans_employee ON loans(employee_id, status);
CREATE INDEX idx_loans_company ON loans(company_id, status);

-- Loan payment records (each payroll deduction or manual payment)
CREATE TABLE loan_payments (
    id BIGSERIAL PRIMARY KEY,
    loan_id BIGINT NOT NULL REFERENCES loans(id),
    payment_date DATE NOT NULL,
    amount NUMERIC(12,2) NOT NULL,
    principal_portion NUMERIC(12,2) NOT NULL DEFAULT 0,
    interest_portion NUMERIC(12,2) NOT NULL DEFAULT 0,
    payment_type VARCHAR(20) NOT NULL DEFAULT 'payroll', -- payroll, manual, adjustment
    payroll_item_id BIGINT, -- link to payroll item if auto-deducted
    remarks TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loan_payments_loan ON loan_payments(loan_id, payment_date);

-- +goose Down
DROP TABLE IF EXISTS loan_payments;
DROP TABLE IF EXISTS loans;
DROP TABLE IF EXISTS loan_types;
