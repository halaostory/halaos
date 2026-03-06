-- name: CreateReviewCycle :one
INSERT INTO review_cycles (company_id, name, cycle_type, period_start, period_end, review_deadline, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListReviewCycles :many
SELECT * FROM review_cycles
WHERE company_id = $1
ORDER BY period_start DESC
LIMIT $2 OFFSET $3;

-- name: GetReviewCycle :one
SELECT * FROM review_cycles WHERE id = $1 AND company_id = $2;

-- name: UpdateReviewCycleStatus :one
UPDATE review_cycles SET status = $3, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreateGoal :one
INSERT INTO goals (company_id, employee_id, review_cycle_id, title, description, category, weight, target_value, due_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListGoals :many
SELECT * FROM goals
WHERE company_id = $1 AND employee_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListGoalsByCycle :many
SELECT g.*, e.first_name, e.last_name, e.employee_no
FROM goals g
JOIN employees e ON e.id = g.employee_id
WHERE g.company_id = $1 AND g.review_cycle_id = $2
ORDER BY e.last_name, e.first_name, g.created_at;

-- name: UpdateGoal :one
UPDATE goals SET
    title = COALESCE(NULLIF($3, ''), title),
    description = COALESCE($4, description),
    status = COALESCE(NULLIF($5, ''), status),
    actual_value = COALESCE($6, actual_value),
    self_rating = COALESCE($7, self_rating),
    manager_rating = COALESCE($8, manager_rating),
    completed_at = CASE WHEN $5 = 'completed' THEN NOW() ELSE completed_at END,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreatePerformanceReview :one
INSERT INTO performance_reviews (company_id, review_cycle_id, employee_id, reviewer_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPerformanceReview :one
SELECT * FROM performance_reviews WHERE id = $1 AND company_id = $2;

-- name: GetReviewByEmployee :one
SELECT * FROM performance_reviews
WHERE company_id = $1 AND review_cycle_id = $2 AND employee_id = $3;

-- name: ListReviewsByCycle :many
SELECT pr.*, e.first_name, e.last_name, e.employee_no,
    rev.first_name as reviewer_first_name, rev.last_name as reviewer_last_name
FROM performance_reviews pr
JOIN employees e ON e.id = pr.employee_id
LEFT JOIN employees rev ON rev.id = pr.reviewer_id
WHERE pr.company_id = $1 AND pr.review_cycle_id = $2
ORDER BY e.last_name, e.first_name;

-- name: ListMyReviews :many
SELECT pr.*, rc.name as cycle_name, rc.cycle_type, rc.period_start, rc.period_end
FROM performance_reviews pr
JOIN review_cycles rc ON rc.id = pr.review_cycle_id
WHERE pr.employee_id = $1
ORDER BY rc.period_start DESC;

-- name: SubmitSelfReview :one
UPDATE performance_reviews SET
    self_rating = $3,
    self_comments = $4,
    self_submitted_at = NOW(),
    status = 'manager_review',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: SubmitManagerReview :one
UPDATE performance_reviews SET
    manager_rating = $3,
    manager_comments = $4,
    strengths = $5,
    improvements = $6,
    final_rating = $7,
    final_comments = $8,
    manager_submitted_at = NOW(),
    status = 'completed',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: GetReviewStats :many
SELECT
    CASE
        WHEN final_rating = 5 THEN 'Outstanding'
        WHEN final_rating = 4 THEN 'Exceeds'
        WHEN final_rating = 3 THEN 'Meets'
        WHEN final_rating = 2 THEN 'Below'
        WHEN final_rating = 1 THEN 'Unsatisfactory'
        ELSE 'Pending'
    END as rating_label,
    COUNT(*) as count
FROM performance_reviews
WHERE company_id = $1 AND review_cycle_id = $2
GROUP BY final_rating
ORDER BY final_rating DESC NULLS LAST;

-- name: InitiateReviews :exec
INSERT INTO performance_reviews (company_id, review_cycle_id, employee_id, reviewer_id)
SELECT $1, $2, e.id, e.manager_id
FROM employees e
WHERE e.company_id = $1 AND e.status = 'active'
ON CONFLICT (review_cycle_id, employee_id) DO NOTHING;
