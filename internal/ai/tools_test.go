package ai

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/ai/provider"
)

func TestRound2(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{1.999, 2.0},
		{0, 0},
		{-1.555, -1.56},
		{100.001, 100.0},
	}
	for _, tc := range tests {
		got := round2(tc.input)
		if got != tc.expected {
			t.Errorf("round2(%v) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"25000.50", 25000.50},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
		{"12345", 12345},
	}
	for _, tc := range tests {
		got := parseFloat(tc.input)
		if got != tc.expected {
			t.Errorf("parseFloat(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestGetFloatOr(t *testing.T) {
	input := map[string]any{
		"hours":  float64(10),
		"name":   "test",
		"zero":   float64(0),
		"nested": map[string]any{"x": 1},
	}

	tests := []struct {
		key      string
		def      float64
		expected float64
	}{
		{"hours", 0, 10},
		{"missing", 22, 22},
		{"name", 5, 5},       // string value → returns default
		{"zero", 99, 0},      // zero float64 is still valid
		{"nested", 1, 1},     // map value → returns default
	}
	for _, tc := range tests {
		got := getFloatOr(input, tc.key, tc.def)
		if got != tc.expected {
			t.Errorf("getFloatOr(input, %q, %v) = %v, want %v", tc.key, tc.def, got, tc.expected)
		}
	}
}

func TestEstimateSSS(t *testing.T) {
	tests := []struct {
		name     string
		salary   float64
		expected float64
	}{
		{"minimum bracket", 3000, 180},
		{"at lower threshold", 4250, 180},
		{"mid range 10k", 10000, 450},
		{"mid range 20k", 20000, 900},
		{"at upper threshold", 29750, 1350},
		{"maximum bracket", 50000, 1350},
		{"zero", 0, 180},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateSSS(tc.salary)
			if got != tc.expected {
				t.Errorf("estimateSSS(%v) = %v, want %v", tc.salary, got, tc.expected)
			}
		})
	}
}

func TestEstimateTax(t *testing.T) {
	tests := []struct {
		name     string
		income   float64
		expected float64
	}{
		{"below threshold - zero tax", 20000, 0},
		{"at threshold", 20833, 0},
		{"first bracket low", 25000, round2((25000 - 20833) * 0.15)},
		{"second bracket boundary", 33333, round2((33333 - 20833) * 0.15)},
		{"second bracket mid", 50000, round2(1875 + (50000-33333)*0.20)},
		{"third bracket boundary", 66667, round2(1875 + (66667-33333)*0.20)},
		{"third bracket mid", 100000, round2(8541.80 + (100000-66667)*0.25)},
		{"fourth bracket boundary", 166667, round2(8541.80 + (166667-66667)*0.25)},
		{"fourth bracket mid", 300000, round2(33541.80 + (300000-166667)*0.30)},
		{"fifth bracket boundary", 666667, round2(33541.80 + (666667-166667)*0.30)},
		{"highest bracket", 1000000, round2(183541.80 + (1000000-666667)*0.35)},
		{"zero income", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateTax(tc.income)
			if got != tc.expected {
				t.Errorf("estimateTax(%v) = %v, want %v", tc.income, got, tc.expected)
			}
		})
	}
}

func TestEstimateTax_Monotonic(t *testing.T) {
	// Tax should increase monotonically with income
	prev := 0.0
	for income := 0.0; income <= 1000000; income += 5000 {
		tax := estimateTax(income)
		if tax < prev {
			t.Errorf("tax decreased: estimateTax(%v)=%v < estimateTax(%v)=%v",
				income, tax, income-5000, prev)
		}
		prev = tax
	}
}

func TestNumericToString(t *testing.T) {
	// Test with invalid/zero pgtype.Numeric
	got := numericToString(pgTypeNumericZero())
	if got != "0" {
		t.Errorf("numericToString(zero) = %q, want %q", got, "0")
	}
}

func TestCoalesceStrPtr(t *testing.T) {
	existing := strPtr("old")
	input := map[string]any{
		"name":  "new",
		"empty": "",
	}

	// Key exists with value → returns new value
	got := coalesceStrPtr(input, "name", existing)
	if got == nil || *got != "new" {
		t.Errorf("coalesceStrPtr with value = %v, want 'new'", got)
	}

	// Key exists but empty → returns existing
	got = coalesceStrPtr(input, "empty", existing)
	if got != existing {
		t.Errorf("coalesceStrPtr with empty = %v, want existing", got)
	}

	// Key missing → returns existing
	got = coalesceStrPtr(input, "missing", existing)
	if got != existing {
		t.Errorf("coalesceStrPtr missing key = %v, want existing", got)
	}

	// Existing is nil
	got = coalesceStrPtr(input, "missing", nil)
	if got != nil {
		t.Errorf("coalesceStrPtr nil existing = %v, want nil", got)
	}
}

func TestJsonSchema(t *testing.T) {
	schema := map[string]any{"type": "object"}
	got := jsonSchema(schema)
	if got["type"] != "object" {
		t.Errorf("jsonSchema should pass through, got %v", got)
	}
}

func TestDefinitionsCount(t *testing.T) {
	// Verify all domain defs functions return non-empty slices
	domains := []struct {
		name string
		defs []provider.ToolDefinition
	}{
		{"leave", leaveDefs()},
		{"attendance", attendanceDefs()},
		{"payroll", payrollDefs()},
		{"knowledge", knowledgeDefs()},
		{"employee", employeeDefs()},
		{"expense", expenseDefs()},
		{"approval", approvalDefs()},
		{"salarySim", salarySimDefs()},
		{"loan", loanDefs()},
		{"benefit", benefitDefs()},
		{"performance", performanceDefs()},
		{"training", trainingDefs()},
		{"disciplinary", disciplinaryDefs()},
		{"analytics", analyticsDefs()},
		{"clearance", clearanceDefs()},
		{"schedule", scheduleDefs()},
	}

	totalTools := 0
	for _, d := range domains {
		if len(d.defs) == 0 {
			t.Errorf("%s defs returned empty slice", d.name)
		}
		for _, def := range d.defs {
			if def.Name == "" {
				t.Errorf("%s has a definition with empty name", d.name)
			}
		}
		totalTools += len(d.defs)
	}

	// Verify total matches expected count (currently 62 tools)
	if totalTools < 60 {
		t.Errorf("total tool definitions = %d, expected at least 60", totalTools)
	}
}

func TestDefinitionsUniqueNames(t *testing.T) {
	// All tool names should be unique
	r := &ToolRegistry{tools: make(map[string]ToolExecutor)}
	defs := r.Definitions()

	seen := make(map[string]bool)
	for _, d := range defs {
		if seen[d.Name] {
			t.Errorf("duplicate tool name: %s", d.Name)
		}
		seen[d.Name] = true
	}
}

// helpers

func pgTypeNumericZero() pgtype.Numeric {
	return pgtype.Numeric{Valid: false}
}

func strPtr(s string) *string { return &s }
