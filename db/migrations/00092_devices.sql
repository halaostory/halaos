-- +goose Up

-- Company device inventory + MDM actions (lock / wipe / return) driven by the
-- employee lifecycle. Wipe always needs a human approval.

CREATE TABLE IF NOT EXISTS company_devices (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    asset_tag VARCHAR(60) NOT NULL,
    kind VARCHAR(20) NOT NULL DEFAULT 'laptop'
        CHECK (kind IN ('laptop', 'desktop', 'phone', 'tablet', 'monitor', 'other')),
    make_model VARCHAR(120) NOT NULL DEFAULT '',
    serial_no VARCHAR(120) NOT NULL DEFAULT '',
    os VARCHAR(40) NOT NULL DEFAULT '',
    mdm_device_id VARCHAR(120),
    assigned_employee_id BIGINT REFERENCES employees(id),
    assigned_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'in_stock'
        CHECK (status IN ('in_stock', 'assigned', 'return_pending', 'locked', 'wiped', 'lost', 'retired')),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, asset_tag)
);
CREATE INDEX IF NOT EXISTS idx_company_devices_employee ON company_devices(assigned_employee_id);

-- One MDM gateway per company (signed webhook to Jamf/Kandji/Intune/Google bridge).
CREATE TABLE IF NOT EXISTS mdm_connections (
    company_id BIGINT PRIMARY KEY REFERENCES companies(id),
    provider VARCHAR(30) NOT NULL DEFAULT 'webhook',
    url TEXT NOT NULL,
    secret_enc TEXT NOT NULL DEFAULT '',
    auto_lock_on_termination BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS device_actions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    device_id BIGINT NOT NULL REFERENCES company_devices(id) ON DELETE CASCADE,
    employee_id BIGINT REFERENCES employees(id),
    action VARCHAR(20) NOT NULL CHECK (action IN ('lock', 'unlock', 'wipe', 'return_request')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'requires_approval', 'running', 'completed', 'manual', 'failed', 'cancelled')),
    trigger VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (trigger IN ('manual', 'termination', 'ai')),
    requested_by BIGINT REFERENCES users(id),
    approved_by BIGINT REFERENCES users(id),
    result TEXT,
    error_message TEXT,
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_device_actions_queue ON device_actions(status, created_at);
CREATE INDEX IF NOT EXISTS idx_device_actions_device ON device_actions(device_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS device_actions;
DROP TABLE IF EXISTS mdm_connections;
DROP TABLE IF EXISTS company_devices;
