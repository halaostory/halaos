-- name: CreateExpenseCategory :one
INSERT INTO expense_categories (company_id, name, description, max_amount, requires_receipt)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateExpenseCategory :one
UPDATE expense_categories SET
    name = COALESCE($3, name),
    description = COALESCE($4, description),
    max_amount = $5,
    requires_receipt = $6,
    is_active = $7
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListExpenseCategories :many
SELECT * FROM expense_categories
WHERE company_id = $1
ORDER BY name;

-- name: ListActiveExpenseCategories :many
SELECT * FROM expense_categories
WHERE company_id = $1 AND is_active = true
ORDER BY name;

-- name: NextExpenseClaimNumber :one
SELECT COALESCE(MAX(CAST(SUBSTRING(claim_number FROM 'EXP-(\d+)') AS INT)), 0) + 1
FROM expense_claims
WHERE company_id = $1;

-- name: CreateExpenseClaim :one
INSERT INTO expense_claims (
    company_id, employee_id, claim_number, category_id,
    description, amount, currency, expense_date,
    receipt_path, status, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetExpenseClaim :one
SELECT ec.*, cat.name as category_name,
    e.first_name, e.last_name, e.employee_no
FROM expense_claims ec
JOIN expense_categories cat ON cat.id = ec.category_id
JOIN employees e ON e.id = ec.employee_id
WHERE ec.id = $1 AND ec.company_id = $2;

-- name: ListExpenseClaims :many
SELECT ec.*, cat.name as category_name,
    e.first_name, e.last_name, e.employee_no
FROM expense_claims ec
JOIN expense_categories cat ON cat.id = ec.category_id
JOIN employees e ON e.id = ec.employee_id
WHERE ec.company_id = $1
  AND (@status::text = '' OR ec.status = @status)
  AND (@employee_id::bigint = 0 OR ec.employee_id = @employee_id)
ORDER BY ec.created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountExpenseClaims :one
SELECT COUNT(*) FROM expense_claims
WHERE company_id = $1
  AND (@status::text = '' OR status = @status)
  AND (@employee_id::bigint = 0 OR employee_id = @employee_id);

-- name: ListMyExpenseClaims :many
SELECT ec.*, cat.name as category_name
FROM expense_claims ec
JOIN expense_categories cat ON cat.id = ec.category_id
WHERE ec.company_id = $1 AND ec.employee_id = $2
ORDER BY ec.created_at DESC;

-- name: SubmitExpenseClaim :one
UPDATE expense_claims SET
    status = 'submitted',
    submitted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND employee_id = $3 AND status = 'draft'
RETURNING *;

-- name: ApproveExpenseClaim :one
UPDATE expense_claims SET
    status = 'approved',
    approver_id = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'submitted'
RETURNING *;

-- name: RejectExpenseClaim :one
UPDATE expense_claims SET
    status = 'rejected',
    approver_id = $3,
    rejection_reason = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'submitted'
RETURNING *;

-- name: MarkExpenseClaimPaid :one
UPDATE expense_claims SET
    status = 'paid',
    paid_at = NOW(),
    paid_reference = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'approved'
RETURNING *;

-- name: GetExpenseSummary :one
SELECT
    COUNT(*) FILTER (WHERE status = 'draft') as draft_count,
    COUNT(*) FILTER (WHERE status = 'submitted') as submitted_count,
    COUNT(*) FILTER (WHERE status = 'approved') as approved_count,
    COUNT(*) FILTER (WHERE status = 'rejected') as rejected_count,
    COUNT(*) FILTER (WHERE status = 'paid') as paid_count,
    COALESCE(SUM(amount) FILTER (WHERE status = 'submitted'), 0) as pending_amount,
    COALESCE(SUM(amount) FILTER (WHERE status = 'approved'), 0) as approved_amount,
    COALESCE(SUM(amount) FILTER (WHERE status = 'paid'), 0) as paid_amount
FROM expense_claims
WHERE company_id = $1;
