-- name: CreateBenefitPlan :one
INSERT INTO benefit_plans (
    company_id, name, category, description, provider,
    employer_share, employee_share, coverage_amount,
    eligibility_type, eligibility_months
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListBenefitPlans :many
SELECT * FROM benefit_plans
WHERE company_id = $1 AND is_active = true
ORDER BY category, name;

-- name: GetBenefitPlan :one
SELECT * FROM benefit_plans WHERE id = $1 AND company_id = $2;

-- name: UpdateBenefitPlan :one
UPDATE benefit_plans SET
    name = COALESCE($3, name),
    category = COALESCE($4, category),
    description = $5,
    provider = $6,
    employer_share = COALESCE($7, employer_share),
    employee_share = COALESCE($8, employee_share),
    coverage_amount = $9,
    eligibility_type = COALESCE($10, eligibility_type),
    eligibility_months = COALESCE($11, eligibility_months),
    is_active = COALESCE($12, is_active),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreateBenefitEnrollment :one
INSERT INTO benefit_enrollments (
    company_id, employee_id, plan_id, status,
    effective_date, employer_share, employee_share, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListBenefitEnrollments :many
SELECT be.*, bp.name as plan_name, bp.category as plan_category, bp.provider,
       e.employee_no, e.first_name, e.last_name
FROM benefit_enrollments be
JOIN benefit_plans bp ON bp.id = be.plan_id
JOIN employees e ON e.id = be.employee_id
WHERE be.company_id = @company_id
  AND (@status = '' OR be.status = @status)
  AND (@employee_id = 0 OR be.employee_id = @employee_id)
ORDER BY e.last_name, e.first_name, bp.name;

-- name: ListMyEnrollments :many
SELECT be.*, bp.name as plan_name, bp.category as plan_category,
       bp.provider, bp.coverage_amount
FROM benefit_enrollments be
JOIN benefit_plans bp ON bp.id = be.plan_id
WHERE be.company_id = $1 AND be.employee_id = $2
ORDER BY bp.category, bp.name;

-- name: CancelBenefitEnrollment :one
UPDATE benefit_enrollments SET
    status = 'cancelled',
    end_date = CURRENT_DATE,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'active'
RETURNING *;

-- name: CreateBenefitDependent :one
INSERT INTO benefit_dependents (
    company_id, employee_id, enrollment_id,
    name, relationship, birth_date
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListBenefitDependents :many
SELECT * FROM benefit_dependents
WHERE enrollment_id = $1 AND company_id = $2
ORDER BY name;

-- name: DeleteBenefitDependent :exec
DELETE FROM benefit_dependents WHERE id = $1 AND company_id = $2;

-- name: CreateBenefitClaim :one
INSERT INTO benefit_claims (
    company_id, employee_id, enrollment_id,
    claim_date, amount, description
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListBenefitClaims :many
SELECT bc.*, bp.name as plan_name, bp.category as plan_category,
       e.employee_no, e.first_name, e.last_name
FROM benefit_claims bc
JOIN benefit_enrollments be ON be.id = bc.enrollment_id
JOIN benefit_plans bp ON bp.id = be.plan_id
JOIN employees e ON e.id = bc.employee_id
WHERE bc.company_id = @company_id
  AND (@status = '' OR bc.status = @status)
  AND (@employee_id = 0 OR bc.employee_id = @employee_id)
ORDER BY bc.claim_date DESC
LIMIT @lim OFFSET @off;

-- name: CountBenefitClaims :one
SELECT COUNT(*) FROM benefit_claims
WHERE company_id = @company_id
  AND (@status = '' OR status = @status)
  AND (@employee_id = 0 OR employee_id = @employee_id);

-- name: ApproveBenefitClaim :one
UPDATE benefit_claims SET
    status = 'approved',
    approved_by = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: RejectBenefitClaim :one
UPDATE benefit_claims SET
    status = 'rejected',
    rejection_reason = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: GetBenefitsSummary :one
SELECT
    COUNT(DISTINCT be.plan_id) as total_plans,
    COUNT(DISTINCT be.employee_id) as enrolled_employees,
    COALESCE(SUM(be.employer_share), 0) as total_employer_cost,
    COALESCE(SUM(be.employee_share), 0) as total_employee_cost
FROM benefit_enrollments be
WHERE be.company_id = $1 AND be.status = 'active';
