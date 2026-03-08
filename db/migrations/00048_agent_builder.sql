-- +goose Up
ALTER TABLE agents ADD COLUMN company_id BIGINT REFERENCES companies(id);
CREATE INDEX idx_agents_company ON agents(company_id) WHERE company_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_agents_company;
ALTER TABLE agents DROP COLUMN IF EXISTS company_id;
