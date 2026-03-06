-- name: CreateTaxFiling :one
INSERT INTO tax_filings (
    company_id, filing_type, period_type, period_year, period_month,
    period_quarter, due_date, amount, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListTaxFilings :many
SELECT * FROM tax_filings
WHERE company_id = @company_id
  AND (@status = '' OR status = @status)
  AND (@filing_type = '' OR filing_type = @filing_type)
  AND period_year = @period_year
ORDER BY due_date;

-- name: GetTaxFiling :one
SELECT * FROM tax_filings WHERE id = $1 AND company_id = $2;

-- name: UpdateTaxFilingStatus :one
UPDATE tax_filings SET
    status = $3,
    filing_date = CASE WHEN $3 IN ('submitted', 'filed') THEN CURRENT_DATE ELSE filing_date END,
    filed_by = $4,
    reference_no = $5,
    notes = $6,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: UpdateTaxFilingAmount :one
UPDATE tax_filings SET
    amount = $3,
    penalty_amount = $4,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: ListOverdueFilings :many
SELECT * FROM tax_filings
WHERE company_id = $1
  AND status IN ('pending', 'generated')
  AND due_date < CURRENT_DATE
ORDER BY due_date;

-- name: ListUpcomingFilings :many
SELECT * FROM tax_filings
WHERE company_id = $1
  AND status IN ('pending', 'generated')
  AND due_date >= CURRENT_DATE
  AND due_date <= CURRENT_DATE + INTERVAL '30 days'
ORDER BY due_date;

-- name: GetFilingSummary :one
SELECT
    COUNT(*) as total,
    COUNT(CASE WHEN status = 'filed' THEN 1 END) as filed,
    COUNT(CASE WHEN status IN ('pending', 'generated') AND due_date < CURRENT_DATE THEN 1 END) as overdue,
    COUNT(CASE WHEN status IN ('pending', 'generated') AND due_date >= CURRENT_DATE THEN 1 END) as upcoming,
    COALESCE(SUM(amount), 0) as total_amount,
    COALESCE(SUM(penalty_amount), 0) as total_penalties
FROM tax_filings
WHERE company_id = $1 AND period_year = $2;

-- name: CreateRemittanceRecord :one
INSERT INTO remittance_records (
    company_id, filing_id, agency, period_year, period_month,
    employee_count, employer_share, employee_share, total_amount
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListRemittanceRecords :many
SELECT * FROM remittance_records
WHERE company_id = $1 AND period_year = $2
ORDER BY period_month, agency;

-- name: MarkOverdueFilings :exec
UPDATE tax_filings SET
    status = 'overdue',
    updated_at = NOW()
WHERE status IN ('pending', 'generated')
  AND due_date < CURRENT_DATE;

-- name: GenerateAnnualFilings :exec
INSERT INTO tax_filings (company_id, filing_type, period_type, period_year, period_month, due_date, status)
SELECT $1, ft.filing_type, ft.period_type, $2, ft.period_month, ft.due_date, 'pending'
FROM (
    -- BIR 1601-C (Monthly withholding tax)
    SELECT 'bir_1601c' as filing_type, 'monthly' as period_type, m.n as period_month,
           make_date($2::int, m.n, 10) as due_date
    FROM generate_series(1, 12) as m(n)
    UNION ALL
    -- SSS (Monthly contribution)
    SELECT 'sss_r3', 'monthly', m.n,
           make_date($2::int, m.n, CASE WHEN m.n = 12 THEN 31 ELSE 10 END)
    FROM generate_series(1, 12) as m(n)
    UNION ALL
    -- PhilHealth (Monthly)
    SELECT 'philhealth_rf1', 'monthly', m.n,
           make_date($2::int, m.n, 15)
    FROM generate_series(1, 12) as m(n)
    UNION ALL
    -- Pag-IBIG (Monthly)
    SELECT 'pagibig_ml1', 'monthly', m.n,
           make_date($2::int, m.n, 10)
    FROM generate_series(1, 12) as m(n)
    UNION ALL
    -- BIR 2316 (Annual ITR)
    SELECT 'bir_2316', 'annual', NULL,
           make_date($2::int, 1, 31)
) ft
ON CONFLICT DO NOTHING;
