package govfiling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

// Notice handling, Warp-style: a letter or e-mail from an agency is read,
// classified, matched against the company's own filings and turned into a
// concrete resolution — proof of filing, the missing return, a payment, or
// an escalation. Responses are drafted, never sent: a person approves and
// sends them.

// NoticeTriage is the structured reading of a notice.
type NoticeTriage struct {
	Agency         string   `json:"agency"`
	NoticeType     string   `json:"notice_type"`
	FormCode       string   `json:"form_code"`
	Period         string   `json:"period"`
	ReferenceNo    string   `json:"reference_no"`
	Amount         *float64 `json:"amount"`
	RespondBy      string   `json:"respond_by"` // YYYY-MM-DD
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Recommendation string   `json:"recommendation"`
	ResolutionPath string   `json:"resolution_path"`
	DraftResponse  string   `json:"draft_response"`
	AutoResolvable bool     `json:"auto_resolvable"`
	MatchedID      *int64   `json:"matched_submission_id"`
	TriagedBy      string   `json:"triaged_by"`
}

var (
	reAmount        = regexp.MustCompile(`(?i)(?:PHP|P|₱|\$|USD|SGD|S\$)\s?([0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]{2})?|[0-9]+(?:\.[0-9]{2})?)`)
	reISODate       = regexp.MustCompile(`\b(20[0-9]{2})-([01][0-9])-([0-3][0-9])\b`)
	reLongDate      = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December|Jan|Feb|Mar|Apr|Jun|Jul|Aug|Sep|Sept|Oct|Nov|Dec)\.?\s+([0-3]?[0-9]),?\s+(20[0-9]{2})\b`)
	reMonthYr       = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(20[0-9]{2})\b`)
	reYM            = regexp.MustCompile(`\b(20[0-9]{2})-(0[1-9]|1[0-2])\b`)
	reNoticeQuarter = regexp.MustCompile(`(?i)\b(?:Q([1-4])|([1-4])(?:st|nd|rd|th)\s+quarter)\D{0,12}(20[0-9]{2})\b`)
	reWithin        = regexp.MustCompile(`(?i)within\s+(\d{1,3})\s+(?:calendar\s+|working\s+)?days`)
	reRef           = regexp.MustCompile(`(?i)(?:reference|ref\.?|control|letter|notice|case)\s*(?:no\.?|number|#)\s*[:.]?\s*([A-Z0-9][A-Z0-9\-/]{3,40})`)
)

// noticeForms maps phrases found in notices to catalog form codes.
var noticeForms = []struct {
	re   *regexp.Regexp
	code string
}{
	{regexp.MustCompile(`(?i)\b1601[\s-]?C\b`), "bir_1601c"},
	{regexp.MustCompile(`(?i)\b1604[\s-]?C\b`), "bir_1604c"},
	{regexp.MustCompile(`(?i)\b0619[\s-]?E\b`), "bir_0619e"},
	{regexp.MustCompile(`(?i)\b1601[\s-]?EQ\b`), "bir_1601eq"},
	{regexp.MustCompile(`(?i)\b1902\b`), "bir_1902"},
	{regexp.MustCompile(`(?i)\bR[\s-]?3\b|contribution collection list`), "sss_r3"},
	{regexp.MustCompile(`(?i)\bR[\s-]?1A\b|employment report`), "sss_r1a"},
	{regexp.MustCompile(`(?i)\bRF[\s-]?1\b`), "philhealth_rf1"},
	{regexp.MustCompile(`(?i)\bER[\s-]?2\b`), "philhealth_er2"},
	{regexp.MustCompile(`(?i)\bMCRF\b|\bML[\s-]?1\b|member(?:'s)? contribution remittance`), "pagibig_mcrf"},
	{regexp.MustCompile(`(?i)\bIR8A\b`), "iras_ir8a"},
	{regexp.MustCompile(`(?i)\b(?:form\s+)?941\b`), "irs_941"},
}

var agencyPatterns = []struct {
	re   *regexp.Regexp
	code string
}{
	{regexp.MustCompile(`(?i)bureau of internal revenue|\bBIR\b|revenue district office|\bRDO\b`), "BIR"},
	{regexp.MustCompile(`(?i)social security system|\bSSS\b`), "SSS"},
	{regexp.MustCompile(`(?i)philhealth|philippine health insurance`), "PHILHEALTH"},
	{regexp.MustCompile(`(?i)pag-?ibig|home development mutual fund|\bHDMF\b`), "PAGIBIG"},
	{regexp.MustCompile(`(?i)inland revenue authority|\bIRAS\b`), "IRAS"},
	{regexp.MustCompile(`(?i)central provident fund|\bCPF\b`), "CPF"},
	{regexp.MustCompile(`(?i)internal revenue service|\bIRS\b`), "IRS"},
	{regexp.MustCompile(`(?i)employment development department|\bEDD\b|department of (?:labor|revenue|taxation)`), "STATE"},
}

