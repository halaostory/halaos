-- Net pay disbursement through a licensed payout partner (Xendit).

-- +goose Up
CREATE TABLE payout_batches (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    cycle_id    BIGINT NOT NULL REFERENCES payroll_cycles(id) ON DELETE CASCADE,
    run_id      BIGINT NOT NULL REFERENCES payroll_runs(id) ON DELETE CASCADE,
    provider    VARCHAR(20) NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    status      VARCHAR(30) NOT NULL DEFAULT 'submitted',
    total       NUMERIC(14, 2) NOT NULL,
    item_count  INT NOT NULL,
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, provider)
);

CREATE TABLE payout_items (
    id           BIGSERIAL PRIMARY KEY,
    batch_id     BIGINT NOT NULL REFERENCES payout_batches(id) ON DELETE CASCADE,
    employee_id  BIGINT NOT NULL REFERENCES employees(id),
    external_id  TEXT NOT NULL UNIQUE,
    amount       NUMERIC(14, 2) NOT NULL,
    bank_code    VARCHAR(40) NOT NULL,
    account_no   VARCHAR(50) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    failure_code TEXT NOT NULL DEFAULT '',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS payout_items;
DROP TABLE IF EXISTS payout_batches;
