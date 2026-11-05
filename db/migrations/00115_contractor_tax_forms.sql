-- Contractor tax information form (Warp: contractors fill in their own tax
-- forms). HR sends a one-time link; the contractor enters tax residency,
-- TIN, address, VAT status and payout details and signs a declaration.
-- The signed snapshot is kept as the record.

-- +goose Up
CREATE TABLE contractor_tax_forms (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    contractor_id     BIGINT NOT NULL REFERENCES contractors(id) ON DELETE CASCADE,
    token_hash        VARCHAR(64) NOT NULL UNIQUE,
    status            VARCHAR(12) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'submitted', 'revoked')),
    expires_at        TIMESTAMPTZ NOT NULL,
    tax_country       VARCHAR(2) NOT NULL DEFAULT '',
    entity_type       VARCHAR(12) NOT NULL DEFAULT '',
    tin               VARCHAR(40) NOT NULL DEFAULT '',
    address           TEXT NOT NULL DEFAULT '',
    is_vat_registered BOOLEAN NOT NULL DEFAULT false,
    declaration       TEXT NOT NULL DEFAULT '',
    signer_name       VARCHAR(160) NOT NULL DEFAULT '',
    signer_ip         VARCHAR(64) NOT NULL DEFAULT '',
    signer_agent      VARCHAR(300) NOT NULL DEFAULT '',
    submitted_at      TIMESTAMPTZ,
    created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_contractor_tax_forms_contractor ON contractor_tax_forms (contractor_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS contractor_tax_forms;
