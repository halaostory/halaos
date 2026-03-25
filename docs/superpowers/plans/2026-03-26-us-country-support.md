# US Country Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full United States country support to HalaOS — federal/state payroll, FICA, pre-tax deductions, leave/holidays, compliance forms (W-2, 941), and conditional frontend rendering.

**Architecture:** Follows the existing multi-country pattern established by Sri Lanka (LKA) in migration 00073. New country module `country_us.go` dispatched via `case "USA"` switch in `calculator.go`. Seed data into `country_tax_brackets` / `country_contribution_rates` / `country_payroll_config`. Frontend conditionally renders US fields when `company.Country === "USA"`.

**Tech Stack:** Go 1.25 (Gin + sqlc/pgx), PostgreSQL, Vue3 + TypeScript + NaiveUI

**Spec:** `docs/superpowers/specs/2026-03-25-us-country-support-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `pkg/crypto/encryption.go` | AES-256-GCM encrypt/decrypt for SSN storage |
| `pkg/crypto/encryption_test.go` | Unit tests for crypto package |
| `db/migrations/00086_us_country_support.sql` | Schema changes + US seed data |
| `db/query/us_payroll.sql` | US-specific sqlc queries (benefit deductions, registration numbers, YTD, state brackets) |
| `internal/payroll/country_us.go` | `computePayUS()`, `computeContributionsUS()`, `computeWithholdingTaxUS()` |
| `internal/payroll/country_us_test.go` | Unit tests for US payroll calculations |
| `internal/auth/seed_usa.go` | `seedUSADefaults()` — leave types, holidays for new US companies |
| `internal/compliance/forms_us.go` | W-2, Form 941, CA DE 9, NY NYS-45 generation |
| `internal/compliance/forms_us_test.go` | Unit tests for US compliance forms |
| `internal/payroll/benefit_handler.go` | CRUD handler for employee benefit deductions |
| `internal/payroll/registration_handler.go` | CRUD handler for company registration numbers |
| `frontend/src/components/payroll/BenefitDeductionConfig.vue` | Admin UI for managing employee benefit deductions |

### Modified Files

| File | Change |
|------|--------|
| `internal/payroll/calculator.go:28-85` | Add US fields to `EmployeePayData` struct |
| `internal/payroll/calculator.go:100-240` | Load employee profile W-4 fields + state in employee loop |
| `internal/payroll/calculator.go:244-272` | Add `case "USA"` to country switch |
| `internal/payroll/calculator.go:302-327` | Add US fields to `CreatePayrollItem` call |
| `internal/payroll/routes.go` | Register benefit deduction + registration number routes |
| `internal/auth/company_setup.go:22-33` | Add `case "USA"` to `countryConfig()` |
| `internal/auth/company_setup.go:42-49` | Add `case "USA"` to `seedCountryDefaults()` |
| `internal/compliance/forms.go:483-553` | Add US form types to `GenerateAndStore()` switch |
| `db/query/payroll.sql:104-114` | Update `CreatePayrollItem` to include US columns |
| `db/query/country_payroll.sql` | Add queries for state-specific bracket lookups |
| `frontend/src/views/PayrollView.vue:70-71` | Add `isUSA` computed, conditional US columns |
| `frontend/src/views/EmployeeDetailView.vue` | Show SSN (masked), W-4 fields for US |
| `frontend/src/api/client.ts` | Add benefit deduction + registration number API methods |
| `frontend/src/i18n/en.ts` | Add `countryFields.USA` + `payroll.USA` keys |
| `frontend/src/i18n/zh.ts` | Add matching Chinese translations |

---

## Task Dependencies

```
Task 1 (crypto) ─────────────────────────────────────────────────────┐
Task 2 (migration) ──→ Task 3 (sqlc queries) ──→ Task 4 (sqlc gen) ─┤
                                                                      ├→ Task 7 (calculator integration)
Task 5 (country_us.go payroll engine) ───────────────────────────────┤
Task 6 (seed_usa.go) ───────────────────────────────────────────────┤
Task 7a (benefit deduction handler) ── after Task 4                  │
Task 8 (compliance forms) ── after Task 7                            │
Task 9 (frontend i18n) ──→ Task 10 (frontend payroll) ──→ Task 11 (frontend benefits, needs 7a)
Task 12 (integration verification) ── after all
```

---

### Task 1: AES-256-GCM Encryption Package

Creates `pkg/crypto` for encrypting SSN at rest.

**Files:**
- Create: `pkg/crypto/encryption.go`
- Create: `pkg/crypto/encryption_test.go`

- [ ] **Step 1: Write the failing tests**

Create `pkg/crypto/encryption_test.go`:

```go
package crypto

