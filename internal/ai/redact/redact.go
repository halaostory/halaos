package redact

import (
	"regexp"
	"strings"
)

// patterns for Philippine government IDs and sensitive data.
var patterns = []struct {
	name    string
	re      *regexp.Regexp
	replace string
}{
	// TIN: XX-XXXXXXX-XXX or XXXXXXXXXXXX (9-12 digits with optional dashes)
	{"TIN", regexp.MustCompile(`\b\d{2,3}-?\d{3,7}-?\d{3,5}\b`), "TIN[***]"},
	// SSS: XX-XXXXXXX-X (10 digits with dashes)
	{"SSS", regexp.MustCompile(`\b\d{2}-\d{7}-\d\b`), "SSS[***]"},
	// PhilHealth: XX-XXXXXXXXX-X (12 digits with dashes)
	{"PhilHealth", regexp.MustCompile(`\b\d{2}-\d{9}-\d\b`), "PHIC[***]"},
	// PagIBIG: XXXX-XXXX-XXXX (12 digits with dashes)
	{"PagIBIG", regexp.MustCompile(`\b\d{4}-\d{4}-\d{4}\b`), "PAGIBIG[***]"},
	// Bank account numbers (8-16 digits)
	{"BankAccount", regexp.MustCompile(`\b\d{8,16}\b`), "[ACCT***]"},
	// Email addresses
	{"Email", regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`), "[EMAIL***]"},
	// Phone numbers (PH format: 09XX-XXX-XXXX or +63)
	{"Phone", regexp.MustCompile(`\b(?:\+63|0)9\d{2}[- ]?\d{3}[- ]?\d{4}\b`), "[PHONE***]"},
}

// RedactText removes PII patterns from free text.
func RedactText(text string) string {
	result := text
	for _, p := range patterns {
		result = p.re.ReplaceAllString(result, p.replace)
	}
	return result
}

// FieldRedactor handles structured field-based redaction.
type FieldRedactor struct {
	sensitiveFields map[string]bool
}

// NewFieldRedactor creates a redactor for known sensitive field names.
func NewFieldRedactor() *FieldRedactor {
	return &FieldRedactor{
		sensitiveFields: map[string]bool{
			"tin":             true,
			"sss_no":          true,
			"philhealth_no":   true,
			"pagibig_no":      true,
			"bank_account_no": true,
			"bank_account":    true,
			"account_no":      true,
			"password":        true,
			"secret":          true,
			"token":           true,
		},
	}
}

// RedactMap removes sensitive fields from a key-value map.
func (r *FieldRedactor) RedactMap(data map[string]any) map[string]any {
	result := make(map[string]any, len(data))
	for k, v := range data {
		key := strings.ToLower(k)
		if r.sensitiveFields[key] {
			result[k] = "[REDACTED]"
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			result[k] = r.RedactMap(nested)
			continue
		}
		if str, ok := v.(string); ok {
			result[k] = RedactText(str)
			continue
		}
		result[k] = v
	}
	return result
}

// ShouldRedactField returns true if the field name is sensitive.
func (r *FieldRedactor) ShouldRedactField(fieldName string) bool {
	return r.sensitiveFields[strings.ToLower(fieldName)]
}
