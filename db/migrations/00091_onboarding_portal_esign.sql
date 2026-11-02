-- +goose Up

-- New-hire self-onboarding portal (tokenized public link) and e-signatures.

CREATE TABLE IF NOT EXISTS onboarding_invites (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'opened', 'submitted', 'revoked', 'expired')),
    opened_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_onboarding_invites_employee ON onboarding_invites(employee_id, created_at DESC);

CREATE TABLE IF NOT EXISTS document_templates (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    kind VARCHAR(30) NOT NULL DEFAULT 'other'
        CHECK (kind IN ('offer_letter', 'employment_contract', 'nda', 'policy_ack', 'other')),
    name VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_document_templates_company ON document_templates(company_id, is_active);

CREATE TABLE IF NOT EXISTS signature_requests (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    template_id BIGINT REFERENCES document_templates(id),
    kind VARCHAR(30) NOT NULL DEFAULT 'other',
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    body_sha256 VARCHAR(64) NOT NULL,
    signer_name VARCHAR(200) NOT NULL,
    signer_email VARCHAR(255),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'viewed', 'signed', 'declined', 'voided', 'expired')),
    signed_name VARCHAR(200),
    signature_image TEXT,
    signed_at TIMESTAMPTZ,
    signed_ip VARCHAR(64),
    signed_user_agent TEXT,
    declined_reason TEXT,
    signed_pdf_path TEXT,
    signed_pdf_sha256 VARCHAR(64),
    viewed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_signature_requests_employee ON signature_requests(employee_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_signature_requests_company ON signature_requests(company_id, status);

-- +goose Down
DROP TABLE IF EXISTS signature_requests;
DROP TABLE IF EXISTS document_templates;
DROP TABLE IF EXISTS onboarding_invites;
