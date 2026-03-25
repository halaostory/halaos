# US Country Support for HalaOS — Design Spec

> **Date:** 2026-03-25
> **Status:** Approved
> **Scope:** Full HR + Payroll + Compliance for the United States
> **Phase 1 States:** California, New York, Texas, Florida, Washington

---

## Goal

Add full United States country support to HalaOS, enabling US-based companies to run payroll with federal and state tax calculations, manage employees with US-specific fields, and generate core compliance forms (W-2, Form 941, state returns).

## Architecture

Follow the existing multi-country pattern: `internal/payroll/country_us.go` (like `country_lk.go`), seed data into `country_tax_brackets` / `country_contribution_rates` / `country_payroll_config` tables, conditional frontend rendering based on `company.Country === "USA"`.

## Non-Goals

- Support for all 50 US states (Phase 1 covers 5 states)
- Employee onboarding forms (W-4 intake, I-9 tracking) — future iteration
- ACA reporting (1095-C) — future iteration
- Public-facing US tax calculators — future iteration
- Workers' compensation tracking
- State-specific paid family leave programs (CA PFL, NY PFL) beyond basic FMLA

---

## 1. Payroll Engine

### 1.1 Federal Income Tax

**Source:** IRS Publication 15-T (2025), percentage method tables.

**Filing Statuses:** Single, Married Filing Jointly, Head of Household

**2025 Federal Tax Brackets (Single filer, annual):**

| Bracket | Rate |
|---------|------|
| $0 – $11,925 | 10% |
| $11,926 – $48,475 | 12% |
| $48,476 – $103,350 | 22% |
| $103,351 – $197,300 | 24% |
| $197,301 – $250,525 | 32% |
| $250,526 – $626,350 | 35% |
| Over $626,350 | 37% |

Married Filing Jointly and Head of Household have different bracket thresholds (wider brackets). Store all three filing status bracket sets in `country_tax_brackets`.

**Withholding Calculation:**
1. Start with gross pay for the period
2. Subtract pre-tax deductions (401k, health insurance, HSA, FSA)
3. Annualize: multiply by number of pay periods per year
4. Look up filing status brackets → compute annualized tax
5. De-annualize: divide by number of pay periods
6. Add any additional withholding from W-4 Step 4(c)
7. Subtract credits from W-4 Step 3 (divided by pay periods)

### 1.2 FICA (Social Security + Medicare)

| Component | Employee | Employer | Wage Base |
|-----------|----------|----------|-----------|
| Social Security | 6.2% | 6.2% | $176,100 (2025) |
| Medicare | 1.45% | 1.45% | No limit |
| Additional Medicare | 0.9% (EE only) | — | Earnings > $200,000 |

**Implementation notes:**
- Track YTD wages per employee to enforce Social Security wage base cap
- Additional Medicare Tax applies only to employee, not employer
- Both Social Security and Medicare are NOT reduced by pre-tax deductions (401k, insurance are subject to FICA)

### 1.3 FUTA (Federal Unemployment Tax)

- Rate: 6.0% on first $7,000 of wages per employee per year
- Credit for state unemployment (SUI): typically 5.4%, making effective FUTA rate = 0.6%
- Employer-only tax (no employee share)
- Store company's FUTA rate in `country_payroll_config` or company settings

### 1.4 State Income Tax

**California (CA):**
- 9 brackets from 1% to 12.3% (2025)
- Mental Health Services Tax: additional 1% on taxable income > $1,000,000
- SDI (State Disability Insurance): 1.1% on first $153,164 of wages (employee-only)
- SUI rate: varies by employer (0.5% - 6.2% on first $7,000)

**New York (NY):**
- 8 brackets from 4% to 10.9% (2025)
- NYC residents pay additional city tax: 4 brackets 3.078% - 3.876%
- SUI rate: varies by employer on first $12,500

**Texas (TX), Florida (FL), Washington (WA):**
- No state income tax
- TX/FL: no SUI employee share; WA: has Paid Family & Medical Leave (0.74% split EE/ER)

