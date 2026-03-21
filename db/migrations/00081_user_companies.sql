-- +goose Up
CREATE TABLE user_companies (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    role       VARCHAR(20) NOT NULL DEFAULT 'employee',
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, company_id)
);

CREATE INDEX idx_user_companies_company ON user_companies(company_id);

-- Seed from existing user→company relationships
INSERT INTO user_companies (user_id, company_id, role)
SELECT id, company_id, role FROM users WHERE company_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS user_companies;
