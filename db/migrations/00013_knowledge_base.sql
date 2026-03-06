-- +goose Up

CREATE TABLE knowledge_articles (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT REFERENCES companies(id),  -- NULL = global/system articles
    category VARCHAR(50) NOT NULL,  -- labor_law, compliance, benefits, payroll, leave, hr_policy
    topic VARCHAR(100) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    tags TEXT[] DEFAULT '{}',
    source VARCHAR(255),  -- e.g., "DOLE Labor Advisory No. 06-20", "RA 11210"
    is_active BOOLEAN NOT NULL DEFAULT true,
    search_vector TSVECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_knowledge_search ON knowledge_articles USING GIN(search_vector);
CREATE INDEX idx_knowledge_category ON knowledge_articles(category, is_active);
CREATE INDEX idx_knowledge_company ON knowledge_articles(company_id, is_active);

-- Auto-update search vector
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION knowledge_search_update() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        COALESCE(NEW.title, '') || ' ' ||
        COALESCE(NEW.topic, '') || ' ' ||
        COALESCE(NEW.content, '') || ' ' ||
        COALESCE(NEW.category, '') || ' ' ||
        COALESCE(array_to_string(NEW.tags, ' '), '')
    );
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_knowledge_search
    BEFORE INSERT OR UPDATE ON knowledge_articles
    FOR EACH ROW EXECUTE FUNCTION knowledge_search_update();

-- Seed with comprehensive Philippine labor law knowledge
INSERT INTO knowledge_articles (category, topic, title, content, tags, source) VALUES
-- Leave Types
('leave', 'service_incentive_leave', 'Service Incentive Leave (SIL)', 'Under Article 95 of the Labor Code, every employee who has rendered at least one year of service is entitled to 5 days of Service Incentive Leave (SIL) with pay per year. SIL may be used for vacation, sick leave, or personal reasons. Unused SIL is commutable to cash at the end of the year. Employees already enjoying at least 5 days of vacation leave with pay are not entitled to SIL in addition. SIL applies to all employees except: government employees, domestic helpers, managerial employees, field personnel, family members of the employer, and those already enjoying the benefit.', '{leave,SIL,service_incentive,vacation}', 'Labor Code Art. 95'),

('leave', 'maternity_leave', 'Expanded Maternity Leave', 'Republic Act No. 11210 (Expanded Maternity Leave Law, 2019) grants 105 days of paid maternity leave for live childbirth, regardless of delivery method. Solo mothers receive an additional 15 days (total 120 days). An optional 30-day extension without pay is available. For miscarriage or emergency termination of pregnancy, 60 days of paid leave is provided. The benefit applies to all female workers in the private and public sectors, regardless of civil status or legitimacy of the child. The SSS maternity benefit is computed based on the average daily salary credit. An employed woman can allocate up to 7 days of her maternity leave to the child''s father (or an alternate caregiver). The leave is non-cumulative and non-convertible to cash. The employer advances the maternity benefit, then seeks reimbursement from SSS.', '{maternity,leave,RA_11210,SSS,childbirth,solo_mother}', 'RA 11210'),

('leave', 'paternity_leave', 'Paternity Leave', 'Republic Act No. 8187 grants 7 days of paid paternity leave to all married male employees in the private and public sectors for the first four (4) deliveries of the legitimate spouse. The leave must be availed within 60 days from the date of delivery. Paternity leave is non-cumulative and non-convertible to cash. The employee must notify the employer of the pregnancy and expected date of delivery.', '{paternity,leave,RA_8187,father}', 'RA 8187'),

('leave', 'solo_parent_leave', 'Solo Parent Leave', 'Republic Act No. 8972 (Solo Parents'' Welfare Act) grants 7 working days of paid parental leave per year to solo parents who have rendered at least 1 year of service. The solo parent must present a Solo Parent Identification Card issued by the DSWD. A solo parent includes: those giving birth as a result of rape, parent left to care for child due to spouse death/detention/absence, or any person who solely provides parental care. This leave is in addition to other leave benefits.', '{solo_parent,leave,RA_8972,DSWD}', 'RA 8972'),

('leave', 'vawc_leave', 'VAWC Leave (Violence Against Women and Children)', 'Republic Act No. 9262 grants 10 days of paid leave to women employees who are victims of violence against women and their children (VAWC). The leave is extendable upon court order. The victim must present a Barangay Protection Order (BPO), Temporary Protection Order (TPO), or Permanent Protection Order (PPO). This leave is in addition to other leave benefits and may be used for medical attendance, court proceedings, or other activities related to the case.', '{VAWC,leave,RA_9262,violence,protection_order}', 'RA 9262'),

