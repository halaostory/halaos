-- +goose Up

ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS max_carryover NUMERIC(5,1) DEFAULT 5;
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS carryover_expiry_months INT DEFAULT 0;

-- +goose Down
ALTER TABLE leave_types DROP COLUMN IF EXISTS carryover_expiry_months;
ALTER TABLE leave_types DROP COLUMN IF EXISTS max_carryover;
