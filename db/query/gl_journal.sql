-- Payroll journal entries

-- name: GetGLAccountMapping :one
SELECT accounts FROM gl_account_mappings WHERE company_id = $1;

-- name: UpsertGLAccountMapping :exec
INSERT INTO gl_account_mappings (company_id, accounts, updated_by) VALUES ($1, $2, $3)
ON CONFLICT (company_id) DO UPDATE SET accounts = EXCLUDED.accounts, updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: GetPayrollRunJournalTotals :one
SELECT r.id, r.cycle_id, c.name AS cycle_name, c.pay_date, c.period_start, c.period_end, r.status, r.total_employees,
       COALESCE(SUM(pi.gross_pay), 0)::float8 AS gross,
       COALESCE(SUM(pi.net_pay), 0)::float8 AS net,
       COALESCE(SUM(pi.withholding_tax), 0)::float8 AS wtax,
       COALESCE(SUM(pi.sss_ee), 0)::float8 AS sss_ee,
       COALESCE(SUM(pi.sss_er), 0)::float8 AS sss_er,
       COALESCE(SUM(pi.sss_ec), 0)::float8 AS sss_ec,
       COALESCE(SUM(pi.philhealth_ee), 0)::float8 AS philhealth_ee,
       COALESCE(SUM(pi.philhealth_er), 0)::float8 AS philhealth_er,
       COALESCE(SUM(pi.pagibig_ee), 0)::float8 AS pagibig_ee,
       COALESCE(SUM(pi.pagibig_er), 0)::float8 AS pagibig_er
FROM payroll_runs r
JOIN payroll_cycles c ON c.id = r.cycle_id
LEFT JOIN payroll_items pi ON pi.run_id = r.id
WHERE r.id = $1 AND r.company_id = $2
GROUP BY r.id, c.id;

-- name: LatestCompletedRunForCycle :one
SELECT id FROM payroll_runs
WHERE cycle_id = $1 AND company_id = $2 AND status IN ('completed', 'ready_for_approval')
ORDER BY id DESC LIMIT 1;