var typePatterns = []struct {
	re  *regexp.Regexp
	typ string
}{
	{regexp.MustCompile(`(?i)letter of authority|\bLOA\b|tax audit|examination of (?:your )?books|subpoena`), "audit"},
	{regexp.MustCompile(`(?i)preliminary assessment|final assessment|\bPAN\b|\bFAN\b|deficiency|assessment notice|formal letter of demand|\bFLD\b`), "assessment"},
	{regexp.MustCompile(`(?i)failure to file|non-?filing|not (?:yet )?filed|no record of (?:your )?(?:filing|return|remittance)|unfiled|delinquen|missing (?:return|report|remittance)|failed to (?:file|remit|submit)`), "missing_filing"},
	{regexp.MustCompile(`(?i)penalt|surcharge|interest|late (?:payment|remittance|filing)|compromise|unpaid|balance due|amount due`), "late_payment"},
	{regexp.MustCompile(`(?i)registration|update (?:your )?(?:records|information)|change of (?:address|RDO)|employer number`), "registration"},
	{regexp.MustCompile(`(?i)for your information|reminder|advisory|announcement|circular`), "informational"},
}

// RuleTriage reads a notice without AI: agency, form, period, amounts,
// deadline and type come from patterns found in the text.
func RuleTriage(text string, received time.Time) NoticeTriage {
	t := NoticeTriage{NoticeType: "other", TriagedBy: "rules"}
	for _, a := range agencyPatterns {
		if a.re.MatchString(text) {
			t.Agency = a.code
			break
		}
	}
	for _, f := range noticeForms {
		if f.re.MatchString(text) {
			t.FormCode = f.code
			if spec, ok := Lookup(f.code); ok && t.Agency == "" {
				t.Agency = spec.Agency
			}
			break
		}
	}
	for _, p := range typePatterns {
		if p.re.MatchString(text) {
			t.NoticeType = p.typ
			break
		}
	}
	t.Period = findPeriod(text, t.FormCode)
	if m := reRef.FindStringSubmatch(text); m != nil {
		t.ReferenceNo = strings.TrimRight(m[1], ".-/")
	}
	var best float64
	for _, m := range reAmount.FindAllStringSubmatch(text, -1) {
		if v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64); err == nil && v > best {
			best = v
		}
	}
	if best > 0 {
		t.Amount = &best
	}
	t.RespondBy = findDeadline(text, received)
	return t
}

func findPeriod(text, form string) string {
	freq := "monthly"
	if spec, ok := Lookup(form); ok {
		freq = spec.Frequency
	}
	if freq == "quarterly" {
		if m := reNoticeQuarter.FindStringSubmatch(text); m != nil {
			q := m[1]
			if q == "" {
				q = m[2]
			}
			return m[3] + "-Q" + q
		}
	}
	if freq == "annual" {
		if m := regexp.MustCompile(`(?i)(?:taxable |calendar )?year\s+(20[0-9]{2})`).FindStringSubmatch(text); m != nil {
			return m[1]
		}
	}
	if m := reMonthYr.FindStringSubmatch(text); m != nil {
		if tm, err := time.Parse("January 2006", strings.Title(strings.ToLower(m[1]))+" "+m[2]); err == nil { //nolint:staticcheck
			return tm.Format("2006-01")
		}
	}
	if m := reYM.FindStringSubmatch(reISODate.ReplaceAllString(text, " ")); m != nil {
		return m[0]
	}
	if m := reNoticeQuarter.FindStringSubmatch(text); m != nil {
		q := m[1]
		if q == "" {
			q = m[2]
		}
		return m[3] + "-Q" + q
	}
	return ""
}

