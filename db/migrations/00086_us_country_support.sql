-- +goose Up

-- ============================================================
-- 1. Schema Changes: Add columns to existing tables
-- ============================================================

-- Add filing_status and state columns to country_tax_brackets
ALTER TABLE country_tax_brackets ADD COLUMN IF NOT EXISTS filing_status VARCHAR(30);
ALTER TABLE country_tax_brackets ADD COLUMN IF NOT EXISTS state VARCHAR(5);

-- Add US columns to payroll_items for typed tax storage
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS federal_tax NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS social_security_ee NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS social_security_er NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS medicare_ee NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS medicare_er NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS additional_medicare NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS state_tax NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS state_disability NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS futa NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS sui NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS pretax_deductions NUMERIC(12,2) NOT NULL DEFAULT 0;

-- Add US-specific fields to employee_profiles
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS ssn_encrypted BYTEA;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS state_of_residence VARCHAR(5);
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_filing_status VARCHAR(30);
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_additional_withholding NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_multiple_jobs BOOLEAN DEFAULT false;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_dependents_credit NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_other_income NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_deductions NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS state_allowances INTEGER DEFAULT 0;

-- Add accrual fields to leave_types (max_carryover already exists from migration 00022)
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS accrual_method VARCHAR(20) DEFAULT 'upfront';
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS accrual_rate NUMERIC(8,4);
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS accrual_period VARCHAR(20);

-- ============================================================
-- 2. New Tables
-- ============================================================

-- Employee benefit deductions (401k, health insurance, HSA, FSA)
CREATE TABLE IF NOT EXISTS employee_benefit_deductions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    deduction_type VARCHAR(50) NOT NULL,
    amount_per_period NUMERIC(12,2) NOT NULL,
    annual_limit NUMERIC(12,2),
    reduces_fica BOOLEAN NOT NULL DEFAULT false,
    effective_date DATE NOT NULL,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benefit_deductions_employee ON employee_benefit_deductions(employee_id);
CREATE INDEX idx_benefit_deductions_company ON employee_benefit_deductions(company_id);

-- Company registration numbers (EIN, state IDs, FUTA rate)
CREATE TABLE IF NOT EXISTS company_registration_numbers (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    country VARCHAR(3) NOT NULL,
    registration_type VARCHAR(50) NOT NULL,
    registration_value VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, registration_type)
);

-- ============================================================
-- 3. Seed US Payroll Config
-- ============================================================

INSERT INTO country_payroll_config (country, config_key, config_value, description) VALUES
('USA', 'ot_rates', '{"regular": 1.5, "ca_daily_ot": 1.5, "ca_double_ot": 2.0}', 'US OT rates: 1.5x federal, CA daily 1.5x/2.0x'),
('USA', 'standard_hours', '{"daily": 8, "weekly": 40}', 'US FLSA standard hours'),
('USA', 'night_diff_rate', '0', 'No federal night differential mandate'),
('USA', 'has_13th_month', 'false', '13th month pay not applicable in USA'),
('USA', 'default_pay_frequency', '"bi_weekly"', 'Most common US pay frequency'),
('USA', 'pay_frequencies', '["weekly", "bi_weekly", "semi_monthly", "monthly"]', 'Supported US pay frequencies'),
('USA', 'fica_ss_wage_base', '176100', 'Social Security wage base 2025'),
('USA', 'fica_additional_medicare_threshold', '200000', 'Additional Medicare Tax threshold'),
('USA', 'futa_wage_base', '7000', 'FUTA wage base per employee per year'),
('USA', 'futa_rate_default', '0.006', 'Default FUTA rate after SUI credit (0.6%)'),
('USA', 'futa_rate_ca', '0.018', 'CA FUTA credit reduction rate (1.8% effective)');

-- ============================================================
-- 4. Seed FICA Contribution Rates
-- ============================================================

