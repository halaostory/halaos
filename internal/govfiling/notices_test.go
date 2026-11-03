package govfiling

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

var received = time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)

const birNotice = `Republic of the Philippines
Department of Finance
BUREAU OF INTERNAL REVENUE
Revenue District Office No. 049 - North Makati

Reference No.: RDO49-2026-11-0457
Date: October 28, 2026

ACME TRADING CORP.
TIN 123-456-789-000

Subject: Failure to File — BIR Form 1601-C for September 2026

Our records show that you have not yet filed BIR Form No. 1601-C (Monthly Remittance Return of Income
Taxes Withheld on Compensation) for the month of September 2026. You are hereby required to file the
return and pay the corresponding penalties of PHP 1,000.00 within ten (10) days... within 10 days from
receipt of this letter.`

const sssNotice = `SOCIAL SECURITY SYSTEM
Billing and Collection — Employer Delinquency Notice
Employer No. 03-1234567-8

Our records show no record of remittance of contributions (R-3) for the applicable period 2026-08.
Total amount due: P 18,450.50 inclusive of penalty. Please settle on or before November 20, 2026.`

const loaNotice = `BUREAU OF INTERNAL REVENUE
LETTER OF AUTHORITY No. LOA-049-2026-00123
Pursuant to Section 6(A) of the NIRC, the Revenue Officer named below is authorized to examine your
books of accounts and other accounting records for all internal revenue taxes for taxable year 2025.`

func TestRuleTriage_BIRMissingFiling(t *testing.T) {
	tr := RuleTriage(birNotice, received)
	if tr.Agency != "BIR" || tr.FormCode != "bir_1601c" || tr.Period != "2026-09" || tr.NoticeType != "missing_filing" {
		t.Fatalf("%+v", tr)
	}
	if tr.ReferenceNo != "RDO49-2026-11-0457" {
		t.Errorf("ref %q", tr.ReferenceNo)
	}
	if tr.Amount == nil || *tr.Amount != 1000 {
		t.Errorf("amount %v", tr.Amount)
	}
	if tr.RespondBy != "2026-11-13" {
		t.Errorf("respond by %q (within 10 days of receipt)", tr.RespondBy)
	}
}

func TestRuleTriage_SSSDelinquency(t *testing.T) {
	tr := RuleTriage(sssNotice, received)
	if tr.Agency != "SSS" || tr.FormCode != "sss_r3" || tr.Period != "2026-08" || tr.NoticeType != "missing_filing" {
		t.Fatalf("%+v", tr)
	}
	if tr.Amount == nil || *tr.Amount != 18450.50 {
		t.Errorf("amount %v", tr.Amount)
	}
	if tr.RespondBy != "2026-11-20" {
		t.Errorf("respond by %q", tr.RespondBy)
	}
}

func TestRuleTriage_AuditEscalates(t *testing.T) {
	tr := RuleTriage(loaNotice, received)
	if tr.NoticeType != "audit" || tr.Agency != "BIR" {
		t.Fatalf("%+v", tr)
	}
	Resolve(&tr, store.Company{Name: "Acme"}, nil)
	if tr.ResolutionPath != "escalate" || noticeStatus(tr) != "escalated" || tr.AutoResolvable {
		t.Errorf("%+v", tr)
	}
}

func TestResolve_ProofOfFiling(t *testing.T) {
	tr := RuleTriage(birNotice, received)
	ref := "FRN-889911"
	sub := &store.GovSubmission{ID: 42, Status: "submitted", ReferenceNo: &ref,
		SubmittedAt: pgtype.Timestamptz{Time: time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC), Valid: true}}
	tin := "123-456-789-000"
	Resolve(&tr, store.Company{Name: "Acme", Tin: &tin}, sub)
	if tr.ResolutionPath != "proof_of_filing" || !tr.AutoResolvable || tr.MatchedID == nil || *tr.MatchedID != 42 {
		t.Fatalf("%+v", tr)
	}
	for _, want := range []string{"FRN-889911", "October 9, 2026", "TIN 123-456-789-000", "RDO49-2026-11-0457", "2026-09"} {
		if !strings.Contains(tr.DraftResponse, want) {
			t.Errorf("draft lacks %q:\n%s", want, tr.DraftResponse)
		}
	}
	if noticeStatus(tr) != "awaiting_approval" {
		t.Errorf("status %s", noticeStatus(tr))
	}
}

func TestResolve_PrepareMissingFiling(t *testing.T) {
	tr := RuleTriage(sssNotice, received)
	Resolve(&tr, store.Company{Name: "Acme"}, &store.GovSubmission{ID: 7, Status: "draft"})
	if tr.ResolutionPath != "prepare_filing" || tr.DraftResponse != "" || noticeStatus(tr) != "open" {
		t.Fatalf("%+v", tr)
	}
	if tr.Title == "" || tr.Summary == "" || !strings.Contains(tr.Recommendation, "prepared but not filed") {
		t.Errorf("%+v", tr)
	}
	fresh := RuleTriage(sssNotice, received)
	Resolve(&fresh, store.Company{Name: "Acme"}, nil)
	if fresh.ResolutionPath != "prepare_filing" || !strings.Contains(fresh.Recommendation, "no submitted") {
		t.Errorf("%+v", fresh)
	}
}

func TestAITriage_MergesWithRules(t *testing.T) {
	base := RuleTriage("Pag-IBIG Fund letter about your remittance", received)
	p := &fakeAI{reply: "Here you go:\n" + `{"agency":"PAG-IBIG","notice_type":"late_payment","form":"MCRF","period":"2026-07","reference_no":"HDMF-77","amount":2500,"respond_by":"2026-11-30","title":"Late Pag-IBIG remittance","summary":"Pag-IBIG says July was paid late."}`}
	tr, err := AITriage(context.Background(), p, "x", base)
	if err != nil {
		t.Fatal(err)
	}
	if tr.TriagedBy != "ai" || tr.Agency != "PAGIBIG" || tr.FormCode != "pagibig_mcrf" || tr.Period != "2026-07" ||
		tr.NoticeType != "late_payment" || tr.RespondBy != "2026-11-30" || *tr.Amount != 2500 || tr.ReferenceNo != "HDMF-77" {
		t.Fatalf("%+v", tr)
	}
}
