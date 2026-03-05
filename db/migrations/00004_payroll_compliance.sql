-- +goose Up

-- Salary Structures
CREATE TABLE salary_structures (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Salary Components (earnings, deductions, contributions)
CREATE TABLE salary_components (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    code VARCHAR(30) NOT NULL,
    name VARCHAR(100) NOT NULL,
    component_type VARCHAR(20) NOT NULL, -- earning, deduction, employer_contribution
    is_taxable BOOLEAN NOT NULL DEFAULT true,
    is_statutory BOOLEAN NOT NULL DEFAULT false, -- SSS, PhilHealth, etc
    is_fixed BOOLEAN NOT NULL DEFAULT true,
    formula JSONB, -- { "type": "percentage", "base": "basic", "rate": 0.05 }
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, code)
);

-- Employee Salary Assignments
CREATE TABLE employee_salaries (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    structure_id BIGINT REFERENCES salary_structures(id),
    basic_salary NUMERIC(12, 2) NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,
    remarks TEXT,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_employee_salaries_employee ON employee_salaries(employee_id, effective_from DESC);

-- Employee Salary Component Overrides
CREATE TABLE employee_salary_components (
    id BIGSERIAL PRIMARY KEY,
    employee_salary_id BIGINT NOT NULL REFERENCES employee_salaries(id),
    component_id BIGINT NOT NULL REFERENCES salary_components(id),
    amount NUMERIC(12, 2) NOT NULL,
    UNIQUE(employee_salary_id, component_id)
);

-- Payroll Cycles
CREATE TABLE payroll_cycles (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    pay_date DATE NOT NULL,
    cycle_type VARCHAR(20) NOT NULL DEFAULT 'regular', -- regular, 13th_month, final_pay
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, processing, computed, approved, paid, void
    created_by BIGINT REFERENCES users(id),
    approved_by BIGINT REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payroll_cycles_company ON payroll_cycles(company_id, period_start DESC);

-- Payroll Runs (each calculation attempt)
CREATE TABLE payroll_runs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    cycle_id BIGINT NOT NULL REFERENCES payroll_cycles(id),
    run_type VARCHAR(20) NOT NULL DEFAULT 'regular', -- regular, simulation, correction
    run_number INT NOT NULL DEFAULT 1,
    total_employees INT NOT NULL DEFAULT 0,
    total_gross NUMERIC(14, 2) NOT NULL DEFAULT 0,
    total_deductions NUMERIC(14, 2) NOT NULL DEFAULT 0,
    total_net NUMERIC(14, 2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
    error_message TEXT,
    initiated_by BIGINT REFERENCES users(id),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Payroll Items (per employee per run)
CREATE TABLE payroll_items (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES payroll_runs(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    basic_pay NUMERIC(12, 2) NOT NULL DEFAULT 0,
    gross_pay NUMERIC(12, 2) NOT NULL DEFAULT 0,
    taxable_income NUMERIC(12, 2) NOT NULL DEFAULT 0,
    total_deductions NUMERIC(12, 2) NOT NULL DEFAULT 0,
    net_pay NUMERIC(12, 2) NOT NULL DEFAULT 0,
    -- Government contributions (employee share)
    sss_ee NUMERIC(10, 2) NOT NULL DEFAULT 0,
    sss_er NUMERIC(10, 2) NOT NULL DEFAULT 0,
    sss_ec NUMERIC(10, 2) NOT NULL DEFAULT 0,
    philhealth_ee NUMERIC(10, 2) NOT NULL DEFAULT 0,
    philhealth_er NUMERIC(10, 2) NOT NULL DEFAULT 0,
    pagibig_ee NUMERIC(10, 2) NOT NULL DEFAULT 0,
    pagibig_er NUMERIC(10, 2) NOT NULL DEFAULT 0,
    withholding_tax NUMERIC(10, 2) NOT NULL DEFAULT 0,
    -- Breakdown detail
    breakdown JSONB NOT NULL DEFAULT '{}',
    work_days NUMERIC(5, 1) NOT NULL DEFAULT 0,
    hours_worked NUMERIC(7, 2) NOT NULL DEFAULT 0,
    ot_hours NUMERIC(7, 2) NOT NULL DEFAULT 0,
    late_deduction NUMERIC(10, 2) NOT NULL DEFAULT 0,
    undertime_deduction NUMERIC(10, 2) NOT NULL DEFAULT 0,
    holiday_pay NUMERIC(10, 2) NOT NULL DEFAULT 0,
    night_diff NUMERIC(10, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, employee_id)
);

CREATE INDEX idx_payroll_items_employee ON payroll_items(employee_id);

-- Payslips (immutable snapshot)
CREATE TABLE payslips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    run_id BIGINT NOT NULL REFERENCES payroll_runs(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    pay_date DATE NOT NULL,
    payload JSONB NOT NULL, -- complete payslip data snapshot
    pdf_path VARCHAR(500),
    viewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payslips_employee ON payslips(employee_id, created_at DESC);

-- SSS Contribution Table (2025)
CREATE TABLE sss_contribution_table (
    id BIGSERIAL PRIMARY KEY,
    effective_from DATE NOT NULL,
    effective_to DATE,
    msc_min NUMERIC(10, 2) NOT NULL, -- monthly salary credit range min
    msc_max NUMERIC(10, 2) NOT NULL,
    msc NUMERIC(10, 2) NOT NULL, -- monthly salary credit
    ee_share NUMERIC(10, 2) NOT NULL, -- employee share
    er_share NUMERIC(10, 2) NOT NULL, -- employer share
    ec NUMERIC(10, 2) NOT NULL DEFAULT 0, -- employee compensation
    total NUMERIC(10, 2) NOT NULL
);

-- PhilHealth Contribution Table
CREATE TABLE philhealth_contribution_table (
    id BIGSERIAL PRIMARY KEY,
    effective_from DATE NOT NULL,
    effective_to DATE,
    salary_min NUMERIC(10, 2) NOT NULL,
    salary_max NUMERIC(10, 2) NOT NULL,
    premium_rate NUMERIC(5, 4) NOT NULL, -- e.g. 0.05 for 5%
    ee_share_rate NUMERIC(5, 4) NOT NULL,
    er_share_rate NUMERIC(5, 4) NOT NULL,
    floor_premium NUMERIC(10, 2), -- minimum premium
    ceiling_premium NUMERIC(10, 2) -- maximum premium
);

-- PagIBIG Contribution Table
CREATE TABLE pagibig_contribution_table (
    id BIGSERIAL PRIMARY KEY,
    effective_from DATE NOT NULL,
    effective_to DATE,
    salary_min NUMERIC(10, 2) NOT NULL,
    salary_max NUMERIC(10, 2) NOT NULL,
    ee_rate NUMERIC(5, 4) NOT NULL,
    er_rate NUMERIC(5, 4) NOT NULL,
    max_ee NUMERIC(10, 2) NOT NULL DEFAULT 100, -- max employee contribution
    max_er NUMERIC(10, 2) NOT NULL DEFAULT 100  -- max employer contribution
);

-- BIR Withholding Tax Table (TRAIN Law)
CREATE TABLE bir_tax_table (
    id BIGSERIAL PRIMARY KEY,
    effective_from DATE NOT NULL,
    effective_to DATE,
    frequency VARCHAR(20) NOT NULL, -- monthly, semi_monthly
    bracket_min NUMERIC(12, 2) NOT NULL,
    bracket_max NUMERIC(12, 2),
    fixed_tax NUMERIC(12, 2) NOT NULL DEFAULT 0,
    rate NUMERIC(5, 4) NOT NULL, -- tax rate for excess
    excess_over NUMERIC(12, 2) NOT NULL DEFAULT 0
);

-- Government Form Submissions
CREATE TABLE government_forms (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    form_type VARCHAR(20) NOT NULL, -- BIR_2316, BIR_1601C, SSS_R3, PHIC_RF1, etc.
    tax_year INT NOT NULL,
    period VARCHAR(20), -- monthly/quarterly period
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, generated, submitted, filed
    payload JSONB NOT NULL DEFAULT '{}',
    file_path VARCHAR(500),
    submitted_at TIMESTAMPTZ,
    submitted_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gov_forms_company ON government_forms(company_id, form_type, tax_year);

-- +goose Down
DROP TABLE IF EXISTS government_forms;
DROP TABLE IF EXISTS bir_tax_table;
DROP TABLE IF EXISTS pagibig_contribution_table;
DROP TABLE IF EXISTS philhealth_contribution_table;
DROP TABLE IF EXISTS sss_contribution_table;
DROP TABLE IF EXISTS payslips;
DROP TABLE IF EXISTS payroll_items;
DROP TABLE IF EXISTS payroll_runs;
DROP TABLE IF EXISTS payroll_cycles;
DROP TABLE IF EXISTS employee_salary_components;
DROP TABLE IF EXISTS employee_salaries;
DROP TABLE IF EXISTS salary_components;
DROP TABLE IF EXISTS salary_structures;
