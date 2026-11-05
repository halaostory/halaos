package glsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/store"
)

type fake struct {
	srv      *httptest.Server
	journals []map[string]any
	refresh  int
}

func newFake(t *testing.T) *fake {
	f := &fake{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if u, p, _ := r.BasicAuth(); u != "cid" || p != "csecret" {
			w.WriteHeader(401)
			return
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "good" {
				w.WriteHeader(400)
				_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"acc1","refresh_token":"ref1","expires_in":1800}`)
		case "refresh_token":
			f.refresh++
			_, _ = io.WriteString(w, `{"access_token":"acc2","refresh_token":"ref2","expires_in":1800}`)
		}
	})
	mux.HandleFunc("/connections", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"tenantId":"t-1","tenantName":"Acme PH","tenantType":"ORGANISATION"}]`)
	})
	mux.HandleFunc("/xero/ManualJournals", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xero-tenant-id") != "t-1" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer acc") {
			w.WriteHeader(401)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.journals = append(f.journals, body)
		_, _ = io.WriteString(w, `{"ManualJournals":[{"ManualJournalID":"mj-1"}]}`)
	})
	mux.HandleFunc("/qbo/v3/company/r-9/query", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"QueryResponse":{"Account":[{"Id":"1","Name":"Salaries and Wages","AcctNum":"6100"},{"Id":"2","Name":"Salaries Payable","AcctNum":"2100"}]}}`)
	})
	mux.HandleFunc("/qbo/v3/company/r-9/journalentry", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.journals = append(f.journals, body)
		_, _ = io.WriteString(w, `{"JournalEntry":{"Id":"je-77"}}`)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) providers() map[string]*Provider {
	return map[string]*Provider{
		"xero": {Name: "xero", ClientID: "cid", ClientSecret: "csecret", Scope: "accounting.transactions",
			EP: Endpoints{Authorize: f.srv.URL + "/authorize", Token: f.srv.URL + "/token", API: f.srv.URL + "/xero", Connections: f.srv.URL + "/connections"}, HTTP: f.srv.Client()},
		"quickbooks": {Name: "quickbooks", ClientID: "cid", ClientSecret: "csecret", Scope: "com.intuit.quickbooks.accounting",
			EP: Endpoints{Authorize: f.srv.URL + "/authorize", Token: f.srv.URL + "/token", API: f.srv.URL + "/qbo"}, HTTP: f.srv.Client()},
	}
}

func sampleJournal() payroll.Journal {
	return payroll.Journal{RunID: 1, Number: "PR-1", Date: "2026-11-15", Memo: "Payroll Nov", Currency: "PHP", Lines: []payroll.JournalLine{
		{AccountCode: "6100", AccountName: "Salaries and Wages", Description: "Gross pay", Debit: 1000},
		{AccountCode: "2100", AccountName: "Salaries Payable", Description: "Net pay", Credit: 1000},
	}}
}

func TestProviders(t *testing.T) {
	f := newFake(t)
	ps := f.providers()
	ctx := context.Background()
	x := ps["xero"]
	if !strings.Contains(x.AuthURL("st"), "state=st") {
		t.Error("auth url")
	}
	if _, err := x.Exchange(ctx, "bad"); err == nil {
		t.Error("bad code accepted")
	}
	tok, err := x.Exchange(ctx, "good")
	if err != nil || tok.Access != "acc1" {
		t.Fatal(err)
	}
	id, name, err := x.XeroTenant(ctx, tok.Access)
	if err != nil || id != "t-1" || name != "Acme PH" {
		t.Fatalf("%v %s %s", err, id, name)
	}
	mj, err := x.PushXero(ctx, tok.Access, id, sampleJournal())
	if err != nil || mj != "mj-1" {
		t.Fatal(err)
	}
	body := f.journals[0]["ManualJournals"].([]any)[0].(map[string]any)
	lines := body["JournalLines"].([]any)
	if body["Status"] != "DRAFT" || lines[0].(map[string]any)["LineAmount"].(float64) != 1000 || lines[1].(map[string]any)["LineAmount"].(float64) != -1000 {
		t.Errorf("xero body %+v", body)
	}
	qb := ps["quickbooks"]
	je, err := qb.PushQuickBooks(ctx, "acc1", "r-9", sampleJournal())
	if err != nil || je != "je-77" {
		t.Fatal(err)
	}
	ql := f.journals[1]["Line"].([]any)
	d := ql[1].(map[string]any)["JournalEntryLineDetail"].(map[string]any)
	if d["PostingType"] != "Credit" || d["AccountRef"].(map[string]any)["value"] != "2" {
		t.Errorf("qbo line %+v", ql[1])
	}
	bad := sampleJournal()
	bad.Lines[0].AccountCode, bad.Lines[0].AccountName = "9999", "Unknown"
	if _, err := qb.PushQuickBooks(ctx, "acc1", "r-9", bad); err == nil || !strings.Contains(err.Error(), "9999") {
		t.Errorf("missing account: %v", err)
	}
}

// TestFlow: connect → callback → push → duplicate push, against Postgres.
func TestFlow(t *testing.T) {
	dsn := os.Getenv("HALAOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HALAOS_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := store.New(pool)
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	enc, _ := crypto.NewCredentialEncryptor(key)
	f := newFake(t)
	s := New(q, enc, "https://people.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)), f.providers())

	sfx := fmt.Sprint(time.Now().UnixNano())
	var companyID, cycleID, runID int64
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "GL Co "+sfx).Scan(&companyID))
	defer pool.Exec(ctx, `DELETE FROM payroll_runs WHERE company_id = $1`, companyID)   //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM payroll_cycles WHERE company_id = $1`, companyID) //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, companyID)              //nolint:errcheck
	must(pool.QueryRow(ctx, `INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date) VALUES ($1, 'Nov', '2026-11-01', '2026-11-15', '2026-11-15') RETURNING id`, companyID).Scan(&cycleID))
	must(pool.QueryRow(ctx, `INSERT INTO payroll_runs (company_id, cycle_id, status, total_employees) VALUES ($1, $2, 'completed', 0) RETURNING id`, companyID, cycleID).Scan(&runID))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/gl/:provider/callback", s.callback)
	must(q.CreateGLOAuthState(ctx, store.CreateGLOAuthStateParams{State: "st-" + sfx, CompanyID: companyID, Provider: "xero"}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/gl/xero/callback?code=good&state=st-"+sfx, nil))
	if loc := w.Header().Get("Location"); w.Code != 302 || !strings.HasSuffix(loc, "/payroll?gl=connected") {
		t.Fatalf("callback %d %s", w.Code, loc)
	}
	// state is single use
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/gl/xero/callback?code=good&state=st-"+sfx, nil))
	if loc, _ := url.QueryUnescape(w.Header().Get("Location")); !strings.Contains(loc, "gl_error") {
		t.Errorf("replay: %s", loc)
	}
	// force a refresh on push
	_, _ = pool.Exec(ctx, `UPDATE gl_connections SET expires_at = now() - interval '1 minute' WHERE company_id = $1`, companyID)
	rec, already, err := s.Push(ctx, companyID, 0, cycleID, "xero")
	if err != nil || already || rec.ExternalID != "mj-1" || f.refresh != 1 {
		t.Fatalf("push: %v %v %+v refresh=%d", err, already, rec, f.refresh)
	}
	_, already, err = s.Push(ctx, companyID, 0, cycleID, "xero")
	if err != nil || !already || len(f.journals) != 1 {
		t.Fatalf("second push must not send again: %v %v %d", err, already, len(f.journals))
	}
}
