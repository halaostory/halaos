-- name: CreateBonusStructure :one
INSERT INTO bonus_structures (
    company_id, name, description, bonus_type, base_amount, base_type,
    rating_map, review_cycle_id, is_taxable, status, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListBonusStructures :many
SELECT bs.*, rc.name AS review_cycle_name
FROM bonus_structures bs
LEFT JOIN review_cycles rc ON rc.id = bs.review_cycle_id
WHERE bs.company_id = $1
  AND ($2::text = '' OR bs.status = $2)
ORDER BY bs.created_at DESC;

-- name: GetBonusStructure :one
SELECT bs.*, rc.name AS review_cycle_name
FROM bonus_structures bs
LEFT JOIN review_cycles rc ON rc.id = bs.review_cycle_id
WHERE bs.id = $1 AND bs.company_id = $2;

-- name: UpdateBonusStructureStatus :one
UPDATE bonus_structures SET status = $3, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CreateBonusAllocation :one
INSERT INTO bonus_allocations (
    company_id, structure_id, employee_id, performance_review_id,
    rating, multiplier, base_amount, final_amount, manual_override, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (structure_id, employee_id) DO UPDATE SET
    performance_review_id = EXCLUDED.performance_review_id,
    rating = EXCLUDED.rating,
    multiplier = EXCLUDED.multiplier,
    base_amount = EXCLUDED.base_amount,
    final_amount = EXCLUDED.final_amount,
    manual_override = EXCLUDED.manual_override,
    notes = EXCLUDED.notes,
    updated_at = NOW()
RETURNING *;

-- name: ListBonusAllocations :many
SELECT ba.*, e.first_name, e.last_name, e.employee_no
FROM bonus_allocations ba
JOIN employees e ON e.id = ba.employee_id
WHERE ba.structure_id = $1
ORDER BY e.last_name, e.first_name;

-- name: UpdateBonusAllocationStatus :exec
UPDATE bonus_allocations SET
    status = $2,
    approved_by = $3,
    approved_at = CASE WHEN $2 = 'approved' THEN NOW() ELSE approved_at END,
    updated_at = NOW()
WHERE id = $1;

-- name: BulkApproveBonusAllocations :exec
UPDATE bonus_allocations SET
    status = 'approved',
    approved_by = $2,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = ANY($1::bigint[]) AND status = 'pending';

-- name: GetApprovedBonusesForPayroll :many
SELECT ba.employee_id, SUM(
    CASE WHEN ba.manual_override IS NOT NULL THEN ba.manual_override ELSE ba.final_amount END
)::NUMERIC(12,2) AS total_bonus
FROM bonus_allocations ba
WHERE ba.company_id = $1
  AND ba.payroll_cycle_id = $2
  AND ba.status = 'approved'
GROUP BY ba.employee_id;

-- name: LinkBonusToPayrollCycle :exec
UPDATE bonus_allocations SET
    payroll_cycle_id = $2,
    updated_at = NOW()
WHERE structure_id = $1 AND status = 'approved';

-- name: GetCompletedReviewsForBonusCalc :many
SELECT pr.employee_id, pr.final_rating, pr.id AS review_id
FROM performance_reviews pr
WHERE pr.company_id = $1
  AND pr.review_cycle_id = $2
  AND pr.status = 'completed'
  AND pr.final_rating IS NOT NULL;

-- name: GetEmployeeSalaryForBonus :one
SELECT es.basic_salary
FROM employee_salaries es
WHERE es.company_id = $1 AND es.employee_id = $2
  AND es.effective_from <= NOW()
  AND (es.effective_to IS NULL OR es.effective_to >= NOW())
ORDER BY es.effective_from DESC
LIMIT 1;