**Implementation:** Each state stored as rows in `country_tax_brackets` with `state` field. State selection based on employee's `state_of_residence`.

### 1.5 Pre-Tax Deductions

These reduce federal and state taxable income (but NOT FICA wages, except for Section 125 cafeteria plan health premiums).

| Deduction Type | Annual Limit (2025) | Reduces Federal Tax | Reduces FICA |
|---------------|--------------------|--------------------|-------------|
| 401(k) Traditional | $23,500 ($31,000 if 50+) | Yes | No |
| Health Insurance (Section 125) | Varies | Yes | Yes |
| Dental/Vision Insurance | Varies | Yes | Yes |
| HSA | $4,300 individual / $8,550 family | Yes | Yes |
| FSA (Healthcare) | $3,300 | Yes | Yes |
| FSA (Dependent Care) | $5,000 | Yes | Yes |

**Implementation:**
- New `employee_benefit_deductions` table: `employee_id, deduction_type, amount_per_period, annual_limit, reduces_fica (boolean)`
- Payroll calculator reads active deductions, applies to taxable income before tax computation
- Track YTD contributions to enforce annual limits

### 1.6 Overtime

- Federal (FLSA): 1.5x regular rate for hours > 40/week
- California: 1.5x for hours > 8/day, 2.0x for hours > 12/day, 2.0x for hours > 8 on 7th consecutive day
- Other supported states: follow federal FLSA rules
- Store in `country_payroll_config` JSON: `{"ot_rate": 1.5, "ca_daily_ot": true}`

### 1.7 Pay Frequencies

| Frequency | Periods/Year | Common In |
|-----------|-------------|-----------|
| Weekly | 52 | Hourly workers |
| Bi-weekly | 26 | Most common overall |
| Semi-monthly | 24 | Salaried workers |
| Monthly | 12 | Executive/salaried |

Company selects pay frequency at setup. Stored in `companies.pay_frequency`.

---

## 2. Leave Policies

### 2.1 Default Leave Types (seeded for USA)

| Leave Type | Default Days | Statutory | Notes |
|-----------|-------------|-----------|-------|
| PTO (Paid Time Off) | 15 | No | Combined vacation/personal, most common US approach |
| Sick Leave | 5 | Varies by state | Mandatory in CA (3 days min) and NY (varies by employer size) |
| FMLA | 60 (12 weeks) | Yes | Unpaid, job-protected. Eligibility: 12+ months, 1,250+ hours |
| Bereavement | 3 | No | Common employer benefit |
| Jury Duty | 5 | Varies | Some states require paid jury duty |

**Notes:**
- US has no federal mandate for paid vacation days
- These defaults are fully editable by admin
- FMLA eligibility tracking: system checks employee tenure and hours worked
- Mark FMLA as `unpaid: true` in leave type config

### 2.2 Accrual Support

US PTO commonly accrues over time rather than being granted upfront:
- Example: 1.25 days/month = 15 days/year
- Add `accrual_method` field to leave types: `upfront` (current behavior) or `accrual` (new)
- For accrual: `accrual_rate` (days per period), `accrual_period` (monthly/bi-weekly), `max_carryover` (days)

---

## 3. Holidays

### 3.1 US Federal Holidays (11)

| Holiday | Date |
|---------|------|
| New Year's Day | January 1 |
| Martin Luther King Jr. Day | 3rd Monday of January |
| Presidents' Day | 3rd Monday of February |
| Memorial Day | Last Monday of May |
| Juneteenth | June 19 |
| Independence Day | July 4 |
| Labor Day | 1st Monday of September |
| Columbus Day | 2nd Monday of October |
| Veterans Day | November 11 |
| Thanksgiving | 4th Thursday of November |
| Christmas Day | December 25 |

Seed 2025 and 2026 dates. All as `regular_holiday` type (affects OT pay calculation at 1.5x or 2.0x depending on company policy).

---

## 4. Employee Fields

### 4.1 New US-Specific Employee Fields

Store in the flexible `country_identifier_fields` pattern (i18n keys + conditional rendering):

