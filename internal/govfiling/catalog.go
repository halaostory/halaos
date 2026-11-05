// Package govfiling is the government filing agent: it prepares statutory
// returns, reviews them (deterministic rules + AI), waits for an HR approval,
// and submits them through whatever channel the agency supports:
//
//   - api      — an official machine interface (e.g. IRAS AIS API), or a
//     licensed e-filing partner reached through a signed webhook
//   - email    — agency e-mail submission (e.g. BIR eSubmission for alphalists)
//   - browser  — supervised headless-browser automation of the agency portal;
//     CAPTCHA / OTP / final confirmation are always handed to a human
//   - manual   — the file plus step-by-step instructions; HR submits and
//     records the reference number
//
// Nothing is ever sent to a government system without an explicit approval.
package govfiling

import (
	"os"
	"sort"
	"strings"
)

// Channel is how a return reaches the agency.
type Channel string

const (
	ChannelAPI     Channel = "api"
	ChannelEmail   Channel = "email"
	ChannelBrowser Channel = "browser"
	ChannelManual  Channel = "manual"
)

// FormSpec describes one statutory return the agent knows how to handle.
type FormSpec struct {
	Code       string    `json:"code"`    // e.g. "bir_1601c"
	Country    string    `json:"country"` // PHL, SGP, USA, LKA
	Agency     string    `json:"agency"`  // BIR, SSS, ...
	Name       string    `json:"name"`
	Frequency  string    `json:"frequency"` // monthly, quarterly, annual
	Channels   []Channel `json:"channels"`  // supported, best first
	Portal     string    `json:"portal"`    // portal key for credentials
	PortalURL  string    `json:"portal_url"`
	PlaybookID string    `json:"playbook_id,omitempty"`
	EmailTo    string    `json:"email_to,omitempty"`
	FileFormat string    `json:"file_format"`
	Steps      []string  `json:"steps"` // manual / supervised instructions
	Notes      string    `json:"notes,omitempty"`
	// Registration forms report new hires to an agency (period = hire month,
	// due a fixed number of days after the earliest hire in the batch).
	Registration     bool `json:"registration,omitempty"`
	DueDaysAfterHire int  `json:"due_days_after_hire,omitempty"`
	// MemberIDOptional: lines without the agency ID are a warning (the
	// registration itself issues it), not a blocking error.
	MemberIDOptional bool `json:"member_id_optional,omitempty"`
}

// DefaultChannel is the best channel the spec supports.
func (f FormSpec) DefaultChannel() Channel {
	if len(f.Channels) == 0 {
		return ChannelManual
	}
	return f.Channels[0]
}

// Supports reports whether the form can go through channel c.
func (f FormSpec) Supports(c Channel) bool {
	for _, x := range f.Channels {
		if x == c {
			return true
		}
	}
	return false
}

