-- Device inventory & MDM actions.

-- name: CreateDevice :one
INSERT INTO company_devices (company_id, asset_tag, kind, make_model, serial_no, os, mdm_device_id, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateDevice :one
UPDATE company_devices SET kind = $3, make_model = $4, serial_no = $5, os = $6, mdm_device_id = $7, notes = $8,
    status = CASE WHEN sqlc.arg(new_status)::text = '' THEN status ELSE sqlc.arg(new_status)::text END, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: GetDevice :one
SELECT * FROM company_devices WHERE id = $1 AND company_id = $2;

-- name: ListDevices :many
SELECT d.*, COALESCE(e.first_name || ' ' || e.last_name, '')::text AS employee_name
FROM company_devices d LEFT JOIN employees e ON e.id = d.assigned_employee_id
WHERE d.company_id = $1
  AND (sqlc.arg(employee_id)::bigint = 0 OR d.assigned_employee_id = sqlc.arg(employee_id)::bigint)
ORDER BY d.asset_tag;

-- name: AssignDevice :one
UPDATE company_devices SET assigned_employee_id = $3, assigned_at = NOW(), status = 'assigned', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('in_stock', 'assigned')
RETURNING *;

-- name: UnassignDevice :one
UPDATE company_devices SET assigned_employee_id = NULL, assigned_at = NULL, status = 'in_stock', updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: SetDeviceStatus :exec
UPDATE company_devices SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: ListAssignedDevicesForEmployee :many
SELECT * FROM company_devices WHERE assigned_employee_id = $1 AND company_id = $2 AND status IN ('assigned', 'locked', 'return_pending');

-- name: UpsertMDMConnection :one
INSERT INTO mdm_connections (company_id, provider, url, secret_enc, auto_lock_on_termination)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id) DO UPDATE SET provider = EXCLUDED.provider, url = EXCLUDED.url,
    secret_enc = CASE WHEN EXCLUDED.secret_enc = '' THEN mdm_connections.secret_enc ELSE EXCLUDED.secret_enc END,
    auto_lock_on_termination = EXCLUDED.auto_lock_on_termination, updated_at = NOW()
RETURNING *;

-- name: GetMDMConnection :one
SELECT * FROM mdm_connections WHERE company_id = $1;

-- name: CreateDeviceAction :one
INSERT INTO device_actions (company_id, device_id, employee_id, action, status, trigger, requested_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListDeviceActions :many
SELECT a.*, d.asset_tag, d.make_model FROM device_actions a JOIN company_devices d ON d.id = a.device_id
WHERE a.company_id = $1 AND (sqlc.arg(device_id)::bigint = 0 OR a.device_id = sqlc.arg(device_id)::bigint)
ORDER BY a.created_at DESC LIMIT 200;

-- name: ApproveDeviceAction :one
UPDATE device_actions SET status = 'pending', approved_by = $3
WHERE id = $1 AND company_id = $2 AND status = 'requires_approval'
RETURNING *;

-- name: CancelDeviceAction :one
UPDATE device_actions SET status = 'cancelled'
WHERE id = $1 AND company_id = $2 AND status IN ('pending', 'requires_approval', 'failed', 'manual')
RETURNING *;

-- name: CompleteManualDeviceAction :one
UPDATE device_actions SET status = 'completed', result = $3, completed_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('manual', 'failed')
RETURNING *;

-- name: ClaimPendingDeviceActions :many
UPDATE device_actions SET status = 'running', attempts = attempts + 1
WHERE id IN (
    SELECT id FROM device_actions WHERE status = 'pending' ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: FinishDeviceAction :exec
UPDATE device_actions SET status = sqlc.arg(status)::text, result = sqlc.narg(result), error_message = sqlc.narg(error_message),
    completed_at = CASE WHEN sqlc.arg(status)::text = 'completed' THEN NOW() ELSE completed_at END
WHERE id = sqlc.arg(id);

-- name: RecoverStuckDeviceActions :exec
UPDATE device_actions SET status = 'failed', error_message = 'interrupted while sending to MDM; check the device in your MDM console, then retry or mark done'
WHERE status = 'running' AND created_at < NOW() - INTERVAL '30 minutes';
