-- Leave policy rules

-- name: ListLeavePolicyRules :many
SELECT * FROM leave_policy_rules WHERE company_id = $1 ORDER BY leave_type_id, priority DESC, id;

-- name: CreateLeavePolicyRule :one
INSERT INTO leave_policy_rules (company_id, leave_type_id, name, priority, department_ids, employment_types, gender,
    min_tenure_months, max_tenure_months, days, is_active, workplace_ids)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateLeavePolicyRule :one
UPDATE leave_policy_rules SET leave_type_id = $3, name = $4, priority = $5, department_ids = $6, employment_types = $7,
    gender = $8, min_tenure_months = $9, max_tenure_months = $10, days = $11, is_active = $12, workplace_ids = $13, updated_at = now()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteLeavePolicyRule :exec
DELETE FROM leave_policy_rules WHERE id = $1 AND company_id = $2;

-- name: ListEmployeesForLeavePolicy :many
SELECT e.id, e.first_name, e.last_name, e.department_id, e.employment_type, COALESCE(e.gender, '')::text AS gender, e.hire_date,
       ew.workplace_id
FROM employees e
LEFT JOIN employee_workplaces ew ON ew.employee_id = e.id
WHERE e.company_id = $1 AND e.status IN ('active', 'probationary') AND e.hire_date <= CURRENT_DATE + 60;

-- name: ListLeaveBalancesForYear :many
SELECT employee_id, leave_type_id, earned FROM leave_balances WHERE company_id = $1 AND year = $2;

-- name: SetLeaveEarned :exec
INSERT INTO leave_balances (company_id, employee_id, leave_type_id, year, earned, used, carried, adjusted)
VALUES ($1, $2, $3, $4, $5, 0, 0, 0)
ON CONFLICT (company_id, employee_id, leave_type_id, year) DO UPDATE SET earned = EXCLUDED.earned, updated_at = now();

-- name: ListCompaniesWithLeavePolicies :many
SELECT DISTINCT company_id FROM leave_policy_rules WHERE is_active;
