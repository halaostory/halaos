-- +goose Up
ALTER TABLE companies ADD COLUMN IF NOT EXISTS sss_er_no VARCHAR(20);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS philhealth_er_no VARCHAR(20);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS pagibig_er_no VARCHAR(20);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS bank_name VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS bank_branch VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS bank_account_no VARCHAR(50);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS bank_account_name VARCHAR(200);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS contact_person VARCHAR(200);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS contact_email VARCHAR(255);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS contact_phone VARCHAR(50);

-- +goose Down
ALTER TABLE companies DROP COLUMN IF EXISTS sss_er_no;
ALTER TABLE companies DROP COLUMN IF EXISTS philhealth_er_no;
ALTER TABLE companies DROP COLUMN IF EXISTS pagibig_er_no;
ALTER TABLE companies DROP COLUMN IF EXISTS bank_name;
ALTER TABLE companies DROP COLUMN IF EXISTS bank_branch;
ALTER TABLE companies DROP COLUMN IF EXISTS bank_account_no;
ALTER TABLE companies DROP COLUMN IF EXISTS bank_account_name;
ALTER TABLE companies DROP COLUMN IF EXISTS contact_person;
ALTER TABLE companies DROP COLUMN IF EXISTS contact_email;
ALTER TABLE companies DROP COLUMN IF EXISTS contact_phone;
