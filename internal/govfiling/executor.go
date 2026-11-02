package govfiling

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/govfiling/browser"
	"github.com/halaostory/halaos/internal/store"
)

// ---------------------------------------------------------------------------
// Partner API (licensed e-file provider / tax agent / official API gateway)
// ---------------------------------------------------------------------------

// PartnerClient posts a prepared return to a filing partner. The partner
// connection is a portal account whose secrets hold "url" and "secret".
// The request body is signed: X-HalaOS-Signature: sha256=<hex HMAC(body)>.
type PartnerClient struct {
	HTTP *http.Client
}

// NewPartnerClient returns a client with a sane timeout.
func NewPartnerClient() *PartnerClient {
	return &PartnerClient{HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// PartnerRequest is what the partner receives.
type PartnerRequest struct {
	SubmissionID int64           `json:"submission_id"`
	CompanyID    int64           `json:"company_id"`
	Agency       string          `json:"agency"`
	FormCode     string          `json:"form_code"`
	Period       string          `json:"period"`
	FileName     string          `json:"file_name"`
	FileSHA256   string          `json:"file_sha256"`
	FileBase64   string          `json:"file_base64"`
	Data         json.RawMessage `json:"data"`
}

// PartnerResponse is what the partner must answer.
type PartnerResponse struct {
	Status      string `json:"status"` // accepted, rejected
	ReferenceNo string `json:"reference_no"`
	Message     string `json:"message"`
}

// Sign computes the request signature.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// Submit sends the return and returns the partner's reference number.
func (p *PartnerClient) Submit(ctx context.Context, url, secret string, req PartnerRequest) (*PartnerResponse, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://127.0.0.1") && !strings.HasPrefix(url, "http://localhost") {
		return nil, errors.New("partner url must use https")
	}
	body, _ := json.Marshal(req)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Idempotency-Key", fmt.Sprintf("halaos-gov-%d", req.SubmissionID))
	if secret != "" {
		hr.Header.Set("X-HalaOS-Signature", Sign(secret, body))
	}
	resp, err := p.HTTP.Do(hr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("partner returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out PartnerResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("partner response: %w", err)
	}
	if out.Status == "rejected" {
		return nil, fmt.Errorf("partner rejected the return: %s", out.Message)
	}
	if strings.TrimSpace(out.ReferenceNo) == "" {
		return nil, errors.New("partner did not return a reference number")
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Queue processing
// ---------------------------------------------------------------------------

var runSlots = make(chan struct{}, 2) // at most 2 concurrent submissions per worker

// ProcessQueue claims approved automated submissions and runs them in the
// background. Safe to call frequently; returns how many runs were started.
func (s *Service) ProcessQueue(ctx context.Context) int {
	if err := s.q.RecoverStuckGovSubmissions(ctx); err != nil {
		s.logger.Warn("govfiling: recover stuck", "error", err)
	}
	free := cap(runSlots) - len(runSlots)
	if free <= 0 {
		return 0
	}
	subs, err := s.q.ClaimApprovedGovSubmissions(ctx, int32(free))
	if err != nil {
		s.logger.Error("govfiling: claim", "error", err)
		return 0
	}
	for _, sub := range subs {
		runSlots <- struct{}{}
		go func(sub store.GovSubmission) {
			defer func() { <-runSlots }()
			rctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
			defer cancel()
			s.Execute(rctx, sub)
		}(sub)
	}
	return len(subs)
}

// Execute runs one claimed submission to completion and records the outcome.
func (s *Service) Execute(ctx context.Context, sub store.GovSubmission) {
	s.event(ctx, sub.ID, "status", "running via "+sub.Channel, map[string]any{"attempt": sub.Attempts, "dry_run": sub.DryRun}, nil)
	var (
		ref     string
		outputs map[string]string
		err     error
		dryDone bool
	)
	switch Channel(sub.Channel) {
	case ChannelEmail:
		ref, outputs, err = s.execEmail(ctx, sub)
	case ChannelAPI:
		ref, outputs, err = s.execPartner(ctx, sub)
	case ChannelBrowser:
		ref, outputs, dryDone, err = s.execBrowser(ctx, sub)
	default:
		err = fmt.Errorf("channel %s is not automated", sub.Channel)
	}
	if err != nil {
		msg := err.Error()
		if cur, gerr := s.q.GetGovSubmissionByID(context.Background(), sub.ID); gerr == nil && cur.Status == "cancelled" {
			s.event(context.Background(), sub.ID, "status", "run stopped: submission was cancelled", nil, nil)
			return
		}
		_ = s.q.SetGovSubmissionStatus(context.Background(), store.SetGovSubmissionStatusParams{ID: sub.ID, Status: "failed", ErrorMessage: &msg})
		s.event(context.Background(), sub.ID, "status", "failed: "+msg, nil, nil)
		if sub.PortalAccountID != nil {
			_ = s.q.MarkGovPortalAccountUsed(context.Background(), store.MarkGovPortalAccountUsedParams{ID: *sub.PortalAccountID, LastError: &msg})
		}
		return
	}
	if sub.PortalAccountID != nil {
		_ = s.q.MarkGovPortalAccountUsed(ctx, store.MarkGovPortalAccountUsedParams{ID: *sub.PortalAccountID})
	}
	if dryDone {
		_ = s.q.SetGovSubmissionDryRun(ctx, store.SetGovSubmissionDryRunParams{ID: sub.ID, DryRun: false})
		_ = s.q.SetGovSubmissionStatus(ctx, store.SetGovSubmissionStatusParams{ID: sub.ID, Status: "ready"})
		s.event(ctx, sub.ID, "status", "dry run finished before the final submit step — review the evidence, verify the playbook, then approve the real submission", nil, nil)
		return
	}
	outJSON, _ := json.Marshal(outputs)
	done, err := s.q.MarkGovSubmissionSubmitted(ctx, store.MarkGovSubmissionSubmittedParams{ID: sub.ID, ReferenceNo: &ref, Outputs: outJSON})
	if err != nil {
		s.logger.Error("govfiling: mark submitted", "submission_id", sub.ID, "error", err)
		return
	}
	s.afterSubmitted(ctx, done, sub.ApprovedBy)
	s.event(ctx, sub.ID, "status", "submitted — reference "+ref, outputs, nil)
}

func (s *Service) readFile(sub store.GovSubmission) ([]byte, string, error) {
	if sub.FilePath == nil {
		return nil, "", errors.New("submission has no prepared file")
	}
	full, err := s.FilePath(sub.CompanyID, *sub.FilePath)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, "", err
	}
	if sub.FileSha256 != nil {
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != *sub.FileSha256 {
			return nil, "", errors.New("prepared file changed after approval (checksum mismatch) — prepare it again")
		}
	}
	return data, full, nil
}

func (s *Service) execEmail(ctx context.Context, sub store.GovSubmission) (string, map[string]string, error) {
	spec, _ := Lookup(sub.FormCode)
	if spec.EmailTo == "" {
		return "", nil, errors.New("this form has no e-mail submission address")
	}
	data, _, err := s.readFile(sub)
	if err != nil {
		return "", nil, err
	}
	company, err := s.q.GetCompanyByID(ctx, sub.CompanyID)
	if err != nil {
		return "", nil, err
	}
	to := spec.EmailTo
	if o := os.Getenv("GOV_EMAIL_OVERRIDE_TO"); o != "" { // staging safety valve
		to = o
	}
	var cc []string
	if sub.PortalAccountID != nil {
		if acc, err := s.q.GetGovPortalAccount(ctx, store.GetGovPortalAccountParams{ID: *sub.PortalAccountID, CompanyID: sub.CompanyID}); err == nil && strings.Contains(acc.Username, "@") {
			cc = append(cc, acc.Username) // the company's own mailbox receives a copy and BIR's acknowledgement
		}
	}
	tin := ""
	if company.Tin != nil {
		tin = *company.Tin
	}
	id, err := s.mail.SendWithAttachments(email.Message{
		To: []string{to}, Cc: cc,
		Subject:     fmt.Sprintf("%s %s – %s – TIN %s", spec.Name, sub.Period, company.Name, tin),
		TextBody:    fmt.Sprintf("Good day.\n\nAttached is the %s of %s (TIN %s) for %s.\n\nThank you.\n", spec.Name, company.Name, tin, sub.Period),
		Attachments: []email.Attachment{{Filename: deref(sub.FileName), Data: data}},
	})
	if err != nil {
		return "", nil, fmt.Errorf("send e-mail: %w", err)
	}
	return "EMAIL " + id, map[string]string{"sent_to": to, "message_id": id, "note": "Keep the agency's acknowledgement e-mail as proof of receipt."}, nil
}

func (s *Service) execPartner(ctx context.Context, sub store.GovSubmission) (string, map[string]string, error) {
	if sub.PortalAccountID == nil {
		return "", nil, errors.New("no filing-partner connection configured")
	}
	acc, err := s.q.GetGovPortalAccount(ctx, store.GetGovPortalAccountParams{ID: *sub.PortalAccountID, CompanyID: sub.CompanyID})
	if err != nil {
		return "", nil, err
	}
	creds, err := s.decryptCreds(acc)
	if err != nil {
		return "", nil, err
	}
	data, _, err := s.readFile(sub)
	if err != nil {
		return "", nil, err
	}
	resp, err := s.partner.Submit(ctx, creds["url"], creds["secret"], PartnerRequest{
		SubmissionID: sub.ID, CompanyID: sub.CompanyID, Agency: sub.Agency, FormCode: sub.FormCode, Period: sub.Period,
		FileName: deref(sub.FileName), FileSHA256: deref(sub.FileSha256), FileBase64: base64.StdEncoding.EncodeToString(data), Data: sub.Data,
	})
	if err != nil {
		return "", nil, err
	}
	return resp.ReferenceNo, map[string]string{"partner_status": resp.Status, "partner_message": resp.Message}, nil
}

func (s *Service) execBrowser(ctx context.Context, sub store.GovSubmission) (string, map[string]string, bool, error) {
	pb, err := loadPlaybook(ctx, s.q, sub.CompanyID, sub.FormCode)
	if err != nil {
		return "", nil, false, err
	}
	if pb == nil {
		return "", nil, false, errors.New("no playbook configured")
	}
	if !pb.Verified && !sub.DryRun {
		return "", nil, false, errors.New("playbook is not verified; only dry runs are allowed")
	}
	if sub.PortalAccountID == nil {
		return "", nil, false, errors.New("no portal credentials configured")
	}
	acc, err := s.q.GetGovPortalAccount(ctx, store.GetGovPortalAccountParams{ID: *sub.PortalAccountID, CompanyID: sub.CompanyID})
	if err != nil {
		return "", nil, false, err
	}
	creds, err := s.decryptCreds(acc)
	if err != nil {
		return "", nil, false, err
	}
	_, full, err := s.readFile(sub)
	if err != nil {
		return "", nil, false, err
	}
	var ds Dataset
	_ = json.Unmarshal(sub.Data, &ds)
	data := map[string]string{"period": sub.Period, "form_code": sub.FormCode, "lines": strconv.Itoa(len(ds.Lines))}
	for k, v := range ds.Totals {
		data["total_"+k] = fmt.Sprintf("%.2f", v)
	}
	for k, v := range ds.EmployerIDs {
		data[k] = v
	}
	if y, m, ok := strings.Cut(sub.Period, "-"); ok {
		data["year"], data["month"] = y, m
	}
	hooks := &dbHooks{s: s, sub: sub}
	res, err := s.runner.Run(ctx, pb.Playbook, browser.Vars{Creds: creds, Data: data, File: full}, hooks, browser.Options{DryRun: sub.DryRun})
	if err != nil {
		return "", nil, false, err
	}
	if res.StoppedBeforeFinal {
		return "", nil, true, nil
	}
	ref := res.Outputs["reference_no"]
	if ref == "" {
		return "", nil, false, browser.ErrMissingOutput
	}
	res.Outputs["evidence"] = strings.Join(res.Evidence, ",")
	return ref, res.Outputs, false, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---------------------------------------------------------------------------
// DB-backed browser hooks
// ---------------------------------------------------------------------------

type dbHooks struct {
	s   *Service
	sub store.GovSubmission
	mu  sync.Mutex
	n   int
}

// BeforeStep aborts the run if the submission was cancelled meanwhile.
func (h *dbHooks) BeforeStep(ctx context.Context, step int, st browser.Step) error {
	cur, err := h.s.q.GetGovSubmissionByID(ctx, h.sub.ID)
	if err != nil {
		return err
	}
	if cur.Status == "cancelled" {
		return errors.New("submission was cancelled")
	}
	return nil
}

func (h *dbHooks) Log(step int, action browser.Action, msg string) {
	h.s.event(context.Background(), h.sub.ID, "log", fmt.Sprintf("[%02d %s] %s", step, action, msg), nil, nil)
}

func (h *dbHooks) saveShot(name string, png []byte) (string, error) {
	h.mu.Lock()
	h.n++
	n := h.n
	h.mu.Unlock()
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, strings.ToLower(name))
	rel := filepath.Join(fmt.Sprint(h.sub.CompanyID), "evidence", fmt.Sprint(h.sub.ID), fmt.Sprintf("%02d-%s.jpg", n, safe))
	full := filepath.Join(h.s.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", err
	}
	return rel, os.WriteFile(full, png, 0o640)
}

func (h *dbHooks) Evidence(name string, png []byte) (string, error) {
	rel, err := h.saveShot(name, png)
	if err != nil {
		return "", err
	}
	h.s.event(context.Background(), h.sub.ID, "evidence", name, map[string]string{"file": rel}, nil)
	return rel, nil
}

func (h *dbHooks) AskHuman(ctx context.Context, name, prompt string, png []byte) (string, error) {
	if h.s.enc == nil {
		return "", errors.New("INTEGRATION_ENCRYPTION_KEY is not configured")
	}
	var shot *string
	if len(png) > 0 {
		if rel, err := h.saveShot("ask-"+name, png); err == nil {
			shot = &rel
		}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Minute)
	}
	req, err := h.s.q.CreateGovHumanRequest(ctx, store.CreateGovHumanRequestParams{
		SubmissionID: h.sub.ID, CompanyID: h.sub.CompanyID, Name: name, Prompt: prompt, ScreenshotPath: shot, ExpiresAt: deadline,
	})
	if err != nil {
		return "", err
	}
	_ = h.s.q.SetGovSubmissionStatus(ctx, store.SetGovSubmissionStatusParams{ID: h.sub.ID, Status: "needs_human"})
	h.s.event(ctx, h.sub.ID, "human_request", prompt, map[string]any{"request_id": req.ID, "name": name}, nil)

	t := time.NewTicker(h.s.HumanPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = h.s.q.ExpireGovHumanRequest(context.Background(), req.ID)
			return "", fmt.Errorf("nobody answered %q in time", name)
		case <-t.C:
			if cur, err := h.s.q.GetGovSubmissionByID(ctx, h.sub.ID); err == nil && cur.Status == "cancelled" {
				_ = h.s.q.ExpireGovHumanRequest(context.Background(), req.ID)
				return "", errors.New("submission was cancelled")
			}
			encAns, err := h.s.q.GetGovHumanAnswer(ctx, req.ID)
			if err != nil || encAns == nil {
				continue
			}
			ct, err := hex.DecodeString(*encAns)
			if err != nil {
				return "", err
			}
			pt, err := h.s.enc.Decrypt(ct)
			if err != nil {
				return "", err
			}
			_, _ = h.s.q.ConsumeGovHumanRequest(ctx, req.ID) // wipe the answer at rest
			_ = h.s.q.SetGovSubmissionStatus(ctx, store.SetGovSubmissionStatusParams{ID: h.sub.ID, Status: "running"})
			return string(pt), nil
		}
	}
}
