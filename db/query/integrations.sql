-- ========== integration_connections ==========

-- name: CreateIntegrationConnection :one
INSERT INTO integration_connections (company_id, provider, display_name, status, auth_type, encrypted_creds, oauth_token_expiry, oauth_scope, config, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetIntegrationConnection :one
SELECT * FROM integration_connections
WHERE id = $1 AND company_id = $2;

-- name: ListIntegrationConnections :many
SELECT * FROM integration_connections
WHERE company_id = $1
ORDER BY created_at DESC;

-- name: UpdateIntegrationConnection :one
UPDATE integration_connections SET
    display_name = CASE WHEN @display_name::text = '' THEN display_name ELSE @display_name END,
    status = CASE WHEN @status::text = '' THEN status ELSE @status END,
    encrypted_creds = COALESCE(@encrypted_creds, encrypted_creds),
    oauth_token_expiry = COALESCE(@oauth_token_expiry, oauth_token_expiry),
    oauth_scope = CASE WHEN @oauth_scope::text = '' THEN oauth_scope ELSE @oauth_scope END,
    config = CASE WHEN @config::jsonb = '{}' THEN config ELSE @config END,
    updated_at = NOW()
WHERE id = @id AND company_id = @company_id
RETURNING *;

-- name: DeleteIntegrationConnection :exec
DELETE FROM integration_connections
WHERE id = $1 AND company_id = $2;

-- name: UpdateConnectionLastUsed :exec
UPDATE integration_connections SET last_used_at = NOW(), error_count = 0, last_error = NULL
WHERE id = $1;

-- name: UpdateConnectionError :exec
UPDATE integration_connections SET
    last_error = $2,
    error_count = error_count + 1,
    status = CASE WHEN error_count + 1 >= 5 THEN 'error' ELSE status END,
    updated_at = NOW()
WHERE id = $1;

-- name: GetConnectionCredentials :one
SELECT id, encrypted_creds, auth_type, provider FROM integration_connections
WHERE id = $1 AND company_id = $2 AND status = 'active';

-- ========== integration_oauth_states ==========

-- name: CreateOAuthState :exec
INSERT INTO integration_oauth_states (state, company_id, user_id, provider, code_verifier, redirect_uri)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ConsumeOAuthState :one
DELETE FROM integration_oauth_states
WHERE state = $1 AND expires_at > NOW()
RETURNING *;

-- name: CleanExpiredOAuthStates :exec
DELETE FROM integration_oauth_states WHERE expires_at < NOW();

-- ========== provisioning_templates ==========

-- name: CreateProvisioningTemplate :one
INSERT INTO provisioning_templates (company_id, connection_id, provider, event_trigger, action_type, filter_department_id, filter_employment_type, params, deprovision_mode, requires_approval, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetProvisioningTemplate :one
SELECT * FROM provisioning_templates
WHERE id = $1 AND company_id = $2;

-- name: ListProvisioningTemplates :many
SELECT * FROM provisioning_templates
WHERE company_id = $1
ORDER BY created_at DESC;

-- name: ListProvisioningTemplatesByConnection :many
SELECT * FROM provisioning_templates
WHERE connection_id = $1 AND company_id = $2
ORDER BY created_at DESC;

-- name: ListActiveTemplatesForEvent :many
SELECT pt.*, ic.encrypted_creds, ic.auth_type, ic.status AS connection_status
FROM provisioning_templates pt
JOIN integration_connections ic ON ic.id = pt.connection_id
WHERE pt.company_id = $1
  AND pt.event_trigger = $2
  AND pt.is_active = true
  AND ic.status = 'active';

-- name: UpdateProvisioningTemplate :one
UPDATE provisioning_templates SET
    event_trigger = CASE WHEN @event_trigger::text = '' THEN event_trigger ELSE @event_trigger END,
    action_type = CASE WHEN @action_type::text = '' THEN action_type ELSE @action_type END,
    filter_department_id = @filter_department_id,
    filter_employment_type = @filter_employment_type,
    params = CASE WHEN @params::jsonb = '{}' THEN params ELSE @params END,
    deprovision_mode = CASE WHEN @deprovision_mode::text = '' THEN deprovision_mode ELSE @deprovision_mode END,
    requires_approval = @requires_approval,
    is_active = @is_active,
    updated_at = NOW()
WHERE id = @id AND company_id = @company_id
RETURNING *;

-- name: DeleteProvisioningTemplate :exec
DELETE FROM provisioning_templates
WHERE id = $1 AND company_id = $2;

-- ========== integration_identities ==========

-- name: UpsertIntegrationIdentity :one
INSERT INTO integration_identities (company_id, employee_id, connection_id, provider, external_id, external_email, external_username, account_status, metadata, provisioned_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
ON CONFLICT (company_id, employee_id, provider)
DO UPDATE SET
    external_id = EXCLUDED.external_id,
    external_email = EXCLUDED.external_email,
    external_username = EXCLUDED.external_username,
    account_status = EXCLUDED.account_status,
    metadata = EXCLUDED.metadata,
    updated_at = NOW()
RETURNING *;

-- name: GetIntegrationIdentity :one
SELECT * FROM integration_identities
WHERE id = $1 AND company_id = $2;

-- name: ListEmployeeIntegrations :many
SELECT ii.*, ic.display_name AS connection_name
FROM integration_identities ii
JOIN integration_connections ic ON ic.id = ii.connection_id
WHERE ii.employee_id = $1 AND ii.company_id = $2
ORDER BY ii.provider;

-- name: MarkIdentityDeprovisioned :exec
UPDATE integration_identities SET
    account_status = 'disabled',
    deprovisioned_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- ========== provisioning_jobs ==========

-- name: CreateProvisioningJob :one
INSERT INTO provisioning_jobs (company_id, employee_id, connection_id, template_id, provider, action_type, trigger_event_id, resolved_params, status, scheduled_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetProvisioningJob :one
SELECT * FROM provisioning_jobs
WHERE id = $1 AND company_id = $2;

-- name: ListProvisioningJobs :many
SELECT pj.*, e.first_name || ' ' || e.last_name AS employee_name
FROM provisioning_jobs pj
JOIN employees e ON e.id = pj.employee_id
WHERE pj.company_id = $1
ORDER BY pj.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ClaimPendingProvisioningJobs :many
UPDATE provisioning_jobs SET
    status = 'running',
    started_at = NOW()
WHERE id IN (
    SELECT id FROM provisioning_jobs
    WHERE status IN ('pending', 'approved')
      AND scheduled_at <= NOW()
      AND retries < 5
    ORDER BY scheduled_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteProvisioningJob :exec
UPDATE provisioning_jobs SET
    status = 'completed',
    result = $2,
    completed_at = NOW()
WHERE id = $1;

-- name: FailProvisioningJob :exec
UPDATE provisioning_jobs SET
    status = CASE WHEN retries + 1 >= 5 THEN 'dead' ELSE 'pending' END,
    retries = retries + 1,
    error_message = $2
WHERE id = $1;

-- name: SkipProvisioningJob :exec
UPDATE provisioning_jobs SET status = 'skipped', completed_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: RetryProvisioningJob :exec
UPDATE provisioning_jobs SET status = 'pending', error_message = NULL, scheduled_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('failed', 'dead');

-- ========== integration_audit_log ==========

-- name: InsertIntegrationAuditLog :exec
INSERT INTO integration_audit_log (company_id, employee_id, connection_id, job_id, provider, action, external_id, success, request_summary, response_summary, error_code, error_message, actor_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: ListIntegrationAuditLogs :many
SELECT * FROM integration_audit_log
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
