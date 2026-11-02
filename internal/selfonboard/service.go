package selfonboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/pdf"
	"github.com/halaostory/halaos/internal/store"
)

// Errors mapped by the handler.
var (
	ErrNotFound = errors.New("not found")
	ErrGone     = errors.New("link expired or no longer valid")
)

// ValidationError is a user-facing problem; Fields maps form field → message.
type ValidationError struct {
	Msg    string
	Fields map[string]string
}

func (e *ValidationError) Error() string { return e.Msg }

func invalid(f string, a ...any) error { return &ValidationError{Msg: fmt.Sprintf(f, a...)} }

const (
	inviteTTL    = 14 * 24 * time.Hour
	signatureTTL = 30 * 24 * time.Hour
)

// Service implements invites, templates and signatures.
type Service struct {
	q       *store.Queries
	logger  *slog.Logger
	mail    *email.Sender
	ai      provider.Provider
	baseURL string
	dir     string
	now     func() time.Time
}

// NewService builds the service. baseURL is the public web app URL used in links.
func NewService(q *store.Queries, logger *slog.Logger, mail *email.Sender, ai provider.Provider, baseURL string) *Service {
	dir := os.Getenv("DOC_FILES_DIR")
	if dir == "" {
		dir = "./data/documents"
	}
	return &Service{q: q, logger: logger, mail: mail, ai: ai, baseURL: strings.TrimRight(baseURL, "/"), dir: dir, now: time.Now}
}

// Link is a freshly generated public link (shown once).
type Link struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Emailed   bool      `json:"emailed"`
}

func (s *Service) send(to, subject, title, msg string) bool {
	if s.mail == nil || to == "" {
		return false
	}
	subj, body := email.GenericNotificationEmail("", title, msg)
	if subject != "" {
		subj = subject
	}
	return s.mail.Send(to, subj, body) == nil
}

// ---------------------------------------------------------------------------
// Invites
// ---------------------------------------------------------------------------

// CreateInvite revokes older open invites and issues a new self-onboarding link.
func (s *Service) CreateInvite(ctx context.Context, companyID, userID, employeeID int64, emailTo string, sendEmail bool) (*Link, error) {
	emp, err := s.q.GetEmployeeForDocument(ctx, store.GetEmployeeForDocumentParams{ID: employeeID, CompanyID: companyID})
	if err != nil {
		return nil, ErrNotFound
	}
	if emailTo == "" && emp.Email != nil {
		emailTo = *emp.Email
	}
	if err := s.q.RevokeOpenOnboardingInvites(ctx, store.RevokeOpenOnboardingInvitesParams{EmployeeID: employeeID, CompanyID: companyID}); err != nil {
		return nil, err
	}
	tok, hash := NewToken()
	exp := s.now().Add(inviteTTL)
	var em *string
	if emailTo != "" {
		em = &emailTo
	}
	if _, err := s.q.CreateOnboardingInvite(ctx, store.CreateOnboardingInviteParams{
		CompanyID: companyID, EmployeeID: employeeID, TokenHash: hash, Email: em, ExpiresAt: exp, CreatedBy: &userID,
	}); err != nil {
		return nil, err
	}
	l := &Link{URL: s.baseURL + "/onboard/" + tok, ExpiresAt: exp}
	if sendEmail {
		l.Emailed = s.send(emailTo, "Welcome to "+emp.CompanyName+" — complete your onboarding",
			"Welcome aboard, "+emp.FirstName+"!",
			"Please complete your personal details, government IDs, bank account and documents before your first day: "+l.URL)
	}
	return l, nil
}

func (s *Service) inviteByToken(ctx context.Context, token string) (*store.OnboardingInvite, error) {
	inv, err := s.q.GetOnboardingInviteByToken(ctx, HashToken(token))
	if err != nil {
		return nil, ErrGone
	}
	if inv.Status == "revoked" || inv.Status == "expired" || s.now().After(inv.ExpiresAt) {
		return nil, ErrGone
	}
	return &inv, nil
}

// PortalView is what the new hire sees.
type PortalView struct {
	CompanyName string            `json:"company_name"`
	Country     string            `json:"country"`
	FirstName   string            `json:"first_name"`
	LastName    string            `json:"last_name"`
	Position    string            `json:"position"`
	StartDate   string            `json:"start_date"`
	Status      string            `json:"status"`
	Profile     map[string]string `json:"profile"`
	Documents   []PortalDocument  `json:"documents"`
}

