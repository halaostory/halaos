-- name: InsertNPSFeedback :one
INSERT INTO nps_feedback (company_id, user_id, score, comment)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at;

-- name: GetLastNPSFeedback :one
-- Returns the most recent NPS submission by a user (to check cooldown).
SELECT created_at FROM nps_feedback
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListNPSFeedback :many
-- Admin view: list all NPS feedback for a company.
SELECT nf.id, nf.score, nf.comment, nf.created_at,
       u.first_name, u.last_name, u.email
FROM nps_feedback nf
JOIN users u ON u.id = nf.user_id
WHERE nf.company_id = $1
ORDER BY nf.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetNPSSummary :one
-- Aggregate NPS stats for a company.
SELECT COUNT(*)::int AS total_responses,
       COALESCE(AVG(score), 0)::float AS avg_score,
       COUNT(*) FILTER (WHERE score >= 9)::int AS promoters,
       COUNT(*) FILTER (WHERE score >= 7 AND score <= 8)::int AS passives,
       COUNT(*) FILTER (WHERE score <= 6)::int AS detractors
FROM nps_feedback
WHERE company_id = $1;
