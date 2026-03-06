-- +goose Up

-- Government filing tracker
CREATE TABLE tax_filings (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    filing_type VARCHAR(30) NOT NULL, -- bir_1601c, bir_2316, sss_r3, philhealth_rf1, pagibig_ml1, bir_0619e
    period_type VARCHAR(20) NOT NULL, -- monthly, quarterly, annual
    period_year INT NOT NULL,
    period_month INT, -- 1-12 for monthly, null for annual
    period_quarter INT, -- 1-4 for quarterly
    due_date DATE NOT NULL,
    filing_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, generated, submitted, filed, overdue
    amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    penalty_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    reference_no VARCHAR(100), -- government reference/confirmation number
    proof_path TEXT, -- file path of payment/filing proof
    filed_by BIGINT REFERENCES users(id),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, filing_type, period_year, period_month, period_quarter)
);

CREATE INDEX idx_tax_filings_company ON tax_filings(company_id, status);
CREATE INDEX idx_tax_filings_due ON tax_filings(due_date);

-- Remittance records (detailed breakdown per filing)
CREATE TABLE remittance_records (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    filing_id BIGINT REFERENCES tax_filings(id),
    agency VARCHAR(20) NOT NULL, -- bir, sss, philhealth, pagibig
    period_year INT NOT NULL,
    period_month INT NOT NULL,
    employee_count INT NOT NULL DEFAULT 0,
    employer_share NUMERIC(12,2) NOT NULL DEFAULT 0,
    employee_share NUMERIC(12,2) NOT NULL DEFAULT 0,
    total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_remittance_company ON remittance_records(company_id);
CREATE INDEX idx_remittance_filing ON remittance_records(filing_id);

-- +goose Down
DROP TABLE IF EXISTS remittance_records;
DROP TABLE IF EXISTS tax_filings;
