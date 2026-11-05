package govfiling

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/halaostory/halaos/internal/ai/provider"
)

// Issue is one finding of the pre-flight review.
type Issue struct {
	Severity string `json:"severity"` // error (blocks approval), warning, info
	Code     string `json:"code"`
	Message  string `json:"message"`
	Source   string `json:"source"` // rules | ai
}

// Preflight is the stored review result.
type Preflight struct {
	Issues    []Issue   `json:"issues"`
	Blocking  bool      `json:"blocking"`
	AISummary string    `json:"ai_summary,omitempty"`
	AIRisk    string    `json:"ai_risk,omitempty"` // low, medium, high
	CheckedAt time.Time `json:"checked_at"`
}

func (p *Preflight) add(sev, code, msg string) {
	p.Issues = append(p.Issues, Issue{Severity: sev, Code: code, Message: msg, Source: "rules"})
	if sev == "error" {
		p.Blocking = true
	}
}

// RuleInput is everything the deterministic checks look at.
type RuleInput struct {
	Spec        FormSpec
	Period      Period
	Dataset     *Dataset
	PrevTotals  map[string]float64 // previous period, if known
	Channel     Channel
	HasPlaybook bool
	PlaybookOK  bool // verified
	HasAccount  bool // portal credentials stored (browser/api)
	DueDate     *time.Time
	Today       time.Time
}

var employerIDFor = map[string][2]string{
	"bir_1601c":            {"tin", "company TIN"},
	"bir_1604c":            {"tin", "company TIN"},
	"sss_r3":               {"sss_er_no", "SSS employer number"},
	"philhealth_rf1":       {"philhealth_er_no", "PhilHealth employer number"},
	"pagibig_mcrf":         {"pagibig_er_no", "Pag-IBIG employer ID"},
	"sss_r1a":              {"sss_er_no", "SSS employer number"},
	"philhealth_er2":       {"philhealth_er_no", "PhilHealth employer number"},
	"pagibig_registration": {"pagibig_er_no", "Pag-IBIG employer ID"},
	"bir_1902":             {"tin", "company TIN"},
	"bir_0619e":            {"tin", "company TIN"},
	"bir_1601eq":           {"tin", "company TIN"},
}

// expected digit counts for member IDs (min, max)
var memberIDDigits = map[string][2]int{
	"bir_1601c":            {9, 14},
	"bir_1604c":            {9, 14},
	"sss_r3":               {10, 10},
	"philhealth_rf1":       {12, 12},
	"pagibig_mcrf":         {12, 12},
	"sss_r1a":              {10, 10},
	"philhealth_er2":       {12, 12},
	"pagibig_registration": {12, 12},
	"bir_0619e":            {9, 14},
	"bir_1601eq":           {9, 14},
}

// RunRules performs the deterministic pre-flight checks.
func RunRules(in RuleInput) *Preflight {
	p := &Preflight{Issues: []Issue{}, CheckedAt: in.Today}
	ds := in.Dataset

	if in.Period.State == "" && !in.Spec.Registration && !dateOnly(in.Period.End()).Before(dateOnly(in.Today)) {
		p.add("error", "period_open", fmt.Sprintf("Period %s has not ended yet.", ds.Period))
	}
	if in.DueDate != nil && dateOnly(*in.DueDate).Before(dateOnly(in.Today)) {
		p.add("warning", "past_due", fmt.Sprintf("Due date %s has passed — penalties and interest may apply.", in.DueDate.Format("2006-01-02")))
	}
	if req, ok := employerIDFor[in.Spec.Code]; ok && ds.EmployerIDs[req[0]] == "" {
		p.add("error", "missing_employer_id", fmt.Sprintf("Missing %s — add it under Settings → Company.", req[1]))
	}

	if ds.Columns != nil { // per-employee returns
		if len(ds.Lines) == 0 {
			if in.Spec.Registration {
				p.add("error", "no_data", "No new employees to report for this hire month.")
			} else {
				if in.Spec.Code == "bir_0619e" || in.Spec.Code == "bir_1601eq" {
					p.add("error", "no_data", "No paid contractor payments with withholding in this period.")
				} else {
					p.add("error", "no_data", "No payroll data for this period. Run and approve payroll first.")
				}
			}
		}
		var missing []string
		seen := map[string]string{}
		var dup []string
		for _, l := range ds.Lines {
			if ds.MemberIDLabel == "" {
				continue // e.g. BIR 1902 lists exactly the employees without a TIN
			}
			id := digits(l.MemberID)
			if id == "" {
				missing = append(missing, l.Name)
				continue
			}
			if rng, ok := memberIDDigits[in.Spec.Code]; ok && (len(id) < rng[0] || len(id) > rng[1]) {
				p.add("warning", "member_id_format", fmt.Sprintf("%s: %s %q does not look valid.", l.Name, ds.MemberIDLabel, l.MemberID))
			}
			if prev, ok := seen[id]; ok {
				dup = append(dup, prev+" / "+l.Name)
			}
			seen[id] = l.Name
			for k, v := range l.Amounts {
				if v < 0 {
					p.add("error", "negative_amount", fmt.Sprintf("%s has a negative %s (%.2f).", l.Name, k, v))
				}
			}
		}
		if len(missing) > 0 {
			msg := fmt.Sprintf("%d employee(s) have no %s: %s.", len(missing), ds.MemberIDLabel, listSome(missing, 5))
			if in.Spec.Registration {
				msg += " Send them the onboarding link so they can enter it."
			}
			if in.Spec.MemberIDOptional {
				p.add("warning", "missing_member_id", msg+" They will be registered as new members.")
			} else {
				p.add("error", "missing_member_id", msg)
			}
		}
		if len(dup) > 0 {
			p.add("error", "duplicate_member_id", "Duplicate "+ds.MemberIDLabel+": "+listSome(dup, 3)+".")
		}
	}

	if in.PrevTotals != nil {
		for k, cur := range ds.Totals {
			prev := in.PrevTotals[k]
			if prev > 0 && math.Abs(cur-prev)/prev > 0.30 {
				p.add("warning", "large_change", fmt.Sprintf("%s changed %.0f%% vs previous period (%.2f → %.2f).", k, (cur-prev)/prev*100, prev, cur))
			}
		}
	}

	switch in.Channel {
	case ChannelBrowser:
		if !in.HasPlaybook {
			p.add("error", "no_playbook", "No browser playbook is configured for this portal. Use the manual channel or configure and dry-run a playbook first.")
		} else if !in.PlaybookOK {
			p.add("warning", "playbook_unverified", "The portal playbook has not been verified yet — run a dry run first; the agent will stop before the final submit step.")
		}
		if !in.HasAccount {
			p.add("error", "no_portal_account", "No portal credentials stored for "+in.Spec.Portal+".")
		}
	case ChannelAPI:
		if !in.HasAccount {
			p.add("error", "no_api_account", "No API / filing-partner connection configured for "+in.Spec.Portal+".")
		}
	}
	if in.Spec.Code != "mock_return" && in.Spec.Code != "irs_941" && in.Spec.Code != "bir_1902" {
		p.add("info", "agency_validation", "File generated by HalaPeople from payroll. If the portal rejects the layout, run it through the agency's validation tool.")
	}
	return p
}

