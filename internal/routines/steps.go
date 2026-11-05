package routines

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
)

// Plain-language steps (Warp: routines "can be written in plain language").
// Each line of a routine's instructions is read as one step. At the
// "autonomous" level the agent carries out the two kinds of step it can do
// safely and reversibly — add a checklist task, or notify someone — and
// logs them with an undo; anything else (and every step at lower levels)
// is left for a person.

// StepPlan is how the agent reads one instruction line.
type StepPlan struct {
	Kind    string `json:"kind"`     // task, notify, slack, none
	Target  string `json:"target"`   // hr, manager, employee
	Text    string `json:"text"`     // task title or message
	DueDays int    `json:"due_days"` // task due, days from the reference date
}

var (
	reNotify  = regexp.MustCompile(`(?i)^(notify|tell|remind|message|ping|let|inform|email|e-mail|send (a )?(note|message|reminder) to)\b`)
	reDueDays = regexp.MustCompile(`(?i)\b(?:within|in|by day|on day|before day)\s+(\d{1,3})\s*(?:days?)?\b`)
	// "Notify the manager to …" → message "…"
	reNotifyLead = regexp.MustCompile(`(?i)^(notify|tell|remind|message|ping|let|inform|email|e-mail)\s+(the\s+|our\s+)?(manager|team lead|supervisor|boss|employee|new hire|newcomer|hr|hr team|admins?|them|him|her)?\s*(know\s+)?(to|that|about|:)?\s+`)
	// "Post in #general that …", "Announce in Slack …", "Post in the
	// Telegram group …", "Share in the team chat …" — kind "slack" is the
	// company's team chat, whichever platform it is.
	reSlack     = regexp.MustCompile(`(?i)\b(slack|ms teams|microsoft teams|google chat|lark|feishu|telegram|line group|team chat|group chat)\b|^(post|announce|share|shout ?out)\b.*\b(in|on|to)\s+(#[\w-]+|(the\s+)?teams\b)`)
	reSlackLead = regexp.MustCompile(`(?i)^(post|announce|share|send|write|shout ?out)\s+(a\s+)?(message\s+|welcome\s+|note\s+)?(in|on|to)\s+(the\s+|our\s+)?(slack|ms teams|microsoft teams|teams|google chat|lark|feishu|telegram|line|team chat|group chat|#[\w-]+)(\s+(channel|group|chat|space))?\s*(that|:|saying|to say|about)?\s*`)
	reFirstDay  = regexp.MustCompile(`(?i)\b(first day|day one|day 1|on day one)\b`)
	reManager   = regexp.MustCompile(`(?i)\b(manager|team lead|supervisor|boss)\b`)
	reEmployee  = regexp.MustCompile(`(?i)\b(employee|new hire|newcomer|them|the hire|leaver)\b`)
)

// RuleInterpret reads a step without AI.
func RuleInterpret(line string) StepPlan {
	p := StepPlan{Kind: "task", Target: "hr", Text: line}
	if m := reDueDays.FindStringSubmatch(line); m != nil {
		p.DueDays, _ = strconv.Atoi(m[1])
	} else if reFirstDay.MatchString(line) {
		p.DueDays = 0
	}
	switch {
	case reSlack.MatchString(line):
		p.Kind = "slack"
		if m := reSlackLead.FindStringIndex(line); m != nil && m[1] < len(line) {
			rest := strings.TrimSpace(line[m[1]:])
			p.Text = strings.ToUpper(rest[:1]) + rest[1:]
		}
	case reNotify.MatchString(line):
		p.Kind = "notify"
		if m := reNotifyLead.FindStringSubmatchIndex(line); m != nil && m[1] < len(line) {
			rest := strings.TrimSpace(line[m[1]:])
			p.Text = strings.ToUpper(rest[:1]) + rest[1:]
		}
		switch {
		case reManager.MatchString(line):
			p.Target = "manager"
		case reEmployee.MatchString(line):
			p.Target = "employee"
		}
	case reManager.MatchString(line):
		p.Target = "manager"
	}
	// Steps that need a tool the agent doesn't have (purchasing,
	// booking) become checklist tasks, so they are still tracked.
	return p
}

const stepSystem = `You turn one line of an HR team's routine instructions into a structured step.
The routine runs for one employee (a new hire or a leaver). Choose:
- "task": add a checklist task (text = short task title; target = who does it: hr, manager or employee; due_days = days after the start/last day, 0 if unspecified)
- "notify": send an in-app message (text = the message to send; target = manager, employee or hr)
- "slack": post a message in the company's team chat — Slack, Microsoft Teams, Google Chat, Lark, Telegram or LINE group (text = the message, written for the whole team)
- "none": the line is not an actionable step.
Respond with JSON only: {"kind":"task|notify|slack|none","target":"hr|manager|employee","text":"...","due_days":0}`

// AIInterpret asks the model; it falls back to the rule reading on error.
func AIInterpret(ctx context.Context, p provider.Provider, routine, subject, line string) StepPlan {
	base := RuleInterpret(line)
	if p == nil {
		return base
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := p.Generate(ctx, provider.Request{
		System:    stepSystem,
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: fmt.Sprintf("Routine: %s\nEmployee: %s\nInstruction: %s", routine, subject, line)}},
		MaxTokens: 200,
	})
	if err != nil {
		return base
	}
	raw := strings.TrimSpace(resp.Content)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out StepPlan
	if json.Unmarshal([]byte(raw), &out) != nil {
		return base
	}
	switch out.Kind {
	case "task", "notify", "slack", "none":
	default:
		return base
	}
	switch out.Target {
	case "hr", "manager", "employee":
	default:
		out.Target = base.Target
	}
	out.Text = strings.TrimSpace(out.Text)
	if out.Text == "" {
		out.Text = line
	}
	if len(out.Text) > 240 {
		out.Text = out.Text[:240]
	}
	if out.DueDays < 0 || out.DueDays > 365 {
		out.DueDays = base.DueDays
	}
	return out
}