// PortalDocument is a document waiting for the new hire's signature.
type PortalDocument struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

var sensitiveProfile = map[string]bool{"tin": true, "sss_no": true, "philhealth_no": true, "pagibig_no": true, "bank_account_no": true}

func v(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func profileMap(p store.EmployeeProfile, e store.GetEmployeeForDocumentRow) map[string]string {
	return map[string]string{
		"address_line1": v(p.AddressLine1), "address_line2": v(p.AddressLine2), "city": v(p.City), "province": v(p.Province),
		"zip_code": v(p.ZipCode), "emergency_name": v(p.EmergencyName), "emergency_phone": v(p.EmergencyPhone),
		"emergency_relation": v(p.EmergencyRelation), "bank_name": v(p.BankName), "bank_account_no": v(p.BankAccountNo),
		"bank_account_name": v(p.BankAccountName), "tin": v(p.Tin), "sss_no": v(p.SssNo), "philhealth_no": v(p.PhilhealthNo),
		"pagibig_no": v(p.PagibigNo),
	}
}

// Portal loads the portal for a token and marks it opened.
func (s *Service) Portal(ctx context.Context, token string) (*PortalView, error) {
	inv, err := s.inviteByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	_ = s.q.MarkOnboardingInviteOpened(ctx, inv.ID)
	emp, err := s.q.GetEmployeeForDocument(ctx, store.GetEmployeeForDocumentParams{ID: inv.EmployeeID, CompanyID: inv.CompanyID})
	if err != nil {
		return nil, ErrGone
	}
	prof, _ := s.q.GetEmployeeProfile(ctx, store.GetEmployeeProfileParams{EmployeeID: inv.EmployeeID, CompanyID: inv.CompanyID})
	pm := profileMap(prof, emp)
	for k := range sensitiveProfile { // already-entered IDs are shown masked
		pm[k] = Mask(pm[k])
	}
	docs, _ := s.q.ListPendingSignaturesForEmployee(ctx, store.ListPendingSignaturesForEmployeeParams{EmployeeID: inv.EmployeeID, CompanyID: inv.CompanyID})
	view := &PortalView{CompanyName: emp.CompanyName, Country: emp.CompanyCountry, FirstName: emp.FirstName, LastName: emp.LastName,
		Position: emp.PositionTitle, StartDate: emp.HireDate.Format("2006-01-02"), Status: inv.Status, Profile: pm, Documents: []PortalDocument{}}
	for _, d := range docs {
		view.Documents = append(view.Documents, PortalDocument{ID: d.ID, Title: d.Title, Kind: d.Kind, Status: d.Status})
	}
	return view, nil
}

// ProfileInput is what the new hire submits. Empty values keep what HR entered.
type ProfileInput struct {
	Phone             string `json:"phone"`
	BirthDate         string `json:"birth_date"`
	Gender            string `json:"gender"`
	CivilStatus       string `json:"civil_status"`
	Nationality       string `json:"nationality"`
	AddressLine1      string `json:"address_line1"`
	AddressLine2      string `json:"address_line2"`
	City              string `json:"city"`
	Province          string `json:"province"`
	ZipCode           string `json:"zip_code"`
	EmergencyName     string `json:"emergency_name"`
	EmergencyPhone    string `json:"emergency_phone"`
	EmergencyRelation string `json:"emergency_relation"`
	BankName          string `json:"bank_name"`
	BankAccountNo     string `json:"bank_account_no"`
	BankAccountName   string `json:"bank_account_name"`
	TIN               string `json:"tin"`
	SSSNo             string `json:"sss_no"`
	PhilHealthNo      string `json:"philhealth_no"`
	PagibigNo         string `json:"pagibig_no"`
	Complete          bool   `json:"complete"` // final submit
}

func keep(newV string, old *string) *string {
	n := strings.TrimSpace(newV)
	if n == "" || strings.Contains(n, "•") { // blank or untouched masked value
		return old
	}
	return &n
}

func opt(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// SubmitProfile saves the new hire's details; with Complete it finalises the invite.
func (s *Service) SubmitProfile(ctx context.Context, token string, in ProfileInput) error {
	inv, err := s.inviteByToken(ctx, token)
	if err != nil {
		return err
	}
	if inv.Status == "submitted" {
		return invalid("this onboarding form was already submitted — contact HR for changes")
	}
	emp, err := s.q.GetEmployeeForDocument(ctx, store.GetEmployeeForDocumentParams{ID: inv.EmployeeID, CompanyID: inv.CompanyID})
	if err != nil {
		return ErrGone
	}
	ids := map[string]string{"tin": in.TIN, "sss_no": in.SSSNo, "philhealth_no": in.PhilHealthNo, "pagibig_no": in.PagibigNo}
	for k, val := range ids {
		if strings.Contains(val, "•") {
			ids[k] = ""
		}
	}
	if errs := ValidateGovIDs(emp.CompanyCountry, ids); len(errs) > 0 {
		return &ValidationError{Msg: "Please check your government ID numbers", Fields: errs}
	}
	var bd pgtype.Date
	if in.BirthDate != "" {
		t, err := time.Parse("2006-01-02", in.BirthDate)
		if err != nil || t.After(s.now()) {
			return &ValidationError{Msg: "Invalid birth date", Fields: map[string]string{"birth_date": "Use YYYY-MM-DD"}}
		}
		bd = pgtype.Date{Time: t, Valid: true}
	}
	if err := s.q.UpdateEmployeeSelfService(ctx, store.UpdateEmployeeSelfServiceParams{
		Phone: opt(in.Phone), BirthDate: bd, Gender: opt(in.Gender), CivilStatus: opt(in.CivilStatus), Nationality: opt(in.Nationality),
		ID: inv.EmployeeID, CompanyID: inv.CompanyID,
	}); err != nil {
		return err
	}
	old, _ := s.q.GetEmployeeProfile(ctx, store.GetEmployeeProfileParams{EmployeeID: inv.EmployeeID, CompanyID: inv.CompanyID})
	if _, err := s.q.UpsertEmployeeProfile(ctx, store.UpsertEmployeeProfileParams{
		EmployeeID: inv.EmployeeID, AddressLine1: keep(in.AddressLine1, old.AddressLine1), AddressLine2: keep(in.AddressLine2, old.AddressLine2),
		City: keep(in.City, old.City), Province: keep(in.Province, old.Province), ZipCode: keep(in.ZipCode, old.ZipCode),
		EmergencyName: keep(in.EmergencyName, old.EmergencyName), EmergencyPhone: keep(in.EmergencyPhone, old.EmergencyPhone),
		EmergencyRelation: keep(in.EmergencyRelation, old.EmergencyRelation), BankName: keep(in.BankName, old.BankName),
		BankAccountNo: keep(in.BankAccountNo, old.BankAccountNo), BankAccountName: keep(in.BankAccountName, old.BankAccountName),
		Tin: keep(in.TIN, old.Tin), SssNo: keep(in.SSSNo, old.SssNo), PhilhealthNo: keep(in.PhilHealthNo, old.PhilhealthNo),
		PagibigNo: keep(in.PagibigNo, old.PagibigNo),
	}); err != nil {
		return err
	}
	if in.Complete {
		if err := s.q.MarkOnboardingInviteSubmitted(ctx, inv.ID); err != nil {
			return err
		}
		_, _ = s.q.InsertHREvent(ctx, store.InsertHREventParams{
			CompanyID: inv.CompanyID, AggregateType: "employee", AggregateID: inv.EmployeeID,
			EventType: "employee.self_onboarded", EventVersion: 1, Payload: []byte(`{}`),
		})
	}
	return nil
}

// ---------------------------------------------------------------------------
// Signatures
// ---------------------------------------------------------------------------

// RequestInput creates a signature request from a template or ad-hoc text.
type RequestInput struct {
	EmployeeID int64  `json:"employee_id" binding:"required"`
	TemplateID *int64 `json:"template_id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Kind       string `json:"kind"`
	SendEmail  bool   `json:"send_email"`
}

// MergeValues builds merge-field values for an employee.
func MergeValues(e store.GetEmployeeForDocumentRow, today time.Time) map[string]string {
	full := e.FirstName + " " + e.LastName
	if e.MiddleName != nil && *e.MiddleName != "" {
		full = e.FirstName + " " + *e.MiddleName + " " + e.LastName
	}
	sal := e.BasicSalary
	if sal != "" {
		var f float64
		if _, err := fmt.Sscanf(sal, "%f", &f); err == nil {
			sal = formatMoney(f)
		}
	}
	return map[string]string{
		"employee.first_name": e.FirstName, "employee.last_name": e.LastName, "employee.full_name": full,
		"employee.employee_no": e.EmployeeNo, "employee.email": v(e.Email), "employee.position": e.PositionTitle,
		"employee.department": e.DepartmentName, "employee.manager": e.ManagerName,
		"employee.hire_date": e.HireDate.Format("January 2, 2006"), "employee.employment_type": strings.ReplaceAll(e.EmploymentType, "_", " "),
		"employee.basic_salary": sal, "company.name": e.CompanyName, "company.address": e.CompanyAddress,
		"today": today.Format("January 2, 2006"),
	}
}

func formatMoney(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	intPart, dec, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteRune(',')
		}
		b.WriteRune(r)
	}
	return b.String() + "." + dec
}

// SignatureLink is returned when a request is created.
type SignatureLink struct {
	Request store.SignatureRequest `json:"request"`
	Link    Link                   `json:"link"`
}

// CreateRequest renders the document and issues a signing link.
func (s *Service) CreateRequest(ctx context.Context, companyID, userID int64, in RequestInput) (*SignatureLink, error) {
	emp, err := s.q.GetEmployeeForDocument(ctx, store.GetEmployeeForDocumentParams{ID: in.EmployeeID, CompanyID: companyID})
	if err != nil {
		return nil, ErrNotFound
	}
	title, body, kind := in.Title, in.Body, in.Kind
	if in.TemplateID != nil {
		t, err := s.q.GetDocumentTemplate(ctx, store.GetDocumentTemplateParams{ID: *in.TemplateID, CompanyID: companyID})
		if err != nil || !t.IsActive {
			return nil, invalid("template not found")
		}
		if title == "" {
			title = t.Name
		}
		body, kind = t.Body, t.Kind
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return nil, invalid("title and body (or template_id) are required")
	}
	if kind == "" {
		kind = "other"
	}
	rendered, missing := Render(body, MergeValues(emp, s.now()))
	if len(missing) > 0 {
		return nil, invalid("fill these fields before sending: %s", strings.Join(missing, ", "))
	}
	tok, hash := NewToken()
	exp := s.now().Add(signatureTTL)
	signer := emp.FirstName + " " + emp.LastName
	req, err := s.q.CreateSignatureRequest(ctx, store.CreateSignatureRequestParams{
		CompanyID: companyID, EmployeeID: in.EmployeeID, TemplateID: in.TemplateID, Kind: kind, Title: title, Body: rendered,
		BodySha256: SHA256Hex(rendered), SignerName: signer, SignerEmail: emp.Email, TokenHash: hash, ExpiresAt: exp, CreatedBy: &userID,
	})
	if err != nil {
		return nil, err
	}
	l := Link{URL: s.baseURL + "/sign/" + tok, ExpiresAt: exp}
	if in.SendEmail {
		l.Emailed = s.send(v(emp.Email), title+" — signature requested", "Please review and sign: "+title,
			emp.CompanyName+" has sent you a document to sign electronically: "+l.URL)
	}
	return &SignatureLink{Request: req, Link: l}, nil
}

// SignView is the document as shown to the signer.
type SignView struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	CompanyName string `json:"company_name"`
	SignerName  string `json:"signer_name"`
	Status      string `json:"status"`
	SignedAt    string `json:"signed_at,omitempty"`
}

func (s *Service) requestByToken(ctx context.Context, token string) (*store.SignatureRequest, error) {
	r, err := s.q.GetSignatureRequestByToken(ctx, HashToken(token))
	if err != nil {
		return nil, ErrGone
	}
	return &r, nil
}

// requestForInvite lets a new hire sign from their onboarding link.
func (s *Service) requestForInvite(ctx context.Context, inviteToken string, requestID int64) (*store.SignatureRequest, error) {
	inv, err := s.inviteByToken(ctx, inviteToken)
	if err != nil {
		return nil, err
	}
	r, err := s.q.GetSignatureRequest(ctx, store.GetSignatureRequestParams{ID: requestID, CompanyID: inv.CompanyID})
	if err != nil || r.EmployeeID != inv.EmployeeID {
		return nil, ErrNotFound
	}
	return &r, nil
}

func (s *Service) view(ctx context.Context, r *store.SignatureRequest) (*SignView, error) {
	if r.Status == "voided" {
		return nil, ErrGone
	}
	if r.Status == "pending" || r.Status == "viewed" {
		if s.now().After(r.ExpiresAt) {
			return nil, ErrGone
		}
		_ = s.q.MarkSignatureViewed(ctx, r.ID)
	}
	emp, err := s.q.GetEmployeeForDocument(ctx, store.GetEmployeeForDocumentParams{ID: r.EmployeeID, CompanyID: r.CompanyID})
	if err != nil {
		return nil, ErrGone
	}
	sv := &SignView{ID: r.ID, Title: r.Title, Body: r.Body, CompanyName: emp.CompanyName, SignerName: r.SignerName, Status: r.Status}
	if r.SignedAt.Valid {
		sv.SignedAt = r.SignedAt.Time.UTC().Format(time.RFC3339)
	}
	return sv, nil
}

// ViewByToken loads a document via its own signing link.
func (s *Service) ViewByToken(ctx context.Context, token string) (*SignView, error) {
	r, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, r)
}

// ViewViaInvite loads a document via the onboarding link.
func (s *Service) ViewViaInvite(ctx context.Context, inviteToken string, id int64) (*SignView, error) {
	r, err := s.requestForInvite(ctx, inviteToken, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, r)
}

// SignInput is the signer's consent.
type SignInput struct {
	SignedName     string `json:"signed_name" binding:"required"`
	SignatureImage string `json:"signature_image"` // optional data:image/png;base64,...
	Consent        bool   `json:"consent"`
	Decline        bool   `json:"decline"`
	DeclineReason  string `json:"decline_reason"`
}

// SignContext carries request metadata for the audit trail.
type SignContext struct {
	IP        string
	UserAgent string
}

func decodeSignature(dataURL string) ([]byte, error) {
	if dataURL == "" {
		return nil, nil
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) || len(dataURL) > 400_000 {
		return nil, invalid("signature image must be a PNG data URL under 300 KB")
	}
	raw, err := base64.StdEncoding.DecodeString(dataURL[len(prefix):])
	if err != nil {
		return nil, invalid("invalid signature image")
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		return nil, invalid("invalid signature image")
	}
	return raw, nil
}

func (s *Service) sign(ctx context.Context, r *store.SignatureRequest, in SignInput, sc SignContext) (*store.SignatureRequest, error) {
	if r.Status != "pending" && r.Status != "viewed" {
		return nil, invalid("this document is already %s", r.Status)
	}
	if s.now().After(r.ExpiresAt) {
		return nil, ErrGone
	}
	if in.Decline {
		out, err := s.q.DeclineSignatureRequest(ctx, store.DeclineSignatureRequestParams{ID: r.ID, DeclinedReason: opt(in.DeclineReason)})
		if err != nil {
			return nil, err
		}
		return &out, nil
	}
	if !in.Consent {
		return nil, invalid("please confirm that you agree to sign electronically")
	}
	if !NameMatches(in.SignedName, r.SignerName) {
		return nil, &ValidationError{Msg: "Type your full name as it appears on the document", Fields: map[string]string{"signed_name": "Must include " + r.SignerName}}
	}
	img, err := decodeSignature(in.SignatureImage)
	if err != nil {
		return nil, err
	}
	var imgPtr *string
	if in.SignatureImage != "" {
		imgPtr = &in.SignatureImage
	}
	ua := sc.UserAgent
	if len(ua) > 500 {
		ua = ua[:500]
	}
	signed, err := s.q.SignSignatureRequest(ctx, store.SignSignatureRequestParams{
		ID: r.ID, SignedName: opt(in.SignedName), SignatureImage: imgPtr, SignedIp: opt(sc.IP), SignedUserAgent: opt(ua),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, invalid("this document can no longer be signed")
		}
		return nil, err
	}
	if err := s.storeSignedPDF(ctx, signed, img); err != nil {
		s.logger.Error("selfonboard: signed pdf", "request_id", signed.ID, "error", err)
	}
	return &signed, nil
}

func (s *Service) storeSignedPDF(ctx context.Context, r store.SignatureRequest, img []byte) error {
	emp, err := s.q.GetEmployeeForDocument(ctx, store.GetEmployeeForDocumentParams{ID: r.EmployeeID, CompanyID: r.CompanyID})
	if err != nil {
		return err
	}
	doc := pdf.SignedDocument{
		CompanyName: emp.CompanyName, CompanyAddress: emp.CompanyAddress, Title: r.Title, Body: r.Body,
		SignerName: r.SignerName, SignerEmail: v(r.SignerEmail), SignedName: v(r.SignedName), SignaturePNG: img,
		SignedAt: r.SignedAt.Time, SignedIP: v(r.SignedIp), UserAgent: v(r.SignedUserAgent), SentAt: r.CreatedAt,
		RequestID: r.ID, BodySHA256: r.BodySha256,
	}
	if r.ViewedAt.Valid {
		t := r.ViewedAt.Time
		doc.ViewedAt = &t
	}
	data, err := pdf.GenerateSignedDocument(doc)
	if err != nil {
		return err
	}
	rel := filepath.Join(fmt.Sprint(r.CompanyID), fmt.Sprint(r.EmployeeID), fmt.Sprintf("signed-%d.pdf", r.ID))
	full := filepath.Join(s.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o640); err != nil {
		return err
	}
	sum := SHA256Hex(string(data))
	if err := s.q.SetSignaturePDF(ctx, store.SetSignaturePDFParams{ID: r.ID, SignedPdfPath: &full, SignedPdfSha256: &sum}); err != nil {
		return err
	}
	mime := "application/pdf"
	name := strings.NewReplacer("/", "-", " ", "_").Replace(r.Title) + "_signed.pdf"
	_, err = s.q.CreateEmployeeDocument(ctx, store.CreateEmployeeDocumentParams{
		CompanyID: r.CompanyID, EmployeeID: r.EmployeeID, DocType: "signed_" + r.Kind, FileName: name, FilePath: full,
		FileSize: int64(len(data)), MimeType: &mime, FileHash: &sum,
	})
	return err
}

// SignByToken signs (or declines) via the document's own link.
func (s *Service) SignByToken(ctx context.Context, token string, in SignInput, sc SignContext) (*store.SignatureRequest, error) {
	r, err := s.requestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.sign(ctx, r, in, sc)
}

// SignViaInvite signs (or declines) from the onboarding portal.
func (s *Service) SignViaInvite(ctx context.Context, inviteToken string, id int64, in SignInput, sc SignContext) (*store.SignatureRequest, error) {
	r, err := s.requestForInvite(ctx, inviteToken, id)
	if err != nil {
		return nil, err
	}
	return s.sign(ctx, r, in, sc)
}

// SignedPDFPath returns the stored signed PDF of a request the caller may access.
func (s *Service) SignedPDFPath(r *store.SignatureRequest) (string, error) {
	if r.SignedPdfPath == nil {
		return "", ErrNotFound
	}
	return *r.SignedPdfPath, nil
}

// ---------------------------------------------------------------------------
// AI drafting
// ---------------------------------------------------------------------------

// DraftTemplate asks the AI to draft a document template using merge fields.
func (s *Service) DraftTemplate(ctx context.Context, companyID int64, kind, instructions string) (string, error) {
	if s.ai == nil {
		return "", invalid("AI is not configured")
	}
	comp, err := s.q.GetCompanyByID(ctx, companyID)
	if err != nil {
		return "", err
	}
	sys := "You draft HR documents for small and mid-size employers. Write plain text (no markdown tables), short paragraphs, " +
		"section headings starting with '## '. Use ONLY these merge fields where the data belongs, exactly as written: " +
		strings.Join(wrapFields(), ", ") + ". Do not invent salary figures, benefits, dates or legal clauses the user did not ask for; " +
		"where the employer must decide something, write [TO CONFIRM: ...]. Follow employment law conventions of country code " + comp.Country + "."
	msg := fmt.Sprintf("Document type: %s\nCompany: %s\nInstructions from HR: %s", kind, comp.Name, instructions)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	resp, err := s.ai.Generate(ctx, provider.Request{System: sys, Messages: []provider.Message{{Role: provider.RoleUser, Content: msg}}, MaxTokens: 1800})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func wrapFields() []string {
	out := make([]string, len(MergeFields))
	for i, f := range MergeFields {
		out[i] = "{{" + f + "}}"
	}
	return out
}