func listSome(xs []string, n int) string {
	if len(xs) <= n {
		return strings.Join(xs, ", ")
	}
	return strings.Join(xs[:n], ", ") + fmt.Sprintf(" and %d more", len(xs)-n)
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// AI review
// ---------------------------------------------------------------------------

// AIReviewer adds a narrative review on top of the rules. Only aggregates are
// sent to the model — never names, government ID numbers or per-employee pay.
type AIReviewer struct {
	Provider provider.Provider
}

type aiFacts struct {
	Form         string             `json:"form"`
	Agency       string             `json:"agency"`
	Country      string             `json:"country"`
	Period       string             `json:"period"`
	Employees    int                `json:"employee_lines"`
	Totals       map[string]float64 `json:"totals"`
	PrevTotals   map[string]float64 `json:"previous_period_totals,omitempty"`
	RuleFindings []string           `json:"rule_findings"`
	Channel      string             `json:"channel"`
	DaysUntilDue *int               `json:"days_until_due,omitempty"`
}

type aiOut struct {
	Summary string `json:"summary"`
	Risk    string `json:"risk"`
	Issues  []struct {
		Severity string `json:"severity"`
		Message  string `json:"message"`
	} `json:"issues"`
}

const aiSystem = `You are a payroll compliance reviewer for Southeast Asia and US statutory filings.
You receive only aggregate facts about one return that is about to be submitted to a government agency.
Check for anything a careful payroll accountant would question before filing (implausible totals, ratios between
contribution shares that look off for the agency's rules, large swings, timing risk). Do not invent facts.
Respond with JSON only: {"summary": "<2 sentences for the HR approver>", "risk": "low|medium|high",
"issues": [{"severity": "warning|info", "message": "..."}]}`

// Review runs the model and merges its findings into p. AI findings can
// never block approval on their own; they are warnings/info for the human.
func (r *AIReviewer) Review(ctx context.Context, in RuleInput, p *Preflight) error {
	if r == nil || r.Provider == nil {
		return nil
	}
	f := aiFacts{Form: in.Spec.Name, Agency: in.Spec.Agency, Country: in.Spec.Country, Period: in.Dataset.Period,
		Employees: len(in.Dataset.Lines), Totals: in.Dataset.Totals, PrevTotals: in.PrevTotals, Channel: string(in.Channel)}
	if in.DueDate != nil {
		d := int(dateOnly(*in.DueDate).Sub(dateOnly(in.Today)).Hours() / 24)
		f.DaysUntilDue = &d
	}
	for _, is := range p.Issues {
		if is.Severity != "info" {
			f.RuleFindings = append(f.RuleFindings, is.Code)
		}
	}
	body, _ := json.Marshal(f)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	resp, err := r.Provider.Generate(ctx, provider.Request{
		System:    aiSystem,
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: string(body)}},
		MaxTokens: 600,
	})
	if err != nil {
		return err
	}
	out, err := parseAIJSON(resp.Content)
	if err != nil {
		return err
	}
	p.AISummary = strings.TrimSpace(out.Summary)
	switch out.Risk {
	case "low", "medium", "high":
		p.AIRisk = out.Risk
	}
	for _, is := range out.Issues {
		sev := "info"
		if is.Severity == "warning" || is.Severity == "error" { // AI never blocks; surface as warning
			sev = "warning"
		}
		if strings.TrimSpace(is.Message) != "" {
			p.Issues = append(p.Issues, Issue{Severity: sev, Code: "ai_review", Message: is.Message, Source: "ai"})
		}
	}
	return nil
}

func parseAIJSON(raw string) (*aiOut, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out aiOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse ai review: %w", err)
	}
	return &out, nil
}
