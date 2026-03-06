-- +goose Up

ALTER TABLE payroll_cycles ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE payroll_cycles ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ;
ALTER TABLE payroll_cycles ADD COLUMN IF NOT EXISTS locked_by BIGINT REFERENCES users(id);

-- +goose Down
ALTER TABLE payroll_cycles DROP COLUMN IF EXISTS locked_by;
ALTER TABLE payroll_cycles DROP COLUMN IF EXISTS locked_at;
ALTER TABLE payroll_cycles DROP COLUMN IF EXISTS is_locked;
