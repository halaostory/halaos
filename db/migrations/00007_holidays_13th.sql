-- +goose Up

-- Philippine Holiday Calendar
CREATE TABLE holidays (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(255) NOT NULL,
    holiday_date DATE NOT NULL,
    holiday_type VARCHAR(30) NOT NULL DEFAULT 'regular', -- regular, special_non_working, special_working
    year INT NOT NULL,
    is_nationwide BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_holidays_company_date ON holidays(company_id, holiday_date);
CREATE INDEX idx_holidays_year ON holidays(company_id, year);

-- 13th Month Pay Records
CREATE TABLE thirteenth_month_pay (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    year INT NOT NULL,
    total_basic_salary NUMERIC(15,2) NOT NULL DEFAULT 0,
    months_worked NUMERIC(5,2) NOT NULL DEFAULT 0,
    amount NUMERIC(15,2) NOT NULL DEFAULT 0,  -- total_basic_salary / 12
    tax_exempt_amount NUMERIC(15,2) NOT NULL DEFAULT 90000, -- first 90K is tax exempt (TRAIN Law)
    taxable_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, calculated, approved, paid
    computed_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_13th_month_emp_year ON thirteenth_month_pay(company_id, employee_id, year);

-- Final Pay Records
CREATE TABLE final_pay (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    separation_date DATE NOT NULL,
    separation_reason VARCHAR(100) NOT NULL, -- resignation, termination, retirement, end_of_contract
    unpaid_salary NUMERIC(15,2) NOT NULL DEFAULT 0,
    prorated_13th NUMERIC(15,2) NOT NULL DEFAULT 0,
    unused_leave_conversion NUMERIC(15,2) NOT NULL DEFAULT 0,
    separation_pay NUMERIC(15,2) NOT NULL DEFAULT 0,
    tax_refund NUMERIC(15,2) NOT NULL DEFAULT 0,
    other_deductions NUMERIC(15,2) NOT NULL DEFAULT 0,
    total_final_pay NUMERIC(15,2) NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, calculated, approved, released
    computed_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_final_pay_emp ON final_pay(company_id, employee_id);

-- +goose Down
DROP TABLE IF EXISTS final_pay;
DROP TABLE IF EXISTS thirteenth_month_pay;
DROP TABLE IF EXISTS holidays;
