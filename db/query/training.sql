-- name: ListTrainings :many
SELECT t.*, u.email as created_by_email,
       (SELECT COUNT(*) FROM training_participants tp WHERE tp.training_id = t.id) as participant_count
FROM trainings t
LEFT JOIN users u ON u.id = t.created_by
WHERE t.company_id = $1
ORDER BY t.start_date DESC
LIMIT $2 OFFSET $3;

-- name: GetTraining :one
SELECT * FROM trainings WHERE id = $1 AND company_id = $2;

-- name: CreateTraining :one
INSERT INTO trainings (
    company_id, title, description, trainer, training_type,
    start_date, end_date, max_participants, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateTrainingStatus :one
UPDATE trainings SET status = $3, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListTrainingParticipants :many
SELECT tp.*, e.employee_no, e.first_name, e.last_name
FROM training_participants tp
JOIN employees e ON e.id = tp.employee_id
WHERE tp.training_id = $1
ORDER BY e.last_name, e.first_name;

-- name: AddTrainingParticipant :one
INSERT INTO training_participants (training_id, employee_id)
VALUES ($1, $2)
ON CONFLICT (training_id, employee_id) DO NOTHING
RETURNING *;

-- name: UpdateParticipantStatus :one
UPDATE training_participants SET
    status = $3,
    score = $4,
    completed_at = CASE WHEN $3 = 'completed' THEN NOW() ELSE completed_at END,
    feedback = $5
WHERE id = $1 AND training_id = $2
RETURNING *;

-- name: ListCertifications :many
SELECT c.*, e.employee_no, e.first_name, e.last_name
FROM certifications c
JOIN employees e ON e.id = c.employee_id
WHERE c.company_id = $1
  AND ($2::bigint IS NULL OR $2 = 0 OR c.employee_id = $2)
ORDER BY c.issue_date DESC
LIMIT $3 OFFSET $4;

-- name: CreateCertification :one
INSERT INTO certifications (
    company_id, employee_id, name, issuing_body,
    credential_id, issue_date, expiry_date
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: DeleteCertification :exec
DELETE FROM certifications WHERE id = $1 AND company_id = $2;

-- name: ListExpiringCertifications :many
SELECT c.*, e.employee_no, e.first_name, e.last_name
FROM certifications c
JOIN employees e ON e.id = c.employee_id
WHERE c.company_id = $1
  AND c.status = 'active'
  AND c.expiry_date IS NOT NULL
  AND c.expiry_date <= CURRENT_DATE + INTERVAL '90 days'
ORDER BY c.expiry_date;

-- name: ListMyTrainings :many
SELECT t.id, t.title, t.trainer, t.training_type, t.start_date, t.end_date,
       t.status as training_status, tp.status as participant_status, tp.score
FROM training_participants tp
JOIN trainings t ON t.id = tp.training_id
WHERE tp.employee_id = $1
ORDER BY t.start_date DESC;

-- name: ListMyCertifications :many
SELECT * FROM certifications
WHERE employee_id = $1
ORDER BY issue_date DESC;
