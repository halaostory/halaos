-- +goose Up

CREATE TABLE brain_links (
    id            BIGSERIAL    PRIMARY KEY,
    company_id    BIGINT       NOT NULL REFERENCES companies(id),
    brain_tenant_id UUID       NOT NULL,
    api_endpoint  TEXT         NOT NULL,
    api_key_enc   VARCHAR(500) NOT NULL,
    webhook_secret VARCHAR(500) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    last_synced_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_brain_links_company ON brain_links(company_id);

CREATE TABLE brain_outbox (
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

CREATE INDEX idx_brain_outbox_pending
    ON brain_outbox(status, next_retry_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_brain_outbox_company
    ON brain_outbox(company_id, created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS brain_outbox;
DROP TABLE IF EXISTS brain_links;
