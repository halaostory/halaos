-- name: ListHolidays :many
SELECT * FROM holidays
WHERE company_id = $1 AND year = $2
ORDER BY holiday_date ASC;

-- name: CreateHoliday :one
INSERT INTO holidays (company_id, name, holiday_date, holiday_type, year, is_nationwide)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteHoliday :exec
DELETE FROM holidays WHERE id = $1 AND company_id = $2;

-- name: CountHolidaysInPeriod :one
SELECT COUNT(*) FROM holidays
WHERE company_id = $1
  AND holiday_date >= $2
  AND holiday_date <= $3
  AND holiday_type IN ('regular', 'special_non_working');

-- name: ListHolidaysInPeriod :many
SELECT * FROM holidays
WHERE company_id = $1
  AND holiday_date >= $2
  AND holiday_date <= $3
ORDER BY holiday_date ASC;

-- name: Get13thMonthPay :one
SELECT * FROM thirteenth_month_pay
WHERE company_id = $1 AND employee_id = $2 AND year = $3;

-- name: List13thMonthPay :many
SELECT * FROM thirteenth_month_pay
WHERE company_id = $1 AND year = $2
ORDER BY employee_id;

-- name: Upsert13thMonthPay :one
INSERT INTO thirteenth_month_pay (
    company_id, employee_id, year, total_basic_salary, months_worked,
    amount, tax_exempt_amount, taxable_amount, status, computed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'calculated', NOW())
ON CONFLICT (company_id, employee_id, year) DO UPDATE SET
    total_basic_salary = EXCLUDED.total_basic_salary,
    months_worked = EXCLUDED.months_worked,
    amount = EXCLUDED.amount,
    taxable_amount = EXCLUDED.taxable_amount,
    status = 'calculated',
    computed_at = NOW(),
    updated_at = NOW()
RETURNING *;

-- name: Update13thMonthStatus :exec
UPDATE thirteenth_month_pay SET
    status = $3,
    paid_at = CASE WHEN $3 = 'paid' THEN NOW() ELSE paid_at END,
    updated_at = NOW()
WHERE company_id = $1 AND year = $2;

-- name: CreateFinalPay :one
INSERT INTO final_pay (
    company_id, employee_id, separation_date, separation_reason,
    unpaid_salary, prorated_13th, unused_leave_conversion,
    separation_pay, tax_refund, other_deductions, total_final_pay,
    payload, status, computed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'calculated', NOW())
RETURNING *;

-- name: GetFinalPay :one
SELECT * FROM final_pay
WHERE company_id = $1 AND employee_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListFinalPays :many
SELECT fp.*, e.employee_no, e.first_name, e.last_name
FROM final_pay fp
JOIN employees e ON e.id = fp.employee_id
WHERE fp.company_id = $1
ORDER BY fp.created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateFinalPayStatus :one
UPDATE final_pay SET
    status = $3,
    released_at = CASE WHEN $3 = 'released' THEN NOW() ELSE released_at END
WHERE id = $1 AND company_id = $2
RETURNING *;
