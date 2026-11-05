-- Perks (Warp: partner perks such as discounts on software and services):
-- offers the company curates for its employees.

-- +goose Up
CREATE TABLE company_perks (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    title       VARCHAR(160) NOT NULL,
    partner     VARCHAR(120) NOT NULL DEFAULT '',
    category    VARCHAR(30) NOT NULL DEFAULT 'other'
        CHECK (category IN ('health', 'wellness', 'food', 'learning', 'travel', 'finance', 'software', 'other')),
    description TEXT NOT NULL DEFAULT '',
    offer       VARCHAR(160) NOT NULL DEFAULT '',
    code        VARCHAR(80) NOT NULL DEFAULT '',
    url         TEXT NOT NULL DEFAULT '',
    expires_on  DATE,
    active      BOOLEAN NOT NULL DEFAULT true,
    claims      INT NOT NULL DEFAULT 0,
    created_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_company_perks_company ON company_perks (company_id, active);

-- +goose Down
DROP TABLE IF EXISTS company_perks;
