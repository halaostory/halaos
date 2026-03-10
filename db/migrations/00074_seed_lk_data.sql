-- +goose Up

-- Sri Lanka Public Holidays 2025
-- Note: These are seeded for company_id=1 (demo company) only.
-- New LKA companies get holidays via company_setup.go at registration time.
-- Poya (full moon) days are public holidays in Sri Lanka.

-- Sri Lanka Knowledge Base articles (global, company_id = NULL)
INSERT INTO knowledge_articles (category, topic, title, content, tags, source) VALUES

-- EPF Act
('compliance', 'lk_epf', 'Sri Lanka Employees'' Provident Fund (EPF)',
'The Employees'' Provident Fund (EPF) Act No. 15 of 1958 is Sri Lanka''s primary retirement savings scheme. Contributions: Employee 8% of total earnings, Employer 12% of total earnings, totaling 20%. Total earnings include basic salary, overtime, allowances, and any other regular payments. Employers must remit contributions to the Central Bank of Sri Lanka by the last working day of the following month. Late payment incurs a surcharge of 5% for the first month and 3% per month thereafter. Members can withdraw: at age 55 (male) or 50 (female) with minimum 10 years of contributions, on permanent emigration, or on disability. Partial withdrawals are allowed for housing (after 5 years). The EPF Department issues annual member statements. Employers must register within 14 days of hiring the first employee.',
'{EPF,provident_fund,retirement,Sri_Lanka,contribution}', 'EPF Act No. 15 of 1958'),

-- ETF Act
('compliance', 'lk_etf', 'Sri Lanka Employees'' Trust Fund (ETF)',
'The Employees'' Trust Fund (ETF) Act No. 46 of 1980 provides additional social security. The employer contributes 3% of total earnings — there is no employee share. ETF benefits include: medical insurance for hospitalized employees, retirement gratuity top-up, and death benefits. Unlike EPF, employees cannot withdraw ETF until retirement, migration, or death. ETF also provides an annual insurance cover for hospitalized employees up to LKR 75,000 for surgical, LKR 35,000 for non-surgical treatment. Employers must remit ETF contributions along with EPF to the ETF Board by the last working day of the following month. Late remittance incurs a surcharge.',
'{ETF,trust_fund,employer,Sri_Lanka,medical_insurance}', 'ETF Act No. 46 of 1980'),

-- APIT
('compliance', 'lk_apit', 'Sri Lanka Advance Personal Income Tax (APIT)',
'APIT is the employer-deducted income tax under the Inland Revenue Act No. 24 of 2017 (as amended). From 2023 onwards, the monthly tax-free threshold is LKR 150,000 (annual LKR 1,800,000). Tax brackets (monthly): LKR 0-150,000: 0%. LKR 150,001-233,333: 6% (fixed deduction LKR 9,000). LKR 233,334-275,000: 18% (fixed deduction LKR 37,000). LKR 275,001-316,667: 24% (fixed deduction LKR 53,500). LKR 316,668-358,333: 30% (fixed deduction LKR 72,500). Over LKR 358,333: 36% (fixed deduction LKR 94,000). Formula: Tax = Gross × Rate − Fixed Amount. Employers must issue APIT certificate (T10) to employees annually. Terminal benefits (gratuity, ETF) may be subject to different tax treatment.',
'{APIT,income_tax,withholding,Sri_Lanka,IRD,tax_brackets}', 'Inland Revenue Act No. 24 of 2017'),

-- Gratuity Act
('payroll', 'lk_gratuity', 'Sri Lanka Payment of Gratuity Act',
'The Payment of Gratuity Act No. 12 of 1983 mandates terminal gratuity payments. Eligibility: employee must have completed at least 5 years of continuous service with the same employer. Gratuity formula: Half month''s salary for each completed year of service. Salary basis: last drawn salary (basic + fixed allowances). Payment trigger: termination of employment for any reason (resignation, retirement, retrenchment, or death). Gratuity must be paid within 30 days of the last working day. Employers with 15 or more employees are covered. Gratuity is in addition to EPF and ETF. Tax treatment: gratuity up to LKR 10 million is exempt from income tax. Employers may set up an approved gratuity fund for tax benefits.',
'{gratuity,terminal_benefit,Sri_Lanka,resignation,retirement}', 'Payment of Gratuity Act No. 12 of 1983'),

-- Shop & Office Act
('labor_law', 'lk_shop_office', 'Sri Lanka Shop and Office Employees Act',
'The Shop and Office Employees (Regulation of Employment and Remuneration) Act No. 19 of 1954 governs working conditions. Working hours: maximum 8 hours/day and 45 hours/week (5.5 day week typical). Overtime: 1.5x hourly rate for work beyond normal hours. Holiday overtime: 2x hourly rate. Annual leave: 14 days per year (after 2 years of service). Casual leave: 7 days per year. Sick leave: 7 days per year (with medical certificate). Maternity leave: 84 working days for 1st and 2nd child, 42 days for 3rd child onwards (Maternity Benefits Ordinance as amended). Rest day: one day per week (usually Sunday). Meal break: minimum 30 minutes after 5 consecutive hours. Night work restrictions apply for women and young persons. Notice period: 1 month for termination by either party (after probation).',
'{shop_office,working_hours,overtime,leave,Sri_Lanka,maternity}', 'Shop and Office Employees Act No. 19 of 1954'),

-- Wages Board
('payroll', 'lk_wages', 'Sri Lanka Wages Board Ordinance',
'The Wages Board Ordinance No. 27 of 1941 establishes minimum wages through trade-specific Wages Boards. Minimum wages vary by trade/industry and are periodically revised. As of 2024, the national minimum wage is approximately LKR 17,500/month (LKR 700/day) for private sector. Different trades have different minimum daily wages set by their respective Wages Boards. Budgetary Relief Allowance (BRA) supplements may apply. Overtime must be calculated on the full wage (not just minimum). Payment must be made at least monthly. Deductions from wages are limited and must be authorized. Night work (10pm-5am) may attract additional allowances depending on the trade.',
'{wages,minimum_wage,Sri_Lanka,wages_board}', 'Wages Board Ordinance No. 27 of 1941');

-- +goose Down
DELETE FROM knowledge_articles WHERE topic IN ('lk_epf', 'lk_etf', 'lk_apit', 'lk_gratuity', 'lk_shop_office', 'lk_wages');
