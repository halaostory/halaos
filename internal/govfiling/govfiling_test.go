package govfiling

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	ok := []struct {
		f, in string
		want  Period
	}{
		{"monthly", "2026-10", Period{Year: 2026, Month: 10}},
		{"quarterly", "2026-q3", Period{Year: 2026, Quarter: 3}},
		{"annual", "2025", Period{Year: 2025}},
		{"once", "2026", Period{Year: 2026}},
	}
	for _, c := range ok {
		got, err := ParsePeriod(c.f, c.in)
		if err != nil || got != c.want {
			t.Errorf("%s %s: %+v %v", c.f, c.in, got, err)
		}
	}
	for _, c := range [][2]string{{"monthly", "2026-13"}, {"monthly", "2026"}, {"quarterly", "2026-Q5"}, {"annual", "26"}} {
		if _, err := ParsePeriod(c[0], c[1]); err == nil {
			t.Errorf("%v should fail", c)
		}
	}
	if e := (Period{Year: 2026, Month: 2}).End(); e.Day() != 28 {
		t.Errorf("Feb end %v", e)
	}
	if e := (Period{Year: 2026, Quarter: 3}).End(); e.Month() != 9 || e.Day() != 30 {
		t.Errorf("Q3 end %v", e)
	}
}

func sssDataset() *Dataset {
	return &Dataset{
		FormCode: "sss_r3", Period: "2026-09", CompanyName: "Acme",
		EmployerIDs:   map[string]string{"sss_er_no": "03-1234567-8"},
		MemberIDLabel: "SSS number", Columns: []string{"ee", "er", "ec"},
		Lines: []Line{
			{Name: "Cruz, Ana", MemberID: "34-1234567-8", Amounts: map[string]float64{"ee": 900, "er": 1900, "ec": 30}},
			{Name: "Lim, Ben", MemberID: "34-7654321-0", Amounts: map[string]float64{"ee": 600, "er": 1250, "ec": 10}},
		},
		Totals: map[string]float64{"ee": 1500, "er": 3150, "ec": 40},
	}
}

func hasCode(p *Preflight, code string) bool {
	for _, i := range p.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestRunRules_CleanReturnIsApprovable(t *testing.T) {
	spec, _ := Lookup("sss_r3")
	today := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	p := RunRules(RuleInput{Spec: spec, Period: Period{Year: 2026, Month: 9}, Dataset: sssDataset(), Channel: ChannelManual, Today: today})
	if p.Blocking {
		t.Fatalf("clean return should not block: %+v", p.Issues)
	}
}

func TestRunRules_BlockingProblems(t *testing.T) {
	spec, _ := Lookup("sss_r3")
	today := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	ds := sssDataset()
	ds.EmployerIDs["sss_er_no"] = ""
	ds.Lines[1].MemberID = ""
	ds.Lines = append(ds.Lines, Line{Name: "Dup, Cy", MemberID: "3412345678", Amounts: map[string]float64{"ee": -1}})
	due := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	p := RunRules(RuleInput{Spec: spec, Period: Period{Year: 2026, Month: 10}, Dataset: ds, Channel: ChannelBrowser,
		PrevTotals: map[string]float64{"ee": 500}, DueDate: &due, Today: today})
	for _, c := range []string{"period_open", "missing_employer_id", "missing_member_id", "duplicate_member_id", "negative_amount", "no_playbook", "no_portal_account", "past_due", "large_change"} {
		if !hasCode(p, c) {
			t.Errorf("expected issue %s; got %+v", c, p.Issues)
		}
	}
	if !p.Blocking {
		t.Error("should block")
	}
}

func TestRunRules_UnverifiedPlaybookWarnsOnly(t *testing.T) {
	spec, _ := Lookup("sss_r3")
	p := RunRules(RuleInput{Spec: spec, Period: Period{Year: 2026, Month: 9}, Dataset: sssDataset(), Channel: ChannelBrowser,
		HasPlaybook: true, HasAccount: true, Today: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)})
	if p.Blocking || !hasCode(p, "playbook_unverified") {
		t.Errorf("%+v", p)
	}
}

func TestRenderFile_SSS(t *testing.T) {
	spec, _ := Lookup("sss_r3")
	name, _, data, err := RenderFile(spec, sssDataset())
	if err != nil {
		t.Fatal(err)
	}
	if name != "R3_0312345678_202609.txt" {
		t.Errorf("name %s", name)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[1], "D|341234567") || lines[3] != "T|2|1500.00|3150.00|40.00" {
		t.Errorf("unexpected file:\n%s", data)
	}
}

