-- name: CreateSalaryStructure :one
INSERT INTO salary_structures (company_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListSalaryStructures :many
SELECT * FROM salary_structures WHERE company_id = $1 AND is_active = true ORDER BY name;

-- name: CreateSalaryComponent :one
INSERT INTO salary_components (company_id, code, name, component_type, is_taxable, is_statutory, is_fixed, formula)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListSalaryComponents :many
SELECT * FROM salary_components WHERE company_id = $1 AND is_active = true ORDER BY component_type, name;

-- name: CreateEmployeeSalary :one
INSERT INTO employee_salaries (company_id, employee_id, structure_id, basic_salary, effective_from, effective_to, remarks, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetCurrentSalary :one
SELECT * FROM employee_salaries
WHERE company_id = $1 AND employee_id = $2
  AND effective_from <= $3
  AND (effective_to IS NULL OR effective_to >= $3)
ORDER BY effective_from DESC
LIMIT 1;

-- name: ListEmployeeSalaryComponents :many
SELECT esc.*, sc.code, sc.name, sc.component_type, sc.is_taxable
FROM employee_salary_components esc
JOIN salary_components sc ON sc.id = esc.component_id
WHERE esc.employee_salary_id = $1;

-- name: CreatePayrollCycle :one
INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date, cycle_type, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPayrollCycle :one
SELECT * FROM payroll_cycles WHERE id = $1 AND company_id = $2;

-- name: ListPayrollCycles :many
SELECT * FROM payroll_cycles WHERE company_id = $1 ORDER BY period_start DESC LIMIT $2 OFFSET $3;

-- name: UpdatePayrollCycleStatus :exec
UPDATE payroll_cycles SET status = $3, updated_at = NOW() WHERE id = $1 AND company_id = $2;

-- name: ApprovePayrollCycle :exec
UPDATE payroll_cycles SET
    status = 'approved',
    approved_by = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: CreatePayrollRun :one
INSERT INTO payroll_runs (company_id, cycle_id, run_type, initiated_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdatePayrollRun :exec
UPDATE payroll_runs SET
    total_employees = $2,
    total_gross = $3,
    total_deductions = $4,
    total_net = $5,
    status = $6,
    error_message = $7,
    completed_at = CASE WHEN $6 IN ('completed', 'failed') THEN NOW() ELSE completed_at END
WHERE id = $1;

-- name: CreatePayrollItem :one
INSERT INTO payroll_items (
    run_id, employee_id, basic_pay, gross_pay, taxable_income, total_deductions, net_pay,
    sss_ee, sss_er, sss_ec, philhealth_ee, philhealth_er,
    pagibig_ee, pagibig_er, withholding_tax,
    breakdown, work_days, hours_worked, ot_hours,
    late_deduction, undertime_deduction, holiday_pay, night_diff
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
) RETURNING *;

-- name: ListPayrollItems :many
SELECT pi.*, e.employee_no, e.first_name, e.last_name
FROM payroll_items pi
JOIN employees e ON e.id = pi.employee_id
WHERE pi.run_id = $1
ORDER BY e.last_name, e.first_name;

-- name: CreatePayslip :one
INSERT INTO payslips (company_id, run_id, employee_id, period_start, period_end, pay_date, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPayslip :one
SELECT * FROM payslips WHERE id = $1 AND employee_id = $2;

-- name: ListPayslips :many
SELECT * FROM payslips
WHERE company_id = $1 AND employee_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
