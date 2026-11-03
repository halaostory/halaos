// Package routines implements the agent's routines, Warp-style: jobs the AI
// agent owns end to end. Each routine starts on an event (an employee is
// hired or terminated, a payroll run is calculated) or on a schedule (the
// statutory filing calendar). Each company chooses an autonomy level per
// routine:
//
//   - recommend  — the agent only lists what it would do;
//   - approve    — the agent prepares everything and waits for a person to
//     approve (default);
//   - autonomous — the agent also carries out reversible steps on its own
//     (notifications, drafts, checklist items).
//
// Some limits hold at every level: government submissions and payroll
// approval/payment always need a person.
//
// Every run is logged with its trigger, what the agent did (with a reason
// and a link), and what it left for a person.
package routines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/store"
)

// Autonomy levels.
const (
	Recommend  = "recommend"
	Approve    = "approve"
	Autonomous = "autonomous"
)

// ValidAutonomy reports whether a is a known autonomy level.
func ValidAutonomy(a string) bool { return a == Recommend || a == Approve || a == Autonomous }

// Routine keys.
const (
	KeyNewHire         = "new_hire"
	KeyOffboarding     = "offboarding"
	KeyPrePayrollCheck = "pre_payroll_check"
	KeyFilingAutopilot = "filing_autopilot"
)

// Definition describes a built-in routine.
type Definition struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Trigger     string   `json:"trigger"`      // human-readable trigger
	TriggerType string   `json:"trigger_type"` // event, schedule
	Module      string   `json:"module"`       // edition module the routine needs
	Steps       []string `json:"steps"`
	// HardLimits are always left for a person, whatever the autonomy.
	HardLimits      []string `json:"hard_limits"`
	DefaultAutonomy string   `json:"default_autonomy"`
	// MaxAutonomy caps the selectable autonomy (filings never go autonomous).
	MaxAutonomy string `json:"max_autonomy"`
}

// Catalog lists the built-in routines in display order.
var Catalog = []Definition{
	{
		Key: KeyNewHire, Name: "New hire", TriggerType: "event", Trigger: "When someone is hired",
		Module:      "lifecycle",
		Description: "Takes a new hire from signed offer to day one: agency registrations, onboarding, equipment, benefits and a heads-up to their manager.",
		Steps: []string{
			"Create the employee, compensation, leave balances and onboarding checklist",
			"Prepare SSS R-1A, PhilHealth ER2, Pag-IBIG and BIR 1902 registrations",
			"Send the self-onboarding invite and offer letter for e-signature",
			"Check that a laptop is assigned and benefits are enrolled",
			"Tell the manager who is starting and when",
		},
		HardLimits:      []string{"Government submissions wait for your approval"},
		DefaultAutonomy: Approve, MaxAutonomy: Autonomous,
	},
	{
		Key: KeyOffboarding, Name: "Offboarding", TriggerType: "event", Trigger: "When someone is terminated",
		Module:      "lifecycle",
		Description: "Runs the exit end to end: clearance, final pay, device recall and access revocation on the last working day.",
		Steps: []string{
			"Open clearance and the offboarding checklist",
			"Compute final pay (pro-rated salary, 13th month, leave conversion)",
			"Lock and recall assigned devices",
			"Revoke app access on the last working day",
		},
		HardLimits:      []string{"Final pay is released only after approval"},
		DefaultAutonomy: Approve, MaxAutonomy: Autonomous,
	},
	{
		Key: KeyPrePayrollCheck, Name: "Pre-payroll check", TriggerType: "event", Trigger: "When a payroll run is calculated",
		Module:      "payroll",
		Description: "Before every payroll, compares each payslip with past runs and flags variances: pay jumps, zero contributions or tax, negative net pay, excessive overtime.",
		Steps: []string{
			"Compare gross and net pay with each employee's history",
			"Check statutory contributions and withholding tax",
			"Flag anything a person should look at before approving",
		},
		HardLimits:      []string{"Payroll is approved and paid only by a person"},
		DefaultAutonomy: Approve, MaxAutonomy: Approve,
	},
	{
		Key: KeyFilingAutopilot, Name: "Filing autopilot", TriggerType: "schedule", Trigger: "Every hour",
		Module:      "compliance",
		Description: "Keeps the statutory calendar moving: prepares every return due soon (BIR, SSS, PhilHealth, Pag-IBIG), runs pre-flight checks and queues it for approval.",
		Steps: []string{
			"Keep this year's filing calendar up to date",
			"Prepare returns due within 10 days once their period closes",
			"Prepare new-hire registrations and expanded withholding (0619-E, 1601-EQ)",
		},
		HardLimits:      []string{"Nothing is submitted to an agency without approval", "CAPTCHAs are never solved automatically"},
		DefaultAutonomy: Approve, MaxAutonomy: Approve,
	},
}

