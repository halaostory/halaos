-- name: CreateSalaryStructure :one
INSERT INTO salary_structures (company_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListSalaryStructures :many
SELECT * FROM salary_structures WHERE company_id = $1 AND is_active = true ORDER BY name;

-- name: GetSalaryStructure :one
SELECT * FROM salary_structures WHERE id = $1 AND company_id = $2;

-- name: UpdateSalaryStructure :one
UPDATE salary_structures SET
    name = $3,
    description = $4
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteSalaryStructure :exec
UPDATE salary_structures SET is_active = false WHERE id = $1 AND company_id = $2;

-- name: GetSalaryComponent :one
SELECT * FROM salary_components WHERE id = $1 AND company_id = $2;

-- name: UpdateSalaryComponent :one
UPDATE salary_components SET
    code = $3,
    name = $4,
    component_type = $5,
    is_taxable = $6,
    is_statutory = $7,
    is_fixed = $8
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteSalaryComponent :exec
UPDATE salary_components SET is_active = false WHERE id = $1 AND company_id = $2;

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
    late_deduction, undertime_deduction, holiday_pay, night_diff, bonus_pay,
    federal_tax, social_security_ee, social_security_er,
    medicare_ee, medicare_er, additional_medicare,
    state_tax, state_disability, futa, sui, pretax_deductions
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
    $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35
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

-- name: GetPayrollRun :one
SELECT * FROM payroll_runs WHERE id = $1 AND company_id = $2;

-- name: ListCurrentSalaries :many
SELECT es.* FROM employee_salaries es
WHERE es.company_id = $1
  AND es.effective_from <= $2
  AND (es.effective_to IS NULL OR es.effective_to >= $2)
ORDER BY es.employee_id, es.effective_from DESC;

-- name: ListPayslips :many
SELECT * FROM payslips
WHERE company_id = $1 AND employee_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetPayrollItemsForRun :many
SELECT pi.*, e.employee_no, e.first_name, e.last_name
FROM payroll_items pi
JOIN employees e ON e.id = pi.employee_id
WHERE pi.run_id = $1
ORDER BY e.last_name, e.first_name;

-- name: GetEmployeePayrollHistory :many
SELECT pi.*, pr.cycle_id, pc.name as cycle_name, pc.period_start, pc.period_end
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
WHERE pr.company_id = $1 AND pi.employee_id = $2 AND pr.status = 'completed'
ORDER BY pc.period_start DESC
LIMIT $3;

-- name: GetLatestCompletedRunForCycle :one
SELECT id FROM payroll_runs
WHERE cycle_id = $1 AND company_id = $2 AND status = 'completed'
ORDER BY created_at DESC
LIMIT 1;

-- name: LockPayrollCycle :exec
UPDATE payroll_cycles SET
    is_locked = true,
    locked_at = NOW(),
    locked_by = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: UnlockPayrollCycle :exec
UPDATE payroll_cycles SET
    is_locked = false,
    locked_at = NULL,
    locked_by = NULL,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: IsPayrollCycleLocked :one
SELECT is_locked FROM payroll_cycles WHERE id = $1 AND company_id = $2;

-- name: ListPayrollItemsWithBank :many
SELECT pi.*, e.employee_no, e.first_name, e.last_name,
       ep.bank_name, ep.bank_account_no, ep.bank_account_name
FROM payroll_items pi
JOIN employees e ON e.id = pi.employee_id
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
WHERE pi.run_id = $1
ORDER BY e.last_name, e.first_name;

-- name: GetPayrollItemsForAccounting :many
SELECT pi.id, pi.employee_id, pi.basic_pay, pi.gross_pay, pi.total_deductions, pi.net_pay,
       pi.sss_ee, pi.sss_er, pi.sss_ec, pi.philhealth_ee, pi.philhealth_er,
       pi.pagibig_ee, pi.pagibig_er, pi.withholding_tax,
       pi.holiday_pay, pi.night_diff, pi.ot_hours, pi.bonus_pay, pi.breakdown,
       e.employee_no, e.first_name, e.last_name, e.department_id,
       ep.tin, ep.sss_no, ep.philhealth_no, ep.pagibig_no,
       COALESCE(d.name, '') as department_name
FROM payroll_items pi
JOIN employees e ON e.id = pi.employee_id
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
LEFT JOIN departments d ON d.id = e.department_id
WHERE pi.run_id = $1
ORDER BY e.department_id, e.last_name;

-- name: ListCompletedPayrollRuns :many
SELECT pr.*, pc.name as cycle_name, pc.period_start, pc.period_end, pc.pay_date
FROM payroll_runs pr
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
WHERE pr.company_id = $1 AND pr.status = 'completed'
ORDER BY pc.period_start DESC
LIMIT $2 OFFSET $3;
