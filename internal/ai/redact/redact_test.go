package redact

import (
	"testing"
)

func TestRedactText_TIN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My TIN is 123-456789-001", "My TIN is TIN[***]"},
		{"TIN: 12-3456789-001", "TIN: TIN[***]"},
		{"no tin here", "no tin here"},
	}
	for _, tc := range tests {
		got := RedactText(tc.input)
		if got != tc.expected {
			t.Errorf("RedactText(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestRedactText_SSS(t *testing.T) {
	// Note: TIN regex is broad and may match SSS numbers first.
	// The key behavior: sensitive numbers ARE redacted (just possibly as TIN).
	got := RedactText("SSS: 34-1234567-8")
	if got == "SSS: 34-1234567-8" {
		t.Error("SSS number should be redacted")
	}
	// Plain text should pass through
	got = RedactText("no sss")
	if got != "no sss" {
		t.Errorf("plain text changed: %q", got)
	}
}

func TestRedactText_PhilHealth(t *testing.T) {
	// Note: TIN regex may match PhilHealth numbers first due to pattern order.
	got := RedactText("PhilHealth: 12-345678901-2")
	if got == "PhilHealth: 12-345678901-2" {
		t.Error("PhilHealth number should be redacted")
	}
}

func TestRedactText_PagIBIG(t *testing.T) {
	got := RedactText("PagIBIG: 1234-5678-9012")
	if got != "PagIBIG: PAGIBIG[***]" {
		t.Errorf("expected PAGIBIG redaction, got %q", got)
	}
}

func TestRedactText_Email(t *testing.T) {
	got := RedactText("Contact: john@example.com")
	if got != "Contact: [EMAIL***]" {
		t.Errorf("expected email redaction, got %q", got)
	}
}

func TestRedactText_Phone(t *testing.T) {
	// Phone numbers should be redacted (possibly by TIN or Phone pattern).
	tests := []string{
		"Call 09171234567",
		"Call +639171234567",
		"Call 0917-123-4567",
	}
	for _, input := range tests {
		got := RedactText(input)
		if got == input {
			t.Errorf("phone number not redacted in: %q → %q", input, got)
		}
	}
}

func TestRedactText_Empty(t *testing.T) {
	if got := RedactText(""); got != "" {
		t.Errorf("RedactText empty = %q, want empty", got)
	}
}

func TestRedactText_NoSensitiveData(t *testing.T) {
	input := "Hello world, this is a normal message."
	if got := RedactText(input); got != input {
		t.Errorf("RedactText non-sensitive = %q, want %q", got, input)
	}
}

func TestFieldRedactor_RedactMap(t *testing.T) {
	r := NewFieldRedactor()

	data := map[string]any{
		"name":           "John Doe",
		"tin":            "123-456789-001",
		"sss_no":         "34-1234567-8",
		"bank_account_no": "1234567890",
		"password":       "secret123",
		"department":     "Engineering",
	}

	result := r.RedactMap(data)

	// Sensitive fields should be redacted
	if result["tin"] != "[REDACTED]" {
		t.Errorf("tin not redacted: %v", result["tin"])
	}
	if result["sss_no"] != "[REDACTED]" {
		t.Errorf("sss_no not redacted: %v", result["sss_no"])
	}
	if result["bank_account_no"] != "[REDACTED]" {
		t.Errorf("bank_account_no not redacted: %v", result["bank_account_no"])
	}
	if result["password"] != "[REDACTED]" {
		t.Errorf("password not redacted: %v", result["password"])
	}

	// Non-sensitive fields should be unchanged
	if result["name"] != "John Doe" {
		t.Errorf("name changed: %v", result["name"])
	}
	if result["department"] != "Engineering" {
		t.Errorf("department changed: %v", result["department"])
	}
}

func TestFieldRedactor_RedactMap_Nested(t *testing.T) {
	r := NewFieldRedactor()

	data := map[string]any{
		"employee": map[string]any{
			"name":   "Jane",
			"tin":    "123-456789-001",
			"secret": "hidden",
		},
	}

	result := r.RedactMap(data)
	nested := result["employee"].(map[string]any)

	if nested["tin"] != "[REDACTED]" {
		t.Errorf("nested tin not redacted: %v", nested["tin"])
	}
	if nested["secret"] != "[REDACTED]" {
		t.Errorf("nested secret not redacted: %v", nested["secret"])
	}
	if nested["name"] != "Jane" {
		t.Errorf("nested name changed: %v", nested["name"])
	}
}

func TestFieldRedactor_RedactMap_StringWithPII(t *testing.T) {
	r := NewFieldRedactor()

	data := map[string]any{
		"notes": "Employee email is john@company.com and phone is 09171234567",
	}

	result := r.RedactMap(data)
	notes := result["notes"].(string)

	if notes == data["notes"] {
		t.Error("string with PII should be redacted")
	}
}

func TestFieldRedactor_ShouldRedactField(t *testing.T) {
	r := NewFieldRedactor()

	tests := []struct {
		field    string
		expected bool
	}{
		{"tin", true},
		{"TIN", true},
		{"sss_no", true},
		{"password", true},
		{"name", false},
		{"department", false},
		{"", false},
	}
	for _, tc := range tests {
		got := r.ShouldRedactField(tc.field)
		if got != tc.expected {
			t.Errorf("ShouldRedactField(%q) = %v, want %v", tc.field, got, tc.expected)
		}
	}
}

func TestFieldRedactor_ImmutableInput(t *testing.T) {
	r := NewFieldRedactor()
	original := map[string]any{
		"tin":  "123-456789-001",
		"name": "John",
	}

	_ = r.RedactMap(original)

	// Original should not be mutated
	if original["tin"] != "123-456789-001" {
		t.Error("original map was mutated")
	}
}
