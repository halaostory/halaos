// Package selfonboard implements Warp-style new-hire self-onboarding (a
// single tokenized link where the new hire enters personal, government ID and
// bank details) and e-signature of offer letters, contracts and policies.
package selfonboard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

// NewToken returns a random URL token and its SHA-256 hash (only the hash is stored).
func NewToken() (token, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token = hex.EncodeToString(b)
	return token, HashToken(token)
}

// HashToken hashes a token for lookup.
func HashToken(t string) string {
	s := sha256.Sum256([]byte(strings.TrimSpace(t)))
	return hex.EncodeToString(s[:])
}

// SHA256Hex hashes arbitrary text.
func SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

var mergeRe = regexp.MustCompile(`\{\{\s*([a-z_.]+)\s*\}\}`)

// MergeFields lists the placeholders templates may use.
var MergeFields = []string{
	"employee.first_name", "employee.last_name", "employee.full_name", "employee.employee_no", "employee.email",
	"employee.position", "employee.department", "employee.manager", "employee.hire_date", "employee.employment_type",
	"employee.basic_salary", "company.name", "company.address", "today",
}

// Render replaces {{field}} placeholders. Unknown or empty fields are
// returned so the sender can fix them before the document goes out.
func Render(body string, values map[string]string) (string, []string) {
	missing := map[string]bool{}
	out := mergeRe.ReplaceAllStringFunc(body, func(m string) string {
		key := mergeRe.FindStringSubmatch(m)[1]
		v, ok := values[key]
		if !ok || strings.TrimSpace(v) == "" {
			missing[key] = true
			return m
		}
		return v
	})
	var ms []string
	for k := range missing {
		ms = append(ms, k)
	}
	sort.Strings(ms)
	return out, ms
}

// Mask shows only the last 4 characters of a sensitive value.
func Mask(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		if s == "" {
			return ""
		}
		return "••••"
	}
	return strings.Repeat("•", len(s)-4) + s[len(s)-4:]
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidateGovIDs checks Philippine government ID formats (other countries
// are accepted as entered). Returns field → message.
func ValidateGovIDs(country string, ids map[string]string) map[string]string {
	errs := map[string]string{}
	if country != "PHL" {
		return errs
	}
	rules := map[string][3]any{
		"tin":           {9, 12, "TIN must have 9–12 digits"},
		"sss_no":        {10, 10, "SSS number must have 10 digits"},
		"philhealth_no": {12, 12, "PhilHealth number must have 12 digits"},
		"pagibig_no":    {12, 12, "Pag-IBIG MID must have 12 digits"},
	}
	for k, r := range rules {
		v := strings.TrimSpace(ids[k])
		if v == "" {
			continue
		}
		n := len(digitsOnly(v))
		if n < r[0].(int) || n > r[1].(int) {
			errs[k] = r[2].(string)
		}
	}
	return errs
}

// NameMatches reports whether the typed signature plausibly belongs to the
// signer (it must contain the signer's last name).
func NameMatches(typed, signerName string) bool {
	typed = strings.ToLower(strings.Join(strings.Fields(typed), " "))
	parts := strings.Fields(strings.ToLower(signerName))
	if typed == "" || len(parts) == 0 {
		return false
	}
	return strings.Contains(typed, parts[len(parts)-1])
}
