-- +goose Up

-- Shifts
CREATE TABLE shifts (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    break_minutes INT NOT NULL DEFAULT 60,
    grace_minutes INT NOT NULL DEFAULT 15,
    is_overnight BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Work Schedules (employee-shift assignments)
CREATE TABLE work_schedules (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    shift_id BIGINT NOT NULL REFERENCES shifts(id),
    work_date DATE NOT NULL,
    is_rest_day BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id, work_date)
);

CREATE INDEX idx_work_schedules_date ON work_schedules(company_id, work_date);
CREATE INDEX idx_work_schedules_employee ON work_schedules(employee_id, work_date);

-- Attendance Logs
CREATE TABLE attendance_logs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    clock_in_at TIMESTAMPTZ,
    clock_out_at TIMESTAMPTZ,
    clock_in_source VARCHAR(20) NOT NULL DEFAULT 'web', -- web, mobile, biometric, manual
    clock_out_source VARCHAR(20),
    clock_in_lat NUMERIC(10, 7),
    clock_in_lng NUMERIC(10, 7),
    clock_out_lat NUMERIC(10, 7),
    clock_out_lng NUMERIC(10, 7),
    clock_in_note TEXT,
    clock_out_note TEXT,
    work_hours NUMERIC(5, 2),
    overtime_hours NUMERIC(5, 2) DEFAULT 0,
    late_minutes INT DEFAULT 0,
    undertime_minutes INT DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'present', -- present, absent, late, half_day, holiday, rest_day
    is_corrected BOOLEAN NOT NULL DEFAULT false,
    corrected_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attendance_company_date ON attendance_logs(company_id, clock_in_at DESC);
CREATE INDEX idx_attendance_employee_date ON attendance_logs(employee_id, clock_in_at DESC);

-- Leave Types
CREATE TABLE leave_types (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    code VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    is_paid BOOLEAN NOT NULL DEFAULT true,
    default_days NUMERIC(5, 1) NOT NULL DEFAULT 0,
    is_convertible BOOLEAN NOT NULL DEFAULT false, -- can convert unused to cash
    requires_attachment BOOLEAN NOT NULL DEFAULT false,
    min_days_notice INT NOT NULL DEFAULT 0,
    accrual_type VARCHAR(20) NOT NULL DEFAULT 'annual', -- annual, monthly, none
    gender_specific VARCHAR(10), -- null=all, male, female
    is_statutory BOOLEAN NOT NULL DEFAULT false, -- PH law mandated
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, code)
);

-- Leave Balances
CREATE TABLE leave_balances (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    leave_type_id BIGINT NOT NULL REFERENCES leave_types(id),
    year INT NOT NULL,
    earned NUMERIC(5, 1) NOT NULL DEFAULT 0,
    used NUMERIC(5, 1) NOT NULL DEFAULT 0,
    carried NUMERIC(5, 1) NOT NULL DEFAULT 0,
    adjusted NUMERIC(5, 1) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id, leave_type_id, year)
);

CREATE INDEX idx_leave_balances_employee ON leave_balances(employee_id, year);

-- Leave Requests
CREATE TABLE leave_requests (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    leave_type_id BIGINT NOT NULL REFERENCES leave_types(id),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    days NUMERIC(5, 1) NOT NULL,
    reason TEXT,
    attachment_path VARCHAR(500),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected, cancelled
    approver_id BIGINT REFERENCES employees(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_leave_requests_employee ON leave_requests(employee_id, created_at DESC);
CREATE INDEX idx_leave_requests_approver ON leave_requests(approver_id) WHERE status = 'pending';
CREATE INDEX idx_leave_requests_status ON leave_requests(company_id, status);

-- Overtime Requests
CREATE TABLE overtime_requests (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    ot_date DATE NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    hours NUMERIC(5, 2) NOT NULL,
    ot_type VARCHAR(20) NOT NULL DEFAULT 'regular', -- regular, rest_day, holiday, special_holiday
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected, cancelled
    approver_id BIGINT REFERENCES employees(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_overtime_requests_employee ON overtime_requests(employee_id, created_at DESC);
CREATE INDEX idx_overtime_requests_approver ON overtime_requests(approver_id) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS overtime_requests;
DROP TABLE IF EXISTS leave_requests;
DROP TABLE IF EXISTS leave_balances;
DROP TABLE IF EXISTS leave_types;
DROP TABLE IF EXISTS attendance_logs;
DROP TABLE IF EXISTS work_schedules;
DROP TABLE IF EXISTS shifts;
