package govfiling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/govfiling/browser"
	"github.com/halaostory/halaos/internal/store"
)

// builtinPlaybooks are shipped with HalaOS. Real government portals are not
// included: their markup changes without notice and must be recorded and
// dry-run against the live site per company (see /gov/playbooks).
func builtinPlaybook(formCode string) (*browser.Playbook, bool) {
	if formCode != "mock_return" {
		return nil, false
	}
	base := strings.TrimRight(os.Getenv("GOV_MOCK_PORTAL_URL"), "/")
	if base == "" {
		return nil, false
	}
	return &browser.Playbook{
		ID: "mock.return", Agency: "MOCK", Portal: "mock", Title: "Mock eFiling portal", StartURL: base + "/",
		Verified: true, RequiredOutputs: []string{"reference_no"},
		Steps: []browser.Step{
			{Action: browser.ActFill, Selector: "#username", Value: "{{.Creds.username}}"},
			{Action: browser.ActFill, Selector: "#password", Value: "{{.Creds.password}}", Secret: true},
			{Action: browser.ActHumanInput, Name: "captcha", Prompt: "Type the characters shown in the CAPTCHA image"},
			{Action: browser.ActFill, Selector: "#captcha", Value: "{{.Input.captcha}}"},
			{Action: browser.ActClick, Selector: "#login-btn"},
			{Action: browser.ActHumanInput, Name: "otp", Prompt: "Enter the one-time PIN sent to the registered mobile number"},
			{Action: browser.ActFill, Selector: "#otp", Value: "{{.Input.otp}}"},
			{Action: browser.ActClick, Selector: "#otp-btn"},
			{Action: browser.ActSelect, Selector: "#period", Value: "{{.Data.period}}"},
			{Action: browser.ActUpload, Selector: "#file"},
			{Action: browser.ActScreenshot, Name: "before-submit"},
			{Action: browser.ActClick, Selector: "#submit-btn", Final: true},
			{Action: browser.ActExtract, Selector: "#confirmation", Name: "reference_no", Pattern: `Reference No:\s*(\S+)`},
			{Action: browser.ActScreenshot, Name: "confirmation"},
		},
	}, true
}

// resolvedPlaybook is a playbook plus where it came from.
type resolvedPlaybook struct {
	Playbook browser.Playbook
	Key      string
	Verified bool
}

func loadPlaybook(ctx context.Context, q *store.Queries, companyID int64, formCode string) (*resolvedPlaybook, error) {
	row, err := q.GetGovPlaybookForForm(ctx, store.GetGovPlaybookForFormParams{CompanyID: &companyID, FormCode: formCode})
	if err == nil {
		var pb browser.Playbook
		if err := json.Unmarshal(row.Definition, &pb); err != nil {
			return nil, fmt.Errorf("playbook %s: %w", row.PlaybookKey, err)
		}
		pb.Verified = row.Verified
		return &resolvedPlaybook{Playbook: pb, Key: row.PlaybookKey, Verified: row.Verified}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if pb, ok := builtinPlaybook(formCode); ok {
		return &resolvedPlaybook{Playbook: *pb, Key: pb.ID, Verified: pb.Verified}, nil
	}
	return nil, nil
}
