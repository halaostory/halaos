-- Payouts

-- name: ListNetPayForRun :many
SELECT pi.employee_id, e.employee_no, e.first_name, e.last_name, pi.net_pay::float8 AS net_pay,
       COALESCE(ep.bank_name, '')::text AS bank_name, COALESCE(ep.bank_account_no, '')::text AS bank_account_no,
       COALESCE(ep.bank_account_name, '')::text AS bank_account_name
FROM payroll_items pi
JOIN employees e ON e.id = pi.employee_id
LEFT JOIN employee_profiles ep ON ep.employee_id = e.id
WHERE pi.run_id = $1
ORDER BY e.last_name, e.first_name;

-- name: GetCycleForPayout :one
SELECT id, name, status, pay_date, cycle_type FROM payroll_cycles WHERE id = $1 AND company_id = $2;

-- name: GetPayoutBatchForRun :one
SELECT * FROM payout_batches WHERE run_id = $1 AND provider = $2;

-- name: CreatePayoutBatch :one
INSERT INTO payout_batches (company_id, cycle_id, run_id, provider, total, item_count, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: SetPayoutBatchSubmitted :exec
UPDATE payout_batches SET external_id = $2, status = $3, updated_at = now() WHERE id = $1;

-- name: CreatePayoutItem :exec
INSERT INTO payout_items (batch_id, employee_id, external_id, amount, bank_code, account_no, account_name)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdatePayoutItemStatus :execrows
UPDATE payout_items SET status = $2, failure_code = $3, updated_at = now() WHERE external_id = $1;

-- name: RefreshPayoutBatchStatus :exec
UPDATE payout_batches b SET status = CASE
        WHEN NOT EXISTS (SELECT 1 FROM payout_items i WHERE i.batch_id = b.id AND i.status = 'pending')
             THEN CASE WHEN EXISTS (SELECT 1 FROM payout_items i WHERE i.batch_id = b.id AND i.status = 'failed') THEN 'completed_with_failures' ELSE 'completed' END
        ELSE b.status END,
    updated_at = now()
WHERE b.id = (SELECT pi.batch_id FROM payout_items pi WHERE pi.external_id = $1);

-- name: ListPayoutBatches :many
SELECT b.*, c.name AS cycle_name,
       (SELECT COUNT(*) FROM payout_items i WHERE i.batch_id = b.id AND i.status = 'completed')::int AS paid,
       (SELECT COUNT(*) FROM payout_items i WHERE i.batch_id = b.id AND i.status = 'failed')::int AS failed
FROM payout_batches b JOIN payroll_cycles c ON c.id = b.cycle_id
WHERE b.company_id = $1 ORDER BY b.created_at DESC LIMIT 50;

-- name: ListPayoutItems :many
SELECT i.*, e.first_name, e.last_name FROM payout_items i JOIN employees e ON e.id = i.employee_id
WHERE i.batch_id = $1 ORDER BY e.last_name;
