package govfiling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/govfiling/browser"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/store"
)

// Errors mapped to HTTP statuses by the handler.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// ValidationError is a user-facing input problem.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(f string, a ...any) error { return &ValidationError{Msg: fmt.Sprintf(f, a...)} }

// Service is the government filing agent.
type Service struct {
	q       *store.Queries
	pool    pgxBeginner
	logger  *slog.Logger
	enc     *crypto.CredentialEncryptor
	dir     string
	ai      *AIReviewer
	mail    *email.Sender
	runner  *browser.Runner
	partner *PartnerClient
	now     func() time.Time
	policy  func(ctx context.Context, companyID int64) string
	alerts  Notifier
	// HumanPoll is how often a waiting browser run checks for an answer.
	HumanPoll time.Duration
}

type pgxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Config wires optional dependencies.
type Config struct {
	Encryptor *crypto.CredentialEncryptor // required for portal credentials / OTP answers
	FileDir   string                      // private root for prepared files and evidence (NOT the public /uploads)
	AI        *AIReviewer                 // optional
	Mail      *email.Sender               // required for the e-mail channel
	Runner    *browser.Runner             // required for the browser channel
}

// NewService builds the filing agent.
func NewService(q *store.Queries, pool pgxBeginner, logger *slog.Logger, cfg Config) *Service {
	dir := cfg.FileDir
	if dir == "" {
		dir = "./data/gov"
	}
	r := cfg.Runner
	if r == nil {
		r = browser.NewRunner()
	}
	return &Service{q: q, pool: pool, logger: logger, enc: cfg.Encryptor, dir: dir,
		ai: cfg.AI, mail: cfg.Mail, runner: r, partner: NewPartnerClient(), now: time.Now, HumanPoll: 2 * time.Second}
}

func (s *Service) event(ctx context.Context, subID int64, kind, msg string, data any, actor *int64) {
	b, _ := json.Marshal(data)
	if data == nil {
		b = []byte("{}")
	}
	if err := s.q.InsertGovSubmissionEvent(ctx, store.InsertGovSubmissionEventParams{
		SubmissionID: subID, Kind: kind, Message: msg, Data: b, ActorUserID: actor,
	}); err != nil {
		s.logger.Warn("govfiling: event insert failed", "submission_id", subID, "error", err)
	}
}

// ---------------------------------------------------------------------------
// Portal accounts
// ---------------------------------------------------------------------------

// AccountInput stores portal credentials. Secrets is merged with Username into
// the encrypted blob; it is never returned by the API.
type AccountInput struct {
	Agency     string            `json:"agency" binding:"required"`
	Portal     string            `json:"portal" binding:"required"`
	Label      string            `json:"label"`
	Username   string            `json:"username"`
	Secrets    map[string]string `json:"secrets"`
	OTPChannel string            `json:"otp_channel"`
}

// SaveAccount creates or updates encrypted portal credentials.
func (s *Service) SaveAccount(ctx context.Context, companyID, userID int64, in AccountInput) (*store.GovPortalAccount, error) {
	if in.OTPChannel == "" {
		in.OTPChannel = "none"
	}
	enc := ""
	if len(in.Secrets) > 0 {
		if s.enc == nil {
			return nil, invalid("INTEGRATION_ENCRYPTION_KEY is not configured; credentials cannot be stored safely")
		}
		creds := map[string]string{}
		for k, v := range in.Secrets {
			creds[k] = v
		}
		if in.Username != "" {
			creds["username"] = in.Username
		}
		raw, _ := json.Marshal(creds)
		ct, err := s.enc.Encrypt(raw)
		if err != nil {
			return nil, err
		}
		enc = hex.EncodeToString(ct)
	}
	a, err := s.q.UpsertGovPortalAccount(ctx, store.UpsertGovPortalAccountParams{
		CompanyID: companyID, Agency: strings.ToUpper(in.Agency), Portal: in.Portal, Label: in.Label,
		Username: in.Username, CredentialsEnc: enc, OtpChannel: in.OTPChannel, CreatedBy: &userID,
	})
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) decryptCreds(a store.GovPortalAccount) (map[string]string, error) {
	creds := map[string]string{}
	if a.CredentialsEnc == "" {
		if a.Username != "" {
			creds["username"] = a.Username
		}
		return creds, nil
	}
	if s.enc == nil {
		return nil, errors.New("INTEGRATION_ENCRYPTION_KEY is not configured")
	}
	ct, err := hex.DecodeString(a.CredentialsEnc)
	if err != nil {
		return nil, err
	}
	pt, err := s.enc.Decrypt(ct)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}
	if err := json.Unmarshal(pt, &creds); err != nil {
		return nil, err
	}
	if creds["username"] == "" && a.Username != "" {
		creds["username"] = a.Username
	}
	return creds, nil
}

