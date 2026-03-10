-- +goose Up
-- Employee recognition / kudos system for peer appreciation

CREATE TABLE IF NOT EXISTS recognitions (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    from_employee_id BIGINT NOT NULL,
    to_employee_id  BIGINT NOT NULL,
    category        VARCHAR(30) NOT NULL DEFAULT 'kudos',  -- kudos, teamwork, innovation, leadership, above_and_beyond, customer_focus
    message         TEXT NOT NULL,
    is_public       BOOLEAN NOT NULL DEFAULT true,
    points          INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recognitions_company ON recognitions(company_id, created_at DESC);
CREATE INDEX idx_recognitions_to ON recognitions(to_employee_id, created_at DESC);
CREATE INDEX idx_recognitions_from ON recognitions(from_employee_id, created_at DESC);

-- Add recognition tool to general agent
UPDATE agents SET tools = tools || ARRAY['send_kudos', 'query_kudos'] WHERE slug = 'general';

-- +goose Down
UPDATE agents SET tools = array_remove(array_remove(tools, 'send_kudos'), 'query_kudos') WHERE slug = 'general';
DROP TABLE IF EXISTS recognitions;
