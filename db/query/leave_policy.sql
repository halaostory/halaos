-- Leave policy rules

-- name: ListLeavePolicyRules :many
SELECT * FROM leave_policy_rules WHERE company_id = $1 ORDER BY leave_type_id, priority DESC, id;

-- name: CreateLeavePolicyRule :one
INSERT INTO leave_policy_rules (company_id, leave_type_id, name, priority, department_ids, employment_types, gender,
    min_tenure_months, max_tenure_months, days, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateLeavePolicyRule :one
UPDATE leave_policy_rules SET leave_type_id = $3, name = $4, priority = $5, department_ids = $6, employment_types = $7,
    gender = $8, min_tenure_months = $9, max_tenure_months = $10, days = $11, is_active = $12, updated_at = now()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteLeavePolicyRule :exec
DELETE FROM leave_policy_rules WHERE id = $1 AND company_id = $2;

-- name: ListEmployeesForLeavePolicy :many
SELECT id, first_name, last_name, department_id, employment_type, COALESCE(gender, '')::text AS gender, hire_date
FROM employees
WHERE company_id = $1 AND status IN ('active', 'probationary') AND hire_date <= CURRENT_DATE + 60;

-- name: ListLeaveBalancesForYear :many
SELECT employee_id, leave_type_id, earned FROM leave_balances WHERE company_id = $1 AND year = $2;

-- name: SetLeaveEarned :exec
INSERT INTO leave_balances (company_id, employee_id, leave_type_id, year, earned, used, carried, adjusted)
VALUES ($1, $2, $3, $4, $5, 0, 0, 0)
ON CONFLICT (company_id, employee_id, leave_type_id, year) DO UPDATE SET earned = EXCLUDED.earned, updated_at = now();

-- name: ListCompaniesWithLeavePolicies :many
SELECT DISTINCT company_id FROM leave_policy_rules WHERE is_active;
