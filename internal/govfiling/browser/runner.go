package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Hooks connect a run to the outside world (log, evidence store, human handoff).
type Hooks interface {
	// Log records progress. Secret values are never passed here.
	Log(step int, action Action, msg string)
	// Evidence stores a screenshot and returns a reference to it.
	Evidence(name string, png []byte) (string, error)
	// AskHuman blocks until a human answers prompt (e.g. CAPTCHA text, OTP)
	// or ctx expires. The screenshot shows the human what the browser sees.
	AskHuman(ctx context.Context, name, prompt string, screenshot []byte) (string, error)
}

// Guard is optionally implemented by Hooks: it is consulted before every
// step (and therefore before the final submit) so a run can be aborted, e.g.
// when the submission was cancelled while the browser was waiting.
type Guard interface {
	BeforeStep(ctx context.Context, step int, s Step) error
}

// Result is what a successful run produced.
type Result struct {
	Outputs  map[string]string `json:"outputs"`
	Evidence []string          `json:"evidence"`
	// StoppedBeforeFinal is true for dry runs: everything up to the submit
	// step ran, nothing was filed.
	StoppedBeforeFinal bool `json:"stopped_before_final"`
}

// Options tune a single run.
type Options struct {
	DryRun bool // stop before the step marked Final
}

// ErrMissingOutput means the portal did not show what proves submission
// (e.g. a reference number), so the run must not be marked submitted.
var ErrMissingOutput = errors.New("required output not found")

// Runner executes playbooks in a headless Chromium.
type Runner struct {
	ExecPath    string        // Chrome/Chromium binary; empty = autodetect
	Headless    bool          // true in production
	StepTimeout time.Duration // default per-step timeout
	HumanWait   time.Duration // how long to wait for a human answer
}

// NewRunner returns a runner with sensible defaults. CHROME_PATH overrides
// the browser binary.
func NewRunner() *Runner {
	return &Runner{
		ExecPath:    os.Getenv("CHROME_PATH"),
		Headless:    true,
		StepTimeout: 30 * time.Second,
		HumanWait:   15 * time.Minute,
	}
}

// Run executes the playbook. The caller's ctx bounds the whole run.
func (r *Runner) Run(ctx context.Context, pb Playbook, vars Vars, hooks Hooks, opt Options) (*Result, error) {
	if err := pb.Validate(); err != nil {
		return nil, err
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", r.Headless),
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1366, 900),
	)
	if r.ExecPath != "" {
		opts = append(opts, chromedp.ExecPath(r.ExecPath))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	bctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	if err := chromedp.Run(bctx, network.Enable()); err != nil {
		return nil, fmt.Errorf("start browser: %w", err)
	}

	if vars.Input == nil {
		vars.Input = map[string]string{}
	}
	res := &Result{Outputs: map[string]string{}}
	snap := func(name string) []byte {
		var buf []byte
		// The page may still be navigating after a click: wait until the
		// document is ready, and retry a couple of times.
		for attempt := 0; attempt < 3 && len(buf) == 0; attempt++ {
			sctx, c := context.WithTimeout(bctx, 10*time.Second)
			_ = chromedp.Run(sctx, chromedp.WaitReady("body", chromedp.ByQuery))
			if err := chromedp.Run(sctx, chromedp.FullScreenshot(&buf, 80)); err != nil {
				buf = nil
			}
			c()
		}
		if len(buf) == 0 {
			return nil
		}
		if name != "" && hooks != nil {
			if ref, err := hooks.Evidence(name, buf); err == nil && ref != "" {
				res.Evidence = append(res.Evidence, ref)
			}
		}
		return buf
	}

	if pb.StartURL != "" {
		if err := r.do(bctx, chromedp.Navigate(pb.StartURL)); err != nil {
			snap("error-start")
			return res, fmt.Errorf("open %s: %w", pb.StartURL, err)
		}
	}

	for i, s := range pb.Steps {
		if g, ok := hooks.(Guard); ok {
			if err := g.BeforeStep(ctx, i, s); err != nil {
				return res, fmt.Errorf("stopped before step %d: %w", i, err)
			}
		}
		if s.Final && opt.DryRun {
			snap("dry-run-before-submit")
			log(hooks, i, s.Action, "dry run: stopped before final submit step")
			res.StoppedBeforeFinal = true
			return res, nil
		}
		if err := r.step(bctx, i, s, &vars, res, hooks, snap); err != nil {
			if s.Optional {
				log(hooks, i, s.Action, "optional step skipped: "+err.Error())
				continue
			}
			snap(fmt.Sprintf("error-step-%02d", i))
			return res, fmt.Errorf("step %d (%s): %w", i, s.Action, err)
		}
	}
	for _, k := range pb.RequiredOutputs {
		if strings.TrimSpace(res.Outputs[k]) == "" {
			snap("error-missing-output")
			return res, fmt.Errorf("%w: %s", ErrMissingOutput, k)
		}
	}
	return res, nil
}