// findDeadline prefers "within N days" (from the date received), then the
// latest explicit date in the text that is after the date received.
func findDeadline(text string, received time.Time) string {
	if m := reWithin.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= 365 {
			return received.AddDate(0, 0, n).Format("2006-01-02")
		}
	}
	var latest time.Time
	for _, m := range reISODate.FindAllString(text, -1) {
		if d, err := time.Parse("2006-01-02", m); err == nil && d.After(latest) {
			latest = d
		}
	}
	for _, m := range reLongDate.FindAllStringSubmatch(text, -1) {
		mon := strings.Title(strings.ToLower(strings.TrimSuffix(m[1], "."))) //nolint:staticcheck
		if mon == "Sept" {
			mon = "Sep"
		}
		for _, layout := range []string{"January 2 2006", "Jan 2 2006"} {
			if d, err := time.Parse(layout, mon+" "+m[2]+" "+m[3]); err == nil {
				if d.After(latest) {
					latest = d
				}
				break
			}
		}
	}
	if !latest.IsZero() && latest.After(dateOnly(received)) {
		return latest.Format("2006-01-02")
	}
	return ""
}

const noticeAISystem = `You read letters and e-mails that employers receive from government agencies in Southeast Asia and the US
(BIR, SSS, PhilHealth, Pag-IBIG, IRAS, CPF, IRS, state agencies). Extract facts only from the text; never invent numbers.
Respond with JSON only:
{"agency":"BIR|SSS|PHILHEALTH|PAGIBIG|IRAS|CPF|IRS|STATE|",
 "notice_type":"missing_filing|late_payment|assessment|audit|registration|informational|other",
 "form":"form name as written, e.g. 1601-C, R-3, RF-1, MCRF, 941, or empty",
 "period":"YYYY-MM, YYYY-Qn or YYYY, or empty",
 "reference_no":"", "amount": number or null, "respond_by":"YYYY-MM-DD or empty",
 "title":"<8 words>", "summary":"<2 sentences in plain language for an HR manager>"}`

// AITriage asks the model to read the notice and merges its answer into the
// rule-based reading (rules win for form/period when the model is unsure).
func AITriage(ctx context.Context, p provider.Provider, text string, base NoticeTriage) (NoticeTriage, error) {
	if p == nil {
		return base, errors.New("no AI provider")
	}
	if len(text) > 12000 {
		text = text[:12000]
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	resp, err := p.Generate(ctx, provider.Request{
		System:    noticeAISystem,
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: text}},
		MaxTokens: 700,
	})
	if err != nil {
		return base, err
	}
	raw := strings.TrimSpace(resp.Content)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out struct {
		Agency      string   `json:"agency"`
		NoticeType  string   `json:"notice_type"`
		Form        string   `json:"form"`
		Period      string   `json:"period"`
		ReferenceNo string   `json:"reference_no"`
		Amount      *float64 `json:"amount"`
		RespondBy   string   `json:"respond_by"`
		Title       string   `json:"title"`
		Summary     string   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return base, fmt.Errorf("parse notice triage: %w", err)
	}
	t := base
	t.TriagedBy = "ai"
	if a := strings.ToUpper(strings.ReplaceAll(out.Agency, "-", "")); a != "" {
		t.Agency = a
	}
	if validNoticeType(out.NoticeType) {
		t.NoticeType = out.NoticeType
	}
	if t.FormCode == "" && out.Form != "" {
		for _, f := range noticeForms {
			if f.re.MatchString(out.Form) {
				t.FormCode = f.code
				break
			}
		}
	}
	if out.Period != "" && (reYM.MatchString(out.Period) || strings.Contains(out.Period, "-Q") || len(out.Period) == 4) {
		t.Period = out.Period
	}
	if out.ReferenceNo != "" {
		t.ReferenceNo = out.ReferenceNo
	}
	if out.Amount != nil && *out.Amount > 0 {
		t.Amount = out.Amount
	}
	if _, err := time.Parse("2006-01-02", out.RespondBy); err == nil {
		t.RespondBy = out.RespondBy
	}
	t.Title = strings.TrimSpace(out.Title)
	t.Summary = strings.TrimSpace(out.Summary)
	return t, nil
}

func validNoticeType(s string) bool {
	switch s {
	case "missing_filing", "late_payment", "assessment", "audit", "registration", "informational", "other":
		return true
	}
	return false
}

var agencyNames = map[string]string{
	"BIR": "Bureau of Internal Revenue", "SSS": "Social Security System", "PHILHEALTH": "PhilHealth",
	"PAGIBIG": "Pag-IBIG Fund", "IRAS": "IRAS", "CPF": "CPF Board", "IRS": "Internal Revenue Service", "STATE": "the state agency",
}