var catalog = map[string]FormSpec{
	// ── Philippines ──────────────────────────────────────────────────────
	"bir_1601c": {
		Code: "bir_1601c", Country: "PHL", Agency: "BIR", Frequency: "monthly",
		Name:     "BIR 1601-C Monthly Remittance Return of Income Taxes Withheld on Compensation",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "efps", PortalURL: "https://efps.bir.gov.ph",
		PlaybookID: "bir.efps.1601c", FileFormat: "summary (PDF/JSON)",
		Steps: []string{
			"Log in to eFPS (https://efps.bir.gov.ph) with the company TIN user.",
			"Open BIR Form 1601-C, choose the return period, and enter the amounts from the attached summary.",
			"Validate, submit, then pay through an eFPS authorised agent bank.",
			"Copy the Filing Reference Number (FRN) back into HalaPeople.",
		},
		Notes: "eBIRForms users file with the offline package; HalaPeople prepares the amounts.",
	},
	"bir_1604c": {
		Code: "bir_1604c", Country: "PHL", Agency: "BIR", Frequency: "annual",
		Name:     "BIR 1604-C Annual Information Return + Alphalist of Employees",
		Channels: []Channel{ChannelEmail, ChannelManual}, Portal: "esubmission",
		EmailTo: "esubmission@bir.gov.ph", FileFormat: "alphalist (.dat/.csv)",
		Steps: []string{
			"Validate the alphalist with the BIR Alphalist Data Entry & Validation Module.",
			"E-mail the validated file to esubmission@bir.gov.ph (eFPS filers may attach it in eFPS).",
			"Keep the BIR acknowledgement e-mail and record its reference in HalaPeople.",
		},
		Notes: "BIR treats an alphalist that fails its validation module as not received.",
	},
	"sss_r3": {
		Code: "sss_r3", Country: "PHL", Agency: "SSS", Frequency: "monthly",
		Name:     "SSS Contribution Collection List (R-3)",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "my_sss", PortalURL: "https://member.sss.gov.ph",
		PlaybookID: "sss.mysss.r3", FileFormat: "R-3 text file",
		Steps: []string{
			"Log in to My.SSS employer account.",
			"Go to Contributions → Contribution Collection List and upload the R-3 file.",
			"Generate the Payment Reference Number (PRN) and pay before the due date.",
			"Record the PRN in HalaPeople.",
		},
	},
	"philhealth_rf1": {
		Code: "philhealth_rf1", Country: "PHL", Agency: "PHILHEALTH", Frequency: "monthly",
		Name:     "PhilHealth RF-1 Employer Remittance Report",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "eprs", PortalURL: "https://eprs.philhealth.gov.ph",
		PlaybookID: "philhealth.eprs.rf1", FileFormat: "RF-1 CSV",
		Steps: []string{
			"Log in to PhilHealth EPRS.",
			"Upload the RF-1 file for the applicable month and generate the Statement of Premium Account (SPA).",
			"Pay the SPA and record its number in HalaPeople.",
		},
	},
	"pagibig_mcrf": {
		Code: "pagibig_mcrf", Country: "PHL", Agency: "PAGIBIG", Frequency: "monthly",
		Name:     "Pag-IBIG Member Contribution Remittance Form (MCRF)",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "esrs", PortalURL: "https://www.pagibigfundservices.com/eSRS",
		PlaybookID: "pagibig.esrs.mcrf", FileFormat: "electronic file layout (.txt)",
		Steps: []string{
			"Log in to Pag-IBIG eSRS (for employers with up to 30 employees) or Virtual Pag-IBIG.",
			"Upload the remittance file, review the schedule and confirm.",
			"Record the Pag-IBIG reference number in HalaPeople.",
		},
	},
	// Expanded withholding on contractor / freelancer payments.
	"bir_0619e": {
		Code: "bir_0619e", Country: "PHL", Agency: "BIR", Frequency: "monthly",
		Name:     "BIR 0619-E Monthly Remittance of Creditable Income Taxes Withheld (Expanded)",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "efps", PortalURL: "https://efps.bir.gov.ph",
		PlaybookID: "bir.efps.0619e", FileFormat: "summary (CSV)",
		Steps: []string{
			"Filed for the 1st and 2nd month of each quarter (the 3rd month is covered by 1601-EQ).",
			"Log in to eFPS (or eBIRForms), open 0619-E, enter the total tax withheld from the attached summary, submit and pay by the 10th of the following month.",
			"Record the filing reference in HalaPeople.",
		},
	},
	"bir_1601eq": {
		Code: "bir_1601eq", Country: "PHL", Agency: "BIR", Frequency: "quarterly",
		Name:     "BIR 1601-EQ Quarterly Remittance Return of Creditable Income Taxes Withheld (Expanded) + QAP",
		Channels: []Channel{ChannelEmail, ChannelManual}, Portal: "esubmission", EmailTo: "esubmission@bir.gov.ph",
		FileFormat: "Quarterly Alphalist of Payees (CSV)",
		Steps: []string{
			"File 1601-EQ in eFPS/eBIRForms by the last day of the month after the quarter, crediting the 0619-E remittances of the first two months.",
			"Submit the Quarterly Alphalist of Payees (QAP) — validated with the BIR Alphalist Data Entry module — to esubmission@bir.gov.ph.",
			"Issue BIR 2307 certificates to each payee (Contractors → 2307).",
		},
	},
	// New-hire registrations (Warp-style "register for you"): batched per hire month.
	"sss_r1a": {
		Code: "sss_r1a", Country: "PHL", Agency: "SSS", Frequency: "hire_month", Registration: true, DueDaysAfterHire: 30,
		Name:     "SSS R-1A Employment Report (new employees)",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "my_sss", PortalURL: "https://member.sss.gov.ph",
		PlaybookID: "sss.mysss.r1a", FileFormat: "R-1A text file",
		Steps: []string{
			"Every new employee needs an SS number first (employees without one register online at My.SSS — send them the HalaPeople onboarding link to record it).",
			"Log in to the My.SSS employer account → Employment Report (R-1A) and upload the file, or encode each employee.",
			"Submit within 30 days of the employee's start date and record the transaction number in HalaPeople.",
		},
	},
	"philhealth_er2": {
		Code: "philhealth_er2", Country: "PHL", Agency: "PHILHEALTH", Frequency: "hire_month", Registration: true, DueDaysAfterHire: 30,
		MemberIDOptional: true,
		Name:             "PhilHealth ER2 Report of Employee-Members (new employees)",
		Channels:         []Channel{ChannelBrowser, ChannelManual}, Portal: "eprs", PortalURL: "https://eprs.philhealth.gov.ph",
		PlaybookID: "philhealth.eprs.er2", FileFormat: "ER2 CSV",
		Steps: []string{
			"Report new employees on ER2 within 30 days of hiring (EPRS or PhilHealth office).",
			"Employees without a PhilHealth Identification Number (PIN) are registered with a PMRF attached to the report.",
			"Record the acknowledgement number in HalaPeople.",
		},
	},
	"pagibig_registration": {
		Code: "pagibig_registration", Country: "PHL", Agency: "PAGIBIG", Frequency: "hire_month", Registration: true, DueDaysAfterHire: 30,
		MemberIDOptional: true,
		Name:             "Pag-IBIG membership registration of new employees",
		Channels:         []Channel{ChannelBrowser, ChannelManual}, Portal: "esrs", PortalURL: "https://www.pagibigfundservices.com/OnlineRegistration",
		PlaybookID: "pagibig.registration", FileFormat: "member list (CSV)",
		Steps: []string{
			"Register employees without a Pag-IBIG MID through Virtual Pag-IBIG / online membership registration (MDF).",
			"Employees who already have a MID are added to the next monthly remittance (MCRF).",
			"Record the issued MIDs on each employee (HalaPeople onboarding link) and the registration reference here.",
		},
	},
	"bir_1902": {
		Code: "bir_1902", Country: "PHL", Agency: "BIR", Frequency: "hire_month", Registration: true, DueDaysAfterHire: 10,
		Name:     "BIR 1902 TIN registration for new employees without a TIN",
		Channels: []Channel{ChannelManual}, Portal: "orus", PortalURL: "https://orus.bir.gov.ph",
		FileFormat: "employee list (CSV)",
		Steps: []string{
			"For each listed employee, file BIR Form 1902 through ORUS or at the employer's RDO within 10 days of the start date.",
			"Never apply for a second TIN: confirm the employee has no TIN from a previous employer (penalties apply to duplicate TINs).",
			"Enter the issued TIN on the employee profile and record the ORUS reference here.",
		},
		Notes: "Lists only new hires with no TIN on file.",
	},
	// ── Singapore ────────────────────────────────────────────────────────
	"iras_ir8a": {
		Code: "iras_ir8a", Country: "SGP", Agency: "IRAS", Frequency: "annual",
		Name:     "IRAS IR8A employment income (Auto-Inclusion Scheme)",
		Channels: []Channel{ChannelManual, ChannelAPI}, Portal: "iras_ais_api",
		FileFormat: "AIS API JSON",
		Steps: []string{
			"Authorise HalaPeople with CorpPass when prompted (IRAS requires a CorpPass login for each submission).",
			"HalaPeople submits IR8A records through the IRAS AIS API and stores the acknowledgement.",
			"Deadline: 1 March (IRAS encourages submission by 10 February).",
		},
		Notes: "Requires an IRAS API subscription (Submission of Employment Income Records, CorpPass).",
	},
	"cpf_contribution": {
		Code: "cpf_contribution", Country: "SGP", Agency: "CPF", Frequency: "monthly",
		Name:     "CPF monthly contribution submission",
		Channels: []Channel{ChannelManual}, Portal: "cpf_ezpay", PortalURL: "https://www.cpf.gov.sg/employer",
		FileFormat: "CPF FTP file",
		Steps: []string{
			"Log in to CPF EZPay with CorpPass.",
			"Upload the contribution file, review and submit before the 14th.",
			"Record the submission reference in HalaPeople.",
		},
	},
	// ── United States ────────────────────────────────────────────────────
	"irs_941": {
		Code: "irs_941", Country: "USA", Agency: "IRS", Frequency: "quarterly",
		Name:     "IRS Form 941 Employer's Quarterly Federal Tax Return",
		Channels: []Channel{ChannelAPI, ChannelManual}, Portal: "efile_partner",
		FileFormat: "941 data (JSON/PDF)",
		Steps: []string{
			"File through an IRS-authorised e-file provider (configure it under Government → Filing partner), or",
			"file on paper / through your tax professional using the attached figures.",
			"Deposits go through EFTPS on your deposit schedule.",
		},
		Notes: "IRS 94x e-file is only available to authorised providers; HalaPeople hands off to one through the partner webhook.",
	},
	"us_state_registration": {
		Code: "us_state_registration", Country: "USA", Agency: "STATE", Frequency: "per_state",
		Name:     "State withholding & unemployment account registration",
		Channels: []Channel{ChannelManual, ChannelAPI}, Portal: "efile_partner",
		FileFormat: "registration data (JSON)",
		Steps: []string{
			"Register for a state withholding account and a state unemployment (SUI) account in the employee's work state.",
			"Enter the issued account numbers and SUI rate under Company → Registration numbers.",
		},
		Notes: "Created automatically when an active employee lives in a state where the company has no withholding registration (period = state code, e.g. CA). Recorded registrations are stored as state_withholding_<ST>.",
	},
	// ── Sri Lanka ────────────────────────────────────────────────────────
	"lk_epf_etf": {
		Code: "lk_epf_etf", Country: "LKA", Agency: "EPF", Frequency: "monthly",
		Name:     "EPF / ETF monthly contribution return",
		Channels: []Channel{ChannelManual}, Portal: "epf_eremit",
		FileFormat: "C-form data",
		Steps: []string{
			"Submit the C-form and pay EPF (Central Bank) and ETF (ETF Board) contributions by the last working day of the following month.",
			"Record the payment references in HalaPeople.",
		},
	},
	// ── Demo / test portal ───────────────────────────────────────────────
	"mock_return": {
		Code: "mock_return", Country: "*", Agency: "MOCK", Frequency: "monthly",
		Name:     "Demo return on the built-in mock portal",
		Channels: []Channel{ChannelBrowser, ChannelManual}, Portal: "mock",
		PlaybookID: "mock.return", FileFormat: "text",
		Steps: []string{"Used to try the browser agent end-to-end without touching a real government system."},
	},
}

