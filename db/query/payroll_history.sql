-- Payroll history import (migration from another provider)

-- name: ListEmployeeNumbers :many
SELECT id, employee_no, first_name, last_name FROM employees WHERE company_id = $1;

-- name: CountPayrollItemsInPeriod :one
-- Pay already on file for an employee in a period (to refuse double counting).
SELECT COUNT(*)::int FROM payroll_items pi
JOIN payroll_runs r ON r.id = pi.run_id
JOIN payroll_cycles c ON c.id = r.cycle_id
WHERE c.company_id = sqlc.arg(company_id) AND pi.employee_id = sqlc.arg(employee_id)
  AND r.status IN ('completed', 'ready_for_approval')
  AND c.period_start <= sqlc.arg(period_end)::date AND c.period_end >= sqlc.arg(period_start)::date;

-- name: CreateImportedCycle :one
INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date, cycle_type, status, created_by, approved_by, approved_at, is_locked, locked_at, locked_by)
VALUES ($1, $2, $3, $4, $5, 'imported', 'paid', $6, $6, now(), true, now(), $6)
RETURNING id;

-- name: CreateImportedRun :one
INSERT INTO payroll_runs (company_id, cycle_id, run_type, total_employees, total_gross, total_deductions, total_net, status, initiated_by, completed_at)
VALUES ($1, $2, 'imported', $3, $4, $5, $6, 'completed', $7, now())
RETURNING id;

-- name: CreateImportedItem :exec
INSERT INTO payroll_items (run_id, employee_id, basic_pay, gross_pay, taxable_income, total_deductions, net_pay,
    sss_ee, sss_er, sss_ec, philhealth_ee, philhealth_er, pagibig_ee, pagibig_er, withholding_tax, breakdown)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, '{"source":"import"}');

-- name: ListImportedCycles :many
SELECT c.id, c.name, c.period_start, c.period_end, c.pay_date, r.total_employees, r.total_gross::float8 AS total_gross, r.total_net::float8 AS total_net, c.created_at
FROM payroll_cycles c JOIN payroll_runs r ON r.cycle_id = c.id
WHERE c.company_id = $1 AND c.cycle_type = 'imported'
ORDER BY c.period_start DESC;

-- name: DeleteImportedItems :exec
DELETE FROM payroll_items WHERE run_id IN (
    SELECT r.id FROM payroll_runs r JOIN payroll_cycles c ON c.id = r.cycle_id
    WHERE c.id = $1 AND c.company_id = $2 AND c.cycle_type = 'imported'
);

-- name: DeleteImportedRuns :exec
DELETE FROM payroll_runs WHERE cycle_id IN (
    SELECT c.id FROM payroll_cycles c WHERE c.id = $1 AND c.company_id = $2 AND c.cycle_type = 'imported'
);

-- name: DeleteImportedCycle :execrows
DELETE FROM payroll_cycles WHERE payroll_cycles.id = $1 AND payroll_cycles.company_id = $2 AND payroll_cycles.cycle_type = 'imported';