// Resolve decides how a notice gets resolved, using the company's own
// filing records. matched is the latest submission of the form/period.
func Resolve(t *NoticeTriage, company store.Company, matched *store.GovSubmission) {
	spec, hasSpec := Lookup(t.FormCode)
	formName := t.FormCode
	if hasSpec {
		formName = spec.Name
		if i := strings.Index(formName, " ("); i > 0 && len(formName) > 60 {
			formName = formName[:i]
		}
	}
	if t.Title == "" {
		t.Title = defaultNoticeTitle(t, formName)
	}
	agency := agencyNames[t.Agency]
	if agency == "" {
		agency = "the agency"
	}
	filedProof := matched != nil && matched.Status == "submitted"
	if matched != nil {
		id := matched.ID
		t.MatchedID = &id
	}
	switch t.NoticeType {
	case "audit", "assessment":
		t.ResolutionPath = "escalate"
		t.Recommendation = "This is a formal tax audit or assessment. Do not reply on your own: share it with your tax adviser today. HalaPeople has the payroll registers, filed returns and payment references for the period ready for them."
		if t.Summary == "" {
			t.Summary = fmt.Sprintf("%s has opened a formal %s. Deadlines on these are strict.", agency, map[string]string{"audit": "audit (letter of authority)", "assessment": "assessment"}[t.NoticeType])
		}
	case "missing_filing", "late_payment":
		switch {
		case filedProof:
			t.ResolutionPath = "proof_of_filing"
			t.AutoResolvable = true
			ref := "the acknowledgement on file"
			if matched.ReferenceNo != nil && *matched.ReferenceNo != "" {
				ref = "reference no. " + *matched.ReferenceNo
			}
			when := ""
			if matched.SubmittedAt.Valid {
				when = " on " + matched.SubmittedAt.Time.Format("January 2, 2006")
			}
			t.Recommendation = fmt.Sprintf("Our records show %s for %s was filed%s (%s). Send the drafted reply with the filing proof attached; no payment should be due.", formName, t.Period, when, ref)
			t.DraftResponse = proofLetter(company, agency, t, formName, when, ref)
		case hasSpec && t.Period != "" && matched != nil:
			t.ResolutionPath = "prepare_filing"
			t.AutoResolvable = true
			t.Recommendation = fmt.Sprintf("%s for %s is prepared but not filed yet (status: %s). Review and approve it in Government filings, file it, then pay any penalty stated in the notice and mark this notice resolved.", formName, t.Period, matched.Status)
		case hasSpec && t.Period != "":
			t.ResolutionPath = "prepare_filing"
			t.AutoResolvable = true
			t.Recommendation = fmt.Sprintf("There is no submitted %s for %s on record. Prepare it now from payroll, approve and file it, then pay any penalty stated in the notice.", formName, t.Period)
		case t.Amount != nil:
			t.ResolutionPath = "pay"
			t.Recommendation = "Check the amount against your payment records. If it is correct, pay it through the agency's payment channel and record the payment reference here."
		default:
			t.ResolutionPath = "review"
			t.Recommendation = "The notice does not name a form and period we can match. Add them (or the notice text) and re-run triage."
		}
	case "registration":
		t.ResolutionPath = "update_records"
		t.Recommendation = "The agency is asking you to register or update company details. Check your employer numbers under Settings → Company and update the agency's records through its portal."
	case "informational":
		t.ResolutionPath = "no_action"
		t.AutoResolvable = true
		t.Recommendation = "No response needed. Read it and mark it resolved."
	default:
		t.ResolutionPath = "review"
		t.Recommendation = "Read the notice and decide what's needed; the agent couldn't classify it with confidence."
	}
	if t.Summary == "" {
		t.Summary = defaultNoticeSummary(t, agency, formName)
	}
}

func defaultNoticeTitle(t *NoticeTriage, formName string) string {
	label := map[string]string{
		"missing_filing": "Missing filing", "late_payment": "Penalty / late payment", "assessment": "Tax assessment",
		"audit": "Audit", "registration": "Registration", "informational": "Advisory", "other": "Notice",
	}[t.NoticeType]
	if t.FormCode != "" {
		short := strings.Fields(formName)
		if len(short) >= 2 {
			label += " — " + short[0] + " " + short[1]
		}
	}
	if t.Period != "" {
		label += " " + t.Period
	}
	return label
}