// filingTypeAlias maps tax_filings.filing_type values to catalog codes.
var filingTypeAlias = map[string]string{
	"bir_1601c":      "bir_1601c",
	"bir_1604c":      "bir_1604c",
	"bir_alphalist":  "bir_1604c",
	"sss_r3":         "sss_r3",
	"philhealth_rf1": "philhealth_rf1",
	"pagibig_ml1":    "pagibig_mcrf",
	"pagibig_mcrf":   "pagibig_mcrf",
	"iras_ir8a":      "iras_ir8a",
	"cpf":            "cpf_contribution",
	"irs_941":        "irs_941",
	"us_941":         "irs_941",
	"lk_epf":         "lk_epf_etf",
}

// Lookup returns the form spec for a catalog code or a tax_filings filing_type.
func Lookup(code string) (FormSpec, bool) {
	code = strings.ToLower(strings.TrimSpace(code))
	if f, ok := catalog[code]; ok {
		return withOverrides(f), true
	}
	if c, ok := filingTypeAlias[code]; ok {
		return withOverrides(catalog[c]), true
	}
	return FormSpec{}, false
}

// Forms lists the catalog, optionally filtered by country (mock form is
// listed only when a mock portal URL is configured).
func Forms(country string) []FormSpec {
	out := []FormSpec{}
	for _, f := range catalog {
		if f.Agency == "MOCK" && os.Getenv("GOV_MOCK_PORTAL_URL") == "" {
			continue
		}
		if country == "" || f.Country == country || f.Country == "*" {
			out = append(out, withOverrides(f))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Country != out[j].Country {
			return out[i].Country < out[j].Country
		}
		return out[i].Code < out[j].Code
	})
	return out
}

func withOverrides(f FormSpec) FormSpec {
	if f.Agency == "MOCK" {
		f.PortalURL = os.Getenv("GOV_MOCK_PORTAL_URL")
	}
	return f
}
