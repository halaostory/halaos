-- name: ListEmployeeBenefitDeductions :many
-- List active benefit deductions for an employee at a given date
SELECT * FROM employee_benefit_deductions
WHERE company_id = $1 AND employee_id = $2
  AND effective_date <= $3
  AND (end_date IS NULL OR end_date >= $3)
ORDER BY deduction_type;

-- name: CreateBenefitDeduction :one
INSERT INTO employee_benefit_deductions (
    company_id, employee_id, deduction_type, amount_per_period,
    annual_limit, reduces_fica, effective_date, end_date
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateBenefitDeduction :one
UPDATE employee_benefit_deductions SET
    deduction_type = $3,
    amount_per_period = $4,
    annual_limit = $5,
    reduces_fica = $6,
    effective_date = $7,
    end_date = $8,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteBenefitDeduction :exec
DELETE FROM employee_benefit_deductions WHERE id = $1 AND company_id = $2;

-- name: ListBenefitDeductionsByCompany :many
SELECT ebd.*, e.first_name, e.last_name, e.employee_no
FROM employee_benefit_deductions ebd
JOIN employees e ON e.id = ebd.employee_id
WHERE ebd.company_id = $1
ORDER BY e.last_name, e.first_name, ebd.deduction_type;

-- name: GetCompanyRegistrationNumber :one
SELECT * FROM company_registration_numbers
WHERE company_id = $1 AND registration_type = $2;

-- name: UpsertCompanyRegistrationNumber :one
INSERT INTO company_registration_numbers (company_id, country, registration_type, registration_value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (company_id, registration_type)
DO UPDATE SET registration_value = EXCLUDED.registration_value
RETURNING *;

-- name: ListCompanyRegistrationNumbers :many
SELECT * FROM company_registration_numbers
WHERE company_id = $1 AND country = $2
ORDER BY registration_type;

-- name: DeleteCompanyRegistrationNumber :exec
DELETE FROM company_registration_numbers WHERE id = $1 AND company_id = $2;

-- name: GetYTDPayrollTotals :one
-- Get YTD totals for an employee (for SS wage base cap, 401k limit tracking)
SELECT
    COALESCE(SUM(CAST(pi.gross_pay AS NUMERIC)), 0) AS ytd_gross,
    COALESCE(SUM(CAST(pi.social_security_ee AS NUMERIC)), 0) AS ytd_ss_ee,
    COALESCE(SUM(CAST(pi.pretax_deductions AS NUMERIC)), 0) AS ytd_pretax
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
WHERE pi.employee_id = $1
  AND pr.company_id = $2
  AND pr.status = 'completed'
  AND EXTRACT(YEAR FROM pc.period_start) = $3;

-- name: GetStateTaxBrackets :many
-- List state tax brackets for a given state and filing status
SELECT * FROM country_tax_brackets
WHERE country = 'USA'
  AND state = $1
  AND filing_status = $2
  AND effective_from <= $3
  AND (effective_to IS NULL OR effective_to >= $3)
ORDER BY bracket_min;

-- name: GetFederalTaxBrackets :many
-- List federal tax brackets for a given filing status
SELECT * FROM country_tax_brackets
WHERE country = 'USA'
  AND state IS NULL
  AND filing_status = $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
ORDER BY bracket_min;
