-- name: GetCountryTaxBracket :one
-- Lookup the tax bracket for a given country, income amount, frequency, and date
SELECT * FROM country_tax_brackets
WHERE country = $1
  AND frequency = $2
  AND bracket_min <= $3
  AND (bracket_max IS NULL OR bracket_max >= $3)
  AND effective_from <= $4
  AND (effective_to IS NULL OR effective_to >= $4)
ORDER BY bracket_min DESC
LIMIT 1;

-- name: ListCountryTaxBrackets :many
-- List all tax brackets for a country at a given date
SELECT * FROM country_tax_brackets
WHERE country = $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2)
ORDER BY bracket_min;

-- name: ListCountryContributionRates :many
-- List all contribution rates for a country at a given date
SELECT * FROM country_contribution_rates
WHERE country = $1
  AND effective_from <= $2
  AND (effective_to IS NULL OR effective_to >= $2);

-- name: GetCountryContributionRate :one
-- Get a specific contribution rate for a country
SELECT * FROM country_contribution_rates
WHERE country = $1
  AND contribution_type = $2
  AND effective_from <= $3
  AND (effective_to IS NULL OR effective_to >= $3)
ORDER BY effective_from DESC
LIMIT 1;

-- name: GetCountryPayrollConfig :one
-- Get a specific payroll config key for a country
SELECT * FROM country_payroll_config
WHERE country = $1 AND config_key = $2;

-- name: ListCountryPayrollConfigs :many
-- List all payroll config for a country
SELECT * FROM country_payroll_config
WHERE country = $1;