// ---------------------------------------------------------------------------
// Prepare
// ---------------------------------------------------------------------------

// PrepareInput asks the agent to prepare one return.
type PrepareInput struct {
	FormCode    string `json:"form_code" binding:"required"`
	Period      string `json:"period" binding:"required"`
	Channel     string `json:"channel"`
	TaxFilingID *int64 `json:"tax_filing_id"`
}

// Prepared is the result of Prepare.
type Prepared struct {
	Submission store.GovSubmission `json:"submission"`
	Preflight  *Preflight          `json:"preflight"`
	Spec       FormSpec            `json:"form"`
}

// Prepare builds the file, runs the pre-flight review and stores the
// submission as "ready" (approvable) or "draft" (blocking issues).
func (s *Service) Prepare(ctx context.Context, companyID int64, userID *int64, in PrepareInput) (*Prepared, error) {
	spec, ok := Lookup(in.FormCode)
	if !ok {
		return nil, invalid("unknown form %q", in.FormCode)
	}
	period, err := ParsePeriod(spec.Frequency, in.Period)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}
	var ch Channel
	if in.Channel != "" {
		ch = Channel(in.Channel)
		if !spec.Supports(ch) {
			return nil, invalid("%s cannot be submitted via %s", spec.Code, ch)
		}
	} else {
		ch = s.bestChannel(ctx, companyID, spec)
	}

	var due *time.Time
	if in.TaxFilingID != nil {
		tf, err := s.q.GetTaxFiling(ctx, store.GetTaxFilingParams{ID: *in.TaxFilingID, CompanyID: companyID})
		if err != nil {
			return nil, invalid("tax filing not found")
		}
		d := tf.DueDate
		due = &d
	}

	ds, err := BuildDataset(ctx, s.q, companyID, spec, period)
	noFile := false
	if errors.Is(err, ErrNoGenerator) && ch == ChannelManual {
		// Guided manual filing: instructions + tracking, HR brings the figures.
		noFile = true
		ds = &Dataset{FormCode: spec.Code, Period: strings.ToUpper(strings.TrimSpace(in.Period)), Totals: map[string]float64{}, Lines: []Line{}}
	} else if err != nil {
		return nil, invalid("%s", err.Error())
	}
	if due == nil && ds.DueDate != nil {
		due = ds.DueDate // registrations: earliest hire + N days
	}
	var prevTotals map[string]float64
	if period.Month > 0 && !spec.Registration {
		pp := Period{Year: period.Year, Month: period.Month - 1}
		if pp.Month == 0 {
			pp = Period{Year: period.Year - 1, Month: 12}
		}
		if prev, err := BuildDataset(ctx, s.q, companyID, spec, pp); err == nil && len(prev.Lines) > 0 {
			prevTotals = prev.Totals
		}
	}

	ri := RuleInput{Spec: spec, Period: period, Dataset: ds, PrevTotals: prevTotals, Channel: ch, DueDate: due, Today: s.now()}
	var accountID *int64
	if acc, err := s.q.FindGovPortalAccount(ctx, store.FindGovPortalAccountParams{CompanyID: companyID, Agency: spec.Agency, Portal: spec.Portal}); err == nil {
		accountID = &acc.ID
		ri.HasAccount = acc.CredentialsEnc != "" || ch == ChannelEmail
	}
	var playbookKey *string
	if ch == ChannelBrowser {
		pb, err := loadPlaybook(ctx, s.q, companyID, spec.Code)
		if err != nil {
			return nil, err
		}
		if pb != nil {
			ri.HasPlaybook, ri.PlaybookOK = true, pb.Verified
			playbookKey = &pb.Key
		}
	}
	pf := RunRules(ri)
	if noFile {
		pf.Issues = append(pf.Issues, Issue{Severity: "info", Code: "no_generator", Source: "rules",
			Message: "HalaPeople does not generate this return's figures yet — follow the steps and record the reference number."})
	}
	if s.ai != nil && !noFile {
		if err := s.ai.Review(ctx, ri, pf); err != nil {
			s.logger.Warn("govfiling: ai review failed", "form", spec.Code, "error", err)
		}
	}

	var namePtr, relPtr, sumPtr *string
	full := ""
	if !noFile {
		name, _, data, err := RenderFile(spec, ds)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		rel := filepath.Join(fmt.Sprint(companyID), s.now().Format("200601"), hex.EncodeToString(sum[:6])+"_"+name)
		full = filepath.Join(s.dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(full, data, 0o640); err != nil {
			return nil, err
		}
		namePtr, relPtr, sumPtr = &name, &rel, strPtr(hex.EncodeToString(sum[:]))
	}

	// Re-preparing replaces an earlier blocked draft of the same return.
	if err := s.q.CancelDraftGovSubmissions(ctx, store.CancelDraftGovSubmissionsParams{
		CompanyID: companyID, Agency: spec.Agency, FormCode: spec.Code, Period: ds.Period,
	}); err != nil {
		return nil, err
	}

	status := "ready"
	if pf.Blocking {
		status = "draft"
	}
	dsJSON, _ := json.Marshal(ds)
	pfJSON, _ := json.Marshal(pf)
	instr := strings.Join(spec.Steps, "\n")
	dd := pgtype.Date{}
	if due != nil {
		dd = pgtype.Date{Time: *due, Valid: true}
	}
	sub, err := s.q.CreateGovSubmission(ctx, store.CreateGovSubmissionParams{
		CompanyID: companyID, Agency: spec.Agency, FormCode: spec.Code, Period: ds.Period, TaxFilingID: in.TaxFilingID,
		Channel: string(ch), PlaybookID: playbookKey, PortalAccountID: accountID, Status: status,
		FileName: namePtr, FilePath: relPtr, FileSha256: sumPtr, Data: dsJSON, Preflight: pfJSON,
		Instructions: &instr, DueDate: dd, CreatedBy: userID,
	})
	if err != nil {
		if full != "" {
			_ = os.Remove(full)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: a submission for %s %s is already in progress", ErrConflict, spec.Code, ds.Period)
		}
		return nil, err
	}
	s.event(ctx, sub.ID, "status", "prepared", map[string]any{"status": status, "issues": len(pf.Issues), "blocking": pf.Blocking}, userID)
	if pf.AISummary != "" {
		s.event(ctx, sub.ID, "ai", pf.AISummary, map[string]any{"risk": pf.AIRisk}, nil)
	}
	return &Prepared{Submission: sub, Preflight: pf, Spec: spec}, nil
}

