package lifecycle

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Lifecycle stages surfaced on the employee overview (Warp-style status pill).
const (
	StagePreStart    = "pre_start"
	StageOnboarding  = "onboarding"
	StageActive      = "active"
	StageOffboarding = "offboarding"
	StageSeparated   = "separated"
)

var separationTypes = map[string]bool{
	"resignation":     true,
	"termination":     true,
	"end_of_contract": true,
	"retirement":      true,
	"layoff":          true,
	"death":           true,
	"other":           true,
}

// ValidSeparationType reports whether t is an accepted separation_type.
func ValidSeparationType(t string) bool { return separationTypes[t] }

var fieldTypes = map[string]bool{"text": true, "number": true, "date": true, "select": true, "boolean": true}

// ValidFieldType reports whether t is an accepted custom field type.
func ValidFieldType(t string) bool { return fieldTypes[t] }

var nonKeyChars = regexp.MustCompile(`[^a-z0-9_]+`)

// NormalizeFieldKey turns a label or user-supplied key into a stable snake_case key.
func NormalizeFieldKey(s string) string {
	k := strings.ToLower(strings.TrimSpace(s))
	k = nonKeyChars.ReplaceAllString(k, "_")
	k = strings.Trim(k, "_")
	if len(k) > 64 {
		k = k[:64]
	}
	return k
}

// FieldSpec is the subset of a custom field definition needed for validation.
type FieldSpec struct {
	Key        string
	Label      string
	Type       string
	Options    []string
	IsRequired bool
}

// ValidateFieldValue checks a single value against its field definition and
// returns the normalized value to store.
func ValidateFieldValue(f FieldSpec, raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		if f.IsRequired {
			return "", fmt.Errorf("%s is required", f.Label)
		}
		return "", nil
	}
	switch f.Type {
	case "number":
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", fmt.Errorf("%s must be a number", f.Label)
		}
	case "date":
		if _, err := time.Parse(dateLayout, v); err != nil {
			return "", fmt.Errorf("%s must be a date (YYYY-MM-DD)", f.Label)
		}
	case "boolean":
		b, err := strconv.ParseBool(v)
		if err != nil {
			return "", fmt.Errorf("%s must be true or false", f.Label)
		}
		v = strconv.FormatBool(b)
	case "select":
		ok := false
		for _, o := range f.Options {
			if o == v {
				ok = true
				break
			}
		}
		if !ok {
			return "", fmt.Errorf("%s must be one of: %s", f.Label, strings.Join(f.Options, ", "))
		}
	default:
		if len(v) > 2000 {
			return "", fmt.Errorf("%s is too long", f.Label)
		}
	}
	return v, nil
}

// ValidateFieldValues validates a key→value map against the company's field
// definitions. Unknown keys are rejected; missing required fields are reported
// only when requireAll is set (hire flow), so partial profile edits still work.
func ValidateFieldValues(specs []FieldSpec, values map[string]string, requireAll bool) (map[string]string, error) {
	byKey := make(map[string]FieldSpec, len(specs))
	for _, s := range specs {
		byKey[s.Key] = s
	}
	out := make(map[string]string, len(values))
	for k, raw := range values {
		spec, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("unknown custom field %q", k)
		}
		v, err := ValidateFieldValue(spec, raw)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	if requireAll {
		for _, s := range specs {
			if !s.IsRequired {
				continue
			}
			if strings.TrimSpace(values[s.Key]) == "" {
				return nil, fmt.Errorf("%s is required", s.Label)
			}
		}
	}
	return out, nil
}

// StageInput carries what is needed to derive an employee's lifecycle stage.
type StageInput struct {
	Status              string
	HireDate            time.Time
	HasActiveSeparation bool
	OnboardingRemaining int64
	Today               time.Time
}

// ComputeStage derives a single lifecycle stage from the employee's state.
func ComputeStage(in StageInput) string {
	switch {
	case in.Status == "separated" || in.Status == "terminated":
		return StageSeparated
	case in.HasActiveSeparation:
		return StageOffboarding
	case dateOnly(in.HireDate).After(dateOnly(in.Today)):
		return StagePreStart
	case in.OnboardingRemaining > 0:
		return StageOnboarding
	default:
		return StageActive
	}
}

// SeparationIsEffective reports whether a separation with the given last
// working day should be applied now (on or before today).
func SeparationIsEffective(lastWorkingDay, today time.Time) bool {
	return !dateOnly(lastWorkingDay).After(dateOnly(today))
}

// FormatEmployeeNo builds an auto-generated employee number.
func FormatEmployeeNo(n int64) string {
	return fmt.Sprintf("EMP-%05d", n)
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
