-- name: ListLoanTypes :many
SELECT * FROM loan_types
WHERE company_id = $1 AND is_active = true
ORDER BY name;

-- name: CreateLoanType :one
INSERT INTO loan_types (company_id, name, code, provider, max_term_months, interest_rate, max_amount, requires_approval, auto_deduct)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CreateLoan :one
INSERT INTO loans (company_id, employee_id, loan_type_id, reference_no, principal_amount, interest_rate, term_months, monthly_amortization, total_amount, remaining_balance, start_date, end_date, remarks)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetLoan :one
SELECT l.*, lt.name as loan_type_name, lt.code as loan_type_code, lt.provider,
    e.first_name, e.last_name, e.employee_no
FROM loans l
JOIN loan_types lt ON lt.id = l.loan_type_id
JOIN employees e ON e.id = l.employee_id
WHERE l.id = $1 AND l.company_id = $2;

-- name: ListLoans :many
SELECT l.*, lt.name as loan_type_name, lt.code as loan_type_code,
    e.first_name, e.last_name, e.employee_no
FROM loans l
JOIN loan_types lt ON lt.id = l.loan_type_id
JOIN employees e ON e.id = l.employee_id
WHERE l.company_id = $1
ORDER BY l.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMyLoans :many
SELECT l.*, lt.name as loan_type_name, lt.code as loan_type_code
FROM loans l
JOIN loan_types lt ON lt.id = l.loan_type_id
WHERE l.employee_id = $1
ORDER BY l.created_at DESC;

-- name: ListActiveLoansForPayroll :many
SELECT l.id, l.employee_id, l.monthly_amortization, l.remaining_balance,
    lt.name as loan_type_name, lt.code as loan_type_code
FROM loans l
JOIN loan_types lt ON lt.id = l.loan_type_id
WHERE l.company_id = $1 AND l.status = 'active' AND lt.auto_deduct = true
ORDER BY l.employee_id;

-- name: ApproveLoan :one
UPDATE loans SET
    status = 'approved',
    approved_by = $3,
    approved_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'pending'
RETURNING *;

-- name: ActivateLoan :one
UPDATE loans SET
    status = 'active',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'approved'
RETURNING *;

-- name: UpdateLoanBalance :one
UPDATE loans SET
    total_paid = total_paid + $3,
    remaining_balance = remaining_balance - $3,
    status = CASE WHEN remaining_balance - $3 <= 0 THEN 'completed' ELSE status END,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: CancelLoan :one
UPDATE loans SET
    status = 'cancelled',
    updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('pending', 'approved')
RETURNING *;

-- name: CreateLoanPayment :one
INSERT INTO loan_payments (loan_id, payment_date, amount, principal_portion, interest_portion, payment_type, payroll_item_id, remarks)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListLoanPayments :many
SELECT * FROM loan_payments
WHERE loan_id = $1
ORDER BY payment_date DESC;

-- name: GetEmployeeActiveLoanSummary :many
SELECT l.id, lt.name as loan_type_name, l.principal_amount, l.remaining_balance, l.monthly_amortization, l.status
FROM loans l
JOIN loan_types lt ON lt.id = l.loan_type_id
WHERE l.employee_id = $1 AND l.status IN ('active', 'approved')
ORDER BY l.created_at;