func strPtr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// Approve / cancel / record
// ---------------------------------------------------------------------------

func (s *Service) get(ctx context.Context, companyID, id int64) (*store.GovSubmission, error) {
	sub, err := s.q.GetGovSubmission(ctx, store.GetGovSubmissionParams{ID: id, CompanyID: companyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &sub, nil
}

// Approve authorises the agent to submit (or, for dryRun, to rehearse up to
// the final submit step). Blocking pre-flight issues prevent approval, and a
// real browser run requires a verified playbook.
func (s *Service) Approve(ctx context.Context, companyID, userID, id int64, dryRun bool) (*store.GovSubmission, error) {
	sub, err := s.get(ctx, companyID, id)
	if err != nil {
		return nil, err
	}
	var pf Preflight
	_ = json.Unmarshal(sub.Preflight, &pf)
	if pf.Blocking {
		return nil, invalid("resolve the blocking pre-flight issues and prepare the return again before approving")
	}
	if dryRun && sub.Channel != string(ChannelBrowser) {
		return nil, invalid("dry runs are only available for the browser channel")
	}
	if sub.Channel == string(ChannelBrowser) && !dryRun {
		pb, err := loadPlaybook(ctx, s.q, companyID, sub.FormCode)
		if err != nil {
			return nil, err
		}
		if pb == nil {
			return nil, invalid("no playbook configured for %s", sub.FormCode)
		}
		if !pb.Verified {
			return nil, invalid("the portal playbook is not verified yet — run a dry run, check the evidence screenshots, then mark the playbook verified")
		}
	}
	if err := s.q.SetGovSubmissionDryRun(ctx, store.SetGovSubmissionDryRunParams{ID: id, DryRun: dryRun}); err != nil {
		return nil, err
	}
	out, err := s.q.ApproveGovSubmission(ctx, store.ApproveGovSubmissionParams{ID: id, CompanyID: companyID, ApprovedBy: &userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: submission is %s and cannot be approved", ErrConflict, sub.Status)
		}
		return nil, err
	}
	msg := "approved for submission"
	if dryRun {
		msg = "approved for a dry run (will stop before the final submit step)"
	} else if sub.Channel == string(ChannelManual) {
		msg = "approved — submit on the portal and record the reference number"
	}
	s.event(ctx, id, "status", msg, map[string]any{"dry_run": dryRun}, &userID)
	out.DryRun = dryRun
	return &out, nil
}

// Cancel stops a submission that has not been filed.
func (s *Service) Cancel(ctx context.Context, companyID, userID, id int64) (*store.GovSubmission, error) {
	out, err := s.q.CancelGovSubmission(ctx, store.CancelGovSubmissionParams{ID: id, CompanyID: companyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: submission cannot be cancelled in its current state", ErrConflict)
		}
		return nil, err
	}
	s.event(ctx, id, "status", "cancelled", nil, &userID)
	return &out, nil
}

// RecordManual records a submission that a human completed on the portal.
func (s *Service) RecordManual(ctx context.Context, companyID, userID, id int64, reference, note string) (*store.GovSubmission, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, invalid("reference_no is required")
	}
	sub, err := s.get(ctx, companyID, id)
	if err != nil {
		return nil, err
	}
	okState := sub.Status == "approved" && sub.Channel == string(ChannelManual) || sub.Status == "failed" || sub.Status == "needs_human"
	if !okState {
		return nil, fmt.Errorf("%w: only approved manual submissions or failed automated ones can be recorded manually", ErrConflict)
	}
	outputs, _ := json.Marshal(map[string]string{"recorded_by": fmt.Sprint(userID), "note": note})
	out, err := s.q.MarkGovSubmissionSubmitted(ctx, store.MarkGovSubmissionSubmittedParams{ID: id, ReferenceNo: &reference, Outputs: outputs})
	if err != nil {
		return nil, err
	}
	s.afterSubmitted(ctx, out, &userID)
	s.event(ctx, id, "status", "submitted (recorded manually): "+reference, map[string]any{"note": note}, &userID)
	return &out, nil
}

func (s *Service) afterSubmitted(ctx context.Context, sub store.GovSubmission, actor *int64) {
	if sub.TaxFilingID == nil {
		return
	}
	var proof *string
	if sub.FilePath != nil {
		proof = sub.FilePath
	}
	if err := s.q.MarkTaxFilingFiled(ctx, store.MarkTaxFilingFiledParams{
		ID: *sub.TaxFilingID, CompanyID: sub.CompanyID, ReferenceNo: sub.ReferenceNo, ProofPath: proofText(proof), FiledBy: actor,
	}); err != nil {
		s.logger.Warn("govfiling: mark tax filing filed", "tax_filing_id", *sub.TaxFilingID, "error", err)
	}
}

func proofText(p *string) *string { return p }

// FilePath returns the absolute path of a stored file (prepared return or evidence),
// refusing anything that escapes the company's directory.
func (s *Service) FilePath(companyID int64, rel string) (string, error) {
	full := filepath.Clean(filepath.Join(s.dir, rel))
	root := filepath.Clean(filepath.Join(s.dir, fmt.Sprint(companyID))) + string(os.PathSeparator)
	if !strings.HasPrefix(full, root) {
		return "", ErrNotFound
	}
	return full, nil
}

// ---------------------------------------------------------------------------
// Human hand-off
// ---------------------------------------------------------------------------

// Answer records a human's answer (CAPTCHA text, OTP, "yes") for a waiting run.
func (s *Service) Answer(ctx context.Context, companyID, userID, requestID int64, answer string) error {
	if s.enc == nil {
		return invalid("INTEGRATION_ENCRYPTION_KEY is not configured")
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return invalid("answer is required")
	}
	ct, err := s.enc.Encrypt([]byte(answer))
	if err != nil {
		return err
	}
	enc := hex.EncodeToString(ct)
	req, err := s.q.AnswerGovHumanRequest(ctx, store.AnswerGovHumanRequestParams{ID: requestID, CompanyID: companyID, AnswerEnc: &enc, AnsweredBy: &userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: request is no longer waiting for an answer", ErrConflict)
		}
		return err
	}
	s.event(ctx, req.SubmissionID, "human_answer", "answered: "+req.Name, nil, &userID)
	return nil
}

// bestChannel picks the most automated channel the company has actually set
// up for this form, falling back to manual.
func (s *Service) bestChannel(ctx context.Context, companyID int64, spec FormSpec) Channel {
	for _, c := range spec.Channels {
		switch c {
		case ChannelEmail:
			if s.mail != nil {
				return c
			}
		case ChannelAPI, ChannelBrowser:
			acc, err := s.q.FindGovPortalAccount(ctx, store.FindGovPortalAccountParams{CompanyID: companyID, Agency: spec.Agency, Portal: spec.Portal})
			if err != nil || acc.CredentialsEnc == "" {
				continue
			}
			if c == ChannelAPI {
				return c
			}
			if pb, err := loadPlaybook(ctx, s.q, companyID, spec.Code); err == nil && pb != nil {
				return c
			}
		case ChannelManual:
			return c
		}
	}
	return ChannelManual
}
