-- name: ListTeamMembers :many
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.email, e.phone,
    e.employment_type, e.hire_date, e.status,
    d.name as department_name, p.title as position_title
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE e.company_id = $1 AND e.manager_id = $2 AND e.status = 'active'
ORDER BY e.last_name, e.first_name;

-- name: GetMyManager :one
SELECT e.id, e.employee_no, e.first_name, e.last_name, e.email, e.phone,
    d.name as department_name, p.title as position_title
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
WHERE e.id = $1;

-- name: GetMyCompensation :one
SELECT es.basic_salary, es.effective_from,
    ss.name as structure_name
FROM employee_salaries es
LEFT JOIN salary_structures ss ON ss.id = es.structure_id
WHERE es.company_id = $1 AND es.employee_id = $2
  AND es.effective_from <= NOW()
ORDER BY es.effective_from DESC
LIMIT 1;

-- name: GetMyLatestPayslip :one
SELECT pi.*, pc.name as cycle_name, pc.period_start, pc.period_end
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
WHERE pr.company_id = $1 AND pi.employee_id = $2 AND pr.status = 'completed'
ORDER BY pc.period_end DESC
LIMIT 1;

-- name: GetEmployeeFullInfo :one
SELECT e.*,
    d.name as department_name,
    p.title as position_title,
    cc.name as cost_center_name,
    mgr.first_name as manager_first_name,
    mgr.last_name as manager_last_name
FROM employees e
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN cost_centers cc ON cc.id = e.cost_center_id
LEFT JOIN employees mgr ON mgr.id = e.manager_id
WHERE e.user_id = $1 AND e.company_id = $2;
