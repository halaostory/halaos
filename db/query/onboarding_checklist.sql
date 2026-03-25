-- name: GetOnboardingChecklist :one
SELECT * FROM onboarding_progress
WHERE company_id = $1 AND user_id = $2 AND persona = $3;

-- name: UpsertOnboardingChecklist :one
INSERT INTO onboarding_progress (company_id, user_id, persona, steps)
VALUES ($1, $2, $3, $4)
ON CONFLICT (company_id, user_id, persona)
DO UPDATE SET steps = $4, updated_at = NOW()
RETURNING *;

-- name: DismissOnboardingChecklist :exec
INSERT INTO onboarding_progress (company_id, user_id, persona, steps, dismissed)
VALUES ($1, $2, $3, '{}', TRUE)
ON CONFLICT (company_id, user_id, persona)
DO UPDATE SET dismissed = TRUE, updated_at = NOW();

-- name: CompleteOnboardingChecklist :exec
UPDATE onboarding_progress
SET completed_at = NOW(), updated_at = NOW()
WHERE company_id = $1 AND user_id = $2 AND persona = $3;

-- name: CheckCompanyInfoComplete :one
SELECT (legal_name IS NOT NULL AND tin IS NOT NULL)::boolean AS complete
FROM companies WHERE id = $1;

-- name: CountEmployeesByCompany :one
SELECT COUNT(*) FROM employees WHERE company_id = $1;

-- name: CountDepartmentsByCompany :one
SELECT COUNT(*) FROM departments WHERE company_id = $1;

-- name: CountPositionsByCompany :one
SELECT COUNT(*) FROM positions WHERE company_id = $1;

-- name: CountNonStatutoryLeaveTypes :one
SELECT COUNT(*) FROM leave_types WHERE company_id = $1 AND is_statutory = false;

-- name: CountScheduleTemplates :one
SELECT COUNT(*) FROM schedule_templates WHERE company_id = $1;

-- name: CountSalaryStructures :one
SELECT COUNT(*) FROM salary_structures WHERE company_id = $1;

-- name: CountCompletedPayrollRuns :one
SELECT COUNT(*) FROM payroll_runs WHERE company_id = $1 AND status = 'completed';
