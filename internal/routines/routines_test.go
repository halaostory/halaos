package routines

import (
	"testing"

	"github.com/halaostory/halaos/internal/lifecycle"
	"github.com/halaostory/halaos/internal/payroll"
)

func TestSummarize(t *testing.T) {
	cases := []struct {
		acts   []Action
		status string
		sum    string
	}{
		{nil, "completed", "Nothing to do"},
		{[]Action{{Status: ActDone}, {Status: ActDone}, {Status: ActSkipped}}, "completed", "2 things done"},
		{[]Action{{Status: ActDone}, {Status: ActPerson}}, "needs_attention", "1 thing done · 1 item left for you"},
		{[]Action{{Status: ActProposed}, {Status: ActProposed}}, "needs_attention", "2 recommendations"},
		{[]Action{{Status: ActFailed}}, "failed", "1 step failed"},
		{[]Action{{Status: ActDone}, {Status: ActFailed}}, "completed", "1 thing done · 1 step failed"},
	}
	for _, c := range cases {
		st, sum := Summarize(c.acts)
		if st != c.status || sum != c.sum {
			t.Errorf("%v → %s %q, want %s %q", c.acts, st, sum, c.status, c.sum)
		}
	}
}

func TestClampAutonomy(t *testing.T) {
	filing, _ := Lookup(KeyFilingAutopilot)
	if got := clampAutonomy(filing, Autonomous); got != Approve {
		t.Errorf("filings must never go autonomous, got %s", got)
	}
	if got := clampAutonomy(filing, Recommend); got != Recommend {
		t.Errorf("got %s", got)
	}
	hire, _ := Lookup(KeyNewHire)
	if got := clampAutonomy(hire, "bogus"); got != Approve {
		t.Errorf("unknown → default, got %s", got)
	}
	if got := clampAutonomy(hire, Autonomous); got != Autonomous {
		t.Errorf("got %s", got)
	}
}

func TestCustomSteps(t *testing.T) {
	acts := customSteps("- Order a laptop\n\n2. Add to #general on Slack\n  * Book a welcome lunch ", "Ana")
	want := []string{"Order a laptop", "Add to #general on Slack", "Book a welcome lunch"}
	if len(acts) != len(want) {
		t.Fatalf("got %d actions: %+v", len(acts), acts)
	}
	for i, a := range acts {
		if a.Title != want[i] || a.Status != ActPerson {
			t.Errorf("%d: %+v", i, a)
		}
	}
}

func TestFromSteps(t *testing.T) {
	acts := fromSteps([]lifecycle.Step{
		{Key: "employee_created", Status: "done", Detail: "EMP-001"},
		{Key: "onboarding_checklist", Status: "skipped"},
		{Key: "gov_registration", Status: "scheduled"},
		{Key: "access_provisioning", Status: "scheduled"},
	}, map[string]bool{"gov_registration": true})
	if len(acts) != 3 || acts[0].Status != ActDone || acts[1].Status != ActSkipped || acts[2].Status != ActWaiting {
		t.Fatalf("%+v", acts)
	}
	if acts[0].Title != "Created the employee record" || acts[0].Rationale != "EMP-001" {
		t.Errorf("%+v", acts[0])
	}
}

func TestAnomalyActions(t *testing.T) {
	clean := anomalyActions(&payroll.AnomalyReport{TotalItems: 12}, "", "Nov 1-15")
	if st, _ := Summarize(clean); st != "completed" || len(clean) != 2 {
		t.Errorf("clean run: %s %+v", st, clean)
	}
	rep := &payroll.AnomalyReport{TotalItems: 3, Anomalies: []payroll.Anomaly{
		{Type: payroll.AnomalyNegativeNet, Severity: payroll.SeverityCritical, EmployeeName: "Ana Cruz", Description: "Net pay is negative"},
		{Type: payroll.AnomalyOTExcessive, Severity: payroll.SeverityLow, EmployeeName: "Ben", Description: "OT 30h"},
		{Type: payroll.AnomalyOTExcessive, Severity: payroll.SeverityMedium, EmployeeName: "Cy", Description: "OT 40h"},
	}}
	acts := anomalyActions(rep, "Double-check commissions", "Nov 1-15")
	st, sum := Summarize(acts)
	if st != "needs_attention" || sum != "2 things done · 2 items left for you" {
		t.Errorf("%s %q %+v", st, sum, acts)
	}
	if acts[1].Title != "Ana Cruz: Net pay is negative" || acts[1].Link != "/payroll" {
		t.Errorf("%+v", acts[1])
	}
}
