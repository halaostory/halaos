-- +goose Up

-- Company-level link to AIStarlight accounting system.
CREATE TABLE accounting_links (
    id            BIGSERIAL    PRIMARY KEY,
    company_id    BIGINT       NOT NULL REFERENCES companies(id),
    provider      VARCHAR(50)  NOT NULL DEFAULT 'aistarlight',
    remote_company_id VARCHAR(100) NOT NULL,
    api_endpoint  VARCHAR(500) NOT NULL,
    api_key_enc   VARCHAR(500) NOT NULL,
    webhook_secret VARCHAR(500) NOT NULL,
    jurisdiction  VARCHAR(3)   NOT NULL DEFAULT 'PH',
    status        VARCHAR(20)  NOT NULL DEFAULT 'active',
    last_synced_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, provider)
);

-- Transactional outbox for reliable event delivery.
CREATE TABLE accounting_outbox (
    id              BIGSERIAL    PRIMARY KEY,
    company_id      BIGINT       NOT NULL,
    event_type      VARCHAR(100) NOT NULL,
    aggregate_type  VARCHAR(50)  NOT NULL,
    aggregate_id    BIGINT       NOT NULL,
    payload         JSONB        NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    retry_count     INT          NOT NULL DEFAULT 0,
    max_retries     INT          NOT NULL DEFAULT 5,
    next_retry_at   TIMESTAMPTZ,
    sent_at         TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounting_outbox_pending
    ON accounting_outbox(status, next_retry_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_accounting_outbox_company
    ON accounting_outbox(company_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS accounting_outbox;
DROP TABLE IF EXISTS accounting_links;
