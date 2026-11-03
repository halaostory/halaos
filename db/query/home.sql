-- Home dashboard (Warp-style: what needs you, what's coming, what the agent did)

-- name: HomeHeadcount :one
SELECT
    COUNT(*) FILTER (WHERE status IN ('active', 'probationary') AND hire_date <= CURRENT_DATE)::int AS active,
    COUNT(*) FILTER (WHERE status IN ('active', 'probationary') AND hire_date > CURRENT_DATE)::int AS starting_soon,
    COUNT(*) FILTER (WHERE status IN ('active', 'probationary') AND hire_date >= date_trunc('month', CURRENT_DATE))::int AS joined_this_month,
    (SELECT COUNT(*) FROM employee_separations s WHERE s.company_id = $1 AND s.status = 'scheduled')::int AS leaving_soon,
    (SELECT COUNT(*) FROM contractors k WHERE k.company_id = $1 AND k.status = 'active')::int AS contractors
FROM employees WHERE company_id = $1;

-- name: HomeUpcomingStarts :many
SELECT e.id, e.first_name, e.last_name, e.hire_date, COALESCE(p.title, '')::text AS position
FROM employees e
LEFT JOIN positions p ON p.id = e.position_id
WHERE e.company_id = $1 AND e.status IN ('active', 'probationary')
  AND e.hire_date BETWEEN CURRENT_DATE - 7 AND CURRENT_DATE + 45
ORDER BY e.hire_date, e.last_name
LIMIT 6;

-- name: HomeUpcomingLeavers :many
SELECT s.employee_id, e.first_name, e.last_name, s.last_working_day, s.separation_type
FROM employee_separations s
JOIN employees e ON e.id = s.employee_id
WHERE s.company_id = $1 AND s.status = 'scheduled'
ORDER BY s.last_working_day
LIMIT 6;

-- name: HomeEmployeeIDCoverage :one
-- How many active employees each Philippine agency already knows about.
SELECT
    COUNT(*)::int AS total,
    COUNT(*) FILTER (WHERE COALESCE(ep.tin, '') <> '')::int AS bir,
    COUNT(*) FILTER (WHERE COALESCE(ep.sss_no, '') <> '')::int AS sss,
    COUNT(*) FILTER (WHERE COALESCE(ep.philhealth_no, '') <> '')::int AS philhealth,
    COUNT(*) FILTER (WHERE COALESCE(ep.pagibig_no, '') <> '')::int AS pagibig
FROM employees e
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
WHERE e.company_id = $1 AND e.status IN ('active', 'probationary');

-- name: HomeGovSubmissionsByAgency :many
-- Open submissions per agency, for the registrations panel.
SELECT agency, status, COUNT(*)::int AS n
FROM gov_submissions
WHERE company_id = $1 AND status NOT IN ('cancelled') AND dry_run = false
  AND (status <> 'submitted' OR submitted_at >= now() - interval '30 days')
GROUP BY agency, status;

-- name: HomeRegistrationNumbers :many
SELECT registration_type, registration_value FROM company_registration_numbers
WHERE company_id = $1 ORDER BY registration_type;

-- name: HomeNextPayroll :one
SELECT id, name, period_start, period_end, pay_date, status
FROM payroll_cycles
WHERE company_id = $1 AND cycle_type = 'regular' AND status NOT IN ('paid', 'cancelled')
  AND pay_date >= CURRENT_DATE - 3
ORDER BY pay_date
LIMIT 1;

-- name: HomeLastPayroll :one
SELECT c.name, c.pay_date, r.total_employees, r.total_gross::float8 AS total_gross, r.total_net::float8 AS total_net
FROM payroll_runs r
JOIN payroll_cycles c ON c.id = r.cycle_id
WHERE r.company_id = $1 AND r.status = 'completed' AND c.status IN ('approved', 'paid') AND c.cycle_type = 'regular'
ORDER BY c.pay_date DESC, r.id DESC
LIMIT 1;

-- name: HomeUpcomingFilings :many
SELECT id, filing_type, period_year, period_month, period_quarter, due_date, status
FROM tax_filings
WHERE company_id = $1 AND status IN ('pending', 'generated')
  AND due_date BETWEEN CURRENT_DATE - 45 AND CURRENT_DATE + 30
ORDER BY due_date
LIMIT 6;

-- name: HomeOverdueFilings :one
SELECT COUNT(*)::int FROM tax_filings
WHERE company_id = $1 AND status IN ('pending', 'generated') AND due_date < CURRENT_DATE;

-- name: HomeSetupCounts :one
SELECT
    (SELECT COUNT(*) FROM employees e WHERE e.company_id = $1 AND e.status IN ('active', 'probationary'))::int AS employees,
    (SELECT COUNT(*) FROM employees e WHERE e.company_id = $1 AND e.status IN ('active', 'probationary')
        AND NOT EXISTS (SELECT 1 FROM employee_salaries s WHERE s.employee_id = e.id))::int AS without_salary,
    (SELECT COUNT(*) FROM pay_schedules p WHERE p.company_id = $1)::int AS pay_schedules,
    (SELECT COUNT(*) FROM payroll_cycles c WHERE c.company_id = $1 AND c.cycle_type = 'imported')::int AS imported_periods,
    (SELECT COUNT(*) FROM payroll_runs r JOIN payroll_cycles c ON c.id = r.cycle_id
        WHERE r.company_id = $1 AND r.status IN ('completed', 'ready_for_approval') AND c.cycle_type <> 'imported')::int AS payroll_runs;
