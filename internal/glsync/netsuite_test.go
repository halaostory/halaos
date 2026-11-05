package glsync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/store"
)

// Reference value computed independently (Python hmac + urllib.quote).
func TestNetSuiteSignatureVector(t *testing.T) {
	n := &NetSuite{
		Creds: NetSuiteCreds{AccountID: "1234567_SB1", ConsumerKey: "ck", ConsumerSecret: "cs secret", TokenID: "tid", TokenSecret: "ts/secret"},
		now:   func() time.Time { return time.Unix(1700000000, 0) },
		nonce: func() string { return "abc123" },
	}
	if got := n.base(); got != "https://1234567-sb1.suitetalk.api.netsuite.com" {
		t.Fatalf("base %s", got)
	}
	h := n.authHeader("POST", n.base()+"/services/rest/query/v1/suiteql?limit=1000")
	want := `oauth_signature="` + url.QueryEscape("XyY9Ic5qkgaNkah+KmUmlrFO3Lqg6EVhMxYB84uayTg=") + `"`
	if !strings.HasPrefix(h, `OAuth realm="1234567_SB1",`) || !strings.Contains(h, want) {
		t.Fatalf("header %s", h)
	}
}

// fakeNetSuite verifies every request's OAuth signature.
type fakeNetSuite struct {
	srv      *httptest.Server
	ck, cs   string
	tid, tsc string
	entries  []map[string]any
	badSig   int
}

func oauthParams(h string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(h, "OAuth "), ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		v, _ = url.QueryUnescape(strings.Trim(v, `"`))
		out[k] = v
	}
	return out
}

