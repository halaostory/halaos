-- name: GetSSSContribution :one
SELECT * FROM sss_contribution_table
WHERE msc_min <= $1 AND msc_max >= $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
LIMIT 1;

-- name: ListSSSTable :many
SELECT * FROM sss_contribution_table
WHERE effective_from <= $1 AND (effective_to IS NULL OR effective_to >= $1)
ORDER BY msc_min;

-- name: GetPhilHealthContribution :one
SELECT * FROM philhealth_contribution_table
WHERE salary_min <= $1 AND salary_max >= $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
LIMIT 1;

-- name: ListPhilHealthTable :many
SELECT * FROM philhealth_contribution_table
WHERE effective_from <= $1 AND (effective_to IS NULL OR effective_to >= $1)
ORDER BY salary_min;

-- name: GetPagIBIGContribution :one
SELECT * FROM pagibig_contribution_table
WHERE salary_min <= $1 AND salary_max >= $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
LIMIT 1;

-- name: ListPagIBIGTable :many
SELECT * FROM pagibig_contribution_table
WHERE effective_from <= $1 AND (effective_to IS NULL OR effective_to >= $1)
ORDER BY salary_min;

-- name: GetBIRTaxBracket :one
SELECT * FROM bir_tax_table
WHERE frequency = $1
  AND bracket_min <= $2
  AND (bracket_max IS NULL OR bracket_max > $2)
  AND effective_from <= $3
  AND (effective_to IS NULL OR effective_to >= $3)
LIMIT 1;

-- name: ListBIRTaxTable :many
SELECT * FROM bir_tax_table
WHERE frequency = $1
  AND effective_from <= $2 AND (effective_to IS NULL OR effective_to >= $2)
ORDER BY bracket_min;

-- name: CreateGovernmentForm :one
INSERT INTO government_forms (company_id, form_type, tax_year, period, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGovernmentForms :many
SELECT * FROM government_forms
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPayrollItemsWithEmployeeForPeriod :many
SELECT pi.employee_id,
       e.employee_no, e.first_name, e.last_name, e.middle_name,
       e.birth_date, e.hire_date,
       ep.tin, ep.sss_no, ep.philhealth_no, ep.pagibig_no,
       pi.basic_pay, pi.gross_pay, pi.taxable_income,
       pi.total_deductions, pi.net_pay,
       pi.sss_ee, pi.sss_er, pi.sss_ec,
       pi.philhealth_ee, pi.philhealth_er,
       pi.pagibig_ee, pi.pagibig_er,
       pi.withholding_tax, pi.ot_hours, pi.work_days
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
JOIN employees e ON e.id = pi.employee_id
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
WHERE pc.company_id = $1
  AND pc.period_start >= $2
  AND pc.period_end <= $3
  AND pr.status = 'completed'
ORDER BY e.last_name, e.first_name;

-- name: GetPayrollItemsForYear :many
SELECT pi.employee_id,
       e.employee_no, e.first_name, e.last_name, e.middle_name,
       e.birth_date, e.hire_date,
       ep.tin, ep.sss_no, ep.philhealth_no, ep.pagibig_no,
       pi.basic_pay, pi.gross_pay, pi.taxable_income,
       pi.total_deductions, pi.net_pay,
       pi.sss_ee, pi.sss_er, pi.sss_ec,
       pi.philhealth_ee, pi.philhealth_er,
       pi.pagibig_ee, pi.pagibig_er,
       pi.withholding_tax, pi.ot_hours, pi.work_days
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
JOIN employees e ON e.id = pi.employee_id
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
WHERE pc.company_id = $1
  AND pc.period_start >= $2
  AND pc.period_start < $3
  AND pr.status = 'completed'
ORDER BY e.last_name, e.first_name, pc.period_start;
