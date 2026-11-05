-- +goose Up

-- Several pay schedules per company (e.g. monthly staff + semi-monthly crew).
-- Employees without an assignment, and cycles without a schedule, use the
-- company's default schedule (or companies.pay_frequency if none exist), so
-- existing data keeps its behaviour.

CREATE TABLE IF NOT EXISTS pay_schedules (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    frequency VARCHAR(20) NOT NULL CHECK (frequency IN ('monthly', 'semi_monthly', 'bi_weekly', 'weekly')),
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, name)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pay_schedules_default ON pay_schedules(company_id) WHERE is_default;

CREATE TABLE IF NOT EXISTS employee_pay_schedules (
    employee_id BIGINT PRIMARY KEY REFERENCES employees(id) ON DELETE CASCADE,
    pay_schedule_id BIGINT NOT NULL REFERENCES pay_schedules(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_employee_pay_schedules_schedule ON employee_pay_schedules(pay_schedule_id);

CREATE TABLE IF NOT EXISTS payroll_cycle_schedules (
    cycle_id BIGINT PRIMARY KEY REFERENCES payroll_cycles(id) ON DELETE CASCADE,
    pay_schedule_id BIGINT NOT NULL REFERENCES pay_schedules(id)
);

-- +goose Down
DROP TABLE IF EXISTS payroll_cycle_schedules;
DROP TABLE IF EXISTS employee_pay_schedules;
DROP TABLE IF EXISTS pay_schedules;