| Field | Key | Format | Storage |
|-------|-----|--------|---------|
| Social Security Number | `ssn` | XXX-XX-XXXX | Encrypted, display masked as XXX-XX-1234 |
| W-4 Filing Status | `w4_filing_status` | Enum: single, married_jointly, head_of_household | Plain |
| W-4 Additional Withholding | `w4_additional_withholding` | Decimal ($/period) | Plain |
| W-4 Multiple Jobs (Step 2) | `w4_multiple_jobs` | Boolean | Plain |
| W-4 Dependents Credit (Step 3) | `w4_dependents_credit` | Decimal ($/year) | Plain |
| W-4 Other Income (Step 4a) | `w4_other_income` | Decimal ($/year) | Plain |
| W-4 Deductions (Step 4b) | `w4_deductions` | Decimal ($/year) | Plain |
| State of Residence | `state_of_residence` | 2-letter code (CA, NY, TX, FL, WA) | Plain |
| State Withholding Allowances | `state_allowances` | Integer | Plain |

**SSN Security:**
- Encrypt at rest using AES-256-GCM (existing `pkg/crypto` or new encryption helper)
- Display as `XXX-XX-1234` (only last 4 digits visible)
- Full SSN accessible only to super_admin with audit logging
- Never include full SSN in API responses (always masked)

### 4.2 New US-Specific Company Fields

Store in `company_registration_numbers` table (flexible key-value):

| Field | Key | Description |
|-------|-----|-------------|
| EIN | `ein` | Employer Identification Number (XX-XXXXXXX) |
| FUTA Rate | `futa_rate` | Federal unemployment rate (default 0.6%) |
| CA EDD Number | `ca_edd_no` | California Employment Development Department |
| CA SUI Rate | `ca_sui_rate` | California state unemployment rate |
| NY UI Number | `ny_ui_no` | New York Unemployment Insurance |
| NY SUI Rate | `ny_sui_rate` | New York state unemployment rate |

---

## 5. Compliance Forms

### 5.1 W-2 (Annual Wage and Tax Statement)

**Generated:** Annually, by January 31 for the prior year.

**Data source:** Aggregate all payroll runs for the employee in the calendar year.

**Key boxes:**
- Box 1: Wages, tips, other compensation (gross - pre-tax deductions)
- Box 2: Federal income tax withheld
- Box 3: Social Security wages (gross - Section 125 deductions, capped at wage base)
- Box 4: Social Security tax withheld
- Box 5: Medicare wages (no cap)
- Box 6: Medicare tax withheld
- Box 12: Various codes (D = 401k, DD = health insurance cost, W = HSA)
- Box 16: State wages
- Box 17: State income tax withheld

**Implementation:** `internal/compliance/forms_us.go` → `GenerateW2(companyID, employeeID, year)` → returns PDF.

### 5.2 Form 941 (Quarterly Federal Tax Return)

**Generated:** Quarterly (Q1: Apr 30, Q2: Jul 31, Q3: Oct 31, Q4: Jan 31).

**Key lines:**
- Line 2: Wages, tips, and other compensation
- Line 3: Federal income tax withheld
- Line 5a: Taxable Social Security wages × 12.4%
- Line 5c: Taxable Medicare wages × 2.9%
- Line 5d: Taxable wages subject to Additional Medicare × 0.9%
- Line 10: Total taxes after adjustments

**Implementation:** `Generate941(companyID, year, quarter)` → aggregates all payroll runs in the quarter.

### 5.3 State Returns (Phase 1)

**California DE 9/DE 9C:**
- Quarterly report of wages and withholdings per employee
- Reports SDI, SUI, PIT (Personal Income Tax) amounts

**New York NYS-45:**
- Quarterly combined withholding, wage reporting, and unemployment insurance return
- Reports state income tax withheld, SUI contributions

**Implementation:** `GenerateCADE9(companyID, year, quarter)`, `GenerateNYS45(companyID, year, quarter)`

---

## 6. Database Changes

### 6.1 New Migration: `000XX_us_country_support.sql`

