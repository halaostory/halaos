-- +migrate Up
ALTER TABLE employee_documents ADD COLUMN IF NOT EXISTS expiry_date DATE;

-- +migrate Down
ALTER TABLE employee_documents DROP COLUMN IF EXISTS expiry_date;
