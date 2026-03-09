-- name: ListOnboardingTemplates :many
SELECT * FROM onboarding_templates
WHERE (company_id = $1 OR company_id = 0) AND is_active = true
ORDER BY workflow_type, sort_order;

-- name: CreateOnboardingTemplate :one
INSERT INTO onboarding_templates (company_id, workflow_type, title, description, sort_order, is_required, assignee_role, due_days)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateOnboardingTemplate :one
UPDATE onboarding_templates SET
    title = $3,
    description = $4,
    sort_order = $5,
    is_required = $6,
    assignee_role = $7,
    due_days = $8,
    is_active = $9,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListOnboardingTasks :many
SELECT * FROM onboarding_tasks
WHERE employee_id = $1 AND workflow_type = $2 AND company_id = $3
ORDER BY sort_order;

-- name: ListOnboardingTasksByCompany :many
SELECT ot.*, e.employee_no, e.first_name, e.last_name
FROM onboarding_tasks ot
JOIN employees e ON e.id = ot.employee_id
WHERE ot.company_id = $1 AND ot.workflow_type = $2 AND ot.status IN ('pending', 'in_progress')
ORDER BY ot.due_date, ot.sort_order;

-- name: CreateOnboardingTask :one
INSERT INTO onboarding_tasks (company_id, employee_id, template_id, workflow_type, title, description, is_required, assignee_role, assigned_to, due_date, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateOnboardingTaskStatus :one
UPDATE onboarding_tasks SET
    status = $3,
    completed_by = $4,
    completed_at = CASE WHEN $3 = 'completed' THEN NOW() ELSE completed_at END,
    notes = COALESCE($5, notes),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: GetOnboardingProgress :many
SELECT
    workflow_type,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'completed') as completed,
    COUNT(*) FILTER (WHERE status = 'skipped') as skipped,
    COUNT(*) FILTER (WHERE status IN ('pending', 'in_progress')) as remaining
FROM onboarding_tasks
WHERE employee_id = $1 AND company_id = $2
GROUP BY workflow_type;

-- name: CountPendingOnboardingTasks :one
SELECT COUNT(*) FROM onboarding_tasks
WHERE company_id = $1 AND status IN ('pending', 'in_progress');