func TestRenderFile_CSVTotals(t *testing.T) {
	spec, _ := Lookup("philhealth_rf1")
	ds := &Dataset{Period: "2026-09", MemberIDLabel: "PhilHealth number", Columns: []string{"ee", "er"},
		Lines:  []Line{{Name: "A", MemberID: "1", Amounts: map[string]float64{"ee": 1, "er": 2}}},
		Totals: map[string]float64{"ee": 1, "er": 2}}
	name, ct, data, err := RenderFile(spec, ds)
	if err != nil || name != "RF1_202609.csv" || ct != "text/csv" {
		t.Fatalf("%s %s %v", name, ct, err)
	}
	if !strings.Contains(string(data), "philhealth_number") || !strings.Contains(string(data), ",TOTAL,,1.00,2.00") {
		t.Errorf("csv:\n%s", data)
	}
}

func TestLookupAliasesAndChannels(t *testing.T) {
	f, ok := Lookup("pagibig_ml1")
	if !ok || f.Code != "pagibig_mcrf" {
		t.Fatal("alias")
	}
	if f.DefaultChannel() != ChannelBrowser || !f.Supports(ChannelManual) || f.Supports(ChannelEmail) {
		t.Error("channels")
	}
	if a, _ := Lookup("bir_1604c"); a.DefaultChannel() != ChannelEmail || a.EmailTo == "" {
		t.Error("1604c should default to BIR eSubmission e-mail")
	}
	for _, f := range Forms("") {
		if f.Agency == "MOCK" {
			t.Error("mock form must be hidden unless GOV_MOCK_PORTAL_URL is set")
		}
	}
}

func TestPartnerClient(t *testing.T) {
	var gotSig string
	var got PartnerRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-HalaOS-Signature")
		if gotSig != Sign("s3", body) {
			http.Error(w, "bad signature", 401)
			return
		}
		_ = json.Unmarshal(body, &got)
		_, _ = w.Write([]byte(`{"status":"accepted","reference_no":"EFILE-42"}`))
	}))
	defer srv.Close()
	c := NewPartnerClient()
	res, err := c.Submit(context.Background(), srv.URL, "s3", PartnerRequest{SubmissionID: 7, FormCode: "irs_941", FileBase64: "e30="})
	if err != nil || res.ReferenceNo != "EFILE-42" || got.SubmissionID != 7 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := c.Submit(context.Background(), srv.URL, "wrong", PartnerRequest{}); err == nil {
		t.Error("bad signature must fail")
	}
	noRef := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"status":"accepted"}`)) }))
	defer noRef.Close()
	if _, err := c.Submit(context.Background(), noRef.URL, "", PartnerRequest{}); err == nil {
		t.Error("missing reference must fail — never mark filed without proof")
	}
	if _, err := c.Submit(context.Background(), "http://example.com/x", "", PartnerRequest{}); err == nil {
		t.Error("plain http to remote host must be refused")
	}
}

func TestAIReviewer_OnlyAggregatesAndNeverBlocks(t *testing.T) {
	fp := &fakeAI{reply: "```json\n{\"summary\":\"Looks consistent.\",\"risk\":\"medium\",\"issues\":[{\"severity\":\"error\",\"message\":\"ER share ratio unusual\"}]}\n```"}
	spec, _ := Lookup("sss_r3")
	in := RuleInput{Spec: spec, Period: Period{Year: 2026, Month: 9}, Dataset: sssDataset(), Channel: ChannelManual, Today: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}
	p := RunRules(in)
	if err := (&AIReviewer{Provider: fp}).Review(context.Background(), in, p); err != nil {
		t.Fatal(err)
	}
	if !hasCode(p, "ai_review") || p.Issues[len(p.Issues)-1].Severity != "warning" {
		t.Errorf("AI finding should surface as a warning: %+v", p.Issues)
	}
	if p.AIRisk != "medium" || p.AISummary == "" || p.Blocking {
		t.Errorf("%+v", p)
	}
	for _, leak := range []string{"Cruz", "34-1234567-8", "Ana"} {
		if strings.Contains(fp.prompt, leak) {
			t.Errorf("personal data %q sent to AI", leak)
		}
	}
}
