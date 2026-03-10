-- name: GetPayrollAutoConfig :one
SELECT * FROM payroll_auto_config WHERE company_id = $1;

-- name: UpsertPayrollAutoConfig :one
INSERT INTO payroll_auto_config (company_id, auto_run_enabled, days_before_pay, auto_approve_enabled, max_auto_approve_amount, notify_on_auto)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (company_id) DO UPDATE SET
    auto_run_enabled = EXCLUDED.auto_run_enabled,
    days_before_pay = EXCLUDED.days_before_pay,
    auto_approve_enabled = EXCLUDED.auto_approve_enabled,
    max_auto_approve_amount = EXCLUDED.max_auto_approve_amount,
    notify_on_auto = EXCLUDED.notify_on_auto,
    updated_at = NOW()
RETURNING *;

-- name: ListCompaniesWithAutoRun :many
SELECT pac.*, c.name AS company_name
FROM payroll_auto_config pac
JOIN companies c ON c.id = pac.company_id
WHERE pac.auto_run_enabled = true;

-- name: ListDraftCyclesForAutoRun :many
SELECT pc.*
FROM payroll_cycles pc
WHERE pc.company_id = $1
  AND pc.status = 'draft'
  AND pc.pay_date - $2::int <= CURRENT_DATE
  AND pc.pay_date >= CURRENT_DATE;

-- name: InsertPayrollAutoLog :one
INSERT INTO payroll_auto_log (company_id, cycle_id, run_id, action, detail)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPayrollAutoLogs :many
SELECT * FROM payroll_auto_log
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetLatestRunForCycle :one
SELECT * FROM payroll_runs
WHERE cycle_id = $1 AND company_id = $2
ORDER BY id DESC
LIMIT 1;