func defaultNoticeSummary(t *NoticeTriage, agency, formName string) string {
	var b strings.Builder
	switch t.NoticeType {
	case "missing_filing":
		b.WriteString(agency + " says a return was not filed")
	case "late_payment":
		b.WriteString(agency + " is asking for payment of a penalty or balance")
	case "registration":
		b.WriteString(agency + " is asking you to register or update records")
	case "informational":
		b.WriteString(agency + " sent an advisory")
	default:
		b.WriteString("A notice from " + agency)
	}
	if t.FormCode != "" {
		b.WriteString(" (" + formName)
		if t.Period != "" {
			b.WriteString(", " + t.Period)
		}
		b.WriteString(")")
	}
	b.WriteString(".")
	if t.Amount != nil {
		b.WriteString(fmt.Sprintf(" Amount stated: %s.", strconv.FormatFloat(*t.Amount, 'f', 2, 64)))
	}
	if t.RespondBy != "" {
		b.WriteString(" Respond by " + t.RespondBy + ".")
	}
	return b.String()
}

func proofLetter(c store.Company, agency string, t *NoticeTriage, formName, when, ref string) string {
	legal := c.Name
	if c.LegalName != nil && *c.LegalName != "" {
		legal = *c.LegalName
	}
	var ids []string
	if c.Tin != nil && *c.Tin != "" {
		ids = append(ids, "TIN "+*c.Tin)
	}
	if t.Agency == "SSS" && c.SssErNo != nil && *c.SssErNo != "" {
		ids = append(ids, "SSS ER No. "+*c.SssErNo)
	}
	if t.Agency == "PHILHEALTH" && c.PhilhealthErNo != nil && *c.PhilhealthErNo != "" {
		ids = append(ids, "PhilHealth ER No. "+*c.PhilhealthErNo)
	}
	if t.Agency == "PAGIBIG" && c.PagibigErNo != nil && *c.PagibigErNo != "" {
		ids = append(ids, "Pag-IBIG ER No. "+*c.PagibigErNo)
	}
	subject := "Re: Notice"
	if t.ReferenceNo != "" {
		subject += " No. " + t.ReferenceNo
	}
	subject += " — " + formName
	if t.Period != "" {
		subject += " for " + t.Period
	}
	return strings.Join([]string{
		time.Now().Format("January 2, 2006"),
		"",
		agency,
		"",
		subject,
		"",
		"Dear Sir/Madam,",
		"",
		fmt.Sprintf("We acknowledge receipt of your notice regarding %s for the period %s. Please be informed that %s (%s) filed this return%s, with %s. A copy of the filed return and its acknowledgement is attached.",
			formName, nonEmpty(t.Period, "stated"), legal, strings.Join(nonEmptySlice(ids, "employer"), ", "), when, ref),
		"",
		"We respectfully request that your records be updated accordingly and that the notice be considered resolved. Should you need further information, please contact the undersigned.",
		"",
		"Very truly yours,",
		"",
		"",
		legal,
	}, "\n")
}

func nonEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func nonEmptySlice(s []string, def string) []string {
	if len(s) == 0 {
		return []string{def}
	}
	return s
}

// Notifier posts alerts (e.g. internal/alerts → Slack).
type Notifier interface {
	Notify(companyID int64, event, text, link string)
}

// SetNotifier wires alerts for new notices.
func (s *Service) SetNotifier(n Notifier) { s.alerts = n }

// IntakeNotice triages a notice and stores it.
func (s *Service) IntakeNotice(ctx context.Context, companyID int64, userID *int64, text string, fileName, filePath *string) (*store.GovNotice, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, &ValidationError{Msg: "paste the notice text (or the e-mail body) so it can be read"}
	}
	t := s.triage(ctx, companyID, text)
	n, err := s.saveNotice(ctx, companyID, userID, 0, text, fileName, filePath, t)
	if err == nil && s.alerts != nil {
		due := ""
		if t.RespondBy != "" {
			due = " · respond by " + t.RespondBy
		}
		s.alerts.Notify(companyID, "notice", fmt.Sprintf(":envelope_with_arrow: *%s notice*: %s%s\n%s", nonEmpty(t.Agency, "Government"), t.Title, due, t.Recommendation), fmt.Sprintf("/notices?id=%d", n.ID))
	}
	return n, err
}

// RetriageNotice re-reads a stored notice (e.g. after filings changed).
func (s *Service) RetriageNotice(ctx context.Context, companyID, id int64) (*store.GovNotice, error) {
	n, err := s.q.GetGovNotice(ctx, store.GetGovNoticeParams{ID: id, CompanyID: companyID})
	if err != nil {
		return nil, ErrNotFound
	}
	t := s.triage(ctx, companyID, n.SourceText)
	return s.saveNotice(ctx, companyID, nil, n.ID, n.SourceText, nil, nil, t)
}

