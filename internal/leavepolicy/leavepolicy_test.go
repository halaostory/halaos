package leavepolicy

import (
	"testing"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

func d(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }
func ptr[T any](v T) *T    { return &v }

func TestTenureMonths(t *testing.T) {
	for _, c := range []struct {
		hire, on string
		want     int
	}{{"2021-03-15", "2026-03-14", 59}, {"2021-03-15", "2026-03-15", 60}, {"2026-11-10", "2026-11-03", 0}, {"2026-01-31", "2026-02-28", 0}} {
		if got := TenureMonths(d(c.hire), d(c.on)); got != c.want {
			t.Errorf("%s→%s: %d want %d", c.hire, c.on, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	eng := int64(7)
	rules := []store.LeavePolicyRule{
		{ID: 1, LeaveTypeID: 1, Name: "Regular staff", EmploymentTypes: []string{"regular"}, Days: numeric(15), IsActive: true},
		{ID: 2, LeaveTypeID: 1, Name: "5+ years", EmploymentTypes: []string{"regular"}, MinTenureMonths: 60, Days: numeric(20), IsActive: true},
		{ID: 3, LeaveTypeID: 1, Name: "Engineering", DepartmentIds: []int64{eng}, Priority: 10, Days: numeric(18), IsActive: true},
		{ID: 4, LeaveTypeID: 2, Name: "Solo parent", Gender: "female", Days: numeric(7), IsActive: true},
		{ID: 5, LeaveTypeID: 1, Name: "Disabled", Days: numeric(99), Priority: 99, IsActive: false},
		{ID: 6, LeaveTypeID: 1, Name: "Probation", EmploymentTypes: []string{"probationary"}, MaxTenureMonths: ptr(int32(6)), Days: numeric(0), IsActive: true},
	}
	now := d("2026-11-03")
	cases := []struct {
		name string
		e    Employee
		lt   int64
		want float64
		rule string
	}{
		{"default when no rule", Employee{EmploymentType: "contractual", HireDate: d("2025-01-01")}, 1, 5, ""},
		{"regular", Employee{EmploymentType: "regular", HireDate: d("2024-01-01")}, 1, 15, "Regular staff"},
		{"tenure upgrade wins on longer min tenure", Employee{EmploymentType: "regular", HireDate: d("2020-01-01")}, 1, 20, "5+ years"},
		{"priority beats tenure", Employee{EmploymentType: "regular", DepartmentID: &eng, HireDate: d("2020-01-01")}, 1, 18, "Engineering"},
		{"gender", Employee{Gender: "Female", HireDate: d("2024-01-01")}, 2, 7, "Solo parent"},
		{"probation band", Employee{EmploymentType: "probationary", HireDate: d("2026-09-01")}, 1, 0, "Probation"},
		{"probation band ends", Employee{EmploymentType: "probationary", HireDate: d("2026-01-01")}, 1, 5, ""},
	}
	for _, c := range cases {
		got, rule := Resolve(rules, c.lt, 5, c.e, now)
		name := ""
		if rule != nil {
			name = rule.Name
		}
		if got != c.want || name != c.rule {
			t.Errorf("%s: %v (%q) want %v (%q)", c.name, got, name, c.want, c.rule)
		}
	}
}
