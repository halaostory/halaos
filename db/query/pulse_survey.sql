-- name: CreatePulseSurvey :one
INSERT INTO pulse_surveys (company_id, title, description, frequency, is_anonymous, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPulseSurvey :one
SELECT * FROM pulse_surveys WHERE id = $1 AND company_id = $2;

-- name: ListPulseSurveys :many
SELECT * FROM pulse_surveys
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdatePulseSurvey :one
UPDATE pulse_surveys SET
    title = $3,
    description = $4,
    frequency = $5,
    is_anonymous = $6,
    is_active = $7,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeactivatePulseSurvey :exec
UPDATE pulse_surveys SET is_active = false, updated_at = NOW()
WHERE id = $1 AND company_id = $2;

-- name: CreatePulseQuestion :one
INSERT INTO pulse_questions (survey_id, question, question_type, sort_order, is_required)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPulseQuestions :many
SELECT * FROM pulse_questions
WHERE survey_id = $1
ORDER BY sort_order;

-- name: DeletePulseQuestions :exec
DELETE FROM pulse_questions WHERE survey_id = $1;

-- name: CreatePulseRound :one
INSERT INTO pulse_rounds (survey_id, company_id, round_date, total_sent)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOpenRound :one
SELECT * FROM pulse_rounds
WHERE survey_id = $1 AND company_id = $2 AND status = 'open'
ORDER BY round_date DESC LIMIT 1;

-- name: ListPulseRounds :many
SELECT * FROM pulse_rounds
WHERE survey_id = $1
ORDER BY round_date DESC
LIMIT $2 OFFSET $3;

-- name: ClosePulseRound :exec
UPDATE pulse_rounds SET status = 'closed', closed_at = NOW()
WHERE id = $1;

-- name: IncrementRoundResponded :exec
UPDATE pulse_rounds SET total_responded = total_responded + 1
WHERE id = $1;

-- name: UpsertPulseResponse :exec
INSERT INTO pulse_responses (round_id, question_id, employee_id, company_id, rating, answer_text)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (round_id, question_id, employee_id)
DO UPDATE SET rating = $5, answer_text = $6, submitted_at = NOW();

-- name: HasEmployeeRespondedToRound :one
SELECT EXISTS(
    SELECT 1 FROM pulse_responses
    WHERE round_id = $1 AND employee_id = $2
) as responded;

-- name: ListActiveSurveysForDistribution :many
SELECT ps.* FROM pulse_surveys ps
WHERE ps.company_id = $1
  AND ps.is_active = true
  AND NOT EXISTS (
    SELECT 1 FROM pulse_rounds pr
    WHERE pr.survey_id = ps.id
      AND pr.status = 'open'
      AND pr.round_date >= CURRENT_DATE - INTERVAL '1 day'
  );

-- name: CountActiveEmployees :one
SELECT COUNT(*) as count FROM employees
WHERE company_id = $1 AND status = 'active';
