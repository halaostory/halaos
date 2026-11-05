package browser

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/halaostory/halaos/internal/govfiling/mockportal"
)

type fakeHooks struct {
	mu       sync.Mutex
	logs     []string
	evidence []string
	answers  map[string]string
	asked    []string
}

func (f *fakeHooks) Log(i int, a Action, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, string(a)+": "+msg)
}
func (f *fakeHooks) Evidence(name string, png []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(png) == 0 {
		return "", errors.New("empty screenshot")
	}
	f.evidence = append(f.evidence, name)
	return name, nil
}
func (f *fakeHooks) AskHuman(ctx context.Context, name, prompt string, png []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, name)
	if len(png) == 0 {
		return "", errors.New("human must be shown a screenshot")
	}
	a, ok := f.answers[name]
	if !ok {
		return "", context.DeadlineExceeded
	}
	return a, nil
}

func chromePath(t *testing.T) string {
	for _, p := range []string{os.Getenv("CHROME_PATH"), "/opt/pw-browsers/chromium-1243/chrome-linux64/chrome", "/usr/bin/chromium", "/usr/bin/google-chrome"} {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no Chrome/Chromium available")
	return ""
}

func mockPlaybook(base string) Playbook {
	return Playbook{
		ID: "mock.return", Agency: "MOCK", Portal: "Mock eFiling", StartURL: base + "/",
		RequiredOutputs: []string{"reference_no"},
		Steps: []Step{
			{Action: ActFill, Selector: "#username", Value: "{{.Creds.username}}"},
			{Action: ActFill, Selector: "#password", Value: "{{.Creds.password}}"},
			{Action: ActHumanInput, Name: "captcha", Prompt: "Type the CAPTCHA characters"},
			{Action: ActFill, Selector: "#captcha", Value: "{{.Input.captcha}}"},
			{Action: ActClick, Selector: "#login-btn"},
			{Action: ActHumanInput, Name: "otp", Prompt: "Enter the OTP sent to the registered mobile"},
			{Action: ActFill, Selector: "#otp", Value: "{{.Input.otp}}"},
			{Action: ActClick, Selector: "#otp-btn"},
			{Action: ActSelect, Selector: "#period", Value: "{{.Data.period}}"},
			{Action: ActUpload, Selector: "#file"},
			{Action: ActClick, Selector: "#submit-btn", Final: true},
			{Action: ActExtract, Selector: "#confirmation", Name: "reference_no", Pattern: `Reference No:\s*(\S+)`},
			{Action: ActScreenshot, Name: "confirmation"},
		},
	}
}

func TestRunner_MockPortalEndToEnd(t *testing.T) {
	exe := chromePath(t)
	portal := mockportal.New()
	srv := httptest.NewServer(portal.Handler())
	defer srv.Close()

	file := filepath.Join(t.TempDir(), "1601C_2026-10.dat")
	if err := os.WriteFile(file, []byte("H1601C,123456789000,2026-10\nD,...\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewRunner()
	r.ExecPath = exe
	r.StepTimeout = 15 * time.Second
	hooks := &fakeHooks{answers: map[string]string{"captcha": mockportal.Captcha, "otp": mockportal.OTP}}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	res, err := r.Run(ctx, mockPlaybook(srv.URL), Vars{
		Creds: map[string]string{"username": mockportal.Username, "password": mockportal.Password},
		Data:  map[string]string{"period": "2026-10"},
		File:  file,
	}, hooks, Options{})
	if err != nil {
		t.Fatalf("run failed: %v\nlogs:\n%s", err, strings.Join(hooks.logs, "\n"))
	}
	if res.Outputs["reference_no"] != "MOCK-000001" {
		t.Errorf("reference = %q", res.Outputs["reference_no"])
	}
	if len(portal.Received) != 1 || portal.Received[0].Period != "2026-10" || portal.Received[0].Filename != "1601C_2026-10.dat" {
		t.Errorf("portal received %+v", portal.Received)
	}
	if strings.Join(hooks.asked, ",") != "captcha,otp" {
		t.Errorf("human was asked %v", hooks.asked)
	}
	for _, l := range hooks.logs {
		for _, secret := range []string{mockportal.Password, mockportal.OTP} {
			if strings.Contains(l, secret) {
				t.Errorf("secret leaked into log: %s", l)
			}
		}
	}
	if len(res.Evidence) == 0 {
		t.Error("expected confirmation screenshot evidence")
	}
}

func TestRunner_NoHumanAnswerFailsSafely(t *testing.T) {
	exe := chromePath(t)
	portal := mockportal.New()
	srv := httptest.NewServer(portal.Handler())
	defer srv.Close()
	r := NewRunner()
	r.ExecPath = exe
	r.HumanWait = 2 * time.Second
	hooks := &fakeHooks{answers: map[string]string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := r.Run(ctx, mockPlaybook(srv.URL), Vars{
		Creds: map[string]string{"username": mockportal.Username, "password": mockportal.Password},
		Data:  map[string]string{"period": "2026-10"}, File: "/nonexistent",
	}, hooks, Options{})
	if err == nil {
		t.Fatal("expected failure when nobody answers the CAPTCHA")
	}
	if len(portal.Received) != 0 {
		t.Error("nothing must be submitted")
	}
}

func TestRunner_WrongCaptchaDoesNotProduceReference(t *testing.T) {
	exe := chromePath(t)
	portal := mockportal.New()
	srv := httptest.NewServer(portal.Handler())
	defer srv.Close()
	r := NewRunner()
	r.ExecPath = exe
	r.StepTimeout = 5 * time.Second
	hooks := &fakeHooks{answers: map[string]string{"captcha": "WRONG", "otp": mockportal.OTP}}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := r.Run(ctx, mockPlaybook(srv.URL), Vars{
		Creds: map[string]string{"username": mockportal.Username, "password": mockportal.Password},
		Data:  map[string]string{"period": "2026-10"}, File: "/tmp/x",
	}, hooks, Options{})
	if err == nil {
		t.Fatal("expected failure on wrong captcha")
	}
}

func TestPlaybookValidate(t *testing.T) {
	bad := []Playbook{
		{ID: "x"},
		{ID: "x", Steps: []Step{{Action: "teleport"}}},
		{ID: "x", Steps: []Step{{Action: ActFill}}},
		{ID: "x", Steps: []Step{{Action: ActExtract, Selector: "#a"}}},
		{ID: "x", Steps: []Step{{Action: ActHumanInput, Name: "otp"}}},
		{ID: "x", Steps: []Step{{Action: ActExtract, Selector: "#a", Name: "n", Pattern: "("}}},
		{ID: "x", Steps: []Step{{Action: ActClick, Selector: "#a"}}}, // no final step
	}
	for i, p := range bad {
		if p.Validate() == nil {
			t.Errorf("case %d should be invalid", i)
		}
	}
	if err := mockPlaybook("http://x").Validate(); err != nil {
		t.Errorf("mock playbook invalid: %v", err)
	}
}

func TestRenderMissingKeyFails(t *testing.T) {
	if _, err := render("{{.Creds.username}}", Vars{Creds: map[string]string{}}); err == nil {
		t.Error("missing credential must fail instead of typing an empty string")
	}
}

func TestRunner_DryRunStopsBeforeSubmit(t *testing.T) {
	exe := chromePath(t)
	portal := mockportal.New()
	srv := httptest.NewServer(portal.Handler())
	defer srv.Close()
	file := filepath.Join(t.TempDir(), "r.dat")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	r := NewRunner()
	r.ExecPath = exe
	hooks := &fakeHooks{answers: map[string]string{"captcha": mockportal.Captcha, "otp": mockportal.OTP}}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := r.Run(ctx, mockPlaybook(srv.URL), Vars{
		Creds: map[string]string{"username": mockportal.Username, "password": mockportal.Password},
		Data:  map[string]string{"period": "2026-10"}, File: file,
	}, hooks, Options{DryRun: true})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if !res.StoppedBeforeFinal || len(portal.Received) != 0 {
		t.Fatalf("dry run must not submit: stopped=%v received=%d", res.StoppedBeforeFinal, len(portal.Received))
	}
}

type cancelHooks struct {
	fakeHooks
	cancelAt int
}

func (c *cancelHooks) BeforeStep(ctx context.Context, i int, s Step) error {
	if i >= c.cancelAt {
		return errors.New("cancelled by user")
	}
	return nil
}

func TestRunner_GuardStopsBeforeSubmit(t *testing.T) {
	exe := chromePath(t)
	portal := mockportal.New()
	srv := httptest.NewServer(portal.Handler())
	defer srv.Close()
	file := filepath.Join(t.TempDir(), "r.dat")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	r := NewRunner()
	r.ExecPath = exe
	pb := mockPlaybook(srv.URL)
	final := 0
	for i, s := range pb.Steps {
		if s.Final {
			final = i
		}
	}
	hooks := &cancelHooks{fakeHooks: fakeHooks{answers: map[string]string{"captcha": mockportal.Captcha, "otp": mockportal.OTP}}, cancelAt: final}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := r.Run(ctx, pb, Vars{Creds: map[string]string{"username": mockportal.Username, "password": mockportal.Password},
		Data: map[string]string{"period": "2026-10"}, File: file}, hooks, Options{})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if len(portal.Received) != 0 {
		t.Fatal("a cancelled run must never submit")
	}
}
