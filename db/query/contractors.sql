-- Contractors & freelancer payments.

-- name: CreateContractor :one
INSERT INTO contractors (company_id, kind, name, email, tin, address, is_vat_registered, ewt_rate, atc_code,
                         bank_name, bank_account_no, bank_account_name, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateContractor :one
UPDATE contractors SET kind = $3, name = $4, email = $5, tin = $6, address = $7, is_vat_registered = $8,
    ewt_rate = $9, atc_code = $10, bank_name = $11, bank_account_no = $12, bank_account_name = $13,
    notes = $14, status = $15, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: GetContractor :one
SELECT * FROM contractors WHERE id = $1 AND company_id = $2;

-- name: ListContractors :many
SELECT c.*,
       COALESCE((SELECT SUM(p.gross_amount) FROM contractor_payments p
                 WHERE p.contractor_id = c.id AND p.status = 'paid'
                   AND p.paid_at >= date_trunc('year', CURRENT_DATE)), 0)::float8 AS ytd_paid
FROM contractors c
WHERE c.company_id = $1
ORDER BY c.status, c.name;

-- name: CreateContractorPayment :one
INSERT INTO contractor_payments (company_id, contractor_id, invoice_no, description, service_date, gross_amount,
    vat_amount, ewt_base, ewt_rate, ewt_amount, net_amount, atc_code, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: ListContractorPayments :many
SELECT p.*, c.name AS contractor_name, c.tin AS contractor_tin
FROM contractor_payments p JOIN contractors c ON c.id = p.contractor_id
WHERE p.company_id = $1
  AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (sqlc.arg(contractor_id)::bigint = 0 OR p.contractor_id = sqlc.arg(contractor_id)::bigint)
ORDER BY p.created_at DESC
LIMIT 300;

-- name: ApproveContractorPayment :one
UPDATE contractor_payments SET status = 'approved', approved_by = $3, approved_at = NOW(), updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'draft'
RETURNING *;

-- name: MarkContractorPaymentPaid :one
UPDATE contractor_payments SET status = 'paid', paid_at = $3, paid_reference = $4, updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'approved'
RETURNING *;

-- name: VoidContractorPayment :one
UPDATE contractor_payments SET status = 'void', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status IN ('draft', 'approved')
RETURNING *;

-- name: ListApprovedPaymentsForBankFile :many
SELECT p.id, p.net_amount, p.invoice_no, c.name, c.bank_name, c.bank_account_no, c.bank_account_name
FROM contractor_payments p JOIN contractors c ON c.id = p.contractor_id
WHERE p.company_id = $1 AND p.status = 'approved'
ORDER BY c.name, p.id;

-- name: ContractorWithholdingByPayee :many
-- Paid amounts and EWT per payee, ATC and month within [start, end] (for 2307, 0619-E, 1601-EQ/QAP).
SELECT c.id AS contractor_id, c.name, c.tin, c.address, c.kind, p.atc_code,
       EXTRACT(MONTH FROM p.paid_at)::int AS month,
       SUM(p.ewt_base)::float8 AS base, SUM(p.ewt_amount)::float8 AS ewt
FROM contractor_payments p JOIN contractors c ON c.id = p.contractor_id
WHERE p.company_id = sqlc.arg(company_id) AND p.status = 'paid'
  AND p.paid_at BETWEEN sqlc.arg(start_date)::date AND sqlc.arg(end_date)::date
  AND (sqlc.arg(contractor_id)::bigint = 0 OR c.id = sqlc.arg(contractor_id)::bigint)
GROUP BY c.id, c.name, c.tin, c.address, c.kind, p.atc_code, month
ORDER BY c.name, p.atc_code, month;
