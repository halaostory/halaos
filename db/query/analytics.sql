-- name: GetHeadcountTrend :many
SELECT
    TO_CHAR(DATE_TRUNC('month', hire_date), 'YYYY-MM') as month,
    COUNT(*) FILTER (WHERE status = 'active') as active_count,
    COUNT(*) FILTER (WHERE status = 'separated') as separated_count,
    COUNT(*) as total_count
FROM employees
WHERE company_id = $1
  AND hire_date >= $2
GROUP BY DATE_TRUNC('month', hire_date)
ORDER BY month;

-- name: GetTurnoverStats :many
SELECT
    TO_CHAR(DATE_TRUNC('month', updated_at), 'YYYY-MM') as month,
    COUNT(*) FILTER (WHERE status = 'separated') as separations,
    COUNT(*) FILTER (WHERE status = 'active') as active_count
FROM employees
WHERE company_id = $1
  AND updated_at >= $2
GROUP BY DATE_TRUNC('month', updated_at)
ORDER BY month;

-- name: GetDepartmentCostAnalysis :many
SELECT
    d.name as department_name,
    COUNT(e.id) as employee_count,
    COALESCE(SUM(es.basic_salary), 0) as total_salary_cost
FROM departments d
LEFT JOIN employees e ON e.department_id = d.id AND e.status = 'active'
LEFT JOIN employee_salaries es ON es.employee_id = e.id
    AND es.effective_from <= NOW()
    AND (es.effective_to IS NULL OR es.effective_to >= NOW())
WHERE d.company_id = $1
GROUP BY d.id, d.name
ORDER BY total_salary_cost DESC;

-- name: GetAttendancePatterns :many
SELECT
    EXTRACT(DOW FROM al.clock_in_at)::int as day_of_week,
    AVG(EXTRACT(EPOCH FROM (al.clock_out_at - al.clock_in_at)) / 3600)::numeric(5,2) as avg_hours,
    AVG(al.late_minutes)::numeric(5,1) as avg_late_minutes,
    COUNT(*) as total_records
FROM attendance_logs al
JOIN employees e ON e.id = al.employee_id
WHERE e.company_id = $1
  AND al.clock_in_at >= $2
  AND al.clock_out_at IS NOT NULL
GROUP BY EXTRACT(DOW FROM al.clock_in_at)
ORDER BY day_of_week;

-- name: GetEmploymentTypeBreakdown :many
SELECT
    employment_type,
    COUNT(*) as count
FROM employees
WHERE company_id = $1 AND status = 'active'
GROUP BY employment_type
ORDER BY count DESC;

-- name: GetLeaveUtilization :many
SELECT
    lt.name as leave_type,
    COUNT(lr.id) as total_requests,
    COALESCE(SUM(lr.days), 0) as total_days_used
FROM leave_types lt
LEFT JOIN leave_requests lr ON lr.leave_type_id = lt.id
    AND lr.status = 'approved'
    AND lr.start_date >= $2
    AND lr.start_date < $2 + INTERVAL '1 year'
WHERE lt.company_id = $1
GROUP BY lt.id, lt.name
ORDER BY total_days_used DESC;

-- name: GetAnalyticsSummary :one
SELECT
    COUNT(*) FILTER (WHERE status = 'active') as active_employees,
    COUNT(*) FILTER (WHERE status = 'separated') as separated_employees,
    COUNT(*) FILTER (WHERE hire_date >= DATE_TRUNC('month', NOW())) as new_hires_this_month,
    COUNT(*) FILTER (WHERE status = 'probationary' OR employment_type = 'probationary') as probationary_count,
    AVG(EXTRACT(YEAR FROM AGE(NOW(), hire_date)))::numeric(4,1) as avg_tenure_years
FROM employees
WHERE company_id = $1;