('leave', 'special_leave_women', 'Special Leave for Women (Gynecological Leave)', 'Republic Act No. 9710 (Magna Carta of Women) grants a special leave benefit of up to 2 months (60 calendar days) with full pay to women employees who have rendered at least 6 months of continuous aggregate employment and have undergone surgery caused by gynecological disorders. The leave is non-cumulative and non-convertible to cash.', '{gynecological,leave,RA_9710,surgery,women}', 'RA 9710'),

-- Government Contributions
('compliance', 'sss_contributions', 'SSS Contributions', 'The Social Security System (SSS) provides social insurance to private sector employees. In 2025, the total contribution rate is 14% of the Monthly Salary Credit (MSC). Employee share: 4.5%, Employer share: 9.5%. The MSC range is PHP 4,000 (minimum) to PHP 30,000 (maximum). Employees'' Compensation (EC) is an additional employer contribution of PHP 10-30 depending on MSC bracket. SSS covers: sickness, maternity, disability, retirement, death, and funeral benefits. Self-employed and voluntary members pay the full 14%. Employers must remit contributions by the 10th of the following month (last digit of SSS employer number determines exact due date). Late remittance incurs 2% monthly penalty.', '{SSS,contribution,social_security,employee_share,employer_share}', 'RA 11199'),

('compliance', 'philhealth_contributions', 'PhilHealth Contributions', 'PhilHealth provides national health insurance. In 2025, the premium rate is 5% of the monthly basic salary (MBS). The contribution is shared equally: Employee 2.5%, Employer 2.5%. Income floor: PHP 10,000, Income ceiling: PHP 100,000. Maximum monthly contribution: PHP 5,000 (PHP 2,500 EE + PHP 2,500 ER). PhilHealth covers: inpatient care, outpatient care, emergency services, Z Benefits (catastrophic conditions), and SDN (single period of confinement). Employers must remit by the 15th of the following month.', '{PhilHealth,health_insurance,contribution,premium}', 'RA 11223'),

('compliance', 'pagibig_contributions', 'Pag-IBIG Fund Contributions', 'Pag-IBIG (HDMF - Home Development Mutual Fund) contributions are mandatory for all employed workers earning at least PHP 1,000/month. Employee contribution: 1% if monthly compensation is PHP 1,500 or below; 2% if above PHP 1,500. Employer always contributes 2%. Maximum monthly salary base: PHP 10,000 (maximum EE: PHP 200, maximum ER: PHP 200). Members may opt for higher contributions up to PHP 5,000/month. Benefits include: housing loans (up to PHP 6 million), multi-purpose loans, calamity loans, and provident savings. Employers remit by the 15th of the following month.', '{PagIBIG,HDMF,housing,contribution,loan}', 'RA 9679'),

('compliance', 'bir_withholding_tax', 'BIR Withholding Tax (TRAIN Law)', 'Under the TRAIN Law (RA 10963, effective 2018), the revised income tax rates for individuals are: Annual income up to PHP 250,000: 0%. Over PHP 250,000 to PHP 400,000: 15% of excess over PHP 250,000. Over PHP 400,000 to PHP 800,000: PHP 22,500 + 20% of excess over PHP 400,000. Over PHP 800,000 to PHP 2,000,000: PHP 102,500 + 25% of excess over PHP 800,000. Over PHP 2,000,000 to PHP 8,000,000: PHP 402,500 + 30% of excess over PHP 2,000,000. Over PHP 8,000,000: PHP 2,202,500 + 35% of excess over PHP 8,000,000. Minimum wage earners are exempt from income tax. Employers withhold tax using the cumulative average method or graduated withholding tax table.', '{BIR,tax,withholding,TRAIN_law,income_tax}', 'RA 10963'),

-- Compensation
('payroll', '13th_month_pay', '13th Month Pay', 'Presidential Decree No. 851 mandates all employers to pay 13th Month Pay to all rank-and-file employees who have worked at least 1 month during a calendar year. Formula: Total Basic Salary Earned During the Year / 12. Must be paid on or before December 24 of each year. Pro-rated for employees who worked less than 12 months. Basic salary excludes: overtime pay, holiday pay, night shift differential, allowances, and monetary benefits not considered as basic salary. Under the TRAIN Law (RA 10963), the first PHP 90,000 of the 13th month pay and other benefits combined is tax-exempt. Managerial employees are excluded from mandatory coverage but employers may voluntarily include them.', '{13th_month,compensation,PD_851,TRAIN_law,tax_exempt}', 'PD 851'),

