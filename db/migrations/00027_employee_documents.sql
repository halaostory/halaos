-- +goose Up

-- Document categories (201 file structure)
CREATE TABLE document_categories (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(50) NOT NULL,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, slug)
);

-- Enhance existing employee_documents table
ALTER TABLE employee_documents
    ADD COLUMN IF NOT EXISTS category_id BIGINT REFERENCES document_categories(id),
    ADD COLUMN IF NOT EXISTS title VARCHAR(255),
    ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS is_required BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_employee_docs_category ON employee_documents(category_id);
CREATE INDEX IF NOT EXISTS idx_employee_docs_status ON employee_documents(company_id, status);
CREATE INDEX IF NOT EXISTS idx_employee_docs_expiry ON employee_documents(expiry_date) WHERE expiry_date IS NOT NULL AND status = 'active';

-- Document compliance requirements
CREATE TABLE document_requirements (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    category_id BIGINT NOT NULL REFERENCES document_categories(id),
    document_name VARCHAR(255) NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT true,
    applies_to VARCHAR(30) NOT NULL DEFAULT 'all',
    expiry_months INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, category_id, document_name)
);

-- +goose Down
DROP TABLE IF EXISTS document_requirements;
ALTER TABLE employee_documents
    DROP COLUMN IF EXISTS category_id,
    DROP COLUMN IF EXISTS title,
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS is_required,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS updated_at;
DROP TABLE IF EXISTS document_categories;
