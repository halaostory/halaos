-- Chat Workflow tables for Telegram/WhatsApp bot integration

-- Per-company bot configuration
CREATE TABLE IF NOT EXISTS bot_configs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    platform VARCHAR(20) NOT NULL DEFAULT 'telegram',
    bot_token TEXT NOT NULL DEFAULT '',
    bot_username VARCHAR(100) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    webhook_url TEXT NOT NULL DEFAULT '',
    mode VARCHAR(20) NOT NULL DEFAULT 'polling'
        CHECK (mode IN ('polling', 'webhook')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, platform)
);

-- Links platform user → AIGoNHR employee
CREATE TABLE IF NOT EXISTS bot_user_links (
    id BIGSERIAL PRIMARY KEY,
    platform VARCHAR(20) NOT NULL DEFAULT 'telegram',
    platform_user_id VARCHAR(100),
    user_id BIGINT NOT NULL REFERENCES users(id),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    link_code VARCHAR(12),
    link_code_exp TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    locale VARCHAR(10) NOT NULL DEFAULT 'en',
    active_session_id UUID REFERENCES chat_sessions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_bot_user_links_platform_user ON bot_user_links(platform, platform_user_id) WHERE platform_user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_bot_user_links_platform_uid ON bot_user_links(platform, user_id);
CREATE INDEX idx_bot_user_links_code ON bot_user_links(link_code) WHERE link_code IS NOT NULL;
