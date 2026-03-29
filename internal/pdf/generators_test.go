package pdf

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

func TestBuildFullName(t *testing.T) {
	tests := []struct {
		first  string
		middle *string
		last   string
		want   string
	}{
		{"Juan", nil, "Cruz", "Juan Cruz"},
		{"Maria", strPtr("Santos"), "Reyes", "Maria Santos Reyes"},
		{"Ana", strPtr(""), "Lopez", "Ana Lopez"},
		{"", nil, "Dela Cruz", "Dela Cruz"},
		{"Jose", nil, "", "Jose"},
	}

	for _, tc := range tests {
		got := buildFullName(tc.first, tc.middle, tc.last)
		if got != tc.want {
			t.Errorf("buildFullName(%q, %v, %q) = %q, want %q", tc.first, tc.middle, tc.last, got, tc.want)
		}
	}
}

func strPtr(s string) *string { return &s }

func stubCompany() store.Company {
	addr := "123 Main St"
	city := "Manila"
	province := "Metro Manila"
	tin := "123-456-789-000"
	legalName := "Test Corp Legal"
	return store.Company{
		Name:      "Test Corp",
		LegalName: &legalName,
		Address:   &addr,
		City:      &city,
		Province:  &province,
		Tin:       &tin,
	}
}

func stubEmployee() store.GetEmployeeForCOERow {
	middle := "Santos"
	return store.GetEmployeeForCOERow{
		EmployeeNo:     "EMP-001",
		FirstName:      "Juan",
		MiddleName:     &middle,
		LastName:       "Cruz",
		HireDate:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
		Status:         "active",
		PositionTitle:  "Software Engineer",
		DepartmentName: "Engineering",
		EmploymentType: "regular",
	}
}

func TestGenerateCOE_Success(t *testing.T) {
	data, err := GenerateCOE(stubCompany(), stubEmployee())
	if err != nil {
		t.Fatalf("GenerateCOE failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("GenerateCOE returned empty bytes")
	}
	// PDF magic bytes: %PDF
	if len(data) > 4 && string(data[:5]) != "%PDF-" {
		t.Errorf("expected PDF header, got %q", string(data[:5]))
	}
}

func TestGenerateCOE_InactiveEmployee(t *testing.T) {
	emp := stubEmployee()
	emp.Status = "separated"

	data, err := GenerateCOE(stubCompany(), emp)
	if err != nil {
		t.Fatalf("GenerateCOE failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("GenerateCOE returned empty bytes")
	}
}

func TestGenerateCOE_NoMiddleName(t *testing.T) {
	emp := stubEmployee()
	emp.MiddleName = nil

	data, err := GenerateCOE(stubCompany(), emp)
	if err != nil {
		t.Fatalf("GenerateCOE failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("GenerateCOE returned empty bytes")
	}
}

func TestGenerateCOE_MinimalCompany(t *testing.T) {
	comp := store.Company{Name: "Minimal Corp"}
	data, err := GenerateCOE(comp, stubEmployee())
	if err != nil {
		t.Fatalf("GenerateCOE failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("GenerateCOE returned empty bytes")
	}
}

func TestGenerateLetter_NTE(t *testing.T) {
	data, err := GenerateLetter(
		stubCompany(), stubEmployee(),
		"nte", "Tardiness", "body text", "3 unexcused absences", "2026-03-15",
		0,
	)
	if err != nil {
		t.Fatalf("GenerateLetter NTE failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}

func TestGenerateLetter_COEC(t *testing.T) {
	data, err := GenerateLetter(
		stubCompany(), stubEmployee(),
		"coec", "", "", "", "",
		35000.00,
	)
	if err != nil {
		t.Fatalf("GenerateLetter COEC failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}

func TestGenerateLetter_Clearance(t *testing.T) {
	data, err := GenerateLetter(
		stubCompany(), stubEmployee(),
		"clearance", "", "", "", "",
		0,
	)
	if err != nil {
		t.Fatalf("GenerateLetter clearance failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}

func TestGenerateLetter_Memo(t *testing.T) {
	data, err := GenerateLetter(
		stubCompany(), stubEmployee(),
		"memo", "Schedule Change", "Your schedule has been updated.", "", "",
		0,
	)
	if err != nil {
		t.Fatalf("GenerateLetter memo failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}

func TestGenerateLetter_UnsupportedType(t *testing.T) {
	_, err := GenerateLetter(
		stubCompany(), stubEmployee(),
		"unknown", "", "", "", "",
		0,
	)
	if err == nil {
		t.Error("expected error for unsupported letter type")
	}
}

func TestGenerateDOLERegister(t *testing.T) {
	emps := []store.ListEmployeesForDOLERegisterRow{
		{
			EmployeeNo:     "EMP-001",
			FirstName:      "Juan",
			MiddleName:     strPtr("Santos"),
			LastName:       "Cruz",
			Gender:         strPtr("male"),
			BirthDate:      pgtype.Date{Time: time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC), Valid: true},
			CivilStatus:    strPtr("single"),
			Nationality:    strPtr("Filipino"),
			HireDate:       time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
			DepartmentName: "Engineering",
			PositionTitle:  "Developer",
			Tin:            "123-456-789-000",
			SssNo:          "34-1234567-8",
			PhilhealthNo:   "12-345678901-2",
			PagibigNo:      "1234-5678-9012",
		},
	}

	data, err := GenerateDOLERegister(stubCompany(), emps)
	if err != nil {
		t.Fatalf("GenerateDOLERegister failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}

func TestGenerateDOLERegister_EmptyList(t *testing.T) {
	data, err := GenerateDOLERegister(stubCompany(), nil)
	if err != nil {
		t.Fatalf("GenerateDOLERegister failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}

func TestGenerateDOLERegister_NilOptionalFields(t *testing.T) {
	emps := []store.ListEmployeesForDOLERegisterRow{
		{
			EmployeeNo:     "EMP-002",
			FirstName:      "Maria",
			LastName:       "Reyes",
			HireDate:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			DepartmentName: "HR",
			PositionTitle:  "Manager",
			// All optional fields left nil
		},
	}

	data, err := GenerateDOLERegister(stubCompany(), emps)
	if err != nil {
		t.Fatalf("GenerateDOLERegister failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty PDF")
	}
}