('payroll', 'overtime_pay', 'Overtime Pay Rules', 'Under Articles 87-93 of the Labor Code: Regular overtime (beyond 8 hours): +25% of hourly rate. Rest day/special non-working holiday OT: +30% of daily rate, then +30% for OT beyond 8 hours. Regular holiday work: 200% of daily rate (double pay). Regular holiday overtime: 200% + 30% = 260% of hourly rate. Special non-working holiday: +30% of daily rate. Night shift differential (10pm-6am): +10% of regular wage. Double holiday (when two holidays fall on same day): 300% of daily rate. Rest day on regular holiday: 260% of daily rate. Overtime must be authorized by the employer. Compressed workweek arrangements may exempt employers from overtime pay if total weekly hours don''t exceed 48.', '{overtime,OT,night_shift,holiday_pay,rest_day}', 'Labor Code Art. 87-93'),

('payroll', 'minimum_wage', 'Minimum Wage by Region', 'Minimum wage rates are set by the Regional Tripartite Wages and Productivity Board (RTWPB) per region. As of 2024-2025: NCR (Metro Manila): PHP 645/day for non-agriculture, PHP 608/day for agriculture. Region III (Central Luzon): PHP 490-530/day. Region IV-A (CALABARZON): PHP 490-573/day. Region VII (Central Visayas): PHP 468/day. Region XI (Davao): PHP 461/day. Minimum wage earners are exempt from income tax. Employers in Barangay Micro Business Enterprises (BMBEs) may be exempt from minimum wage requirements. Domestic workers (kasambahay) have separate minimum wages: PHP 6,000/month in NCR, PHP 5,000 in chartered cities, PHP 4,000 in other municipalities.', '{minimum_wage,RTWPB,NCR,regional_wage,kasambahay}', 'RA 6727'),

('payroll', 'final_pay', 'Final Pay and Separation Pay', 'DOLE Labor Advisory No. 06-20 requires employers to release final pay within 30 days from the date of separation or termination. Final pay components: last salary (unpaid wages), pro-rated 13th month pay, cash conversion of accrued/unused leave credits (if company policy or CBA provides), separation pay (if applicable), tax refund or tax due, and any other company benefits or incentives. Deductions may include: outstanding company loans, unreturned property, or other accountabilities. Separation pay is required for authorized causes: redundancy (1 month or 1 month per year of service, whichever is higher), retrenchment (1/2 month per year of service or 1 month, whichever is higher), closure/cessation not due to serious losses (1/2 month per year), and disease (1 month per year or 1/2 month per year, whichever is higher). No separation pay for just causes (serious misconduct, fraud, etc.).', '{final_pay,separation_pay,termination,DOLE}', 'DOLE Labor Advisory No. 06-20'),

('payroll', 'de_minimis_benefits', 'De Minimis Benefits', 'De minimis benefits are facilities/privileges of relatively small value given by the employer as means of promoting health, goodwill, contentment, or efficiency. Tax-exempt limits (per RR 11-2018): Rice subsidy: PHP 2,000/month or 1 sack (50kg) of rice/month. Uniform/clothing allowance: PHP 6,000/year. Medical cash allowance to dependents: PHP 1,500/quarter. Laundry allowance: PHP 300/month. Employee achievement awards (length of service/safety): PHP 10,000/year. Christmas gifts/major anniversary celebrations: PHP 5,000/year. Daily meal allowance for overtime/night shift: not exceeding 25% of basic minimum wage. Benefits given for CBA negotiations: PHP 10,000/year. Any excess over the ceiling is taxable. Total de minimis benefits exceeding PHP 90,000 combined with 13th month pay and other benefits become taxable.', '{de_minimis,benefits,tax_exempt,allowance}', 'RR 11-2018'),

-- Labor Law
('labor_law', 'security_of_tenure', 'Security of Tenure', 'Under Article 294 (formerly Art. 279) of the Labor Code, regular employees enjoy security of tenure. They can only be terminated for just causes (Art. 297) or authorized causes (Art. 298-299). Just Causes: serious misconduct, willful disobedience, gross and habitual neglect of duties, fraud or breach of trust, commission of a crime against the employer or family, analogous causes. Authorized Causes: installation of labor-saving devices, redundancy, retrenchment, closure or cessation of business, and disease. Procedural due process requirements: For just causes - two-notice rule (notice to explain + notice of decision with hearing/conference). For authorized causes - 30-day advance notice to DOLE and the employee plus separation pay.', '{security_of_tenure,termination,just_cause,authorized_cause,due_process}', 'Labor Code Art. 294-299'),

