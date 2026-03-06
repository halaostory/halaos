-- name: ListProbationaryEmployees :many
SELECT id, company_id, employee_no, first_name, last_name,
       hire_date, regularization_date, employment_type, status
FROM employees
WHERE company_id = $1
  AND employment_type = 'probationary'
  AND status IN ('active', 'probationary')
ORDER BY hire_date;

-- name: ListContractualEmployees :many
SELECT id, company_id, employee_no, first_name, last_name,
       hire_date, contract_end_date, employment_type, status
FROM employees
WHERE company_id = $1
  AND employment_type = 'contractual'
  AND status = 'active'
  AND contract_end_date IS NOT NULL
ORDER BY contract_end_date;

-- name: ListUpcomingAnniversaries :many
SELECT id, company_id, employee_no, first_name, last_name,
       hire_date, employment_type, status
FROM employees
WHERE company_id = $1
  AND status = 'active'
  AND EXTRACT(MONTH FROM hire_date) = $2::int
  AND EXTRACT(DAY FROM hire_date) BETWEEN $3::int AND $4::int
ORDER BY hire_date;

-- name: UpsertContractMilestone :one
INSERT INTO contract_milestones (company_id, employee_id, milestone_type, milestone_date, days_remaining)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, employee_id, milestone_type, milestone_date) DO UPDATE SET
    days_remaining = EXCLUDED.days_remaining,
    updated_at = NOW()
RETURNING *;

-- name: ListContractMilestones :many
SELECT cm.*, e.employee_no, e.first_name, e.last_name, e.employment_type,
       COALESCE(d.name, '') as department_name
FROM contract_milestones cm
JOIN employees e ON e.id = cm.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE cm.company_id = @company_id
  AND (@status = '' OR cm.status = @status)
  AND (@milestone_type = '' OR cm.milestone_type = @milestone_type)
ORDER BY cm.milestone_date ASC
LIMIT @lim OFFSET @off;

-- name: CountContractMilestones :one
SELECT COUNT(*) FROM contract_milestones
WHERE company_id = @company_id
  AND (@status = '' OR status = @status)
  AND (@milestone_type = '' OR milestone_type = @milestone_type);

-- name: AcknowledgeMilestone :one
UPDATE contract_milestones SET
    status = 'acknowledged',
    acknowledged_by = $3,
    acknowledged_at = NOW(),
    notes = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: ActionMilestone :one
UPDATE contract_milestones SET
    status = 'actioned',
    notes = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListPendingMilestonesByCompany :many
SELECT cm.*, e.employee_no, e.first_name, e.last_name, e.employment_type,
       COALESCE(d.name, '') as department_name
FROM contract_milestones cm
JOIN employees e ON e.id = cm.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE cm.company_id = $1
  AND cm.status = 'pending'
  AND cm.milestone_date <= CURRENT_DATE + INTERVAL '30 days'
ORDER BY cm.milestone_date ASC;

-- name: DeleteOldMilestones :exec
DELETE FROM contract_milestones
WHERE status IN ('acknowledged', 'actioned')
  AND milestone_date < CURRENT_DATE - INTERVAL '90 days';
