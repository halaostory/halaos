-- Chat sessions for AI conversation memory
CREATE TABLE chat_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    user_id     BIGINT NOT NULL REFERENCES users(id),
    agent_slug  VARCHAR(50) NOT NULL DEFAULT 'general',
    title       VARCHAR(200) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_chat_sessions_user ON chat_sessions(user_id, updated_at DESC);

-- Chat messages within a session
CREATE TABLE chat_messages (
    id          BIGSERIAL PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role        VARCHAR(20) NOT NULL,  -- 'user', 'assistant'
    content     TEXT NOT NULL,
    tokens_used INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_chat_messages_session ON chat_messages(session_id, created_at ASC);
