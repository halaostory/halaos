package payouts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

func TestBankCode(t *testing.T) {
	for in, want := range map[string]string{"BDO": "PH_BDO", "Banco de Oro Unibank": "PH_BDO", "bpi family": "PH_BPI", "GCash": "PH_GCASH", "ph_rcbc": "PH_RCBC", "Bank of Nowhere": "", "": ""} {
		if got := BankCode(in); got != want {
			t.Errorf("%q → %q want %q", in, got, want)
		}
	}
}

func TestPayoutFlow(t *testing.T) {
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
	var got []map[string]any
	var idem string
	xendit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, _, _ := r.BasicAuth(); u != "sk_test" || r.URL.Path != "/batch_disbursements" {
			w.WriteHeader(401)
			return
		}
		idem = r.Header.Get("X-IDEMPOTENCY-KEY")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		_, _ = io.WriteString(w, `{"id":"batch_1","status":"NEEDS_APPROVAL"}`)
	}))
	defer xendit.Close()
	s := &Service{q: store.New(pool), pool: pool, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), apiBase: xendit.URL,
		secret: "sk_test", callback: "cb-token", http: xendit.Client()}

	sfx := fmt.Sprint(time.Now().UnixNano())
	var companyID, empID, cycleID, runID int64
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Pay Co "+sfx).Scan(&companyID))
	defer func() {
		for _, q := range []string{
			`DELETE FROM payout_batches WHERE company_id = $1`, `DELETE FROM payroll_items WHERE run_id IN (SELECT id FROM payroll_runs WHERE company_id = $1)`,
			`DELETE FROM payroll_runs WHERE company_id = $1`, `DELETE FROM payroll_cycles WHERE company_id = $1`,
			`DELETE FROM employee_profiles WHERE employee_id IN (SELECT id FROM employees WHERE company_id = $1)`,
			`DELETE FROM employees WHERE company_id = $1`, `DELETE FROM companies WHERE id = $1`,
		} {
			_, _ = pool.Exec(ctx, q, companyID)
		}
	}()
	must(pool.QueryRow(ctx, `INSERT INTO employees (company_id, employee_no, first_name, last_name, hire_date) VALUES ($1, 'E1', 'Ana', 'Cruz', '2026-01-01') RETURNING id`, companyID).Scan(&empID))
	_, err = pool.Exec(ctx, `INSERT INTO employee_profiles (employee_id, bank_name, bank_account_no, bank_account_name) VALUES ($1, 'BDO', '0012 3456 7890', 'ANA CRUZ')`, empID)
	must(err)
	must(pool.QueryRow(ctx, `INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date, status) VALUES ($1, 'Nov A', '2026-11-01', '2026-11-15', '2026-11-15', 'draft') RETURNING id`, companyID).Scan(&cycleID))
	must(pool.QueryRow(ctx, `INSERT INTO payroll_runs (company_id, cycle_id, status, total_employees) VALUES ($1, $2, 'completed', 1) RETURNING id`, companyID, cycleID).Scan(&runID))
	_, err = pool.Exec(ctx, `INSERT INTO payroll_items (run_id, employee_id, gross_pay, net_pay) VALUES ($1, $2, 30000, 25432.10)`, runID, empID)
	must(err)

	p, err := s.Preview(ctx, companyID, cycleID)
	must(err)
	if p.Blocker != "approve the pay run first" || p.Total != 25432.10 || p.Lines[0].BankCode != "PH_BDO" || p.Lines[0].AccountNo != "001234567890" {
		t.Fatalf("draft preview %+v", p)
	}
	if _, err := s.Send(ctx, companyID, 0, cycleID); err == nil {
		t.Fatal("sent an unapproved run")
	}
	_, _ = pool.Exec(ctx, `UPDATE payroll_cycles SET status = 'approved' WHERE id = $1`, cycleID)
	b, err := s.Send(ctx, companyID, 0, cycleID)
	must(err)
	if b.ExternalID != "batch_1" || b.Status != "needs_approval" || len(got) != 1 || idem != fmt.Sprintf("hp-run-%d", runID) {
		t.Fatalf("send %+v %d %s", b, len(got), idem)
	}
	d := got[0]["disbursements"].([]any)[0].(map[string]any)
	if d["amount"].(float64) != 25432.10 || d["bank_code"] != "PH_BDO" || d["bank_account_number"] != "001234567890" {
		t.Errorf("disbursement %+v", d)
	}
	if _, err := s.Send(ctx, companyID, 0, cycleID); err == nil || len(got) != 1 {
		t.Fatal("second send must be refused")
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/cb", s.callbackHandler)
	post := func(token, body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/cb", strings.NewReader(body))
		req.Header.Set("x-callback-token", token)
		r.ServeHTTP(w, req)
		return w.Code
	}
	ext := fmt.Sprintf("hp-%d-%d", runID, empID)
	if post("forged", `{"external_id":"`+ext+`","status":"COMPLETED"}`) != 401 {
		t.Error("forged callback accepted")
	}
	if post("cb-token", `{"id":"batch_1","disbursements":[{"external_id":"`+ext+`","status":"COMPLETED"}]}`) != 200 {
		t.Fatal("callback")
	}
	var bs, is string
	_ = pool.QueryRow(ctx, `SELECT b.status, i.status FROM payout_batches b JOIN payout_items i ON i.batch_id = b.id WHERE b.id = $1`, b.ID).Scan(&bs, &is)
	if bs != "completed" || is != "completed" {
		t.Errorf("after callback: batch %s item %s", bs, is)
	}
}
