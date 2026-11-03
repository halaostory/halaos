package routines

import (
	"context"
	"errors"
	"testing"

	"github.com/halaostory/halaos/internal/ai/provider"
)

func TestRuleInterpretMessage(t *testing.T) {
	for line, want := range map[string]string{
		"Notify the manager to plan the first week":   "Plan the first week",
		"Remind the new hire that IDs are due Friday": "IDs are due Friday",
		"Tell HR: book the welcome lunch":             "Book the welcome lunch",
	} {
		if got := RuleInterpret(line).Text; got != want {
			t.Errorf("%q → %q want %q", line, got, want)
		}
	}
}

func TestRuleInterpret(t *testing.T) {
	cases := []struct {
		line   string
		kind   string
		target string
		due    int
	}{
		{"Order a laptop from IT", "task", "hr", 0},
		{"Notify the manager to plan the first week", "notify", "manager", 0},
		{"Remind the new hire to bring two IDs", "notify", "employee", 0},
		{"Tell HR to book the welcome lunch", "notify", "hr", 0},
		{"Manager schedules a 30-60-90 plan within 7 days", "task", "manager", 7},
		{"Collect the ID card on day 1", "task", "hr", 1},
	}
	for _, c := range cases {
		p := RuleInterpret(c.line)
		if p.Kind != c.kind || p.Target != c.target || p.DueDays != c.due || (p.Kind == "task" && p.Text != c.line) {
			t.Errorf("%q → %+v", c.line, p)
		}
	}
}

type stepAI struct{ reply string }

func (s stepAI) Name() string                 { return "fake" }
func (s stepAI) Health(context.Context) error { return nil }
func (s stepAI) Generate(context.Context, provider.Request) (*provider.Response, error) {
	return &provider.Response{Content: s.reply}, nil
}
func (s stepAI) Stream(ctx context.Context, r provider.Request, _ func(provider.StreamChunk)) (*provider.Response, error) {
	return s.Generate(ctx, r)
}

func TestAIInterpret(t *testing.T) {
	p := AIInterpret(context.Background(), stepAI{`Sure: {"kind":"notify","target":"manager","text":"Ana starts Monday — please plan her first week","due_days":0}`}, "New hire", "Ana", "let her boss know")
	if p.Kind != "notify" || p.Target != "manager" || p.Text == "let her boss know" {
		t.Errorf("%+v", p)
	}
	// unusable answers fall back to the rules
	for _, bad := range []string{"no json here", `{"kind":"delete_everything"}`} {
		if q := AIInterpret(context.Background(), stepAI{bad}, "New hire", "Ana", "Order a laptop"); q.Kind != "task" || q.Text != "Order a laptop" {
			t.Errorf("%q → %+v", bad, q)
		}
	}
	if q := AIInterpret(context.Background(), stepAI{`{"kind":"task","target":"ceo","text":"x","due_days":9999}`}, "New hire", "Ana", "Order a laptop within 3 days"); q.Target != "hr" || q.DueDays != 3 {
		t.Errorf("clamping: %+v", q)
	}
}

func TestInstructionLines(t *testing.T) {
	got := instructionLines("- a\n\n 2) b \n* c")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("%q", got)
	}
}

func TestRuleInterpretSlack(t *testing.T) {
	for line, want := range map[string]string{
		"Post in Slack that a new teammate joins Monday":      "A new teammate joins Monday",
		"Announce in #general: welcome our new designer":      "Welcome our new designer",
		"Share in #team-ops that the laptop has been ordered": "The laptop has been ordered",
	} {
		p := RuleInterpret(line)
		if p.Kind != "slack" || p.Text != want {
			t.Errorf("%q → %+v, want slack %q", line, p, want)
		}
	}
	if p := RuleInterpret("Notify the manager to plan the first week"); p.Kind != "notify" {
		t.Errorf("notify step read as %s", p.Kind)
	}
}

type fakeSlack struct {
	posted []string
	err    error
}

func (f *fakeSlack) Notify(int64, string, string, string) {}
func (f *fakeSlack) PostMessage(_ context.Context, _ int64, text, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.posted = append(f.posted, text)
	return nil
}

func TestRunSlackStep(t *testing.T) {
	ctx := context.Background()
	plan := StepPlan{Kind: "slack", Text: "Welcome Ana to the team!"}
	sc := stepContext{companyID: 1, routine: "new_hire", autonomy: Autonomous}

	e := &Engine{Service: &Service{}}
	if a := e.runStep(ctx, sc, plan, "Post in Slack: welcome Ana", "Ana Cruz"); a.Status != ActPerson {
		t.Errorf("without Slack the step is left for a person: %+v", a)
	}
	slack := &fakeSlack{}
	e.SetNotifier(slack)
	a := e.runStep(ctx, sc, plan, "Post in Slack: welcome Ana", "Ana Cruz")
	if a.Status != ActDone || len(slack.posted) != 1 || slack.posted[0] != "Welcome Ana to the team!" || a.Undo != nil {
		t.Fatalf("posted: %+v %v", a, slack.posted)
	}
	slack.err = errors.New("not connected")
	if a := e.runStep(ctx, sc, plan, "Post in Slack: welcome Ana", "Ana Cruz"); a.Status != ActPerson {
		t.Errorf("a failed post is left for a person: %+v", a)
	}
}
