-- Agent routines

-- name: ListAgentRoutineSettings :many
SELECT * FROM agent_routines WHERE company_id = $1;

-- name: GetAgentRoutineSetting :one
SELECT * FROM agent_routines WHERE company_id = $1 AND routine_key = $2;

-- name: UpsertAgentRoutineSetting :one
INSERT INTO agent_routines (company_id, routine_key, enabled, autonomy, instructions, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (company_id, routine_key) DO UPDATE
SET enabled = EXCLUDED.enabled, autonomy = EXCLUDED.autonomy,
    instructions = EXCLUDED.instructions, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: InsertAgentRoutineRun :one
INSERT INTO agent_routine_runs (company_id, routine_key, trigger, subject_type, subject_id, subject_name, autonomy, status, summary, actions)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListAgentRoutineRuns :many
SELECT * FROM agent_routine_runs
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(routine_key)::varchar = '' OR routine_key = sqlc.arg(routine_key)::varchar)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim);

-- name: GetAgentRoutineRun :one
SELECT * FROM agent_routine_runs WHERE id = $1 AND company_id = $2;

-- name: LatestAgentRoutineRuns :many
SELECT DISTINCT ON (routine_key) * FROM agent_routine_runs
WHERE company_id = $1
ORDER BY routine_key, created_at DESC, id DESC;

-- name: CountAgentRoutineRunsSince :many
SELECT routine_key, status, COUNT(*)::int AS n FROM agent_routine_runs
WHERE company_id = $1 AND created_at >= $2
GROUP BY routine_key, status;

-- name: HasAgentRoutineRun :one
SELECT EXISTS (
    SELECT 1 FROM agent_routine_runs
    WHERE company_id = $1 AND routine_key = $2 AND subject_type = $3 AND subject_id = $4
)::bool AS found;

-- name: ListPayrollRunsToCheck :many
-- Calculated payroll runs (not yet approved) the pre-payroll check has not reviewed.
SELECT r.id, r.company_id, r.cycle_id, c.name AS cycle_name, c.pay_date
FROM payroll_runs r
JOIN payroll_cycles c ON c.id = r.cycle_id
WHERE r.status IN ('completed', 'ready_for_approval') AND c.status NOT IN ('approved', 'paid', 'cancelled')
  AND r.created_at >= now() - interval '14 days'
  AND NOT EXISTS (
      SELECT 1 FROM agent_routine_runs ar
      WHERE ar.company_id = r.company_id AND ar.routine_key = 'pre_payroll_check'
        AND ar.subject_type = 'payroll_run' AND ar.subject_id = r.id
  )
ORDER BY r.id
LIMIT 50;

-- name: UpdateAgentRoutineRunActions :one
UPDATE agent_routine_runs SET actions = $3, status = $4, summary = $5
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: RetractNotification :exec
DELETE FROM notifications WHERE id = $1 AND company_id = $2;

-- name: DeleteOnboardingTaskByID :exec
DELETE FROM onboarding_tasks WHERE id = $1 AND company_id = $2 AND status <> 'completed';

-- name: RetractNotifications :exec
DELETE FROM notifications WHERE company_id = $1 AND id = ANY(sqlc.arg(ids)::bigint[]);