func (s *Service) triage(ctx context.Context, companyID int64, text string) NoticeTriage {
	t := RuleTriage(text, s.now())
	if s.ai != nil && s.ai.Provider != nil {
		if at, err := AITriage(ctx, s.ai.Provider, text, t); err == nil {
			t = at
		} else {
			s.logger.Warn("govfiling: AI notice triage failed, using rules", "error", err)
		}
	}
	comp, err := s.q.GetCompanyByID(ctx, companyID)
	if err != nil {
		comp = store.Company{ID: companyID}
	}
	var matched *store.GovSubmission
	if t.FormCode != "" && t.Period != "" {
		if sub, err := s.q.FindSubmissionForNotice(ctx, store.FindSubmissionForNoticeParams{CompanyID: companyID, FormCode: t.FormCode, Period: t.Period}); err == nil {
			matched = &sub
		} else if !errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("govfiling: notice match", "error", err)
		}
	}
	Resolve(&t, comp, matched)
	return t
}

func noticeStatus(t NoticeTriage) string {
	switch {
	case t.ResolutionPath == "escalate":
		return "escalated"
	case t.DraftResponse != "":
		return "awaiting_approval"
	default:
		return "open"
	}
}

func (s *Service) saveNotice(ctx context.Context, companyID int64, userID *int64, id int64, text string, fileName, filePath *string, t NoticeTriage) (*store.GovNotice, error) {
	var amount pgtype.Numeric
	if t.Amount != nil {
		amount = pgtype.Numeric{Int: big.NewInt(int64(*t.Amount*100 + 0.5)), Exp: -2, Valid: true}
	}
	var respond pgtype.Date
	if d, err := time.Parse("2006-01-02", t.RespondBy); err == nil {
		respond = pgtype.Date{Time: d, Valid: true}
	}
	if id == 0 {
		n, err := s.q.CreateGovNotice(ctx, store.CreateGovNoticeParams{
			CompanyID: companyID, Agency: t.Agency, NoticeType: t.NoticeType, FormCode: t.FormCode, Period: t.Period,
			ReferenceNo: truncate(t.ReferenceNo, 100), Amount: amount, RespondBy: respond, Title: truncate(t.Title, 300),
			Summary: t.Summary, Recommendation: t.Recommendation, ResolutionPath: t.ResolutionPath, DraftResponse: t.DraftResponse,
			MatchedSubmissionID: t.MatchedID, Status: noticeStatus(t), AutoResolvable: t.AutoResolvable, TriagedBy: t.TriagedBy,
			SourceText: text, FileName: fileName, FilePath: filePath, CreatedBy: userID,
		})
		return &n, err
	}
	n, err := s.q.UpdateGovNoticeTriage(ctx, store.UpdateGovNoticeTriageParams{
		ID: id, CompanyID: companyID, Agency: t.Agency, NoticeType: t.NoticeType, FormCode: t.FormCode, Period: t.Period,
		ReferenceNo: truncate(t.ReferenceNo, 100), Amount: amount, RespondBy: respond, Title: truncate(t.Title, 300),
		Summary: t.Summary, Recommendation: t.Recommendation, ResolutionPath: t.ResolutionPath, DraftResponse: t.DraftResponse,
		MatchedSubmissionID: t.MatchedID, Status: noticeStatus(t), AutoResolvable: t.AutoResolvable, TriagedBy: t.TriagedBy,
	})
	return &n, err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// maxNoticeFile caps uploaded notice scans.
const maxNoticeFile = 15 << 20

var noticeFileExt = map[string]bool{".pdf": true, ".png": true, ".jpg": true, ".jpeg": true, ".txt": true, ".eml": true}

// StoreNoticeFile saves an uploaded notice under the company's private
// directory and returns its relative path.
func (s *Service) StoreNoticeFile(companyID int64, name string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if !noticeFileExt[ext] {
		return "", &ValidationError{Msg: "upload a PDF, image, .txt or .eml file"}
	}
	if len(data) > maxNoticeFile {
		return "", &ValidationError{Msg: "file is larger than 15 MB"}
	}
	sum := sha256.Sum256(data)
	rel := filepath.Join(fmt.Sprint(companyID), "notices", hex.EncodeToString(sum[:8])+ext)
	full := filepath.Join(s.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, data, 0o640); err != nil {
		return "", err
	}
	return rel, nil
}
