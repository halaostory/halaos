-- +goose Up

-- Country-specific tax brackets (generic table, starting with LK APIT)
CREATE TABLE country_tax_brackets (
    id BIGSERIAL PRIMARY KEY,
    country VARCHAR(3) NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,
    frequency VARCHAR(20) NOT NULL DEFAULT 'monthly',
    bracket_min NUMERIC(12,2) NOT NULL,
    bracket_max NUMERIC(12,2),
    tax_rate NUMERIC(5,4) NOT NULL,
    fixed_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_country_tax_brackets_lookup ON country_tax_brackets(country, frequency, effective_from);

-- Country-specific contribution rates (EPF, ETF, etc.)
CREATE TABLE country_contribution_rates (
    id BIGSERIAL PRIMARY KEY,
    country VARCHAR(3) NOT NULL,
    contribution_type VARCHAR(30) NOT NULL,
    rate NUMERIC(5,4) NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(country, contribution_type, effective_from)
);

-- Country-specific payroll config (OT rates, night diff, standard hours, etc.)
CREATE TABLE country_payroll_config (
    id BIGSERIAL PRIMARY KEY,
    country VARCHAR(3) NOT NULL,
    config_key VARCHAR(50) NOT NULL,
    config_value JSONB NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(country, config_key)
);

-- Seed Sri Lanka APIT 2025/2026 monthly brackets
-- Formula: T = P × rate − fixed_amount (where P = monthly taxable income)
INSERT INTO country_tax_brackets (country, effective_from, frequency, bracket_min, bracket_max, tax_rate, fixed_amount, description) VALUES
('LKA', '2025-01-01', 'monthly', 0, 150000, 0.0000, 0, 'Tax-free threshold'),
('LKA', '2025-01-01', 'monthly', 150001, 233333, 0.0600, 9000, 'APIT 6% bracket'),
('LKA', '2025-01-01', 'monthly', 233334, 275000, 0.1800, 37000, 'APIT 18% bracket'),
('LKA', '2025-01-01', 'monthly', 275001, 316667, 0.2400, 53500, 'APIT 24% bracket'),
('LKA', '2025-01-01', 'monthly', 316668, 358333, 0.3000, 72500, 'APIT 30% bracket'),
('LKA', '2025-01-01', 'monthly', 358334, 999999999, 0.3600, 94000, 'APIT 36% bracket');

-- Seed Sri Lanka contribution rates
INSERT INTO country_contribution_rates (country, contribution_type, rate, effective_from, description) VALUES
('LKA', 'epf_employee', 0.0800, '2025-01-01', 'EPF Employee share 8%'),
('LKA', 'epf_employer', 0.1200, '2025-01-01', 'EPF Employer share 12%'),
('LKA', 'etf_employer', 0.0300, '2025-01-01', 'ETF Employer share 3% (no employee share)');

-- Seed Sri Lanka payroll config
INSERT INTO country_payroll_config (country, config_key, config_value, description) VALUES
('LKA', 'ot_rates', '{"regular": 1.5, "holiday": 2.0}', 'Sri Lanka OT rates: 1.5x regular, 2.0x holiday'),
('LKA', 'night_diff_rate', '0', 'Night differential not statutorily mandated in Sri Lanka'),
('LKA', 'standard_hours', '{"daily": 8, "weekly": 45}', 'Sri Lanka standard hours: 8/day, 45/week'),
('LKA', 'has_13th_month', 'false', '13th month pay is NOT mandatory in Sri Lanka'),
('LKA', 'default_pay_frequency', '"monthly"', 'Most common pay frequency in Sri Lanka');

-- Seed Philippine payroll config (for completeness)
INSERT INTO country_payroll_config (country, config_key, config_value, description) VALUES
('PHL', 'ot_rates', '{"regular": 1.25, "rest_day": 1.69, "holiday": 2.60, "special_holiday": 1.69}', 'PH OT rates per Labor Code'),
('PHL', 'night_diff_rate', '0.10', 'PH night differential 10% per Art. 86'),
('PHL', 'standard_hours', '{"daily": 8, "weekly": 48}', 'PH standard hours: 8/day, 48/week'),
('PHL', 'has_13th_month', 'true', '13th month pay is mandatory in Philippines'),
('PHL', 'default_pay_frequency', '"semi_monthly"', 'Most common pay frequency in Philippines');

-- +goose Down
DROP TABLE IF EXISTS country_payroll_config;
DROP TABLE IF EXISTS country_contribution_rates;
DROP TABLE IF EXISTS country_tax_brackets;