```sql
-- Seed USA into country_payroll_config
INSERT INTO country_payroll_config (country, config) VALUES ('USA', '{
  "standard_hours_per_week": 40,
  "ot_rate": 1.5,
  "ca_daily_ot_rate": 1.5,
  "ca_double_ot_rate": 2.0,
  "night_diff_rate": 0,
  "has_13th_month": false,
  "pay_frequencies": ["weekly", "bi_weekly", "semi_monthly", "monthly"]
}');

-- Seed federal tax brackets (Single, MFJ, HoH) into country_tax_brackets
-- Seed CA and NY state brackets into country_tax_brackets (with state field)
-- Seed FICA rates into country_contribution_rates
-- Seed CA SDI, SUI rates into country_contribution_rates
```

### 6.2 New Table: `employee_benefit_deductions`

```sql
CREATE TABLE employee_benefit_deductions (
  id BIGSERIAL PRIMARY KEY,
  company_id BIGINT NOT NULL REFERENCES companies(id),
  employee_id BIGINT NOT NULL REFERENCES employees(id),
  deduction_type VARCHAR(50) NOT NULL, -- '401k', 'health_insurance', 'hsa', 'fsa_health', 'fsa_dependent'
  amount_per_period NUMERIC(12,2) NOT NULL,
  annual_limit NUMERIC(12,2),
  reduces_fica BOOLEAN NOT NULL DEFAULT false,
  effective_date DATE NOT NULL,
  end_date DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 6.3 New Table: `company_registration_numbers`

```sql
CREATE TABLE company_registration_numbers (
  id BIGSERIAL PRIMARY KEY,
  company_id BIGINT NOT NULL REFERENCES companies(id),
  country VARCHAR(3) NOT NULL,
  registration_type VARCHAR(50) NOT NULL, -- 'ein', 'ca_edd_no', 'ny_ui_no', 'futa_rate', etc.
  registration_value VARCHAR(100) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (company_id, registration_type)
);
```

### 6.4 Schema Changes

- Add `state_of_residence VARCHAR(5)` to `employees` table
- Add `w4_filing_status VARCHAR(30)` to `employees` table
- Add `w4_additional_withholding NUMERIC(12,2) DEFAULT 0` to `employees` table
- Add `w4_multiple_jobs BOOLEAN DEFAULT false` to `employees` table
- Add `w4_dependents_credit NUMERIC(12,2) DEFAULT 0` to `employees` table
- Add `w4_other_income NUMERIC(12,2) DEFAULT 0` to `employees` table
- Add `w4_deductions NUMERIC(12,2) DEFAULT 0` to `employees` table
- Add `ssn_encrypted BYTEA` to `employees` table (encrypted at rest)
- Add `accrual_method VARCHAR(20) DEFAULT 'upfront'` to `leave_types` table
- Add `accrual_rate NUMERIC(8,4)` to `leave_types` table
- Add `max_carryover INTEGER` to `leave_types` table
- Add `state VARCHAR(5)` to `country_tax_brackets` table (for state-level brackets)

---

## 7. Backend Files

### 7.1 New Files

| File | Purpose |
|------|---------|
| `internal/payroll/country_us.go` | `computePayUS()`, `computeContributionsUS()`, `computeWithholdingTaxUS()` |
| `internal/payroll/country_us_test.go` | Unit tests for US payroll calculations |
| `internal/compliance/forms_us.go` | W-2, Form 941, CA DE 9, NY NYS-45 generation |
| `internal/compliance/forms_us_test.go` | Unit tests for US compliance forms |
| `internal/auth/seed_usa.go` | `seedUSADefaults()` — leave types, holidays, tax data |
| `db/migrations/000XX_us_country_support.sql` | Migration for US tables, seeds, schema changes |
| `db/query/us_payroll.sql` | US-specific queries (benefit deductions, state brackets, YTD tracking) |

### 7.2 Modified Files

| File | Change |
|------|--------|
| `internal/payroll/calculator.go` | Add `case "USA"` to country switch, call `computePayUS()` |
| `internal/auth/company_setup.go` | Add `case "USA"` to `countryConfig()` and `seedCountryDefaults()` |
| `internal/compliance/forms.go` | Add `case "USA"` to `GenerateAndStore()` dispatcher |
| `db/query/payroll.sql` | Add queries for `employee_benefit_deductions`, `company_registration_numbers` |

---

## 8. Frontend Changes

### 8.1 i18n Keys (en.ts + zh.ts)

Add `USA` section:
```typescript
countryFields: {
  USA: {
    ssn: "SSN",
    ein: "EIN",
    w4FilingStatus: "W-4 Filing Status",
    single: "Single",
    marriedJointly: "Married Filing Jointly",
    headOfHousehold: "Head of Household",
    stateOfResidence: "State of Residence",
    // ...
  }
},
payroll: {
  USA: {
    federalTax: "Federal Income Tax",
    socialSecurity: "Social Security",
    medicare: "Medicare",
    additionalMedicare: "Additional Medicare Tax",
    stateTax: "State Income Tax",
    sdi: "State Disability Insurance",
    futa: "FUTA",
    sui: "State Unemployment",
    preTax401k: "401(k) Contribution",
    healthInsurance: "Health Insurance",
    dentalVision: "Dental/Vision",
    hsa: "HSA Contribution",
    fsa: "FSA Contribution",
    // ...
  }
}
```

### 8.2 Conditional UI Components

| View | Changes for USA |
|------|----------------|
| **PayrollView** | Show US payslip breakdown (Federal Tax, SS, Medicare, State Tax, 401k, Insurance). Hide 13th Month tab. |
| **SettingsView** | Show EIN field, state employer IDs. Hide SSS/PhilHealth/Pag-IBIG/BIR fields. |
| **EmployeeDetailView** | Show SSN (masked), W-4 fields, state of residence. Hide PH identifier fields. |
| **EmployeeEditView** | W-4 filing status selector, state dropdown (CA/NY/TX/FL/WA), benefit deduction configuration. |
| **SetupWizardView** | US-specific setup: EIN, state registrations, benefit plan configuration. |

### 8.3 New Component: BenefitDeductionConfig

Admin UI for managing employee benefit deductions:
- Add/edit/remove deductions per employee
- Deduction type selector (401k, health, dental, vision, HSA, FSA)
- Amount per period + annual limit
- Effective date range

---

## 9. Testing

### 9.1 Unit Tests (target: 80%+ coverage)

**Payroll calculations (`country_us_test.go`):**
- Federal tax: Single filer at $50K, $100K, $200K, $500K salary
- Federal tax: MFJ and HoH at same salary points
- FICA: Below wage base, at wage base, above wage base (cap test)
- Additional Medicare: Below and above $200K threshold
- State tax: CA at various brackets, NY at various brackets, TX (zero)
- Pre-tax deductions: 401k reduces federal but not FICA, health insurance reduces both
- Annual limit enforcement: 401k exceeding $23,500
- OT: Federal 1.5x, CA daily OT 1.5x/2.0x
- Pay frequency: Same salary computed across weekly/bi-weekly/semi-monthly/monthly

**Compliance forms (`forms_us_test.go`):**
- W-2 box calculations from sample payroll data
- Form 941 quarterly aggregation
- State return data accuracy

### 9.2 Integration Verification
- `go vet ./...` — clean
- `go test ./... -count=1` — all pass
- `cd frontend && npm run build` — no errors
- Manual test: Create USA company → add employee → run payroll → verify deductions

---

## 10. Summary of Deliverables

1. **Migration** — US tax brackets, FICA rates, benefit deductions table, company registration numbers table, employee W-4 fields
2. **Payroll engine** — `country_us.go` with federal + state tax, FICA, pre-tax deductions, OT
3. **Country setup** — `seed_usa.go` with leave types, holidays, defaults
4. **Compliance forms** — W-2, Form 941, CA DE 9, NY NYS-45
5. **Frontend** — Conditional rendering for USA, i18n keys, benefit deduction config
6. **Tests** — 80%+ coverage for US payroll calculations and compliance forms
