-- name: ListEmployeesDueForRegularization :many
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.hire_date,
       e.regularization_date,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE e.company_id = $1
  AND e.status = 'active'
  AND e.employment_type = 'probationary'
  AND (e.regularization_date IS NULL OR e.regularization_date <= $2::date + interval '30 days')
ORDER BY e.hire_date;

-- name: ListUpcomingBirthdays :many
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.birth_date,
       COALESCE(d.name, '') as department_name
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
WHERE e.company_id = $1
  AND e.status = 'active'
  AND e.birth_date IS NOT NULL
  AND (
    (EXTRACT(MONTH FROM e.birth_date) = EXTRACT(MONTH FROM $2::date)
     AND EXTRACT(DAY FROM e.birth_date) >= EXTRACT(DAY FROM $2::date))
    OR
    (EXTRACT(MONTH FROM e.birth_date) = EXTRACT(MONTH FROM $2::date + interval '30 days')
     AND EXTRACT(DAY FROM e.birth_date) <= EXTRACT(DAY FROM $2::date + interval '30 days'))
  )
ORDER BY EXTRACT(MONTH FROM e.birth_date), EXTRACT(DAY FROM e.birth_date);

-- name: ListExpiringContracts :many
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.separation_date,
       COALESCE(d.name, '') as department_name,
       COALESCE(p.title, '') as position_title
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE e.company_id = $1
  AND e.status = 'active'
  AND e.employment_type = 'contractual'
  AND e.separation_date IS NOT NULL
  AND e.separation_date <= $2::date + interval '60 days'
ORDER BY e.separation_date;

-- name: ListPendingOnboardingTasks :many
SELECT ot.id, ot.employee_id, ot.title, ot.due_date, ot.workflow_type,
       e.first_name, e.last_name
FROM onboarding_tasks ot
JOIN employees e ON e.id = ot.employee_id
WHERE ot.company_id = $1
  AND ot.status = 'pending'
ORDER BY ot.due_date
LIMIT 20;

-- name: CountEmployeesWithNoSalary :one
SELECT COUNT(*) FROM employees e
LEFT JOIN employee_salaries es ON es.employee_id = e.id
  AND es.effective_from <= $2
  AND (es.effective_to IS NULL OR es.effective_to >= $2)
WHERE e.company_id = $1 AND e.status = 'active' AND es.id IS NULL;