// stepContext is who a routine run is about.
type stepContext struct {
	companyID int64
	employee  *store.Employee
	workflow  string    // onboarding / offboarding
	ref       time.Time // start date or last working day
	routine   string
	autonomy  string
}

// planSteps turns the routine's instructions into run-log actions,
// carrying them out at the autonomous level.
func (e *Engine) planSteps(ctx context.Context, sc stepContext, instructions, subject string) []Action {
	lines := instructionLines(instructions)
	if len(lines) == 0 {
		return nil
	}
	if sc.autonomy != Autonomous || sc.employee == nil {
		return customSteps(instructions, subject)
	}
	var out []Action
	for _, line := range lines {
		plan := AIInterpret(ctx, e.AI, sc.routine, subject, line)
		out = append(out, e.runStep(ctx, sc, plan, line, subject))
	}
	return out
}

func instructionLines(instructions string) []string {
	var lines []string
	for _, line := range strings.Split(instructions, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*•0123456789.) "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (e *Engine) runStep(ctx context.Context, sc stepContext, plan StepPlan, line, subject string) Action {
	why := "Your routine instruction: “" + line + "”"
	switch plan.Kind {
	case "notify":
		ids := e.notify(ctx, sc, plan.Target, plan.Text, subject)
		if len(ids) == 0 {
			return Action{Key: "custom", Title: line, Status: ActPerson, Rationale: "No one to notify (" + plan.Target + " has no login). " + why}
		}
		return Action{Key: "custom_notify", Title: fmt.Sprintf("Told %s: %s", targetLabel(plan.Target), plan.Text), Status: ActDone,
			Rationale: why, Undo: &Undo{Kind: UndoRetractNotification, ID: ids[0], IDs: ids}}
	case "slack":
		poster, ok := e.alerts.(SlackPoster)
		if !ok || e.alerts == nil {
			return Action{Key: "custom", Title: line, Status: ActPerson, Rationale: "Connect a team chat (Routines → Team chat alerts) so the agent can post it. " + why}
		}
		name := "the team chat"
		if n, ok := e.alerts.(ChannelNamer); ok {
			if v := n.ChannelName(ctx, sc.companyID); v != "" {
				name = v
			}
		}
		if err := poster.PostMessage(ctx, sc.companyID, plan.Text, ""); err != nil {
			return Action{Key: "custom", Title: line, Status: ActPerson, Rationale: "No team chat is connected, or it rejected the message — post it yourself, or connect one under Routines. " + why}
		}
		return Action{Key: "custom_slack", Title: "Posted in " + name + ": " + plan.Text, Status: ActDone,
			Rationale: why + " (Chat messages can't be withdrawn from here.)"}
	case "task":
		role := plan.Target
		due := pgtype.Date{Time: sc.ref.AddDate(0, 0, plan.DueDays), Valid: true}
		desc := "Added by the " + sc.routine + " routine from: " + line
		task, err := e.q.CreateOnboardingTask(ctx, store.CreateOnboardingTaskParams{
			CompanyID: sc.companyID, EmployeeID: sc.employee.ID, WorkflowType: sc.workflow, Title: plan.Text,
			Description: &desc, IsRequired: false, AssigneeRole: &role, DueDate: due, SortOrder: 900,
		})
		if err != nil {
			return Action{Key: "custom", Title: line, Status: ActPerson, Rationale: "Could not add the task: " + err.Error()}
		}
		return Action{Key: "custom_task", Title: fmt.Sprintf("Added task for %s: %s (due %s)", targetLabel(role), plan.Text, due.Time.Format("Jan 2")),
			Status: ActDone, Rationale: why, Link: "/onboarding", Undo: &Undo{Kind: UndoDeleteTask, ID: task.ID}}
	default:
		return Action{Key: "custom", Title: line, Status: ActSkipped, Rationale: "Not an actionable step. " + why}
	}
}

func targetLabel(t string) string {
	switch t {
	case "manager":
		return "the manager"
	case "employee":
		return "the employee"
	default:
		return "HR"
	}
}

// notify sends an in-app notification and returns the notification IDs.
func (e *Engine) notify(ctx context.Context, sc stepContext, target, text, subject string) []int64 {
	var users []int64
	switch target {
	case "employee":
		if sc.employee.UserID != nil {
			users = append(users, *sc.employee.UserID)
		}
	case "manager":
		if sc.employee.ManagerID != nil {
			if m, err := e.q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: *sc.employee.ManagerID, CompanyID: sc.companyID}); err == nil && m.UserID != nil {
				users = append(users, *m.UserID)
			}
		}
	default:
		if admins, err := e.q.ListAdminUsersByCompany(ctx, sc.companyID); err == nil {
			for _, a := range admins {
				users = append(users, a.ID)
			}
		}
	}
	var ids []int64
	for _, u := range users {
		n, err := e.q.CreateNotification(ctx, store.CreateNotificationParams{
			CompanyID: sc.companyID, UserID: u, Category: sc.workflow, Title: subject, Message: text,
		})
		if err == nil {
			ids = append(ids, n.ID)
			notification.Push(sc.companyID, u, subject, text, sc.workflow)
		}
	}
	return ids
}
