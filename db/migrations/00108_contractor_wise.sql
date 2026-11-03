-- Cross-border contractor payouts through Wise (licensed money transmitter).

-- +goose Up
CREATE TABLE contractor_payout_accounts (
    contractor_id BIGINT PRIMARY KEY REFERENCES contractors(id) ON DELETE CASCADE,
    company_id    BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider      VARCHAR(20) NOT NULL DEFAULT 'wise',
    recipient_id  BIGINT NOT NULL,
    currency      VARCHAR(3) NOT NULL,
    label         TEXT NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contractor_wise_transfers (
    payment_id      BIGINT PRIMARY KEY REFERENCES contractor_payments(id) ON DELETE CASCADE,
    company_id      BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    quote_id        TEXT NOT NULL,
    transfer_id     BIGINT NOT NULL,
    source_amount   NUMERIC(14, 2) NOT NULL,
    source_currency VARCHAR(3) NOT NULL,
    target_amount   NUMERIC(14, 2) NOT NULL,
    target_currency VARCHAR(3) NOT NULL,
    rate            NUMERIC(18, 8) NOT NULL,
    status          VARCHAR(40) NOT NULL,
    created_by      BIGINT REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS contractor_wise_transfers;
DROP TABLE IF EXISTS contractor_payout_accounts;
