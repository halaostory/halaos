-- +goose Up

-- Contractors / freelancers paid against invoices, with Philippine expanded
-- withholding tax (EWT), BIR 2307 certificates and 0619-E / 1601-EQ data.

CREATE TABLE IF NOT EXISTS contractors (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    kind VARCHAR(20) NOT NULL DEFAULT 'individual' CHECK (kind IN ('individual', 'company')),
    name VARCHAR(200) NOT NULL,
    email VARCHAR(255),
    tin VARCHAR(20) NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    is_vat_registered BOOLEAN NOT NULL DEFAULT false,
    ewt_rate NUMERIC(5,4) NOT NULL DEFAULT 0.05 CHECK (ewt_rate >= 0 AND ewt_rate <= 0.5),
    atc_code VARCHAR(10) NOT NULL DEFAULT 'WI010',
    bank_name VARCHAR(100),
    bank_account_no VARCHAR(50),
    bank_account_name VARCHAR(200),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_contractors_company ON contractors(company_id, status);

CREATE TABLE IF NOT EXISTS contractor_payments (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    contractor_id BIGINT NOT NULL REFERENCES contractors(id),
    invoice_no VARCHAR(60) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    service_date DATE NOT NULL,
    gross_amount NUMERIC(14,2) NOT NULL CHECK (gross_amount > 0),
    vat_amount NUMERIC(14,2) NOT NULL DEFAULT 0,
    ewt_base NUMERIC(14,2) NOT NULL,
    ewt_rate NUMERIC(5,4) NOT NULL,
    ewt_amount NUMERIC(14,2) NOT NULL,
    net_amount NUMERIC(14,2) NOT NULL,
    atc_code VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'paid', 'void')),
    approved_by BIGINT REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    paid_at DATE,
    paid_reference VARCHAR(120),
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_contractor_payments_company ON contractor_payments(company_id, status, paid_at);
CREATE INDEX IF NOT EXISTS idx_contractor_payments_contractor ON contractor_payments(contractor_id, paid_at);

-- AI agents: contractor tools for payroll and compliance agents (agents with
-- an empty tool list already see every tool).
UPDATE agents SET tools = ARRAY(SELECT DISTINCT unnest(tools || ARRAY['list_contractors', 'create_contractor_payment']))
WHERE slug IN ('payroll', 'compliance') AND company_id IS NULL AND cardinality(tools) > 0;

-- +goose Down
UPDATE agents SET tools = ARRAY(SELECT t FROM unnest(tools) t WHERE t NOT IN ('list_contractors', 'create_contractor_payment'))
WHERE company_id IS NULL AND slug IN ('payroll', 'compliance');
DROP TABLE IF EXISTS contractor_payments;
DROP TABLE IF EXISTS contractors;
