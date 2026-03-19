-- +goose Up
CREATE TABLE nps_feedback (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    user_id     BIGINT NOT NULL REFERENCES users(id),
    score       SMALLINT NOT NULL CHECK (score >= 0 AND score <= 10),
    comment     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_nps_feedback_company ON nps_feedback(company_id);
CREATE INDEX idx_nps_feedback_user_created ON nps_feedback(user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS nps_feedback;