func (f *fakeNetSuite) verify(r *http.Request) bool {
	p := oauthParams(r.Header.Get("Authorization"))
	if p["oauth_consumer_key"] != f.ck || p["oauth_token"] != f.tid || p["oauth_signature_method"] != "HMAC-SHA256" {
		return false
	}
	enc := func(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
	var pairs []string
	for k, v := range p {
		if k != "oauth_signature" && k != "realm" {
			pairs = append(pairs, enc(k)+"="+enc(v))
		}
	}
	for k, vs := range r.URL.Query() {
		for _, v := range vs {
			pairs = append(pairs, enc(k)+"="+enc(v))
		}
	}
	sort.Strings(pairs)
	base := r.Method + "&" + enc("http://"+r.Host+r.URL.Path) + "&" + enc(strings.Join(pairs, "&"))
	mac := hmac.New(sha256.New, []byte(enc(f.cs)+"&"+enc(f.tsc)))
	mac.Write([]byte(base))
	return hmac.Equal([]byte(base64.StdEncoding.EncodeToString(mac.Sum(nil))), []byte(p["oauth_signature"]))
}

func newFakeNetSuite(t *testing.T) *fakeNetSuite {
	f := &fakeNetSuite{ck: "ck", cs: "cs", tid: "tid", tsc: "tsec"}
	mux := http.NewServeMux()
	mux.HandleFunc("/services/rest/query/v1/suiteql", func(w http.ResponseWriter, r *http.Request) {
		if !f.verify(r) {
			f.badSig++
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"title":"Invalid login attempt."}`))
			return
		}
		if r.Header.Get("Prefer") != "transient" {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"101","acctnumber":"6000","name":"Salaries Expense"},{"id":"102","acctnumber":"","name":"Net Pay Payable"}]}`))
	})
	mux.HandleFunc("/services/rest/record/v1/journalEntry", func(w http.ResponseWriter, r *http.Request) {
		if !f.verify(r) {
			f.badSig++
			w.WriteHeader(401)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.entries = append(f.entries, body)
		w.Header().Set("Location", fmt.Sprintf("http://%s/services/rest/record/v1/journalEntry/%d", r.Host, 500+len(f.entries)))
		w.WriteHeader(204)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func TestNetSuitePush(t *testing.T) {
	f := newFakeNetSuite(t)
	n := &NetSuite{Creds: NetSuiteCreds{AccountID: "TSTDRV1", ConsumerKey: "ck", ConsumerSecret: "cs", TokenID: "tid", TokenSecret: "tsec", SubsidiaryID: "3"},
		Base: f.srv.URL, HTTP: f.srv.Client()}
	j := payroll.Journal{Number: "PR-7", Date: "2026-11-15", Memo: "Payroll Nov", Lines: []payroll.JournalLine{
		{AccountCode: "6000", AccountName: "Salaries Expense", Debit: 1000.004, Description: "Gross pay"},
		{AccountCode: "2100", AccountName: "Net Pay Payable", Credit: 1000, Description: "Net pay"},
	}}
	id, err := n.Push(context.Background(), j)
	if err != nil || id != "501" || f.badSig != 0 {
		t.Fatalf("push: %q %v bad=%d", id, err, f.badSig)
	}
	e := f.entries[0]
	items := e["line"].(map[string]any)["items"].([]any)
	l0, l1 := items[0].(map[string]any), items[1].(map[string]any)
	if e["externalId"] != "halapeople-PR-7" || e["tranDate"] != "2026-11-15" || e["subsidiary"].(map[string]any)["id"] != "3" ||
		l0["account"].(map[string]any)["id"] != "101" || l0["debit"] != 1000.0 || l1["account"].(map[string]any)["id"] != "102" || l1["credit"] != 1000.0 {
		t.Fatalf("entry %+v", e)
	}
	// unmapped account → no entry sent
	j.Lines = append(j.Lines, payroll.JournalLine{AccountCode: "9999", AccountName: "Nope", Debit: 1})
	if _, err := n.Push(context.Background(), j); err == nil || !strings.Contains(err.Error(), "9999 Nope") || len(f.entries) != 1 {
		t.Fatalf("missing account: %v entries=%d", err, len(f.entries))
	}
	// wrong secret is rejected by the server
	n.Creds.TokenSecret = "wrong"
	if err := n.Check(context.Background()); err == nil || f.badSig != 1 {
		t.Fatalf("bad secret accepted: %v", err)
	}
}

// TestNetSuiteFlow: save credentials via the API → push a cycle → no double push.
func TestNetSuiteFlow(t *testing.T) {
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
	f := newFakeNetSuite(t)
	s := New(q, enc, "https://people.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)), map[string]*Provider{})
	s.NetSuiteBase, s.NetSuiteHTTP = f.srv.URL, f.srv.Client()

	sfx := fmt.Sprint(time.Now().UnixNano())
	var companyID, cycleID int64
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "NS Co "+sfx).Scan(&companyID))
	defer pool.Exec(ctx, `DELETE FROM payroll_runs WHERE company_id = $1`, companyID)   //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM payroll_cycles WHERE company_id = $1`, companyID) //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, companyID)              //nolint:errcheck
	must(pool.QueryRow(ctx, `INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date) VALUES ($1, 'Nov', '2026-11-01', '2026-11-15', '2026-11-15') RETURNING id`, companyID).Scan(&cycleID))
	_, err = pool.Exec(ctx, `INSERT INTO payroll_runs (company_id, cycle_id, status, total_employees) VALUES ($1, $2, 'completed', 0)`, companyID, cycleID)
	must(err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(auth.ContextKeyCompanyID, companyID) })
	r.PUT("/gl/netsuite/credentials", s.netsuiteConnect)
	r.GET("/gl/connections", s.list)
	put := func(body map[string]string) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/gl/netsuite/credentials", bytes.NewReader(b)))
		return w
	}
	creds := map[string]string{"account_id": "tstdrv1", "consumer_key": "ck", "consumer_secret": "cs", "token_id": "tid", "token_secret": "nope"}
	if w := put(creds); w.Code != 400 || !strings.Contains(w.Body.String(), "rejected") {
		t.Fatalf("bad creds saved: %d %s", w.Code, w.Body.String())
	}
	creds["token_secret"] = "tsec"
	if w := put(creds); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/gl/connections", nil))
	if !strings.Contains(w.Body.String(), `"provider":"netsuite"`) || !strings.Contains(w.Body.String(), "NetSuite account TSTDRV1") {
		t.Fatalf("list: %s", w.Body.String())
	}
	var stored []byte
	must(pool.QueryRow(ctx, `SELECT access_token_enc FROM gl_connections WHERE company_id = $1 AND provider = 'netsuite'`, companyID).Scan(&stored))
	if bytes.Contains(stored, []byte("tsec")) {
		t.Fatal("token secret stored in clear text")
	}
	rec, already, err := s.Push(ctx, companyID, 0, cycleID, "netsuite")
	if err != nil || already || rec.ExternalID != "501" {
		t.Fatalf("push: %v %v %+v", err, already, rec)
	}
	_, already, err = s.Push(ctx, companyID, 0, cycleID, "netsuite")
	if err != nil || !already || len(f.entries) != 1 {
		t.Fatalf("second push must not send again: %v %v %d", err, already, len(f.entries))
	}
}
