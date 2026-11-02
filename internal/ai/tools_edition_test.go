package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/halaostory/halaos/internal/edition"
)

func TestToolRegistry_PeopleEditionHidesHalaOSOnlyTools(t *testing.T) {
	r := NewToolRegistry(nil, nil)
	r.edition = edition.New(edition.People, "", "")
	r.disabledTools = disabledToolsFor(r.edition)

	names := map[string]bool{}
	for _, d := range r.Definitions() {
		names[d.Name] = true
	}
	for _, want := range []string{"hire_employee", "prepare_gov_filing", "query_leave_balance", "create_final_pay", "explain_policy"} {
		if !names[want] {
			t.Errorf("people edition should expose %s", want)
		}
	}
	for _, hidden := range []string{"query_flight_risk_scores", "assign_schedule", "query_workflow_rules"} {
		if names[hidden] {
			t.Errorf("people edition must not expose %s", hidden)
		}
	}
	// Even if a model asks for a hidden tool by name, it is refused.
	if _, err := r.Execute(context.Background(), "query_flight_risk_scores", 1, 1, map[string]any{}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("hidden tool must not execute: %v", err)
	}
}

func TestToolRegistry_HalaOSExposesEverything(t *testing.T) {
	r := NewToolRegistry(nil, nil)
	r.edition = edition.New(edition.HalaOS, "", "")
	r.disabledTools = disabledToolsFor(r.edition)
	if len(r.Definitions()) != len(r.tools) {
		t.Errorf("defs %d != registered %d", len(r.Definitions()), len(r.tools))
	}
}