// Lookup returns a built-in routine.
func Lookup(key string) (Definition, bool) {
	for _, d := range Catalog {
		if d.Key == key {
			return d, true
		}
	}
	return Definition{}, false
}

// Action statuses.
const (
	ActDone     = "done"     // the agent did it
	ActProposed = "proposed" // the agent would do it (recommend mode)
	ActPerson   = "person"   // left for a person
	ActSkipped  = "skipped"  // not applicable
	ActFailed   = "failed"
	ActWaiting  = "waiting" // scheduled / waiting on a date or an agency
)

// Action is one line of a run log.
type Action struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Rationale string `json:"rationale,omitempty"`
	Link      string `json:"link,omitempty"`
}

// Run is the outcome of one routine execution.
type Run struct {
	Trigger     string
	SubjectType string
	SubjectID   int64
	SubjectName string
	Actions     []Action
}

// Setting is a company's configuration of a routine.
type Setting struct {
	Enabled      bool   `json:"enabled"`
	Autonomy     string `json:"autonomy"`
	Instructions string `json:"instructions"`
}

// Service runs and records routines.
type Service struct {
	q      *store.Queries
	logger *slog.Logger
	now    func() time.Time
}

// NewService builds the routines service.
func NewService(q *store.Queries, logger *slog.Logger) *Service {
	return &Service{q: q, logger: logger, now: time.Now}
}

// Setting returns a company's setting for a routine (defaults when unset).
func (s *Service) Setting(ctx context.Context, companyID int64, key string) Setting {
	def, _ := Lookup(key)
	st := Setting{Enabled: true, Autonomy: def.DefaultAutonomy}
	if s.q == nil {
		return st
	}
	row, err := s.q.GetAgentRoutineSetting(ctx, store.GetAgentRoutineSettingParams{CompanyID: companyID, RoutineKey: key})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("routines: load setting", "company_id", companyID, "routine", key, "error", err)
		}
		return st
	}
	return Setting{Enabled: row.Enabled, Autonomy: clampAutonomy(def, row.Autonomy), Instructions: row.Instructions}
}

func clampAutonomy(def Definition, a string) string {
	rank := map[string]int{Recommend: 0, Approve: 1, Autonomous: 2}
	if _, ok := rank[a]; !ok {
		return def.DefaultAutonomy
	}
	if def.MaxAutonomy != "" && rank[a] > rank[def.MaxAutonomy] {
		return def.MaxAutonomy
	}
	return a
}

// Record stores a run. The status is derived from the actions: anything
// left for a person (or merely proposed) needs attention.
func (s *Service) Record(ctx context.Context, companyID int64, key, autonomy string, run Run) (*store.AgentRoutineRun, error) {
	status, summary := Summarize(run.Actions)
	raw, err := json.Marshal(run.Actions)
	if err != nil {
		return nil, err
	}
	row, err := s.q.InsertAgentRoutineRun(ctx, store.InsertAgentRoutineRunParams{
		CompanyID:   companyID,
		RoutineKey:  key,
		Trigger:     truncate(run.Trigger, 200),
		SubjectType: run.SubjectType,
		SubjectID:   run.SubjectID,
		SubjectName: truncate(run.SubjectName, 200),
		Autonomy:    autonomy,
		Status:      status,
		Summary:     summary,
		Actions:     raw,
	})
	if err != nil {
		s.logger.Error("routines: record run", "company_id", companyID, "routine", key, "error", err)
		return nil, err
	}
	return &row, nil
}

// Summarize derives a run's status and one-line summary from its actions.
func Summarize(actions []Action) (status, summary string) {
	var done, person, proposed, failed int
	for _, a := range actions {
		switch a.Status {
		case ActDone:
			done++
		case ActPerson:
			person++
		case ActProposed:
			proposed++
		case ActFailed:
			failed++
		}
	}
	status = "completed"
	if person > 0 || proposed > 0 {
		status = "needs_attention"
	}
	if failed > 0 && done == 0 && person == 0 && proposed == 0 {
		status = "failed"
	}
	var parts []string
	if done > 0 {
		parts = append(parts, plural(done, "thing done", "things done"))
	}
	if proposed > 0 {
		parts = append(parts, plural(proposed, "recommendation", "recommendations"))
	}
	if person > 0 {
		parts = append(parts, plural(person, "item left for you", "items left for you"))
	}
	if failed > 0 {
		parts = append(parts, plural(failed, "step failed", "steps failed"))
	}
	if len(parts) == 0 {
		parts = append(parts, "Nothing to do")
	}
	return status, strings.Join(parts, " · ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
