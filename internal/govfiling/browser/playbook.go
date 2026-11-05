// Package browser drives government web portals with a headless browser using
// declarative playbooks. It never solves CAPTCHAs or bypasses MFA: those steps
// are "human_input" steps that pause the run, send a screenshot to an HR user,
// and resume with what the human typed.
package browser

import (
	"bytes"
	"fmt"
	"regexp"
	"text/template"
	"time"
)

// Action is a single kind of browser step.
type Action string

const (
	ActGoto        Action = "goto"         // navigate to Value (URL)
	ActFill        Action = "fill"         // type Value into Selector
	ActClick       Action = "click"        // click Selector
	ActSelect      Action = "select"       // set <select> Selector to Value
	ActUpload      Action = "upload"       // set file input Selector to the prepared file
	ActWaitVisible Action = "wait_visible" // wait for Selector
	ActAssertText  Action = "assert_text"  // fail unless page text contains Value
	ActExtract     Action = "extract"      // read text of Selector into output Name (optional Pattern regex)
	ActScreenshot  Action = "screenshot"   // store a screenshot as evidence (Name)
	ActHumanInput  Action = "human_input"  // pause; ask a human (Prompt) — CAPTCHA / OTP / confirmation
	ActSleep       Action = "sleep"        // wait Value duration (e.g. "2s")
)

// Step is one instruction in a playbook. Value and Selector are Go templates
// rendered with Vars (.Creds, .Input, .Data, .File).
type Step struct {
	Action   Action        `json:"action"`
	Selector string        `json:"selector,omitempty"`
	Value    string        `json:"value,omitempty"`
	Name     string        `json:"name,omitempty"`
	Pattern  string        `json:"pattern,omitempty"`
	Prompt   string        `json:"prompt,omitempty"`
	Optional bool          `json:"optional,omitempty"`
	Timeout  time.Duration `json:"timeout,omitempty"`
	// Secret marks Value as sensitive so it is never written to the run log.
	Secret bool `json:"secret,omitempty"`
	// Final marks the step that actually files the return (e.g. the
	// "Submit" click). Dry runs stop right before it.
	Final bool `json:"final,omitempty"`
}

// Playbook describes how to submit one kind of filing on one portal.
type Playbook struct {
	ID       string `json:"id"`
	Agency   string `json:"agency"`
	Portal   string `json:"portal"`
	Title    string `json:"title"`
	StartURL string `json:"start_url"`
	// Verified is false until the playbook has been exercised against the
	// live portal. Unverified playbooks always run in supervised mode.
	Verified bool   `json:"verified"`
	Steps    []Step `json:"steps"`
	// Outputs that must be extracted for the run to count as submitted.
	RequiredOutputs []string `json:"required_outputs"`
}

// Vars are the inputs a playbook is rendered with.
type Vars struct {
	Creds map[string]string // portal credentials (username, password, ...)
	Data  map[string]string // filing data (period, amounts, TIN, ...)
	Input map[string]string // answers given by humans during the run
	File  string            // absolute path of the prepared file to upload
}

func render(tpl string, v Vars) (string, error) {
	if tpl == "" {
		return "", nil
	}
	t, err := template.New("v").Option("missingkey=error").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", tpl, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, v); err != nil {
		return "", fmt.Errorf("render %q: %w", tpl, err)
	}
	return buf.String(), nil
}

// Validate checks a playbook for structural errors before it is run.
func (p Playbook) Validate() error {
	if p.ID == "" || len(p.Steps) == 0 {
		return fmt.Errorf("playbook needs id and steps")
	}
	finals := 0
	for i, s := range p.Steps {
		if s.Final {
			finals++
		}
		switch s.Action {
		case ActGoto:
			if s.Value == "" {
				return fmt.Errorf("step %d: goto needs value", i)
			}
		case ActFill, ActClick, ActSelect, ActUpload, ActWaitVisible:
			if s.Selector == "" {
				return fmt.Errorf("step %d: %s needs selector", i, s.Action)
			}
		case ActExtract:
			if s.Selector == "" || s.Name == "" {
				return fmt.Errorf("step %d: extract needs selector and name", i)
			}
			if s.Pattern != "" {
				if _, err := regexp.Compile(s.Pattern); err != nil {
					return fmt.Errorf("step %d: bad pattern: %w", i, err)
				}
			}
		case ActHumanInput:
			if s.Name == "" || s.Prompt == "" {
				return fmt.Errorf("step %d: human_input needs name and prompt", i)
			}
		case ActAssertText, ActScreenshot, ActSleep:
		default:
			return fmt.Errorf("step %d: unknown action %q", i, s.Action)
		}
	}
	if finals != 1 {
		return fmt.Errorf("playbook must mark exactly one step as final (the submit action), found %d", finals)
	}
	return nil
}