func (r *Runner) do(ctx context.Context, actions ...chromedp.Action) error {
	tctx, cancel := context.WithTimeout(ctx, r.StepTimeout)
	defer cancel()
	return chromedp.Run(tctx, actions...)
}

func log(h Hooks, i int, a Action, msg string) {
	if h != nil {
		h.Log(i, a, msg)
	}
}

func (r *Runner) step(ctx context.Context, i int, s Step, vars *Vars, res *Result, hooks Hooks, snap func(string) []byte) error {
	sel, err := render(s.Selector, *vars)
	if err != nil {
		return err
	}
	val, err := render(s.Value, *vars)
	if err != nil {
		return err
	}
	shown := val
	if s.Secret || sensitiveTemplate(s.Value) {
		shown = "••••"
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = r.StepTimeout
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch s.Action {
	case ActGoto:
		log(hooks, i, s.Action, val)
		return chromedp.Run(tctx, chromedp.Navigate(val))
	case ActFill:
		log(hooks, i, s.Action, fmt.Sprintf("%s = %s", sel, shown))
		return chromedp.Run(tctx,
			chromedp.WaitVisible(sel, chromedp.ByQuery),
			chromedp.Clear(sel, chromedp.ByQuery),
			chromedp.SendKeys(sel, val, chromedp.ByQuery),
		)
	case ActClick:
		log(hooks, i, s.Action, sel)
		return chromedp.Run(tctx, chromedp.WaitVisible(sel, chromedp.ByQuery), chromedp.Click(sel, chromedp.ByQuery))
	case ActSelect:
		log(hooks, i, s.Action, fmt.Sprintf("%s = %s", sel, shown))
		return chromedp.Run(tctx, chromedp.SetValue(sel, val, chromedp.ByQuery))
	case ActUpload:
		if vars.File == "" {
			return errors.New("no prepared file to upload")
		}
		log(hooks, i, s.Action, fmt.Sprintf("%s ← %s", sel, vars.File))
		return chromedp.Run(tctx, chromedp.SetUploadFiles(sel, []string{vars.File}, chromedp.ByQuery))
	case ActWaitVisible:
		log(hooks, i, s.Action, sel)
		return chromedp.Run(tctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
	case ActAssertText:
		var body string
		if err := chromedp.Run(tctx, chromedp.Text("body", &body, chromedp.ByQuery)); err != nil {
			return err
		}
		if !strings.Contains(body, val) {
			return fmt.Errorf("page does not contain %q", val)
		}
		log(hooks, i, s.Action, "found "+val)
		return nil
	case ActExtract:
		var txt string
		if err := chromedp.Run(tctx, chromedp.WaitVisible(sel, chromedp.ByQuery), chromedp.Text(sel, &txt, chromedp.ByQuery)); err != nil {
			return err
		}
		txt = strings.TrimSpace(txt)
		if s.Pattern != "" {
			m := regexp.MustCompile(s.Pattern).FindStringSubmatch(txt)
			if m == nil {
				return fmt.Errorf("pattern %q not found in %q", s.Pattern, txt)
			}
			txt = m[len(m)-1]
		}
		res.Outputs[s.Name] = txt
		log(hooks, i, s.Action, fmt.Sprintf("%s = %s", s.Name, txt))
		return nil
	case ActScreenshot:
		snap(s.Name)
		log(hooks, i, s.Action, s.Name)
		return nil
	case ActSleep:
		d, err := time.ParseDuration(val)
		if err != nil {
			return err
		}
		select {
		case <-time.After(d):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case ActHumanInput:
		if hooks == nil {
			return errors.New("human input required but no handler configured")
		}
		png := snap("")
		log(hooks, i, s.Action, "waiting for human: "+s.Prompt)
		hctx, hcancel := context.WithTimeout(ctx, r.HumanWait)
		defer hcancel()
		ans, err := hooks.AskHuman(hctx, s.Name, s.Prompt, png)
		if err != nil {
			return fmt.Errorf("human input %q: %w", s.Name, err)
		}
		vars.Input[s.Name] = ans
		log(hooks, i, s.Action, "human answered "+s.Name)
		return nil
	}
	return fmt.Errorf("unknown action %q", s.Action)
}

// sensitiveTemplate reports whether a step value is filled from a human
// answer (OTP, CAPTCHA) or a credential other than the username; such values
// are masked in the run log.
func sensitiveTemplate(v string) bool {
	if strings.Contains(v, ".Input.") {
		return true
	}
	return strings.Contains(v, ".Creds.") && strings.TrimSpace(v) != "{{.Creds.username}}"
}
