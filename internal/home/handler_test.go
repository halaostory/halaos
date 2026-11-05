package home

import "testing"

func TestAgencyStatus(t *testing.T) {
	cases := []struct {
		a      Agency
		open   map[string]int
		status string
	}{
		{Agency{}, nil, "needs_info"},
		{Agency{EmployerNo: "03-1"}, map[string]int{"needs_human": 1, "ready": 2}, "needs_info"},
		{Agency{EmployerNo: "03-1"}, map[string]int{"ready": 2, "submitted": 1}, "needs_approval"},
		{Agency{EmployerNo: "03-1"}, map[string]int{"running": 1}, "filing_started"},
		{Agency{EmployerNo: "03-1"}, map[string]int{"submitted": 1}, "awaiting_agency"},
		{Agency{EmployerNo: "03-1", Employees: 5, Registered: 4}, nil, "needs_info"},
		{Agency{EmployerNo: "03-1", Employees: 5, Registered: 5}, nil, "active"},
	}
	for i, c := range cases {
		if st, _ := agencyStatus(c.a, c.open); st != c.status {
			t.Errorf("%d: got %s want %s", i, st, c.status)
		}
	}
	if _, d := agencyStatus(Agency{EmployerNo: "x", Employees: 3, Registered: 1}, nil); d != "2 employees without an ID number" {
		t.Errorf("detail %q", d)
	}
}
