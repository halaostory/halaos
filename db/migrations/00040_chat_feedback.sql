-- +goose Up
CREATE TABLE chat_message_feedback (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    rating VARCHAR(10) NOT NULL CHECK (rating IN ('positive', 'negative')),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(message_id, user_id)
);
CREATE INDEX idx_chat_feedback_company ON chat_message_feedback(company_id, created_at DESC);
CREATE INDEX idx_chat_feedback_rating ON chat_message_feedback(company_id, rating);

-- +goose Down
DROP TABLE IF EXISTS chat_message_feedback;
