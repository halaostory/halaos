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
    UNION ALL
    SELECT CASE gs.status WHEN 'ready' THEN 'gov_filing_approval' WHEN 'draft' THEN 'gov_filing_blocked'
                          WHEN 'approved' THEN 'gov_filing_manual' ELSE 'gov_filing_failed' END,
           gs.id, 0::bigint, ''::text, (gs.agency || ' ' || gs.form_code || ' ' || gs.period)::text, gs.due_date,
           CASE WHEN gs.status = 'failed' OR (gs.due_date IS NOT NULL AND gs.due_date <= CURRENT_DATE + 3) THEN 'high' ELSE 'normal' END,
           gs.updated_at
    FROM gov_submissions gs
    WHERE gs.company_id = @company_id
      AND (gs.status IN ('ready', 'draft', 'failed') OR (gs.status = 'approved' AND gs.channel = 'manual'))
    UNION ALL
    SELECT 'gov_notice', gn.id, 0::bigint, ''::text,
           (CASE WHEN gn.agency = '' THEN 'Government' ELSE gn.agency END || ' notice: ' || COALESCE(NULLIF(gn.title, ''), gn.notice_type))::text,
           gn.respond_by, CASE WHEN gn.respond_by <= CURRENT_DATE + 7 OR gn.notice_type IN ('assessment', 'audit') THEN 'high' ELSE 'normal' END, gn.created_at
    FROM gov_notices gn
    WHERE gn.company_id = @company_id AND gn.status IN ('open', 'awaiting_approval', 'escalated')
    UNION ALL
    SELECT 'gov_human_input', hr.id, 0::bigint, ''::text, (gs.agency || ' ' || gs.form_code || ': ' || hr.name)::text,
           hr.expires_at::date, 'high', hr.created_at
    FROM gov_human_requests hr JOIN gov_submissions gs ON gs.id = hr.submission_id
    WHERE hr.company_id = @company_id AND hr.status = 'open' AND hr.expires_at > NOW()
      AND gs.status IN ('running', 'needs_human')
    UNION ALL
    SELECT 'self_onboarding_review', oi.id, oi.employee_id, e.first_name || ' ' || e.last_name,
           'New hire submitted onboarding details'::text, e.hire_date, 'normal', oi.submitted_at
    FROM onboarding_invites oi JOIN employees e ON e.id = oi.employee_id
    WHERE oi.company_id = @company_id AND oi.status = 'submitted' AND oi.submitted_at > NOW() - INTERVAL '7 days'
    UNION ALL
    SELECT 'signature_declined', sr.id, sr.employee_id, e.first_name || ' ' || e.last_name, sr.title::text,
           NULL::date, 'high', sr.created_at
    FROM signature_requests sr JOIN employees e ON e.id = sr.employee_id
    WHERE sr.company_id = @company_id AND sr.status = 'declined' AND sr.created_at > NOW() - INTERVAL '30 days'
    UNION ALL
    SELECT 'device_action', da.id, COALESCE(da.employee_id, 0), COALESCE(e.first_name || ' ' || e.last_name, '')::text,
           (da.action || ' ' || d.asset_tag)::text, NULL::date,
           CASE WHEN da.status IN ('requires_approval', 'failed') THEN 'high' ELSE 'normal' END, da.created_at
    FROM device_actions da JOIN company_devices d ON d.id = da.device_id LEFT JOIN employees e ON e.id = da.employee_id
    WHERE da.company_id = @company_id AND da.status IN ('requires_approval', 'manual', 'failed')
    UNION ALL
    SELECT 'final_pay_approval', fp.id, fp.employee_id, e.first_name || ' ' || e.last_name,
           'Final pay ready for review'::text, fp.separation_date, 'normal', fp.created_at
    FROM final_pay fp JOIN employees e ON e.id = fp.employee_id
    WHERE fp.company_id = @company_id AND fp.status = 'calculated'
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

-- name: GetEmployeePayrollCoverage :one
-- Last day already covered by a completed payroll run, and basic pay paid this year.
SELECT COALESCE(MAX(pc.period_end), '0001-01-01'::date)::date AS last_paid_through,
       COALESCE(SUM(pi.basic_pay) FILTER (WHERE pc.period_start >= make_date(sqlc.arg(year)::int, 1, 1)), 0)::float8 AS ytd_basic
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
WHERE pi.employee_id = sqlc.arg(employee_id) AND pr.company_id = sqlc.arg(company_id) AND pr.status IN ('completed', 'ready_for_approval');

-- name: FinalPayExistsForSeparation :one
SELECT EXISTS (
    SELECT 1 FROM final_pay WHERE company_id = $1 AND employee_id = $2 AND separation_date = $3
)::boolean;

-- name: GetFinalPayByID :one
SELECT * FROM final_pay WHERE id = $1 AND company_id = $2;

-- name: MarkFinalPayReleased :one
-- Records which off-cycle payroll cycle paid it out.
UPDATE final_pay SET
    status = 'released',
    released_at = NOW(),
    payload = payload || jsonb_build_object('payroll_cycle_id', sqlc.arg(cycle_id)::bigint, 'payroll_run_id', sqlc.arg(run_id)::bigint)
WHERE id = sqlc.arg(id) AND company_id = sqlc.arg(company_id) AND status = 'approved'
RETURNING *;