INSERT INTO country_contribution_rates (country, contribution_type, rate, effective_from, description) VALUES
('USA', 'fica_ss_employee', 0.0620, '2025-01-01', 'Social Security employee 6.2%'),
('USA', 'fica_ss_employer', 0.0620, '2025-01-01', 'Social Security employer 6.2%'),
('USA', 'fica_medicare_employee', 0.0145, '2025-01-01', 'Medicare employee 1.45%'),
('USA', 'fica_medicare_employer', 0.0145, '2025-01-01', 'Medicare employer 1.45%'),
('USA', 'fica_additional_medicare', 0.0090, '2025-01-01', 'Additional Medicare 0.9% (EE only, >$200K)'),
('USA', 'ca_sdi', 0.0120, '2025-01-01', 'CA SDI 1.2% (no wage cap, per SB 951)'),
('USA', 'wa_pfml_total', 0.0092, '2025-01-01', 'WA PFML total 0.92%'),
('USA', 'wa_pfml_employee', 0.0066, '2025-01-01', 'WA PFML employee share ~71.5%'),
('USA', 'wa_pfml_employer', 0.0026, '2025-01-01', 'WA PFML employer share ~28.5%');

-- ============================================================
-- 5. Seed Federal Tax Brackets (2025, annual)
-- ============================================================

-- Single filer
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, description) VALUES
('USA', '2025-01-01', 'annual', 0, 11925, 0.1000, 0, 'single', 'Federal 10%'),
('USA', '2025-01-01', 'annual', 11925.01, 48475, 0.1200, 0, 'single', 'Federal 12%'),
('USA', '2025-01-01', 'annual', 48475.01, 103350, 0.2200, 0, 'single', 'Federal 22%'),
('USA', '2025-01-01', 'annual', 103350.01, 197300, 0.2400, 0, 'single', 'Federal 24%'),
('USA', '2025-01-01', 'annual', 197300.01, 250525, 0.3200, 0, 'single', 'Federal 32%'),
('USA', '2025-01-01', 'annual', 250525.01, 626350, 0.3500, 0, 'single', 'Federal 35%'),
('USA', '2025-01-01', 'annual', 626350.01, 999999999, 0.3700, 0, 'single', 'Federal 37%');

-- Married Filing Jointly
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, description) VALUES
('USA', '2025-01-01', 'annual', 0, 23850, 0.1000, 0, 'married_jointly', 'Federal 10%'),
('USA', '2025-01-01', 'annual', 23850.01, 96950, 0.1200, 0, 'married_jointly', 'Federal 12%'),
('USA', '2025-01-01', 'annual', 96950.01, 206700, 0.2200, 0, 'married_jointly', 'Federal 22%'),
('USA', '2025-01-01', 'annual', 206700.01, 394600, 0.2400, 0, 'married_jointly', 'Federal 24%'),
('USA', '2025-01-01', 'annual', 394600.01, 501050, 0.3200, 0, 'married_jointly', 'Federal 32%'),
('USA', '2025-01-01', 'annual', 501050.01, 751600, 0.3500, 0, 'married_jointly', 'Federal 35%'),
('USA', '2025-01-01', 'annual', 751600.01, 999999999, 0.3700, 0, 'married_jointly', 'Federal 37%');

-- Head of Household
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, description) VALUES
('USA', '2025-01-01', 'annual', 0, 17000, 0.1000, 0, 'head_of_household', 'Federal 10%'),
('USA', '2025-01-01', 'annual', 17000.01, 64850, 0.1200, 0, 'head_of_household', 'Federal 12%'),
('USA', '2025-01-01', 'annual', 64850.01, 103350, 0.2200, 0, 'head_of_household', 'Federal 22%'),
('USA', '2025-01-01', 'annual', 103350.01, 197300, 0.2400, 0, 'head_of_household', 'Federal 24%'),
('USA', '2025-01-01', 'annual', 197300.01, 250500, 0.3200, 0, 'head_of_household', 'Federal 32%'),
('USA', '2025-01-01', 'annual', 250500.01, 626350, 0.3500, 0, 'head_of_household', 'Federal 35%'),
('USA', '2025-01-01', 'annual', 626350.01, 999999999, 0.3700, 0, 'head_of_household', 'Federal 37%');

-- ============================================================
-- 6. Seed California State Tax Brackets (2025, annual, 9 brackets)
-- ============================================================

INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, state, description) VALUES
('USA', '2025-01-01', 'annual', 0, 10412, 0.0100, 0, 'single', 'CA', 'CA 1%'),
('USA', '2025-01-01', 'annual', 10412.01, 24684, 0.0200, 0, 'single', 'CA', 'CA 2%'),
('USA', '2025-01-01', 'annual', 24684.01, 38959, 0.0400, 0, 'single', 'CA', 'CA 4%'),
('USA', '2025-01-01', 'annual', 38959.01, 54081, 0.0600, 0, 'single', 'CA', 'CA 6%'),
('USA', '2025-01-01', 'annual', 54081.01, 68350, 0.0800, 0, 'single', 'CA', 'CA 8%'),
('USA', '2025-01-01', 'annual', 68350.01, 349137, 0.0930, 0, 'single', 'CA', 'CA 9.3%'),
('USA', '2025-01-01', 'annual', 349137.01, 418961, 0.1030, 0, 'single', 'CA', 'CA 10.3%'),
('USA', '2025-01-01', 'annual', 418961.01, 698271, 0.1130, 0, 'single', 'CA', 'CA 11.3%'),
('USA', '2025-01-01', 'annual', 698271.01, 999999999, 0.1230, 0, 'single', 'CA', 'CA 12.3%');

-- CA Mental Health Services Tax: additional 1% on income > $1,000,000 (handled in code, not bracket table)

-- ============================================================
-- 7. Seed New York State Tax Brackets (2025, annual, 9 brackets)
-- ============================================================

INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, state, description) VALUES
('USA', '2025-01-01', 'annual', 0, 8500, 0.0400, 0, 'single', 'NY', 'NY 4%'),
('USA', '2025-01-01', 'annual', 8500.01, 11700, 0.0450, 0, 'single', 'NY', 'NY 4.5%'),
('USA', '2025-01-01', 'annual', 11700.01, 13900, 0.0525, 0, 'single', 'NY', 'NY 5.25%'),
('USA', '2025-01-01', 'annual', 13900.01, 80650, 0.0550, 0, 'single', 'NY', 'NY 5.5%'),
('USA', '2025-01-01', 'annual', 80650.01, 215400, 0.0600, 0, 'single', 'NY', 'NY 6%'),
('USA', '2025-01-01', 'annual', 215400.01, 1077550, 0.0685, 0, 'single', 'NY', 'NY 6.85%'),
('USA', '2025-01-01', 'annual', 1077550.01, 5000000, 0.0965, 0, 'single', 'NY', 'NY 9.65%'),
('USA', '2025-01-01', 'annual', 5000000.01, 25000000, 0.1030, 0, 'single', 'NY', 'NY 10.3%'),
('USA', '2025-01-01', 'annual', 25000000.01, 999999999, 0.1090, 0, 'single', 'NY', 'NY 10.9%');

-- ============================================================
-- 8. Indexes
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_country_tax_brackets_state ON country_tax_brackets(country, state, filing_status, effective_from);

-- +goose Down
DROP INDEX IF EXISTS idx_country_tax_brackets_state;
DROP TABLE IF EXISTS company_registration_numbers;
DROP TABLE IF EXISTS employee_benefit_deductions;
ALTER TABLE leave_types DROP COLUMN IF EXISTS accrual_period;
ALTER TABLE leave_types DROP COLUMN IF EXISTS accrual_rate;
ALTER TABLE leave_types DROP COLUMN IF EXISTS accrual_method;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS state_allowances;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_deductions;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_other_income;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_dependents_credit;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_multiple_jobs;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_additional_withholding;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_filing_status;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS state_of_residence;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS ssn_encrypted;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS pretax_deductions;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS sui;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS futa;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS state_disability;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS state_tax;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS additional_medicare;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS medicare_er;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS medicare_ee;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS social_security_er;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS social_security_ee;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS federal_tax;
ALTER TABLE country_tax_brackets DROP COLUMN IF EXISTS state;
ALTER TABLE country_tax_brackets DROP COLUMN IF EXISTS filing_status;
DELETE FROM country_contribution_rates WHERE country = 'USA';
DELETE FROM country_payroll_config WHERE country = 'USA';
DELETE FROM country_tax_brackets WHERE country = 'USA';
