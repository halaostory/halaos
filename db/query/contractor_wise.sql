-- Contractor payouts via Wise

-- name: GetContractorPayoutAccount :one
SELECT * FROM contractor_payout_accounts WHERE contractor_id = $1 AND company_id = $2;

-- name: UpsertContractorPayoutAccount :one
INSERT INTO contractor_payout_accounts (contractor_id, company_id, recipient_id, currency, label)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (contractor_id) DO UPDATE SET recipient_id = EXCLUDED.recipient_id, currency = EXCLUDED.currency,
    label = EXCLUDED.label, updated_at = now()
RETURNING *;

-- name: GetContractorPaymentForPayout :one
SELECT p.id, p.contractor_id, p.status, p.net_amount::float8 AS net_amount, p.invoice_no, c.name AS contractor_name
FROM contractor_payments p JOIN contractors c ON c.id = p.contractor_id
WHERE p.id = $1 AND p.company_id = $2;

-- name: GetWiseTransfer :one
SELECT * FROM contractor_wise_transfers WHERE payment_id = $1 AND company_id = $2;

-- name: CreateWiseTransfer :one
INSERT INTO contractor_wise_transfers (payment_id, company_id, quote_id, transfer_id, source_amount, source_currency,
    target_amount, target_currency, rate, status, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING *;

-- name: SetWiseTransferStatus :exec
UPDATE contractor_wise_transfers SET status = $3, updated_at = now() WHERE payment_id = $1 AND company_id = $2;