('labor_law', 'regular_employment', 'Types of Employment', 'Under Philippine law, employment types include: Regular employment: employee performs activities necessary/desirable in the usual business; becomes regular after probation. Probationary employment: up to 6 months; must meet reasonable standards for regularization. Project employment: employment fixed for a specific project; ends with project completion. Seasonal employment: work performed only during a certain season. Fixed-term/contractual: specific period agreed upon; must have legitimate purpose (not to circumvent regularization). Casual employment: not covered above; becomes regular after 1 year of service. An employee who has rendered at least 1 year of service, whether continuous or broken, is considered regular with respect to the activity they performed (Article 295). Labor-only contracting is prohibited (DO 174-17).', '{employment,regular,probationary,contractual,DO_174}', 'Labor Code Art. 295'),

('labor_law', 'working_hours', 'Working Hours and Rest Periods', 'Under the Labor Code: Normal working hours: 8 hours/day (Article 83). Meal period: not less than 60 minutes (Article 85) - not compensable unless on-call. Rest period: at least 24 consecutive hours after every 6 consecutive working days (Article 91). Night shift: work between 10:00 PM and 6:00 AM entitled to night differential of 10% of regular wage. Compressed workweek: allowed under DOLE DA 02-09; total hours in a week cannot exceed 48. Flexible work arrangements: telecommuting (RA 11165), compressed work, staggered hours. Overtime beyond 8 hours must be compensated. No employee shall be compelled to work overtime except in specified emergency situations (Article 89).', '{working_hours,rest_period,overtime,night_shift,compressed_workweek}', 'Labor Code Art. 83-93'),

('labor_law', 'holiday_pay', 'Holiday Pay Rules', 'Regular Holidays (RA 9849 + Proclamation): New Year''s Day, Maundy Thursday, Good Friday, Araw ng Kagitingan (Apr 9), Labor Day (May 1), Independence Day (Jun 12), National Heroes Day (last Mon of Aug), Bonifacio Day (Nov 30), Christmas Day (Dec 25), Rizal Day (Dec 30). Special Non-Working Holidays: Ninoy Aquino Day (Aug 21), All Saints Day (Nov 1), Immaculate Conception (Dec 8), Last Day of the Year (Dec 31), EDSA Revolution (Feb 25), Chinese New Year. Pay rules: Regular holiday - 200% of daily rate even if unworked (for monthly-paid, already included). Special non-working holiday - 130% if worked, no pay if unworked (for daily-paid). Double holiday: 300% of daily rate. These are subject to annual Presidential Proclamation.', '{holiday,regular_holiday,special_holiday,holiday_pay}', 'RA 9849'),

('labor_law', 'retirement', 'Retirement Benefits', 'Republic Act No. 7641 (Retirement Pay Law): Optional retirement age: 60 years old with at least 5 years of service. Compulsory retirement: 65 years old. Retirement pay: at least 1/2 month salary for every year of service. "1/2 month salary" includes: 15 days salary based on latest salary rate + cash equivalent of 5 days SIL + 1/12 of 13th month pay. Fraction of at least 6 months is considered 1 year. This applies in the absence of a retirement plan or CBA providing higher benefits. Underground/surface mining employees may retire at age 50 after 5 years. Retirement benefits are exempt from income tax (RA 4917). SSS retirement benefit is separate and computed based on contributions.', '{retirement,RA_7641,retirement_pay,SSS}', 'RA 7641'),

-- HR Policy
('hr_policy', 'employee_records', 'Employee Record Keeping Requirements', 'Under DOLE Department Order No. 183-17, employers must maintain employee records for at least 3 years from last entry. Required records include: payroll records (name, rate of pay, daily hours worked, earnings, deductions), employee 201 file (personal data sheet, employment contract, government ID copies, pre-employment requirements), time records/DTR (daily time record), SSS/PhilHealth/PagIBIG enrollment forms, BIR Form 2316 (annual). Data Privacy Act (RA 10173) requires employers to protect personal information, obtain consent for data collection, and appoint a Data Protection Officer for organizations with 250+ employees or that process sensitive personal information.', '{records,201_file,DOLE,data_privacy,RA_10173}', 'DO 183-17'),

('hr_policy', 'occupational_safety', 'Occupational Safety and Health Standards', 'Republic Act No. 11058 (OSH Law, 2018) strengthens OSH standards. Key requirements: employers must provide a safe and healthy workplace, appoint a safety officer (based on company size), maintain a first-aid kit, conduct safety training, report accidents to DOLE. Penalties for non-compliance: PHP 100,000 per day per violation. Companies with 200+ employees must have a full-time safety officer, registered nurse, and first-aid room. Monthly safety meetings are required. DOLE may conduct unannounced workplace inspections. Workers have the right to refuse unsafe work without fear of retaliation. COVID-19 protocols may still apply per DOLE-DTI guidelines.', '{OSH,safety,RA_11058,workplace,DOLE}', 'RA 11058');

-- +goose Down
DROP TABLE IF EXISTS knowledge_articles;
DROP FUNCTION IF EXISTS knowledge_search_update();
