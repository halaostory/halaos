-- Integration Hub tables for Employee OS
-- Manages SaaS connections, provisioning templates, identity mapping, and job queue

-- Per-company SaaS connections (one row per provider per company)
CREATE TABLE IF NOT EXISTS integration_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    provider VARCHAR(50) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'error', 'revoked')),
    auth_type VARCHAR(20) NOT NULL DEFAULT 'oauth2'
        CHECK (auth_type IN ('oauth2', 'api_key', 'bot_token')),
    encrypted_creds BYTEA,
    oauth_token_expiry TIMESTAMPTZ,
    oauth_scope TEXT NOT NULL DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    last_error TEXT,
    error_count INT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, provider)
);

CREATE INDEX idx_integration_connections_company ON integration_connections(company_id);
CREATE INDEX idx_integration_connections_status ON integration_connections(status);

-- PKCE state for OAuth2 flows (short-lived)
CREATE TABLE IF NOT EXISTS integration_oauth_states (
    state VARCHAR(64) PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    provider VARCHAR(50) NOT NULL,
    code_verifier VARCHAR(128) NOT NULL DEFAULT '',
    redirect_uri TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '10 minutes')
);

CREATE INDEX idx_oauth_states_expires ON integration_oauth_states(expires_at);

-- Provisioning templates: what to do per provider per event
CREATE TABLE IF NOT EXISTS provisioning_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    connection_id UUID NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    event_trigger VARCHAR(50) NOT NULL
        CHECK (event_trigger IN ('employee.hired', 'employee.terminated', 'employee.transferred')),
    action_type VARCHAR(20) NOT NULL
        CHECK (action_type IN ('provision', 'deprovision', 'update')),
    filter_department_id BIGINT,
    filter_employment_type VARCHAR(50),
    params JSONB NOT NULL DEFAULT '{}',
    deprovision_mode VARCHAR(20) NOT NULL DEFAULT 'disable'
        CHECK (deprovision_mode IN ('disable', 'delete', 'transfer', 'none')),
    requires_approval BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_provisioning_templates_company ON provisioning_templates(company_id);
CREATE INDEX idx_provisioning_templates_trigger ON provisioning_templates(company_id, event_trigger, is_active);

-- Maps employee ↔ external account (one per provider per employee)
CREATE TABLE IF NOT EXISTS integration_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    connection_id UUID NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    external_id VARCHAR(255) NOT NULL DEFAULT '',
    external_email VARCHAR(255),
    external_username VARCHAR(255),
    account_status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (account_status IN ('active', 'disabled', 'deleted', 'pending')),
    metadata JSONB NOT NULL DEFAULT '{}',
    provisioned_at TIMESTAMPTZ,
    deprovisioned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, employee_id, provider)
);

CREATE INDEX idx_integration_identities_employee ON integration_identities(employee_id);
CREATE INDEX idx_integration_identities_connection ON integration_identities(connection_id);

-- Job queue for async provisioning
CREATE TABLE IF NOT EXISTS provisioning_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    connection_id UUID NOT NULL REFERENCES integration_connections(id),
    template_id UUID REFERENCES provisioning_templates(id),
    provider VARCHAR(50) NOT NULL,
    action_type VARCHAR(20) NOT NULL
        CHECK (action_type IN ('provision', 'deprovision', 'update')),
    trigger_event_id BIGINT REFERENCES hr_events(id),
    resolved_params JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(30) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'requires_approval', 'approved', 'running', 'completed', 'failed', 'dead', 'skipped')),
    draft_id UUID REFERENCES action_drafts(id),
    result JSONB,
    error_message TEXT,
    retries INT NOT NULL DEFAULT 0,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_provisioning_jobs_status ON provisioning_jobs(status, scheduled_at);
CREATE INDEX idx_provisioning_jobs_company ON provisioning_jobs(company_id);
CREATE INDEX idx_provisioning_jobs_employee ON provisioning_jobs(employee_id);

-- Immutable audit log for all external API actions
CREATE TABLE IF NOT EXISTS integration_audit_log (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT,
    connection_id UUID,
    job_id UUID,
    provider VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    external_id VARCHAR(255),
    success BOOLEAN NOT NULL DEFAULT false,
    request_summary JSONB,
    response_summary JSONB,
    error_code VARCHAR(50),
    error_message TEXT,
    actor_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_integration_audit_company ON integration_audit_log(company_id, created_at DESC);
CREATE INDEX idx_integration_audit_provider ON integration_audit_log(provider, created_at DESC);
