-- Employee lifecycle: hire / terminate orchestration, HR inbox, custom fields.

-- name: MarkEmployeeSeparated :one
UPDATE employees SET
    status = 'separated',
    separation_date = @separation_date,
    updated_at = NOW()
WHERE id = @id AND company_id = @company_id
RETURNING id, status, separation_date;

-- name: CreateEmployeeSeparation :one
INSERT INTO employee_separations (
    company_id, employee_id, separation_type, reason, notice_date,
    last_working_day, rehire_eligible, clearance_id, revoke_access, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetActiveSeparation :one
SELECT * FROM employee_separations
WHERE employee_id = $1 AND company_id = $2 AND status = 'scheduled'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListDueSeparations :many
-- Scheduled separations whose last working day has arrived (all companies; worker job).
SELECT * FROM employee_separations
WHERE status = 'scheduled' AND last_working_day <= CURRENT_DATE
ORDER BY last_working_day
LIMIT 200;

-- name: CompleteEmployeeSeparation :exec
UPDATE employee_separations SET status = 'completed', completed_at = NOW()
WHERE id = $1 AND status = 'scheduled';

-- name: CancelEmployeeSeparation :one
UPDATE employee_separations SET status = 'cancelled'
WHERE id = $1 AND company_id = $2 AND status = 'scheduled'
RETURNING *;

-- name: NextEmployeeNumber :one
SELECT (COALESCE(MAX(NULLIF(regexp_replace(employee_no, '\D', '', 'g'), '')::bigint), 0) + 1)::bigint AS next_no
FROM employees
WHERE company_id = $1;

-- name: GetLatestEmployeeSeparation :one
SELECT * FROM employee_separations
WHERE employee_id = $1 AND company_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestClearanceForEmployee :one
SELECT * FROM clearance_requests
WHERE employee_id = $1 AND company_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListEmployeeProvisioningJobs :many
SELECT id, provider, action_type, status, error_message, scheduled_at, completed_at, created_at
FROM provisioning_jobs
WHERE employee_id = $1 AND company_id = $2
ORDER BY created_at DESC
LIMIT 50;

-- name: GetFinalPayStatusForEmployee :one
SELECT id, status, total_final_pay, computed_at, released_at
FROM final_pay
WHERE employee_id = $1 AND company_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListHRInboxItems :many
-- Everything across modules that needs a human decision, most urgent first.
SELECT inbox.kind, inbox.ref_id, inbox.employee_id, inbox.employee_name, inbox.title,
       COALESCE(to_char(inbox.due_date, 'YYYY-MM-DD'), '')::text AS due_date, inbox.priority, inbox.created_at
FROM (
    SELECT 'leave_approval'::text AS kind, lr.id AS ref_id, lr.employee_id,
           (e.first_name || ' ' || e.last_name)::text AS employee_name,
           'Leave request'::text AS title, lr.start_date::date AS due_date,
           'normal'::text AS priority, lr.created_at
    FROM leave_requests lr JOIN employees e ON e.id = lr.employee_id
    WHERE lr.company_id = @company_id AND lr.status = 'pending'
    UNION ALL
    SELECT 'overtime_approval', o.id, o.employee_id, e.first_name || ' ' || e.last_name,
           'Overtime request', o.ot_date, 'normal', o.created_at
    FROM overtime_requests o JOIN employees e ON e.id = o.employee_id
    WHERE o.company_id = @company_id AND o.status = 'pending'
    UNION ALL
    SELECT 'expense_approval', x.id, x.employee_id, e.first_name || ' ' || e.last_name,
           x.description, x.expense_date, 'normal', x.created_at
    FROM expense_claims x JOIN employees e ON e.id = x.employee_id
    WHERE x.company_id = @company_id AND x.status = 'submitted'
    UNION ALL
    SELECT 'attendance_correction', ac.id, ac.employee_id, e.first_name || ' ' || e.last_name,
           'Attendance correction', ac.correction_date, 'normal', ac.created_at
    FROM attendance_corrections ac JOIN employees e ON e.id = ac.employee_id
    WHERE ac.company_id = @company_id AND ac.status = 'pending'
    UNION ALL
    SELECT CASE WHEN ot.workflow_type = 'offboarding' THEN 'offboarding_task' ELSE 'onboarding_task' END,
           ot.id, ot.employee_id, e.first_name || ' ' || e.last_name,
           ot.title::text, ot.due_date,
           CASE WHEN ot.due_date IS NOT NULL AND ot.due_date < CURRENT_DATE THEN 'high' ELSE 'normal' END,
           ot.created_at
    FROM onboarding_tasks ot JOIN employees e ON e.id = ot.employee_id
    WHERE ot.company_id = @company_id AND ot.status IN ('pending', 'in_progress')
    UNION ALL
    SELECT 'document_expiring', 0::bigint, d.employee_id, e.first_name || ' ' || e.last_name,
           (d.doc_type || ': ' || d.file_name)::text, d.expiry_date,
           CASE WHEN d.expiry_date < CURRENT_DATE THEN 'high' ELSE 'normal' END, d.created_at
    FROM employee_documents d JOIN employees e ON e.id = d.employee_id
    WHERE d.company_id = @company_id AND e.status <> 'separated'
      AND d.expiry_date IS NOT NULL AND d.expiry_date <= CURRENT_DATE + 30
    UNION ALL
    SELECT 'compliance_alert', ca.id, 0::bigint, ''::text, ca.title, ca.due_date,
           CASE WHEN ca.severity IN ('high', 'critical') THEN 'high' ELSE 'normal' END,
           ca.calculated_at
    FROM compliance_alerts ca
    WHERE ca.company_id = @company_id AND ca.is_resolved = false
    UNION ALL
    SELECT 'contract_milestone', cm.id, cm.employee_id, e.first_name || ' ' || e.last_name,
           cm.milestone_type::text, cm.milestone_date,
           CASE WHEN cm.milestone_date <= CURRENT_DATE + 7 THEN 'high' ELSE 'normal' END,
           cm.created_at
    FROM contract_milestones cm JOIN employees e ON e.id = cm.employee_id
    WHERE cm.company_id = @company_id AND cm.status = 'pending'
    UNION ALL
    SELECT 'provisioning_approval', 0::bigint, pj.employee_id, e.first_name || ' ' || e.last_name,
           (pj.action_type || ' ' || pj.provider)::text, pj.scheduled_at::date,
           'normal', pj.created_at
    FROM provisioning_jobs pj JOIN employees e ON e.id = pj.employee_id
    WHERE pj.company_id = @company_id AND pj.status IN ('requires_approval', 'failed', 'dead')
) inbox
ORDER BY (inbox.priority = 'high') DESC, inbox.due_date NULLS LAST, inbox.created_at DESC
LIMIT @lim;

-- name: ListCustomFields :many
SELECT * FROM employee_custom_fields
WHERE company_id = $1 AND (sqlc.arg(include_inactive)::boolean OR is_active)
ORDER BY sort_order, id;

-- name: CreateCustomField :one
INSERT INTO employee_custom_fields (
    company_id, field_key, label, field_type, options, is_required, is_sensitive, sort_order
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateCustomField :one
UPDATE employee_custom_fields SET
    label = $3,
    options = $4,
    is_required = $5,
    is_sensitive = $6,
    sort_order = $7,
    is_active = $8,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListEmployeeCustomFieldValues :many
SELECT f.id AS field_id, f.field_key, f.label, f.field_type, f.options,
       f.is_required, f.is_sensitive, v.value, v.updated_at
FROM employee_custom_fields f
LEFT JOIN employee_custom_field_values v
       ON v.field_id = f.id AND v.employee_id = @employee_id
WHERE f.company_id = @company_id AND f.is_active
ORDER BY f.sort_order, f.id;

-- name: UpsertCustomFieldValue :exec
INSERT INTO employee_custom_field_values (company_id, employee_id, field_id, value, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (employee_id, field_id) DO UPDATE SET
    value = EXCLUDED.value,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW();

-- name: DeleteCustomFieldValue :exec
DELETE FROM employee_custom_field_values
WHERE employee_id = $1 AND field_id = $2 AND company_id = $3;

-- name: SkipOpenChecklistTasks :execrows
-- Closes a checklist that no longer applies (e.g. onboarding of someone who left,
-- offboarding of a cancelled separation).
UPDATE onboarding_tasks SET
    status = 'skipped',
    notes = COALESCE(notes, sqlc.arg(note)::text),
    updated_at = NOW()
WHERE employee_id = $1 AND company_id = $2 AND workflow_type = $3
  AND status IN ('pending', 'in_progress');
