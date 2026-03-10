-- name: InsertFlightRiskHistory :exec
INSERT INTO score_history_flight_risk (company_id, employee_id, risk_score, factors, week_date)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, employee_id, week_date) DO UPDATE
SET risk_score = EXCLUDED.risk_score, factors = EXCLUDED.factors;

-- name: InsertBurnoutHistory :exec
INSERT INTO score_history_burnout (company_id, employee_id, burnout_score, factors, week_date)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, employee_id, week_date) DO UPDATE
SET burnout_score = EXCLUDED.burnout_score, factors = EXCLUDED.factors;

-- name: InsertTeamHealthHistory :exec
INSERT INTO score_history_team_health (company_id, department_id, department_name, health_score, factors, week_date)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (company_id, department_id, week_date) DO UPDATE
SET department_name = EXCLUDED.department_name, health_score = EXCLUDED.health_score, factors = EXCLUDED.factors;

-- name: UpsertOrgScoreSnapshot :exec
INSERT INTO org_score_snapshots (company_id, week_date, avg_flight_risk, avg_burnout, avg_team_health,
    high_risk_count, high_burnout_count, low_health_dept_count, total_employees, total_departments, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (company_id, week_date) DO UPDATE
SET avg_flight_risk = EXCLUDED.avg_flight_risk, avg_burnout = EXCLUDED.avg_burnout,
    avg_team_health = EXCLUDED.avg_team_health, high_risk_count = EXCLUDED.high_risk_count,
    high_burnout_count = EXCLUDED.high_burnout_count, low_health_dept_count = EXCLUDED.low_health_dept_count,
    total_employees = EXCLUDED.total_employees, total_departments = EXCLUDED.total_departments,
    metadata = EXCLUDED.metadata;

-- name: GetOrgScoreTrend :many
SELECT id, company_id, week_date, avg_flight_risk, avg_burnout, avg_team_health,
    high_risk_count, high_burnout_count, low_health_dept_count, total_employees, total_departments, metadata
FROM org_score_snapshots
WHERE company_id = $1 AND week_date >= $2
ORDER BY week_date ASC;

-- name: GetLatestOrgSnapshot :one
SELECT id, company_id, week_date, avg_flight_risk, avg_burnout, avg_team_health,
    high_risk_count, high_burnout_count, low_health_dept_count, total_employees, total_departments, metadata
FROM org_score_snapshots
WHERE company_id = $1
ORDER BY week_date DESC
LIMIT 1;

-- name: GetPreviousOrgSnapshot :one
SELECT id, company_id, week_date, avg_flight_risk, avg_burnout, avg_team_health,
    high_risk_count, high_burnout_count, low_health_dept_count, total_employees, total_departments, metadata
FROM org_score_snapshots
WHERE company_id = $1
ORDER BY week_date DESC
LIMIT 1 OFFSET 1;

-- name: GetFlightRiskTrend :many
SELECT week_date, risk_score, factors
FROM score_history_flight_risk
WHERE company_id = $1 AND employee_id = $2 AND week_date >= $3
ORDER BY week_date ASC;

-- name: GetBurnoutTrend :many
SELECT week_date, burnout_score, factors
FROM score_history_burnout
WHERE company_id = $1 AND employee_id = $2 AND week_date >= $3
ORDER BY week_date ASC;

-- name: GetTeamHealthTrend :many
SELECT week_date, department_name, health_score, factors
FROM score_history_team_health
WHERE company_id = $1 AND department_id = $2 AND week_date >= $3
ORDER BY week_date ASC;

-- name: GetDeptFlightRiskAvg :many
SELECT sh.week_date, e.department_id, COALESCE(d.name, '') AS department_name,
    AVG(sh.risk_score)::NUMERIC(5,2) AS avg_risk_score
FROM score_history_flight_risk sh
JOIN employees e ON e.id = sh.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE sh.company_id = $1 AND sh.week_date >= $2
GROUP BY sh.week_date, e.department_id, d.name
ORDER BY sh.week_date ASC, avg_risk_score DESC;

-- name: GetDeptBurnoutAvg :many
SELECT sh.week_date, e.department_id, COALESCE(d.name, '') AS department_name,
    AVG(sh.burnout_score)::NUMERIC(5,2) AS avg_burnout_score
FROM score_history_burnout sh
JOIN employees e ON e.id = sh.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE sh.company_id = $1 AND sh.week_date >= $2
GROUP BY sh.week_date, e.department_id, d.name
ORDER BY sh.week_date ASC, avg_burnout_score DESC;

-- name: UpsertExecutiveBriefing :exec
INSERT INTO executive_briefings (company_id, week_date, narrative, data_snapshot, generated_at, tokens_used)
VALUES ($1, $2, $3, $4, NOW(), $5)
ON CONFLICT (company_id, week_date) DO UPDATE
SET narrative = EXCLUDED.narrative, data_snapshot = EXCLUDED.data_snapshot,
    generated_at = NOW(), tokens_used = EXCLUDED.tokens_used;

-- name: GetLatestExecutiveBriefing :one
SELECT id, company_id, week_date, narrative, data_snapshot, generated_at, tokens_used
FROM executive_briefings
WHERE company_id = $1
ORDER BY week_date DESC
LIMIT 1;

-- name: GetExecutiveBriefingByWeek :one
SELECT id, company_id, week_date, narrative, data_snapshot, generated_at, tokens_used
FROM executive_briefings
WHERE company_id = $1 AND week_date = $2;

-- name: ListAllFlightRiskScores :many
SELECT ers.employee_id, ers.risk_score, ers.factors,
    e.first_name, e.last_name, e.employee_no,
    COALESCE(d.name, '') AS department
FROM employee_risk_scores ers
JOIN employees e ON e.id = ers.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE ers.company_id = $1
ORDER BY ers.risk_score DESC;

-- name: ListAllBurnoutScores :many
SELECT ebs.employee_id, ebs.burnout_score, ebs.factors,
    e.first_name, e.last_name, e.employee_no,
    COALESCE(d.name, '') AS department
FROM employee_burnout_scores ebs
JOIN employees e ON e.id = ebs.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE ebs.company_id = $1
ORDER BY ebs.burnout_score DESC;

-- name: ListAllTeamHealthScores :many
SELECT ths.department_id, ths.department_name, ths.health_score, ths.factors
FROM team_health_scores ths
WHERE ths.company_id = $1
ORDER BY ths.health_score ASC;

-- name: CountFlightRiskByTier :many
SELECT
    CASE
        WHEN risk_score >= 70 THEN 'critical'
        WHEN risk_score >= 50 THEN 'high'
        WHEN risk_score >= 30 THEN 'medium'
        ELSE 'low'
    END AS tier,
    COUNT(*) AS count
FROM employee_risk_scores
WHERE company_id = $1
GROUP BY tier
ORDER BY count DESC;

-- name: GetCorrelationHighBurnoutHighRisk :one
SELECT COUNT(*) AS count
FROM employee_risk_scores ers
JOIN employee_burnout_scores ebs ON ebs.company_id = ers.company_id AND ebs.employee_id = ers.employee_id
WHERE ers.company_id = $1 AND ers.risk_score >= 50 AND ebs.burnout_score >= 50;