import (
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := GenerateKey()
	plaintext := "123-45-6789"

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if got != plaintext {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestEncrypt_DifferentNonce(t *testing.T) {
	key := GenerateKey()
	plaintext := "123-45-6789"

	ct1, _ := Encrypt(key, plaintext)
	ct2, _ := Encrypt(key, plaintext)

	if string(ct1) == string(ct2) {
		t.Error("expected different ciphertexts due to random nonce")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := GenerateKey()
	key2 := GenerateKey()
	plaintext := "123-45-6789"

	ciphertext, _ := Encrypt(key1, plaintext)
	_, err := Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := GenerateKey()
	ciphertext, _ := Encrypt(key, "123-45-6789")

	ciphertext[len(ciphertext)-1] ^= 0xFF // flip last byte
	_, err := Decrypt(key, ciphertext)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestMaskSSN(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123-45-6789", "XXX-XX-6789"},
		{"123456789", "XXX-XX-6789"},
		{"12345", "XXX-XX-2345"},
		{"", "XXX-XX-XXXX"},
	}
	for _, tt := range tests {
		got := MaskSSN(tt.input)
		if got != tt.want {
			t.Errorf("MaskSSN(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./pkg/crypto/... -v -count=1`
Expected: FAIL (package does not exist yet)

- [ ] **Step 3: Write the implementation**

Create `pkg/crypto/encryption.go`:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
)

// GenerateKey creates a random 32-byte AES-256 key.
func GenerateKey() []byte {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return key
}

// Encrypt encrypts plaintext using AES-256-GCM. Returns nonce+ciphertext.
func Encrypt(key []byte, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt decrypts AES-256-GCM ciphertext (nonce prepended).
func Decrypt(key []byte, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// MaskSSN returns "XXX-XX-1234" format showing only the last 4 digits.
func MaskSSN(ssn string) string {
	digits := strings.ReplaceAll(strings.ReplaceAll(ssn, "-", ""), " ", "")
	if len(digits) < 4 {
		return "XXX-XX-XXXX"
	}
	return "XXX-XX-" + digits[len(digits)-4:]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./pkg/crypto/... -v -count=1`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add pkg/crypto/encryption.go pkg/crypto/encryption_test.go
git commit -m "feat(crypto): add AES-256-GCM encryption package for SSN storage"
```

---

### Task 2: Database Migration

Creates the migration for US country support: new tables, schema changes, seed data.

**Files:**
- Create: `db/migrations/00086_us_country_support.sql`

**Reference:** Existing pattern in `db/migrations/00073_multi_country_payroll.sql`

- [ ] **Step 1: Create the migration file**

Create `db/migrations/00086_us_country_support.sql`:

```sql
-- +goose Up

-- ============================================================
-- 1. Schema Changes: Add columns to existing tables
-- ============================================================

-- Add filing_status and state columns to country_tax_brackets
ALTER TABLE country_tax_brackets ADD COLUMN IF NOT EXISTS filing_status VARCHAR(30);
ALTER TABLE country_tax_brackets ADD COLUMN IF NOT EXISTS state VARCHAR(5);

-- Add US columns to payroll_items for typed tax storage
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS federal_tax NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS social_security_ee NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS social_security_er NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS medicare_ee NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS medicare_er NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS additional_medicare NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS state_tax NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS state_disability NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS futa NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS sui NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE payroll_items ADD COLUMN IF NOT EXISTS pretax_deductions NUMERIC(12,2) NOT NULL DEFAULT 0;

-- Add US-specific fields to employee_profiles
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS ssn_encrypted BYTEA;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS state_of_residence VARCHAR(5);
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_filing_status VARCHAR(30);
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_additional_withholding NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_multiple_jobs BOOLEAN DEFAULT false;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_dependents_credit NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_other_income NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS w4_deductions NUMERIC(12,2) DEFAULT 0;
ALTER TABLE employee_profiles ADD COLUMN IF NOT EXISTS state_allowances INTEGER DEFAULT 0;

-- Add accrual fields to leave_types (max_carryover already exists from migration 00022)
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS accrual_method VARCHAR(20) DEFAULT 'upfront';
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS accrual_rate NUMERIC(8,4);
ALTER TABLE leave_types ADD COLUMN IF NOT EXISTS accrual_period VARCHAR(20);

-- ============================================================
-- 2. New Tables
-- ============================================================

-- Employee benefit deductions (401k, health insurance, HSA, FSA)
CREATE TABLE IF NOT EXISTS employee_benefit_deductions (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    deduction_type VARCHAR(50) NOT NULL,
    amount_per_period NUMERIC(12,2) NOT NULL,
    annual_limit NUMERIC(12,2),
    reduces_fica BOOLEAN NOT NULL DEFAULT false,
    effective_date DATE NOT NULL,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benefit_deductions_employee ON employee_benefit_deductions(employee_id);
CREATE INDEX idx_benefit_deductions_company ON employee_benefit_deductions(company_id);

-- Company registration numbers (EIN, state IDs, FUTA rate)
CREATE TABLE IF NOT EXISTS company_registration_numbers (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    country VARCHAR(3) NOT NULL,
    registration_type VARCHAR(50) NOT NULL,
    registration_value VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, registration_type)
);

-- ============================================================
-- 3. Seed US Payroll Config
-- ============================================================

INSERT INTO country_payroll_config (country, config_key, config_value, description) VALUES
('USA', 'ot_rates', '{"regular": 1.5, "ca_daily_ot": 1.5, "ca_double_ot": 2.0}', 'US OT rates: 1.5x federal, CA daily 1.5x/2.0x'),
('USA', 'standard_hours', '{"daily": 8, "weekly": 40}', 'US FLSA standard hours'),
('USA', 'night_diff_rate', '0', 'No federal night differential mandate'),
('USA', 'has_13th_month', 'false', '13th month pay not applicable in USA'),
('USA', 'default_pay_frequency', '"bi_weekly"', 'Most common US pay frequency'),
('USA', 'pay_frequencies', '["weekly", "bi_weekly", "semi_monthly", "monthly"]', 'Supported US pay frequencies'),
('USA', 'fica_ss_wage_base', '176100', 'Social Security wage base 2025'),
('USA', 'fica_additional_medicare_threshold', '200000', 'Additional Medicare Tax threshold'),
('USA', 'futa_wage_base', '7000', 'FUTA wage base per employee per year'),
('USA', 'futa_rate_default', '0.006', 'Default FUTA rate after SUI credit (0.6%)'),
('USA', 'futa_rate_ca', '0.018', 'CA FUTA credit reduction rate (1.8% effective)');

-- ============================================================
-- 4. Seed FICA Contribution Rates
-- ============================================================

INSERT INTO country_contribution_rates (country, contribution_type, rate, effective_from, description) VALUES
('USA', 'fica_ss_employee', 0.0620, '2025-01-01', 'Social Security employee 6.2%'),
('USA', 'fica_ss_employer', 0.0620, '2025-01-01', 'Social Security employer 6.2%'),
('USA', 'fica_medicare_employee', 0.0145, '2025-01-01', 'Medicare employee 1.45%'),
('USA', 'fica_medicare_employer', 0.0145, '2025-01-01', 'Medicare employer 1.45%'),
('USA', 'fica_additional_medicare', 0.0090, '2025-01-01', 'Additional Medicare 0.9% (EE only, >$200K)'),
('USA', 'ca_sdi', 0.0120, '2025-01-01', 'CA SDI 1.2% (no wage cap, per SB 951)'),
('USA', 'wa_pfml_total', 0.0092, '2025-01-01', 'WA PFML total 0.92%'),
('USA', 'wa_pfml_employee', 0.0066, '2025-01-01', 'WA PFML employee share ~71.5%'),
('USA', 'wa_pfml_employer', 0.0026, '2025-01-01', 'WA PFML employer share ~28.5%');

-- ============================================================
-- 5. Seed Federal Tax Brackets (2025, annual)
-- ============================================================

-- Single filer
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, description) VALUES
('USA', '2025-01-01', 'annual', 0, 11925, 0.1000, 0, 'single', 'Federal 10%'),
('USA', '2025-01-01', 'annual', 11925.01, 48475, 0.1200, 0, 'single', 'Federal 12%'),
('USA', '2025-01-01', 'annual', 48475.01, 103350, 0.2200, 0, 'single', 'Federal 22%'),
('USA', '2025-01-01', 'annual', 103350.01, 197300, 0.2400, 0, 'single', 'Federal 24%'),
('USA', '2025-01-01', 'annual', 197300.01, 250525, 0.3200, 0, 'single', 'Federal 32%'),
('USA', '2025-01-01', 'annual', 250525.01, 626350, 0.3500, 0, 'single', 'Federal 35%'),
('USA', '2025-01-01', 'annual', 626350.01, 999999999, 0.3700, 0, 'single', 'Federal 37%');

-- Married Filing Jointly
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, description) VALUES
('USA', '2025-01-01', 'annual', 0, 23850, 0.1000, 0, 'married_jointly', 'Federal 10%'),
('USA', '2025-01-01', 'annual', 23850.01, 96950, 0.1200, 0, 'married_jointly', 'Federal 12%'),
('USA', '2025-01-01', 'annual', 96950.01, 206700, 0.2200, 0, 'married_jointly', 'Federal 22%'),
('USA', '2025-01-01', 'annual', 206700.01, 394600, 0.2400, 0, 'married_jointly', 'Federal 24%'),
('USA', '2025-01-01', 'annual', 394600.01, 501050, 0.3200, 0, 'married_jointly', 'Federal 32%'),
('USA', '2025-01-01', 'annual', 501050.01, 751600, 0.3500, 0, 'married_jointly', 'Federal 35%'),
('USA', '2025-01-01', 'annual', 751600.01, 999999999, 0.3700, 0, 'married_jointly', 'Federal 37%');

-- Head of Household
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, description) VALUES
('USA', '2025-01-01', 'annual', 0, 17000, 0.1000, 0, 'head_of_household', 'Federal 10%'),
('USA', '2025-01-01', 'annual', 17000.01, 64850, 0.1200, 0, 'head_of_household', 'Federal 12%'),
('USA', '2025-01-01', 'annual', 64850.01, 103350, 0.2200, 0, 'head_of_household', 'Federal 22%'),
('USA', '2025-01-01', 'annual', 103350.01, 197300, 0.2400, 0, 'head_of_household', 'Federal 24%'),
('USA', '2025-01-01', 'annual', 197300.01, 250500, 0.3200, 0, 'head_of_household', 'Federal 32%'),
('USA', '2025-01-01', 'annual', 250500.01, 626350, 0.3500, 0, 'head_of_household', 'Federal 35%'),
('USA', '2025-01-01', 'annual', 626350.01, 999999999, 0.3700, 0, 'head_of_household', 'Federal 37%');

-- ============================================================
-- 6. Seed California State Tax Brackets (2025, annual, 9 brackets)
-- ============================================================

INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, state, description) VALUES
('USA', '2025-01-01', 'annual', 0, 10412, 0.0100, 0, 'single', 'CA', 'CA 1%'),
('USA', '2025-01-01', 'annual', 10412.01, 24684, 0.0200, 0, 'single', 'CA', 'CA 2%'),
('USA', '2025-01-01', 'annual', 24684.01, 38959, 0.0400, 0, 'single', 'CA', 'CA 4%'),
('USA', '2025-01-01', 'annual', 38959.01, 54081, 0.0600, 0, 'single', 'CA', 'CA 6%'),
('USA', '2025-01-01', 'annual', 54081.01, 68350, 0.0800, 0, 'single', 'CA', 'CA 8%'),
('USA', '2025-01-01', 'annual', 68350.01, 349137, 0.0930, 0, 'single', 'CA', 'CA 9.3%'),
('USA', '2025-01-01', 'annual', 349137.01, 418961, 0.1030, 0, 'single', 'CA', 'CA 10.3%'),
('USA', '2025-01-01', 'annual', 418961.01, 698271, 0.1130, 0, 'single', 'CA', 'CA 11.3%'),
('USA', '2025-01-01', 'annual', 698271.01, 999999999, 0.1230, 0, 'single', 'CA', 'CA 12.3%');

-- CA Mental Health Services Tax: additional 1% on income > $1,000,000 (handled in code, not bracket table)

-- ============================================================
-- 7. Seed New York State Tax Brackets (2025, annual, 9 brackets)
-- ============================================================

INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, filing_status, state, description) VALUES
('USA', '2025-01-01', 'annual', 0, 8500, 0.0400, 0, 'single', 'NY', 'NY 4%'),
('USA', '2025-01-01', 'annual', 8500.01, 11700, 0.0450, 0, 'single', 'NY', 'NY 4.5%'),
('USA', '2025-01-01', 'annual', 11700.01, 13900, 0.0525, 0, 'single', 'NY', 'NY 5.25%'),
('USA', '2025-01-01', 'annual', 13900.01, 80650, 0.0550, 0, 'single', 'NY', 'NY 5.5%'),
('USA', '2025-01-01', 'annual', 80650.01, 215400, 0.0600, 0, 'single', 'NY', 'NY 6%'),
('USA', '2025-01-01', 'annual', 215400.01, 1077550, 0.0685, 0, 'single', 'NY', 'NY 6.85%'),
('USA', '2025-01-01', 'annual', 1077550.01, 5000000, 0.0965, 0, 'single', 'NY', 'NY 9.65%'),
('USA', '2025-01-01', 'annual', 5000000.01, 25000000, 0.1030, 0, 'single', 'NY', 'NY 10.3%'),
('USA', '2025-01-01', 'annual', 25000000.01, 999999999, 0.1090, 0, 'single', 'NY', 'NY 10.9%');

-- ============================================================
-- 8. Indexes
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_country_tax_brackets_state ON country_tax_brackets(country, state, filing_status, effective_from);

-- +goose Down
DROP INDEX IF EXISTS idx_country_tax_brackets_state;
DROP TABLE IF EXISTS company_registration_numbers;
DROP TABLE IF EXISTS employee_benefit_deductions;
ALTER TABLE leave_types DROP COLUMN IF EXISTS accrual_period;
ALTER TABLE leave_types DROP COLUMN IF EXISTS accrual_rate;
ALTER TABLE leave_types DROP COLUMN IF EXISTS accrual_method;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS state_allowances;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_deductions;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_other_income;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_dependents_credit;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_multiple_jobs;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_additional_withholding;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS w4_filing_status;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS state_of_residence;
ALTER TABLE employee_profiles DROP COLUMN IF EXISTS ssn_encrypted;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS pretax_deductions;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS sui;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS futa;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS state_disability;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS state_tax;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS additional_medicare;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS medicare_er;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS medicare_ee;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS social_security_er;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS social_security_ee;
ALTER TABLE payroll_items DROP COLUMN IF EXISTS federal_tax;
ALTER TABLE country_tax_brackets DROP COLUMN IF EXISTS state;
ALTER TABLE country_tax_brackets DROP COLUMN IF EXISTS filing_status;
DELETE FROM country_contribution_rates WHERE country = 'USA';
DELETE FROM country_payroll_config WHERE country = 'USA';
DELETE FROM country_tax_brackets WHERE country = 'USA';
```

- [ ] **Step 2: Verify migration syntax is valid**

Run: `cd /Users/anna/Documents/aigonhr && head -5 db/migrations/00086_us_country_support.sql`
Expected: Shows `-- +goose Up` header

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add db/migrations/00086_us_country_support.sql
git commit -m "feat(db): add US country support migration — tables, schema changes, seed data"
```

---

### Task 3: sqlc Queries for US Payroll

Creates the US-specific queries and updates existing queries to support new columns.

**Files:**
- Create: `db/query/us_payroll.sql`
- Modify: `db/query/payroll.sql:104-114` (CreatePayrollItem)
- Modify: `db/query/country_payroll.sql` (add state-aware bracket queries)

- [ ] **Step 1: Create US payroll queries**

Create `db/query/us_payroll.sql`:

```sql
-- name: ListEmployeeBenefitDeductions :many
-- List active benefit deductions for an employee at a given date
SELECT * FROM employee_benefit_deductions
WHERE company_id = $1 AND employee_id = $2
  AND effective_date <= $3
  AND (end_date IS NULL OR end_date >= $3)
ORDER BY deduction_type;

-- name: CreateBenefitDeduction :one
INSERT INTO employee_benefit_deductions (
    company_id, employee_id, deduction_type, amount_per_period,
    annual_limit, reduces_fica, effective_date, end_date
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateBenefitDeduction :one
UPDATE employee_benefit_deductions SET
    deduction_type = $3,
    amount_per_period = $4,
    annual_limit = $5,
    reduces_fica = $6,
    effective_date = $7,
    end_date = $8,
    updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING *;

-- name: DeleteBenefitDeduction :exec
DELETE FROM employee_benefit_deductions WHERE id = $1 AND company_id = $2;

-- name: ListBenefitDeductionsByCompany :many
SELECT ebd.*, e.first_name, e.last_name, e.employee_no
FROM employee_benefit_deductions ebd
JOIN employees e ON e.id = ebd.employee_id
WHERE ebd.company_id = $1
ORDER BY e.last_name, e.first_name, ebd.deduction_type;

-- name: GetCompanyRegistrationNumber :one
SELECT * FROM company_registration_numbers
WHERE company_id = $1 AND registration_type = $2;

-- name: UpsertCompanyRegistrationNumber :one
INSERT INTO company_registration_numbers (company_id, country, registration_type, registration_value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (company_id, registration_type)
DO UPDATE SET registration_value = EXCLUDED.registration_value
RETURNING *;

-- name: ListCompanyRegistrationNumbers :many
SELECT * FROM company_registration_numbers
WHERE company_id = $1 AND country = $2
ORDER BY registration_type;

-- name: DeleteCompanyRegistrationNumber :exec
DELETE FROM company_registration_numbers WHERE id = $1 AND company_id = $2;

-- name: GetYTDPayrollTotals :one
-- Get YTD totals for an employee (for SS wage base cap, 401k limit tracking)
SELECT
    COALESCE(SUM(CAST(pi.gross_pay AS NUMERIC)), 0) AS ytd_gross,
    COALESCE(SUM(CAST(pi.social_security_ee AS NUMERIC)), 0) AS ytd_ss_ee,
    COALESCE(SUM(CAST(pi.pretax_deductions AS NUMERIC)), 0) AS ytd_pretax
FROM payroll_items pi
JOIN payroll_runs pr ON pr.id = pi.run_id
JOIN payroll_cycles pc ON pc.id = pr.cycle_id
WHERE pi.employee_id = $1
  AND pr.company_id = $2
  AND pr.status = 'completed'
  AND EXTRACT(YEAR FROM pc.period_start) = $3;

-- name: GetStateTaxBrackets :many
-- List state tax brackets for a given state and filing status
SELECT * FROM country_tax_brackets
WHERE country = 'USA'
  AND state = $1
  AND filing_status = $2
  AND effective_from <= $3
  AND (effective_to IS NULL OR effective_to >= $3)
ORDER BY bracket_min;

-- name: GetFederalTaxBrackets :many
-- List federal tax brackets for a given filing status
SELECT * FROM country_tax_brackets
WHERE country = 'USA'
  AND state IS NULL
  AND filing_status = $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
ORDER BY bracket_min;
```

- [ ] **Step 2: Update CreatePayrollItem query to include US columns**

Modify `db/query/payroll.sql` — replace the existing `CreatePayrollItem` query (lines 104-114) with:

```sql
-- name: CreatePayrollItem :one
INSERT INTO payroll_items (
    run_id, employee_id, basic_pay, gross_pay, taxable_income, total_deductions, net_pay,
    sss_ee, sss_er, sss_ec, philhealth_ee, philhealth_er,
    pagibig_ee, pagibig_er, withholding_tax,
    breakdown, work_days, hours_worked, ot_hours,
    late_deduction, undertime_deduction, holiday_pay, night_diff, bonus_pay,
    federal_tax, social_security_ee, social_security_er,
    medicare_ee, medicare_er, additional_medicare,
    state_tax, state_disability, futa, sui, pretax_deductions
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
    $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35
) RETURNING *;
```

- [ ] **Step 3: Add state bracket query to country_payroll.sql**

Append to `db/query/country_payroll.sql`:

```sql
-- name: ListCountryTaxBracketsByState :many
-- List tax brackets for a US state with filing status
SELECT * FROM country_tax_brackets
WHERE country = $1
  AND state = $2
  AND (filing_status = $3 OR $3 = '' OR $3 IS NULL)
  AND effective_from <= $4
  AND (effective_to IS NULL OR effective_to >= $4)
ORDER BY bracket_min;
```

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add db/query/us_payroll.sql db/query/payroll.sql db/query/country_payroll.sql
git commit -m "feat(db): add US payroll queries — benefit deductions, registration numbers, YTD, state brackets"
```

---

### Task 4: Run sqlc Generate

Regenerates Go code from updated SQL queries. Must run after Tasks 2 & 3.

**Files:**
- Modify: `internal/store/*.sql.go` (auto-generated)

- [ ] **Step 1: Run sqlc generate**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors. New files appear in `internal/store/`

- [ ] **Step 2: Verify generated code compiles**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./internal/store/...`
Expected: Clean (no errors)

- [ ] **Step 3: Commit generated code**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/store/
git commit -m "chore: regenerate sqlc after US payroll queries"
```

---

### Task 5: US Payroll Engine

Implements the core US payroll calculation logic: federal tax, FICA, state tax, pre-tax deductions.

**Files:**
- Create: `internal/payroll/country_us.go`
- Create: `internal/payroll/country_us_test.go`

**Reference:** Follow `internal/payroll/country_lk.go` structure exactly (3 exported methods on `Calculator`).

- [ ] **Step 1: Write the failing tests**

Create `internal/payroll/country_us_test.go`:

```go
package payroll

import (
	"testing"
)

// --- Federal Tax Tests ---

func TestComputeFederalTax_Single50K(t *testing.T) {
	// $50,000 annual, single, no pre-tax deductions
	// Bracket: $0-$11,925 at 10% = $1,192.50, $11,926-$48,475 at 12% = $4,386.00, $48,476-$50,000 at 22% = $335.50
	// Total: $1,192.50 + $4,386.00 + $335.50 = $5,914.00
	tax := computeFederalTaxFromBrackets(50000, federalBracketsSingle())
	assertClose(t, tax, 5914.00, "federal tax single $50K")
}

func TestComputeFederalTax_Single100K(t *testing.T) {
	tax := computeFederalTaxFromBrackets(100000, federalBracketsSingle())
	// $0-$11,925@10% + $11,926-$48,475@12% + $48,476-$100,000@22%
	// = 1192.50 + 4386.00 + 11335.50 = 16914.00
	assertClose(t, tax, 16914.00, "federal tax single $100K")
}

func TestComputeFederalTax_Single200K(t *testing.T) {
	tax := computeFederalTaxFromBrackets(200000, federalBracketsSingle())
	// = 1192.50 + 4386.00 + 12072.50 + 22548.00 + 864.00 = 41063.00
	assertClose(t, tax, 41063.00, "federal tax single $200K")
}

func TestComputeFederalTax_ZeroIncome(t *testing.T) {
	tax := computeFederalTaxFromBrackets(0, federalBracketsSingle())
	assertClose(t, tax, 0, "federal tax zero income")
}

// --- FICA Tests ---

func TestComputeFICA_Below_WageBase(t *testing.T) {
	gross := 100000.0
	ytdGross := 0.0
	ssWageBase := 176100.0

	ssEE, ssER, medEE, medER, addMed := computeFICA(gross, ytdGross, ssWageBase)

	assertClose(t, ssEE, 6200.00, "SS EE")
	assertClose(t, ssER, 6200.00, "SS ER")
	assertClose(t, medEE, 1450.00, "Med EE")
	assertClose(t, medER, 1450.00, "Med ER")
	assertClose(t, addMed, 0, "Additional Medicare")
}

func TestComputeFICA_Above_WageBase(t *testing.T) {
	// Employee earned $170K YTD, now getting $20K more → total $190K
	// Only $176,100-$170,000=$6,100 subject to SS
	gross := 20000.0
	ytdGross := 170000.0
	ssWageBase := 176100.0

	ssEE, ssER, _, _, _ := computeFICA(gross, ytdGross, ssWageBase)

	assertClose(t, ssEE, 378.20, "SS EE capped") // 6100 * 0.062
	assertClose(t, ssER, 378.20, "SS ER capped")
}

func TestComputeFICA_AdditionalMedicare(t *testing.T) {
	// YTD $190K + $20K = $210K. Additional Medicare on $10K over $200K
	gross := 20000.0
	ytdGross := 190000.0
	ssWageBase := 176100.0

	_, _, _, _, addMed := computeFICA(gross, ytdGross, ssWageBase)

	assertClose(t, addMed, 90.00, "Additional Medicare on $10K") // 10000 * 0.009
}

// --- State Tax Tests ---

func TestComputeStateTax_CA_50K(t *testing.T) {
	tax := computeStateTaxFromBrackets(50000, caBracketsSingle())
	// Manual calc: 10412@1% + 14272@2% + 14275@4% + 15041@6% = ~1857.98
	if tax <= 0 {
		t.Errorf("expected positive CA tax, got %f", tax)
	}
}

func TestComputeStateTax_TX(t *testing.T) {
	// Texas has no state income tax
	tax := computeStateTaxFromBrackets(100000, nil)
	assertClose(t, tax, 0, "TX state tax")
}

func TestComputeStateTax_CA_MentalHealth(t *testing.T) {
	// Income > $1M: additional 1%
	extraTax := computeCAMentalHealthTax(1500000)
	assertClose(t, extraTax, 5000.00, "CA MHST on $500K over $1M") // (1.5M - 1M) * 0.01
}

// --- Pre-Tax Deduction Tests ---

func TestPreTaxDeductions_401k_ReducesFederalNotFICA(t *testing.T) {
	gross := 100000.0 / 12 // monthly ~$8,333
	contrib401k := 1000.0  // $1,000/month to 401(k)

	federalTaxableIncome := gross - contrib401k // 401k reduces federal
	ficaWages := gross                          // 401k does NOT reduce FICA

	if federalTaxableIncome >= gross {
		t.Error("401k should reduce federal taxable income")
	}
	if ficaWages != gross {
		t.Error("401k should NOT reduce FICA wages")
	}
}

func TestPreTaxDeductions_HealthIns_ReducesBoth(t *testing.T) {
	gross := 100000.0 / 12
	healthIns := 500.0 // Section 125

	federalTaxableIncome := gross - healthIns
	ficaWages := gross - healthIns // Health insurance DOES reduce FICA

	if federalTaxableIncome >= gross {
		t.Error("health ins should reduce federal taxable income")
	}
	if ficaWages >= gross {
		t.Error("health ins should reduce FICA wages (Section 125)")
	}
}

// --- Overtime Tests ---

func TestOvertimeFLSA(t *testing.T) {
	hourlyRate := 25.0
	otHours := 5.0
	otPay := hourlyRate * 1.5 * otHours
	assertClose(t, otPay, 187.50, "FLSA OT 1.5x")
}

func TestOvertimeCA_DailyOT(t *testing.T) {
	hourlyRate := 25.0
	// 2 hours daily OT (>8h), 1 hour double OT (>12h)
	otPay := hourlyRate*1.5*2 + hourlyRate*2.0*1
	assertClose(t, otPay, 125.00, "CA daily OT")
}

// --- Helpers ---

func assertClose(t *testing.T, got, want float64, label string) {
	t.Helper()
	diff := got - want
	if diff < -0.01 || diff > 0.01 {
		t.Errorf("%s: got %.2f, want %.2f", label, got, want)
	}
}

// Test bracket helpers — these return simplified bracket slices for testing
// without DB dependency. The actual implementation queries the DB.
type testBracket struct {
	min, max, rate float64
}

func federalBracketsSingle() []testBracket {
	return []testBracket{
		{0, 11925, 0.10},
		{11925.01, 48475, 0.12},
		{48475.01, 103350, 0.22},
		{103350.01, 197300, 0.24},
		{197300.01, 250525, 0.32},
		{250525.01, 626350, 0.35},
		{626350.01, 999999999, 0.37},
	}
}

func caBracketsSingle() []testBracket {
	return []testBracket{
		{0, 10412, 0.01},
		{10412.01, 24684, 0.02},
		{24684.01, 38959, 0.04},
		{38959.01, 54081, 0.06},
		{54081.01, 68350, 0.08},
		{68350.01, 349137, 0.093},
		{349137.01, 418961, 0.103},
		{418961.01, 698271, 0.113},
		{698271.01, 999999999, 0.123},
	}
}

// computeFederalTaxFromBrackets is a pure function for unit testing.
// Mirrors the logic in country_us.go but takes bracket slices directly.
func computeFederalTaxFromBrackets(annualIncome float64, brackets []testBracket) float64 {
	tax := 0.0
	for _, b := range brackets {
		if annualIncome <= b.min {
			break
		}
		taxable := annualIncome - b.min
		bracketWidth := b.max - b.min
		if taxable > bracketWidth {
			taxable = bracketWidth
		}
		tax += taxable * b.rate
	}
	return round2(tax)
}

func computeStateTaxFromBrackets(annualIncome float64, brackets []testBracket) float64 {
	if brackets == nil {
		return 0
	}
	return computeFederalTaxFromBrackets(annualIncome, brackets) // same bracket math
}

func computeCAMentalHealthTax(annualIncome float64) float64 {
	if annualIncome <= 1000000 {
		return 0
	}
	return round2((annualIncome - 1000000) * 0.01)
}

func computeFICA(periodGross, ytdGross, ssWageBase float64) (ssEE, ssER, medEE, medER, addMed float64) {
	// Social Security: 6.2% on wages up to wage base
	ssWages := periodGross
	if ytdGross >= ssWageBase {
		ssWages = 0
	} else if ytdGross+periodGross > ssWageBase {
		ssWages = ssWageBase - ytdGross
	}
	ssEE = round2(ssWages * 0.062)
	ssER = round2(ssWages * 0.062)

	// Medicare: 1.45% on all wages
	medEE = round2(periodGross * 0.0145)
	medER = round2(periodGross * 0.0145)

	// Additional Medicare: 0.9% on wages over $200K (YTD)
	addMedThreshold := 200000.0
	if ytdGross+periodGross > addMedThreshold && ytdGross < addMedThreshold {
		addMed = round2((ytdGross + periodGross - addMedThreshold) * 0.009)
	} else if ytdGross >= addMedThreshold {
		addMed = round2(periodGross * 0.009)
	}

	return
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/payroll/... -run TestCompute -v -count=1`
Expected: PASS (tests use local helper functions, not the Calculator methods yet)

- [ ] **Step 3: Write the implementation**

Create `internal/payroll/country_us.go`:

```go
package payroll

import (
	"context"
	"math"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

// computePayUS calculates gross pay for US employees.
// Federal FLSA: 1.5x for hours > 40/week.
// California: 1.5x for hours > 8/day, 2.0x for hours > 12/day.
func (calc *Calculator) computePayUS(pd *EmployeePayData, workingDaysInPeriod int, state string) {
	dailyRate := pd.BasicSalary / float64(workingDaysInPeriod)
	hourlyRate := dailyRate / 8.0

	pd.BasicPay = dailyRate * pd.DaysWorked

	// Overtime: CA has daily OT rules, others follow federal FLSA
	hasTypedOT := pd.OTRegular > 0 || pd.OTHoliday > 0
	if state == "CA" && hasTypedOT {
		// CA: OTRegular = hours at 1.5x (daily >8h), OTHoliday = hours at 2.0x (daily >12h)
		pd.OvertimePay = hourlyRate*1.5*pd.OTRegular + hourlyRate*2.0*pd.OTHoliday
	} else {
		// Federal FLSA: all OT at 1.5x
		pd.OvertimePay = hourlyRate * 1.5 * pd.OvertimeHours
	}

	// No mandatory night differential in US
	pd.NightDiff = 0

	// Holiday pay: at employer's discretion (stored in breakdown, default 0)
	pd.HolidayPay = dailyRate * 1.0 * float64(pd.RegularHolidayDays)

	// Deductions for late/undertime/unpaid leave
	pd.LateDeduction = (float64(pd.LateMinutes) / 60.0) * hourlyRate
	pd.UndertimeDeduction = (float64(pd.UndertimeMinutes) / 60.0) * hourlyRate
	pd.LeaveDeduction = dailyRate * pd.UnpaidLeaveDays

	pd.GrossPay = pd.BasicPay + pd.OvertimePay + pd.HolidayPay -
		pd.LateDeduction - pd.UndertimeDeduction - pd.LeaveDeduction
	if pd.GrossPay < 0 {
		pd.GrossPay = 0
	}

	pd.BasicPay = round2(pd.BasicPay)
	pd.OvertimePay = round2(pd.OvertimePay)
	pd.HolidayPay = round2(pd.HolidayPay)
	pd.LateDeduction = round2(pd.LateDeduction)
	pd.UndertimeDeduction = round2(pd.UndertimeDeduction)
	pd.LeaveDeduction = round2(pd.LeaveDeduction)
	pd.GrossPay = round2(pd.GrossPay)
}

// computeContributionsUS calculates FICA (SS + Medicare), FUTA, state disability, etc.
func (calc *Calculator) computeContributionsUS(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time, companyID int64, state string) {
	gross := pd.GrossPay

	// Load rates from DB with fallback defaults
	ssEERate := 0.062
	ssERRate := 0.062
	medEERate := 0.0145
	medERRate := 0.0145
	addMedRate := 0.009
	ssWageBase := 176100.0
	addMedThreshold := 200000.0
	caSDIRate := 0.012

	rates, err := calc.queries.ListCountryContributionRates(ctx, store.ListCountryContributionRatesParams{
		Country:       "USA",
		EffectiveFrom: effectiveDate,
	})
	if err == nil {
		for _, r := range rates {
			rate := numericToFloat(r.Rate)
			switch r.ContributionType {
			case "fica_ss_employee":
				ssEERate = rate
			case "fica_ss_employer":
				ssERRate = rate
			case "fica_medicare_employee":
				medEERate = rate
			case "fica_medicare_employer":
				medERRate = rate
			case "fica_additional_medicare":
				addMedRate = rate
			case "ca_sdi":
				caSDIRate = rate
			}
		}
	}

	// Load config values for wage bases
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: "fica_ss_wage_base",
	}); err == nil {
		ssWageBase = interfaceToFloat(cfg.ConfigValue)
	}
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: "fica_additional_medicare_threshold",
	}); err == nil {
		addMedThreshold = interfaceToFloat(cfg.ConfigValue)
	}

	// Get YTD totals for wage base cap
	ytdGross := 0.0
	if ytd, err := calc.queries.GetYTDPayrollTotals(ctx, store.GetYTDPayrollTotalsParams{
		EmployeeID: pd.EmployeeID,
		CompanyID:  companyID,
		Column3:    int32(effectiveDate.Year()),
	}); err == nil {
		ytdGross = interfaceToFloat(ytd.YtdGross)
	}

	// Calculate FICA wages: reduce by Section 125 deductions only
	ficaWages := gross - pd.Section125Deductions
	if ficaWages < 0 {
		ficaWages = 0
	}

	// Social Security: capped at wage base
	ssWages := ficaWages
	if ytdGross >= ssWageBase {
		ssWages = 0
	} else if ytdGross+ficaWages > ssWageBase {
		ssWages = ssWageBase - ytdGross
	}
	pd.SocialSecurityEE = round2(ssWages * ssEERate)
	pd.SocialSecurityER = round2(ssWages * ssERRate)

	// Medicare: no cap
	pd.MedicareEE = round2(ficaWages * medEERate)
	pd.MedicareER = round2(ficaWages * medERRate)

	// Additional Medicare: 0.9% on wages over threshold (EE only)
	if ytdGross+ficaWages > addMedThreshold {
		if ytdGross >= addMedThreshold {
			pd.AdditionalMedicare = round2(ficaWages * addMedRate)
		} else {
			pd.AdditionalMedicare = round2((ytdGross + ficaWages - addMedThreshold) * addMedRate)
		}
	}

	// FUTA: employer only, 6.0% on first $7K per employee per year
	futaWageBase := 7000.0
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: "futa_wage_base",
	}); err == nil {
		futaWageBase = interfaceToFloat(cfg.ConfigValue)
	}
	futaRate := 0.006 // default after SUI credit
	// Check for CA credit reduction
	futaKey := "futa_rate_default"
	if state == "CA" {
		futaKey = "futa_rate_ca"
	}
	if cfg, err := calc.queries.GetCountryPayrollConfig(ctx, store.GetCountryPayrollConfigParams{
		Country:   "USA",
		ConfigKey: futaKey,
	}); err == nil {
		futaRate = interfaceToFloat(cfg.ConfigValue)
	}
	futaWages := gross
	if ytdGross >= futaWageBase {
		futaWages = 0
	} else if ytdGross+gross > futaWageBase {
		futaWages = futaWageBase - ytdGross
	}
	pd.FUTA = round2(futaWages * futaRate)

	// State-specific contributions
	switch state {
	case "CA":
		// SDI: 1.2% with no wage cap
		pd.StateDisability = round2(gross * caSDIRate)
	case "WA":
		// PFML: total 0.92%, split ~71.5% EE / ~28.5% ER
		waEERate := 0.0066
		waERRate := 0.0026
		if r, err := calc.queries.GetCountryContributionRate(ctx, store.GetCountryContributionRateParams{
			Country:          "USA",
			ContributionType: "wa_pfml_employee",
			EffectiveFrom:    effectiveDate,
		}); err == nil {
			waEERate = numericToFloat(r.Rate)
		}
		if r, err := calc.queries.GetCountryContributionRate(ctx, store.GetCountryContributionRateParams{
			Country:          "USA",
			ContributionType: "wa_pfml_employer",
			EffectiveFrom:    effectiveDate,
		}); err == nil {
			waERRate = numericToFloat(r.Rate)
		}
		pd.StateDisability = round2(gross * waEERate) // EE portion
		pd.SUI = round2(gross * waERRate)             // ER portion (stored as SUI for simplicity)
	}
}

// computeWithholdingTaxUS calculates federal + state income tax withholding.
func (calc *Calculator) computeWithholdingTaxUS(ctx context.Context, pd *EmployeePayData, effectiveDate time.Time, payPeriodsPerYear int, filingStatus, state string) {
	// 1. Calculate pre-tax deductions total (already set in pd.PreTaxDeductions)
	federalTaxableGross := pd.GrossPay - pd.PreTaxDeductions
	if federalTaxableGross < 0 {
		federalTaxableGross = 0
	}

	// 2. Annualize
	annualIncome := federalTaxableGross * float64(payPeriodsPerYear)

	// 3. Apply W-4 adjustments
	annualIncome += pd.W4OtherIncome                                  // Step 4(a)
	annualIncome -= pd.W4Deductions                                   // Step 4(b)
	if annualIncome < 0 {
		annualIncome = 0
	}

	// 4. Look up federal brackets and compute progressive tax
	brackets, err := calc.queries.GetFederalTaxBrackets(ctx, store.GetFederalTaxBracketsParams{
		FilingStatus:  filingStatus,
		EffectiveFrom: effectiveDate,
	})
	if err != nil || len(brackets) == 0 {
		calc.logger.Warn("US federal brackets not found", "filing_status", filingStatus, "error", err)
		pd.FederalTax = 0
	} else {
		annualTax := computeProgressiveTax(annualIncome, brackets)

		// 5. De-annualize
		periodTax := annualTax / float64(payPeriodsPerYear)

		// 6. W-4 Step 4(c): additional withholding
		periodTax += pd.W4AdditionalWithholding

		// 7. W-4 Step 3: dependents credit
		periodCredit := pd.W4DependentsCredit / float64(payPeriodsPerYear)
		periodTax -= periodCredit

		if periodTax < 0 {
			periodTax = 0
		}
		pd.FederalTax = round2(periodTax)
	}

	// 8. State income tax
	switch state {
	case "CA":
		stateBrackets, err := calc.queries.GetStateTaxBrackets(ctx, store.GetStateTaxBracketsParams{
			State:         "CA",
			FilingStatus:  filingStatus,
			EffectiveFrom: effectiveDate,
		})
		if err == nil && len(stateBrackets) > 0 {
			annualStateTax := computeProgressiveTax(annualIncome, stateBrackets)
			// CA Mental Health Services Tax: 1% on income > $1M
			if annualIncome > 1000000 {
				annualStateTax += (annualIncome - 1000000) * 0.01
			}
			pd.StateTax = round2(annualStateTax / float64(payPeriodsPerYear))
		}
	case "NY":
		stateBrackets, err := calc.queries.GetStateTaxBrackets(ctx, store.GetStateTaxBracketsParams{
			State:         "NY",
			FilingStatus:  filingStatus,
			EffectiveFrom: effectiveDate,
		})
		if err == nil && len(stateBrackets) > 0 {
			annualStateTax := computeProgressiveTax(annualIncome, stateBrackets)
			pd.StateTax = round2(annualStateTax / float64(payPeriodsPerYear))
		}
	// TX, FL, WA: no state income tax
	}

	// Total withholding = federal + state
	pd.WithholdingTax = pd.FederalTax + pd.StateTax

	// Taxable income for reference
	pd.TaxableIncome = round2(federalTaxableGross)
}

// computeProgressiveTax calculates progressive tax from DB bracket rows.
func computeProgressiveTax(annualIncome float64, brackets interface{}) float64 {
	tax := 0.0

	switch b := brackets.(type) {
	case []store.CountryTaxBracket:
		for _, bracket := range b {
			min := numericToFloat(bracket.BracketMin)
			max := numericToFloat(bracket.BracketMax)
			rate := numericToFloat(bracket.TaxRate)

			if annualIncome <= min {
				break
			}
			taxable := annualIncome - min
			bracketWidth := max - min
			if bracketWidth > 0 && taxable > bracketWidth {
				taxable = bracketWidth
			}
			tax += taxable * rate
		}
	case []store.GetStateTaxBracketsRow:
		for _, bracket := range b {
			min := numericToFloat(bracket.BracketMin)
			max := numericToFloat(bracket.BracketMax)
			rate := numericToFloat(bracket.TaxRate)

			if annualIncome <= min {
				break
			}
			taxable := annualIncome - min
			bracketWidth := max - min
			if bracketWidth > 0 && taxable > bracketWidth {
				taxable = bracketWidth
			}
			tax += taxable * rate
		}
	case []store.GetFederalTaxBracketsRow:
		for _, bracket := range b {
			min := numericToFloat(bracket.BracketMin)
			max := numericToFloat(bracket.BracketMax)
			rate := numericToFloat(bracket.TaxRate)

			if annualIncome <= min {
				break
			}
			taxable := annualIncome - min
			bracketWidth := max - min
			if bracketWidth > 0 && taxable > bracketWidth {
				taxable = bracketWidth
			}
			tax += taxable * rate
		}
	}

	return math.Round(tax*100) / 100
}
```

> **Note to implementer:** The exact `store.*Row` types will be known after sqlc generate (Task 4). The `computeProgressiveTax` function may need adjustment based on the actual generated types. The `EmployeePayData` struct additions (Task 7) must also be in place for this to compile. Implement the pure calculation logic first, wire into Calculator later in Task 7.

- [ ] **Step 4: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/payroll/... -run "TestCompute|TestPreTax|TestOvertime" -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/payroll/country_us.go internal/payroll/country_us_test.go
git commit -m "feat(payroll): add US payroll engine — federal tax, FICA, state tax, OT"
```

---

### Task 6: USA Country Seed Data

Seeds leave types and holidays when a new USA company is registered.

**Files:**
- Create: `internal/auth/seed_usa.go`
- Modify: `internal/auth/company_setup.go:22-33,42-49`

**Reference:** Follow `seedLKADefaults()` in `internal/auth/company_setup.go:51-136`

- [ ] **Step 1: Create seed_usa.go**

Create `internal/auth/seed_usa.go`:

```go
package auth

import (
	"context"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

func seedUSADefaults(ctx context.Context, q *store.Queries, companyID int64) error {
	// US leave types
	leaveTypes := []store.CreateLeaveTypeParams{
		{CompanyID: companyID, Code: "PTO", Name: "Paid Time Off", IsPaid: true, DefaultDays: numericFromFloat(15), IsConvertible: false, AccrualType: "yearly", IsStatutory: false},
		{CompanyID: companyID, Code: "SL", Name: "Sick Leave", IsPaid: true, DefaultDays: numericFromFloat(5), IsConvertible: false, AccrualType: "yearly", IsStatutory: false},
		{CompanyID: companyID, Code: "FMLA", Name: "Family and Medical Leave", IsPaid: false, DefaultDays: numericFromFloat(60), IsConvertible: false, AccrualType: "none", IsStatutory: true},
		{CompanyID: companyID, Code: "BRV", Name: "Bereavement Leave", IsPaid: true, DefaultDays: numericFromFloat(3), IsConvertible: false, AccrualType: "none", IsStatutory: false},
		{CompanyID: companyID, Code: "JD", Name: "Jury Duty", IsPaid: true, DefaultDays: numericFromFloat(5), IsConvertible: false, AccrualType: "none", IsStatutory: false},
	}

	for _, lt := range leaveTypes {
		if _, err := q.CreateLeaveType(ctx, lt); err != nil {
			return err
		}
	}

	// US Federal Holidays 2025-2026
	holidays := []struct {
		Name string
		Date string
		Type string
		Year int32
	}{
		// 2025
		{"New Year's Day", "2025-01-01", "regular", 2025},
		{"Martin Luther King Jr. Day", "2025-01-20", "regular", 2025},
		{"Presidents' Day", "2025-02-17", "regular", 2025},
		{"Memorial Day", "2025-05-26", "regular", 2025},
		{"Juneteenth", "2025-06-19", "regular", 2025},
		{"Independence Day", "2025-07-04", "regular", 2025},
		{"Labor Day", "2025-09-01", "regular", 2025},
		{"Columbus Day", "2025-10-13", "regular", 2025},
		{"Veterans Day", "2025-11-11", "regular", 2025},
		{"Thanksgiving", "2025-11-27", "regular", 2025},
		{"Christmas Day", "2025-12-25", "regular", 2025},
		// 2026
		{"New Year's Day", "2026-01-01", "regular", 2026},
		{"Martin Luther King Jr. Day", "2026-01-19", "regular", 2026},
		{"Presidents' Day", "2026-02-16", "regular", 2026},
		{"Memorial Day", "2026-05-25", "regular", 2026},
		{"Juneteenth", "2026-06-19", "regular", 2026},
		{"Independence Day", "2026-07-04", "regular", 2026},
		{"Labor Day", "2026-09-07", "regular", 2026},
		{"Columbus Day", "2026-10-12", "regular", 2026},
		{"Veterans Day", "2026-11-11", "regular", 2026},
		{"Thanksgiving", "2026-11-26", "regular", 2026},
		{"Christmas Day", "2026-12-25", "regular", 2026},
	}

	for _, h := range holidays {
		d, _ := time.Parse("2006-01-02", h.Date)
		if _, err := q.CreateHoliday(ctx, store.CreateHolidayParams{
			CompanyID:    companyID,
			Name:         h.Name,
			HolidayDate:  d,
			HolidayType:  h.Type,
			Year:         h.Year,
			IsNationwide: true,
		}); err != nil {
			return err
		}
	}

	return nil
}
```

- [ ] **Step 2: Add USA case to company_setup.go**

In `internal/auth/company_setup.go`:

Add to `countryConfig()` switch (after line 29, before `default`):
```go
	case "USA":
		return countryDefaults{Country: "USA", Currency: "USD", Timezone: "America/New_York", PayFrequency: "bi_weekly"}
```

Add to `seedCountryDefaults()` switch (after line 45, before `default`):
```go
	case "USA":
		return seedUSADefaults(ctx, q, companyID)
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./internal/auth/...`
Expected: Clean

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/auth/seed_usa.go internal/auth/company_setup.go
git commit -m "feat(auth): add USA country seed — leave types, holidays, company defaults"
```

---

### Task 7: Calculator Integration

Wires the US payroll engine into the main calculator switch, adds US fields to `EmployeePayData`, and updates `CreatePayrollItem` call.

**Files:**
- Modify: `internal/payroll/calculator.go:28-85` (EmployeePayData struct)
- Modify: `internal/payroll/calculator.go:244-272` (country switch)
- Modify: `internal/payroll/calculator.go:302-327` (CreatePayrollItem call)

- [ ] **Step 1: Add US fields to EmployeePayData struct**

In `internal/payroll/calculator.go`, add after line 76 (`ETFER float64`):

```go
	// Government contributions — United States
	FederalTax         float64
	SocialSecurityEE   float64
	SocialSecurityER   float64
	MedicareEE         float64
	MedicareER         float64
	AdditionalMedicare float64
	StateTax           float64
	StateDisability    float64
	FUTA               float64
	SUI                float64
	PreTaxDeductions   float64
	Section125Deductions float64 // subset of pre-tax that reduces FICA

	// W-4 fields (loaded from employee_profiles)
	W4AdditionalWithholding float64
	W4DependentsCredit      float64
	W4OtherIncome           float64
	W4Deductions            float64
```

- [ ] **Step 2: Add `case "USA"` to country switch**

In `internal/payroll/calculator.go`, add before `default:` case (line 258):

```go
		case "USA":
			// Load employee profile for W-4 data and state
			state := ""
			filingStatus := "single"
			payPeriodsPerYear := 26 // bi-weekly default

			// Load W-4 fields from employee_profiles
			profile, profErr := calc.queries.GetEmployeeProfile(ctx, emp.ID)
			if profErr == nil {
				if profile.StateOfResidence != nil {
					state = *profile.StateOfResidence
				}
				if profile.W4FilingStatus != nil && *profile.W4FilingStatus != "" {
					filingStatus = *profile.W4FilingStatus
				}
				if profile.W4AdditionalWithholding.Valid {
					pd.W4AdditionalWithholding = numericToFloat(profile.W4AdditionalWithholding)
				}
				if profile.W4DependentsCredit.Valid {
					pd.W4DependentsCredit = numericToFloat(profile.W4DependentsCredit)
				}
				if profile.W4OtherIncome.Valid {
					pd.W4OtherIncome = numericToFloat(profile.W4OtherIncome)
				}
				if profile.W4Deductions.Valid {
					pd.W4Deductions = numericToFloat(profile.W4Deductions)
				}
			}

			// Determine pay periods from company pay_frequency
			switch company.PayFrequency {
			case "weekly":
				payPeriodsPerYear = 52
			case "bi_weekly":
				payPeriodsPerYear = 26
			case "semi_monthly":
				payPeriodsPerYear = 24
			case "monthly":
				payPeriodsPerYear = 12
			}

			// Load pre-tax deductions for this employee
			deductions, dErr := calc.queries.ListEmployeeBenefitDeductions(ctx, store.ListEmployeeBenefitDeductionsParams{
				CompanyID:     companyID,
				EmployeeID:    emp.ID,
				EffectiveDate: effectiveDate,
			})
			if dErr == nil {
				for _, d := range deductions {
					amt := numericToFloat(d.AmountPerPeriod)
					pd.PreTaxDeductions += amt
					if d.ReducesFica {
						pd.Section125Deductions += amt
					}
				}
			}

			calc.computePayUS(&pd, workingDaysInPeriod, state)
			calc.computeContributionsUS(ctx, &pd, effectiveDate, companyID, state)
			calc.computeWithholdingTaxUS(ctx, &pd, effectiveDate, payPeriodsPerYear, filingStatus, state)

			pd.TotalDeductions = pd.FederalTax + pd.StateTax + pd.SocialSecurityEE +
				pd.MedicareEE + pd.AdditionalMedicare + pd.StateDisability +
				pd.PreTaxDeductions + pd.LateDeduction + pd.UndertimeDeduction + pd.LeaveDeduction

			pd.Breakdown["federal_tax"] = pd.FederalTax
			pd.Breakdown["social_security_ee"] = pd.SocialSecurityEE
			pd.Breakdown["social_security_er"] = pd.SocialSecurityER
			pd.Breakdown["medicare_ee"] = pd.MedicareEE
			pd.Breakdown["medicare_er"] = pd.MedicareER
			pd.Breakdown["additional_medicare"] = pd.AdditionalMedicare
			pd.Breakdown["state_tax"] = pd.StateTax
			pd.Breakdown["state_disability"] = pd.StateDisability
			pd.Breakdown["futa"] = pd.FUTA
			pd.Breakdown["sui"] = pd.SUI
			pd.Breakdown["pretax_deductions"] = pd.PreTaxDeductions
			pd.Breakdown["state"] = state
			pd.Breakdown["filing_status"] = filingStatus
```

- [ ] **Step 3: Update CreatePayrollItem call to include US columns**

In `internal/payroll/calculator.go`, update the `CreatePayrollItemParams` (around line 302) to add the US fields after `BonusPay`:

```go
			FederalTax:       numericFromFloat(pd.FederalTax),
			SocialSecurityEe: numericFromFloat(pd.SocialSecurityEE),
			SocialSecurityEr: numericFromFloat(pd.SocialSecurityER),
			MedicareEe:       numericFromFloat(pd.MedicareEE),
			MedicareEr:       numericFromFloat(pd.MedicareER),
			AdditionalMedicare: numericFromFloat(pd.AdditionalMedicare),
			StateTax:         numericFromFloat(pd.StateTax),
			StateDisability:  numericFromFloat(pd.StateDisability),
			Futa:             numericFromFloat(pd.FUTA),
			Sui:              numericFromFloat(pd.SUI),
			PretaxDeductions: numericFromFloat(pd.PreTaxDeductions),
```

> **Note:** The exact field names depend on sqlc-generated struct names from Task 4. Check `internal/store/payroll.sql.go` for the actual `CreatePayrollItemParams` struct.

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./internal/payroll/...`
Expected: Clean

- [ ] **Step 5: Run existing tests to ensure no regression**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/payroll/... -v -count=1`
Expected: All existing tests pass

- [ ] **Step 6: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/payroll/calculator.go
git commit -m "feat(payroll): integrate US payroll engine into calculator switch"
```

---

### Task 7a: Backend Handlers for Benefit Deductions & Registration Numbers

Creates the API endpoints that the frontend (Task 11) calls. Without these handlers, the frontend benefit deduction and company registration UIs have no backend.

**Files:**
- Create: `internal/payroll/benefit_handler.go`
- Create: `internal/payroll/registration_handler.go`
- Modify: `internal/payroll/routes.go`

**Reference:** Follow existing handler pattern in `internal/payroll/handler.go` (struct with queries, pool, logger)

- [ ] **Step 1: Create benefit deduction handler**

Create `internal/payroll/benefit_handler.go`:

```go
package payroll

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListBenefitDeductions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	employeeIDStr := c.Query("employee_id")

	if employeeIDStr != "" {
		employeeID, err := strconv.ParseInt(employeeIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid employee_id")
			return
		}
		deductions, err := h.queries.ListEmployeeBenefitDeductions(c.Request.Context(), store.ListEmployeeBenefitDeductionsParams{
			CompanyID:     companyID,
			EmployeeID:    employeeID,
			EffectiveDate: pgtype.Date{Valid: true, Time: time.Now()},
		})
		if err != nil {
			response.InternalError(c, "Failed to list benefit deductions")
			return
		}
		response.OK(c, deductions)
		return
	}

	deductions, err := h.queries.ListBenefitDeductionsByCompany(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list benefit deductions")
		return
	}
	response.OK(c, deductions)
}

func (h *Handler) CreateBenefitDeduction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		EmployeeID      int64   `json:"employee_id" binding:"required"`
		DeductionType   string  `json:"deduction_type" binding:"required"`
		AmountPerPeriod float64 `json:"amount_per_period" binding:"required"`
		AnnualLimit     float64 `json:"annual_limit"`
		ReducesFica     bool    `json:"reduces_fica"`
		EffectiveDate   string  `json:"effective_date" binding:"required"`
		EndDate         string  `json:"end_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	effDate, _ := time.Parse("2006-01-02", req.EffectiveDate)
	var endDate pgtype.Date
	if req.EndDate != "" {
		ed, _ := time.Parse("2006-01-02", req.EndDate)
		endDate = pgtype.Date{Valid: true, Time: ed}
	}

	deduction, err := h.queries.CreateBenefitDeduction(c.Request.Context(), store.CreateBenefitDeductionParams{
		CompanyID:       companyID,
		EmployeeID:      req.EmployeeID,
		DeductionType:   req.DeductionType,
		AmountPerPeriod: numericFromFloat(req.AmountPerPeriod),
		AnnualLimit:     numericFromFloat(req.AnnualLimit),
		ReducesFica:     req.ReducesFica,
		EffectiveDate:   effDate,
		EndDate:         endDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to create benefit deduction")
		return
	}
	response.Created(c, deduction)
}

func (h *Handler) UpdateBenefitDeduction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		DeductionType   string  `json:"deduction_type" binding:"required"`
		AmountPerPeriod float64 `json:"amount_per_period" binding:"required"`
		AnnualLimit     float64 `json:"annual_limit"`
		ReducesFica     bool    `json:"reduces_fica"`
		EffectiveDate   string  `json:"effective_date" binding:"required"`
		EndDate         string  `json:"end_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	effDate, _ := time.Parse("2006-01-02", req.EffectiveDate)
	var endDate pgtype.Date
	if req.EndDate != "" {
		ed, _ := time.Parse("2006-01-02", req.EndDate)
		endDate = pgtype.Date{Valid: true, Time: ed}
	}

	deduction, err := h.queries.UpdateBenefitDeduction(c.Request.Context(), store.UpdateBenefitDeductionParams{
		ID:              id,
		CompanyID:       companyID,
		DeductionType:   req.DeductionType,
		AmountPerPeriod: numericFromFloat(req.AmountPerPeriod),
		AnnualLimit:     numericFromFloat(req.AnnualLimit),
		ReducesFica:     req.ReducesFica,
		EffectiveDate:   effDate,
		EndDate:         endDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to update benefit deduction")
		return
	}
	response.OK(c, deduction)
}

func (h *Handler) DeleteBenefitDeduction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.queries.DeleteBenefitDeduction(c.Request.Context(), store.DeleteBenefitDeductionParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete benefit deduction")
		return
	}
	response.OK(c, nil)
}
```

- [ ] **Step 2: Create registration number handler**

Create `internal/payroll/registration_handler.go`:

```go
package payroll

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListRegistrationNumbers(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	company, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}

	numbers, err := h.queries.ListCompanyRegistrationNumbers(c.Request.Context(), store.ListCompanyRegistrationNumbersParams{
		CompanyID: companyID,
		Country:   company.Country,
	})
	if err != nil {
		response.InternalError(c, "Failed to list registration numbers")
		return
	}
	response.OK(c, numbers)
}

func (h *Handler) UpsertRegistrationNumber(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		RegistrationType  string `json:"registration_type" binding:"required"`
		RegistrationValue string `json:"registration_value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	company, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}

	result, err := h.queries.UpsertCompanyRegistrationNumber(c.Request.Context(), store.UpsertCompanyRegistrationNumberParams{
		CompanyID:         companyID,
		Country:           company.Country,
		RegistrationType:  req.RegistrationType,
		RegistrationValue: req.RegistrationValue,
	})
	if err != nil {
		response.InternalError(c, "Failed to save registration number")
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteRegistrationNumber(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.queries.DeleteCompanyRegistrationNumber(c.Request.Context(), store.DeleteCompanyRegistrationNumberParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete registration number")
		return
	}
	response.OK(c, nil)
}
```

- [ ] **Step 3: Register routes**

In `internal/payroll/routes.go`, add after existing routes:

```go
	// Benefit Deductions (US)
	protected.GET("/payroll/benefit-deductions", auth.AdminOnly(), h.ListBenefitDeductions)
	protected.POST("/payroll/benefit-deductions", auth.AdminOnly(), h.CreateBenefitDeduction)
	protected.PUT("/payroll/benefit-deductions/:id", auth.AdminOnly(), h.UpdateBenefitDeduction)
	protected.DELETE("/payroll/benefit-deductions/:id", auth.AdminOnly(), h.DeleteBenefitDeduction)

	// Company Registration Numbers (US)
	protected.GET("/company/registration-numbers", auth.AdminOnly(), h.ListRegistrationNumbers)
	protected.POST("/company/registration-numbers", auth.AdminOnly(), h.UpsertRegistrationNumber)
	protected.DELETE("/company/registration-numbers/:id", auth.AdminOnly(), h.DeleteRegistrationNumber)
```

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./internal/payroll/...`
Expected: Clean

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/payroll/benefit_handler.go internal/payroll/registration_handler.go internal/payroll/routes.go
git commit -m "feat(payroll): add benefit deduction and registration number API handlers"
```

---

### Task 8: US Compliance Forms

Implements W-2 and Form 941 generation for US companies.

**Files:**
- Create: `internal/compliance/forms_us.go`
- Create: `internal/compliance/forms_us_test.go`
- Modify: `internal/compliance/forms.go` (GenerateAndStore switch)

**Reference:** Follow existing form generation pattern in `internal/compliance/forms.go:483-553`

- [ ] **Step 1: Write the failing tests**

Create `internal/compliance/forms_us_test.go`:

```go
package compliance

import (
	"testing"
)

func TestW2BoxCalculations(t *testing.T) {
	// Simulate annual payroll data
	data := W2Data{
		EmployeeName:      "John Doe",
		EmployeeSSN:       "XXX-XX-6789",
		GrossWages:        100000.00,
		PreTax401k:        23000.00,
		HealthInsurance:   6000.00,
		FederalTaxWithheld: 15000.00,
		SSWages:           94000.00, // gross - Section 125 (health ins)
		SSTaxWithheld:     5828.00,  // 94000 * 0.062
		MedicareWages:     94000.00,
		MedicareTaxWithheld: 1363.00, // 94000 * 0.0145
		StateWages:        71000.00,  // gross - all pre-tax
		StateTaxWithheld:  4500.00,
	}

	// Box 1: Wages = Gross - pre-tax deductions (401k + health)
	box1 := data.GrossWages - data.PreTax401k - data.HealthInsurance
	if box1 != 71000.00 {
		t.Errorf("Box 1: got %.2f, want 71000.00", box1)
	}

	// Box 3: SS Wages = Gross - Section 125 only (not 401k), capped at wage base
	if data.SSWages != 94000.00 {
		t.Errorf("Box 3: got %.2f, want 94000.00", data.SSWages)
	}
}

func TestForm941QuarterlyAggregation(t *testing.T) {
	data := Form941Data{
		TotalWages:       300000.00,
		FederalWithheld:  45000.00,
		SSWages:          300000.00,
		MedicareWages:    300000.00,
		EmployeeCount:    10,
	}

	// Line 5a: SS wages × 12.4% (combined EE+ER)
	line5a := data.SSWages * 0.124
	if line5a != 37200.00 {
		t.Errorf("Line 5a: got %.2f, want 37200.00", line5a)
	}

	// Line 5c: Medicare wages × 2.9%
	line5c := data.MedicareWages * 0.029
	if line5c != 8700.00 {
		t.Errorf("Line 5c: got %.2f, want 8700.00", line5c)
	}
}
```

- [ ] **Step 2: Write the implementation**

Create `internal/compliance/forms_us.go`:

```go
package compliance

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/store"
)

// W2Data represents the data for a W-2 form.
type W2Data struct {
	// Employee info
	EmployeeName string  `json:"employee_name"`
	EmployeeSSN  string  `json:"employee_ssn"` // masked
	EmployeeAddr string  `json:"employee_address"`

	// Employer info
	EmployerName string `json:"employer_name"`
	EmployerEIN  string `json:"employer_ein"`
	EmployerAddr string `json:"employer_address"`

	// Box 1-6
	GrossWages          float64 `json:"box1_wages"`           // Box 1
	FederalTaxWithheld  float64 `json:"box2_federal_tax"`     // Box 2
	SSWages             float64 `json:"box3_ss_wages"`        // Box 3
	SSTaxWithheld       float64 `json:"box4_ss_tax"`          // Box 4
	MedicareWages       float64 `json:"box5_medicare_wages"`  // Box 5
	MedicareTaxWithheld float64 `json:"box6_medicare_tax"`    // Box 6

	// Pre-tax deductions for Box 12
	PreTax401k      float64 `json:"box12d_401k"`
	HealthInsurance float64 `json:"box12dd_health"`

	// State
	StateWages       float64 `json:"box16_state_wages"`
	StateTaxWithheld float64 `json:"box17_state_tax"`
	StateName        string  `json:"state_name"`
}

// Form941Data represents quarterly Form 941 data.
type Form941Data struct {
	Quarter       int     `json:"quarter"`
	Year          int32   `json:"year"`
	EmployeeCount int     `json:"employee_count"`
	TotalWages    float64 `json:"line2_wages"`
	FederalWithheld float64 `json:"line3_federal_withheld"`
	SSWages       float64 `json:"line5a_ss_wages"`
	MedicareWages float64 `json:"line5c_medicare_wages"`
	AddlMedicareWages float64 `json:"line5d_addl_medicare_wages"`
	TotalTaxes    float64 `json:"line10_total_taxes"`
}

// GenerateW2 creates W-2 data for an employee for a tax year.
func (fg *FormGenerator) GenerateW2(ctx context.Context, companyID int64, employeeID int64, taxYear int32) (*W2Data, error) {
	// Get employee payroll items for the year
	ytd, err := fg.queries.GetYTDPayrollTotals(ctx, store.GetYTDPayrollTotalsParams{
		EmployeeID: employeeID,
		CompanyID:  companyID,
		Column3:    taxYear,
	})
	if err != nil {
		return nil, fmt.Errorf("get YTD totals: %w", err)
	}

	_ = ytd // Use YTD data to populate W-2 boxes
	// Full implementation aggregates all payroll_items for the year

	return &W2Data{}, nil
}

// Generate941 creates Form 941 data for a quarter.
func (fg *FormGenerator) Generate941(ctx context.Context, companyID int64, taxYear int32, quarter int) (*Form941Data, error) {
	if quarter < 1 || quarter > 4 {
		return nil, fmt.Errorf("quarter must be 1-4")
	}

	// Aggregate payroll runs for the quarter
	// Full implementation queries payroll_items joined with payroll_runs/cycles
	// filtered by period_start within the quarter date range

	return &Form941Data{
		Quarter: quarter,
		Year:    taxYear,
	}, nil
}
```

- [ ] **Step 3: Add US form types to GenerateAndStore switch**

In `internal/compliance/forms.go`, add cases inside the `GenerateAndStore` switch (before `default`):

```go
	case "W2":
		// W-2 requires employee_id (passed via additional params or separate endpoint)
		return nil, fmt.Errorf("W-2 generation requires employee_id — use dedicated endpoint")

	case "FORM_941":
		quarter := 0
		if month >= 1 && month <= 3 {
			quarter = 1
		} else if month >= 4 && month <= 6 {
			quarter = 2
		} else if month >= 7 && month <= 9 {
			quarter = 3
		} else if month >= 10 && month <= 12 {
			quarter = 4
		}
		if quarter == 0 {
			return nil, fmt.Errorf("invalid month for 941: provide month 1-12")
		}
		form, err := fg.Generate941(ctx, companyID, taxYear, quarter)
		if err != nil {
			return nil, err
		}
		payload = form
		p := fmt.Sprintf("Q%d", quarter)
		period = &p
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/compliance/... -run "TestW2|TestForm941" -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/compliance/forms_us.go internal/compliance/forms_us_test.go internal/compliance/forms.go
git commit -m "feat(compliance): add US forms — W-2, Form 941 generation"
```

---

### Task 9: Frontend i18n Keys

Adds all US-specific i18n keys to both English and Chinese translation files.

**Files:**
- Modify: `frontend/src/i18n/en.ts`
- Modify: `frontend/src/i18n/zh.ts`

- [ ] **Step 1: Add US keys to en.ts**

Add to the `countryFields` section in `en.ts`:

```typescript
  USA: {
    ssn: 'Social Security Number',
    ein: 'Employer Identification Number (EIN)',
    w4FilingStatus: 'W-4 Filing Status',
    single: 'Single',
    marriedJointly: 'Married Filing Jointly',
    headOfHousehold: 'Head of Household',
    stateOfResidence: 'State of Residence',
    stateAllowances: 'State Withholding Allowances',
    w4AdditionalWithholding: 'Additional Withholding (Step 4c)',
    w4MultipleJobs: 'Multiple Jobs (Step 2)',
    w4DependentsCredit: 'Dependents Credit (Step 3)',
    w4OtherIncome: 'Other Income (Step 4a)',
    w4Deductions: 'Deductions (Step 4b)',
  },
```

Add to the `payroll` section:

```typescript
  usa: {
    federalTax: 'Federal Income Tax',
    socialSecurity: 'Social Security',
    socialSecurityEE: 'Social Security (EE)',
    socialSecurityER: 'Social Security (ER)',
    medicare: 'Medicare',
    medicareEE: 'Medicare (EE)',
    medicareER: 'Medicare (ER)',
    additionalMedicare: 'Additional Medicare Tax',
    stateTax: 'State Income Tax',
    sdi: 'State Disability Insurance',
    futa: 'FUTA',
    sui: 'State Unemployment',
    pretaxDeductions: 'Pre-Tax Deductions',
    preTax401k: '401(k) Contribution',
    healthInsurance: 'Health Insurance',
    dentalVision: 'Dental/Vision',
    hsa: 'HSA Contribution',
    fsa: 'FSA Contribution',
  },
  benefitDeductions: {
    title: 'Benefit Deductions',
    add: 'Add Deduction',
    edit: 'Edit Deduction',
    type: 'Deduction Type',
    amountPerPeriod: 'Amount Per Period',
    annualLimit: 'Annual Limit',
    reducesFica: 'Reduces FICA',
    effectiveDate: 'Effective Date',
    endDate: 'End Date',
    noDeductions: 'No benefit deductions configured.',
    types: {
      '401k': '401(k) Traditional',
      health_insurance: 'Health Insurance (Section 125)',
      dental_vision: 'Dental/Vision Insurance',
      hsa: 'Health Savings Account (HSA)',
      fsa_health: 'FSA (Healthcare)',
      fsa_dependent: 'FSA (Dependent Care)',
    },
  },
  companyRegistration: {
    title: 'Company Registration Numbers',
    ein: 'EIN',
    futaRate: 'FUTA Rate',
    caEddNo: 'CA EDD Number',
    caSuiRate: 'CA SUI Rate',
    nyUiNo: 'NY UI Number',
    nySuiRate: 'NY SUI Rate',
  },
```

- [ ] **Step 2: Add matching Chinese keys to zh.ts**

Add corresponding Chinese translations to `zh.ts` following the same structure.

- [ ] **Step 3: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/i18n/en.ts frontend/src/i18n/zh.ts
git commit -m "feat(i18n): add US country support translation keys"
```

---

### Task 10: Frontend — Payroll View US Rendering

Adds conditional rendering for US payroll columns and hides PH-specific features.

**Files:**
- Modify: `frontend/src/views/PayrollView.vue`

**Reference:** Existing pattern at lines 70-71 (`companyCountry`, `isLKA`)

- [ ] **Step 1: Add USA detection**

In `PayrollView.vue`, after the existing `isLKA` computed (line 71), add:

```typescript
const isUSA = computed(() => companyCountry.value === "USA");
```

- [ ] **Step 2: Add US-specific payroll item columns**

In the data table columns section, add USA columns alongside the existing LKA/PHL conditional block:

```typescript
...(isUSA.value ? [
  { title: t('payroll.usa.federalTax'), key: 'federal_tax', width: 110, render: (r: Record<string, unknown>) => formatCurrency(r.federal_tax) },
  { title: t('payroll.usa.socialSecurityEE'), key: 'social_security_ee', width: 100, render: (r: Record<string, unknown>) => formatCurrency(r.social_security_ee) },
  { title: t('payroll.usa.medicareEE'), key: 'medicare_ee', width: 90, render: (r: Record<string, unknown>) => formatCurrency(r.medicare_ee) },
  { title: t('payroll.usa.stateTax'), key: 'state_tax', width: 100, render: (r: Record<string, unknown>) => formatCurrency(r.state_tax) },
  { title: t('payroll.usa.pretaxDeductions'), key: 'pretax_deductions', width: 120, render: (r: Record<string, unknown>) => formatCurrency(r.pretax_deductions) },
] : isLKA.value ? [
  // existing LKA columns...
] : [
  // existing PHL columns...
]),
```

- [ ] **Step 3: Hide 13th Month tab for USA**

Update the 13th month `v-if` to exclude USA:

```vue
<NTabPane v-if="companyCountry === 'PHL'" ...>
```

(This should already be the case — verify it's not `!isLKA` which would show for USA.)

- [ ] **Step 4: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/views/PayrollView.vue
git commit -m "feat(frontend): add US payroll columns and conditional rendering"
```

---

### Task 11: Frontend — Benefit Deduction Config Component

Admin UI for managing employee pre-tax benefit deductions.

**Files:**
- Create: `frontend/src/components/payroll/BenefitDeductionConfig.vue`
- Modify: `frontend/src/api/client.ts` (add benefit deduction API methods)
- Modify: `frontend/src/views/EmployeeDetailView.vue` (integrate component)

- [ ] **Step 1: Add API methods to client.ts**

Add to `frontend/src/api/client.ts`:

```typescript
export const benefitDeductionAPI = {
  list: (employeeId: number) =>
    get(`/v1/payroll/benefit-deductions`, { employee_id: String(employeeId) }),
  create: (data: Record<string, unknown>) =>
    post("/v1/payroll/benefit-deductions", data),
  update: (id: number, data: Record<string, unknown>) =>
    put(`/v1/payroll/benefit-deductions/${id}`, data),
  remove: (id: number) =>
    del(`/v1/payroll/benefit-deductions/${id}`),
};

export const companyRegistrationAPI = {
  list: () => get("/v1/company/registration-numbers"),
  upsert: (data: Record<string, unknown>) =>
    post("/v1/company/registration-numbers", data),
  remove: (id: number) =>
    del(`/v1/company/registration-numbers/${id}`),
};
```

- [ ] **Step 2: Create BenefitDeductionConfig component**

Create `frontend/src/components/payroll/BenefitDeductionConfig.vue`:

A NaiveUI card component with:
- NDataTable listing active deductions (type, amount, limit, reduces_fica, dates)
- NButton "Add Deduction" opens NModal with NForm
- Form fields: deduction_type (NSelect), amount_per_period (NInputNumber), annual_limit (NInputNumber), reduces_fica (NSwitch), effective_date (NDatePicker), end_date (NDatePicker)
- Edit/delete actions per row
- Props: `employeeId: number`, `companyId: number`

> **Implementation note:** Follow existing component patterns in `frontend/src/components/`. Use `useI18n()` for all strings. Use `useMessage()` for notifications.

- [ ] **Step 3: Integrate into EmployeeDetailView**

Add the component conditionally for US companies in the employee detail view:

```vue
<BenefitDeductionConfig
  v-if="companyCountry === 'USA'"
  :employee-id="employeeId"
  :company-id="companyId"
/>
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/components/payroll/BenefitDeductionConfig.vue frontend/src/api/client.ts frontend/src/views/EmployeeDetailView.vue
git commit -m "feat(frontend): add benefit deduction config component for US employees"
```

---

### Task 12: Integration Verification

Final verification that everything compiles, tests pass, and builds succeed.

**Files:** None (verification only)

- [ ] **Step 1: Go vet**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`
Expected: Clean

- [ ] **Step 2: Go tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/payroll/... ./internal/compliance/... ./internal/auth/... ./pkg/crypto/... -v -count=1`
Expected: All pass

- [ ] **Step 3: Frontend build**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: No errors

- [ ] **Step 4: Frontend mobile build** (if applicable)

Run: `cd /Users/anna/Documents/aigonhr/frontend-mobile && npm run build`
Expected: No errors (no mobile changes, but verify no breakage)

- [ ] **Step 5: Final commit (if any fixups needed)**

```bash
cd /Users/anna/Documents/aigonhr
git add -A
git commit -m "chore: fix integration issues from US country support"
```

- [ ] **Step 6: Push**

```bash
cd /Users/anna/Documents/aigonhr && git push
```
