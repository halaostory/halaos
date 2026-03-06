-- +goose Up
ALTER TABLE employee_documents ADD COLUMN IF NOT EXISTS expiry_date DATE;

-- +goose Down
ALTER TABLE employee_documents DROP COLUMN IF EXISTS expiry_date;
