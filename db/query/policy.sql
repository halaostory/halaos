-- name: CreatePolicy :one
INSERT INTO company_policies (
    company_id, title, content, category, version,
    effective_date, requires_acknowledgment, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListPolicies :many
SELECT cp.*,
    (SELECT COUNT(*) FROM policy_acknowledgments pa WHERE pa.policy_id = cp.id) as ack_count
FROM company_policies cp
WHERE cp.company_id = $1 AND cp.is_active = true
ORDER BY cp.effective_date DESC, cp.title;

-- name: GetPolicy :one
SELECT * FROM company_policies WHERE id = $1 AND company_id = $2;

-- name: UpdatePolicy :one
UPDATE company_policies SET
    title = COALESCE($3, title),
    content = COALESCE($4, content),
    category = COALESCE($5, category),
    version = COALESCE($6, version),
    effective_date = COALESCE($7, effective_date),
    requires_acknowledgment = $8,
    is_active = $9,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeactivatePolicy :one
UPDATE company_policies SET
    is_active = false,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: AcknowledgePolicy :one
INSERT INTO policy_acknowledgments (company_id, policy_id, employee_id, ip_address)
VALUES ($1, $2, $3, $4)
ON CONFLICT (company_id, policy_id, employee_id) DO NOTHING
RETURNING *;

-- name: ListPolicyAcknowledgments :many
SELECT pa.*, e.employee_no, e.first_name, e.last_name
FROM policy_acknowledgments pa
JOIN employees e ON e.id = pa.employee_id
WHERE pa.policy_id = $1 AND pa.company_id = $2
ORDER BY pa.acknowledged_at DESC;

-- name: ListUnacknowledgedPolicies :many
SELECT cp.*
FROM company_policies cp
WHERE cp.company_id = $1
  AND cp.is_active = true
  AND cp.requires_acknowledgment = true
  AND NOT EXISTS (
    SELECT 1 FROM policy_acknowledgments pa
    WHERE pa.policy_id = cp.id AND pa.employee_id = $2
  )
ORDER BY cp.effective_date DESC;

-- name: GetPolicyAckStats :one
SELECT
    (SELECT COUNT(*) FROM employees e2 WHERE e2.company_id = $1 AND e2.status = 'active') as total_employees,
    (SELECT COUNT(DISTINCT pa2.employee_id) FROM policy_acknowledgments pa2 WHERE pa2.policy_id = $2 AND pa2.company_id = $1) as acknowledged_count;
