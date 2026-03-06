-- +goose Up

CREATE TABLE attendance_corrections (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    attendance_id BIGINT REFERENCES attendance_logs(id),
    correction_date DATE NOT NULL,
    original_clock_in TIMESTAMPTZ,
    original_clock_out TIMESTAMPTZ,
    requested_clock_in TIMESTAMPTZ,
    requested_clock_out TIMESTAMPTZ,
    reason TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    review_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attendance_corrections_company ON attendance_corrections(company_id, status);
CREATE INDEX idx_attendance_corrections_employee ON attendance_corrections(employee_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS attendance_corrections;
