package lifecycle

import (
	"testing"
	"time"
)

func TestNormalizeFieldKey(t *testing.T) {
	cases := map[string]string{
		"T-Shirt Size":     "t_shirt_size",
		"  Emergency #2 ":  "emergency_2",
		"already_snake":    "already_snake",
		"__Weird--Key!!__": "weird_key",
	}
	for in, want := range cases {
		if got := NormalizeFieldKey(in); got != want {
			t.Errorf("NormalizeFieldKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateFieldValue(t *testing.T) {
	sel := FieldSpec{Key: "size", Label: "Size", Type: "select", Options: []string{"S", "M", "L"}}
	tests := []struct {
		name    string
		spec    FieldSpec
		in      string
		want    string
		wantErr bool
	}{
		{"text ok", FieldSpec{Label: "Note", Type: "text"}, " hi ", "hi", false},
		{"number ok", FieldSpec{Label: "N", Type: "number"}, "12.5", "12.5", false},
		{"number bad", FieldSpec{Label: "N", Type: "number"}, "abc", "", true},
		{"date ok", FieldSpec{Label: "D", Type: "date"}, "2026-01-31", "2026-01-31", false},
		{"date bad", FieldSpec{Label: "D", Type: "date"}, "31/01/2026", "", true},
		{"bool normalized", FieldSpec{Label: "B", Type: "boolean"}, "TRUE", "true", false},
		{"bool bad", FieldSpec{Label: "B", Type: "boolean"}, "yes", "", true},
		{"select ok", sel, "M", "M", false},
		{"select bad", sel, "XL", "", true},
		{"required empty", FieldSpec{Label: "R", Type: "text", IsRequired: true}, "  ", "", true},
		{"optional empty", FieldSpec{Label: "O", Type: "text"}, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateFieldValue(tt.spec, tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateFieldValues(t *testing.T) {
	specs := []FieldSpec{
		{Key: "size", Label: "Size", Type: "select", Options: []string{"S", "M"}, IsRequired: true},
		{Key: "note", Label: "Note", Type: "text"},
	}
	if _, err := ValidateFieldValues(specs, map[string]string{"bogus": "x"}, false); err == nil {
		t.Error("expected error for unknown key")
	}
	if _, err := ValidateFieldValues(specs, map[string]string{"note": "x"}, true); err == nil {
		t.Error("expected error for missing required field on hire")
	}
	if _, err := ValidateFieldValues(specs, map[string]string{"note": "x"}, false); err != nil {
		t.Errorf("partial update should pass: %v", err)
	}
	out, err := ValidateFieldValues(specs, map[string]string{"size": "M"}, true)
	if err != nil || out["size"] != "M" {
		t.Errorf("unexpected result %v %v", out, err)
	}
}

func TestComputeStage(t *testing.T) {
	today := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	base := StageInput{Status: "active", HireDate: today.AddDate(0, -6, 0), Today: today}

	if got := ComputeStage(base); got != StageActive {
		t.Errorf("active: got %s", got)
	}
	s := base
	s.Status = "separated"
	s.HasActiveSeparation = true
	if got := ComputeStage(s); got != StageSeparated {
		t.Errorf("separated wins: got %s", got)
	}
	s = base
	s.HasActiveSeparation = true
	if got := ComputeStage(s); got != StageOffboarding {
		t.Errorf("offboarding: got %s", got)
	}
	s = base
	s.HireDate = today.AddDate(0, 0, 7)
	s.OnboardingRemaining = 3
	if got := ComputeStage(s); got != StagePreStart {
		t.Errorf("pre_start: got %s", got)
	}
	s = base
	s.HireDate = today
	s.OnboardingRemaining = 2
	if got := ComputeStage(s); got != StageOnboarding {
		t.Errorf("onboarding (start today): got %s", got)
	}
}

func TestSeparationIsEffective(t *testing.T) {
	today := time.Date(2026, 11, 2, 23, 59, 0, 0, time.UTC)
	if !SeparationIsEffective(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), today) {
		t.Error("same day should be effective")
	}
	if !SeparationIsEffective(today.AddDate(0, 0, -1), today) {
		t.Error("past should be effective")
	}
	if SeparationIsEffective(today.AddDate(0, 0, 1), today) {
		t.Error("future should be scheduled")
	}
}

func TestFormatEmployeeNo(t *testing.T) {
	if got := FormatEmployeeNo(42); got != "EMP-00042" {
		t.Errorf("got %s", got)
	}
}
