package contractors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
)

type fakeWise struct {
	srv       *httptest.Server
	transfers []map[string]any
	funded    int
}

func newFakeWise(t *testing.T) *fakeWise {
	f := &fakeWise{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer wtok" {
			w.WriteHeader(401)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case r.URL.Path == "/v3/profiles/77/quotes":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "q-1", "sourceCurrency": body["sourceCurrency"], "targetCurrency": body["targetCurrency"],
				"sourceAmount": body["sourceAmount"], "targetAmount": 170.12, "rate": 0.01793,
				"paymentOptions": []map[string]any{{"payIn": "BALANCE", "targetAmount": 169.9, "fee": map[string]any{"total": 12.5}}}})
		case r.URL.Path == "/v1/transfers" && r.Method == "POST":
			f.transfers = append(f.transfers, body)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9001, "status": "incoming_payment_waiting"})
		case r.URL.Path == "/v3/profiles/77/transfers/9001/payments":
			f.funded++
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "COMPLETED"})
		case r.URL.Path == "/v1/transfers/9001":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "outgoing_payment_sent"})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func TestWiseClient(t *testing.T) {
	f := newFakeWise(t)
	w := &Wise{Base: f.srv.URL, Token: "wtok", ProfileID: 77, HTTP: f.srv.Client()}
	ctx := context.Background()
	q, err := w.Quote(ctx, "PHP", "USD", 9500, 555)
	if err != nil || q.ID != "q-1" || q.Fee != 12.5 || q.TargetAmount != 169.9 {
		t.Fatalf("%v %+v", err, q)
	}
	id, st, err := w.TransferAndFund(ctx, q.ID, 555, 42, "Invoice INV-123456789012345678901234567890")
	if err != nil || id != 9001 || st != "funded" || f.funded != 1 {
		t.Fatalf("%v %d %s", err, id, st)
	}
	ref := f.transfers[0]["details"].(map[string]any)["reference"].(string)
	if len(ref) > 35 {
		t.Errorf("reference too long: %q", ref)
	}
	if u := f.transfers[0]["customerTransactionId"].(string); !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(u) || u != idempotencyUUID(42) || u == idempotencyUUID(43) {
		t.Errorf("idempotency uuid %q", u)
	}
}

func TestWisePayFlow(t *testing.T) {
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
	f := newFakeWise(t)
	h := NewHandler(store.New(pool), slog.New(slog.NewTextHandler(io.Discard, nil)), "HalaPeople")
	h.SetWise(&Wise{Base: f.srv.URL, Token: "wtok", ProfileID: 77, HTTP: f.srv.Client()})

	sfx := fmt.Sprint(time.Now().UnixNano())
	var companyID, contractorID, paymentID int64
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Wise Co "+sfx).Scan(&companyID))
	defer func() {
		for _, q := range []string{`DELETE FROM contractor_payments WHERE company_id = $1`, `DELETE FROM contractors WHERE company_id = $1`, `DELETE FROM companies WHERE id = $1`} {
			_, _ = pool.Exec(ctx, q, companyID)
		}
	}()
	must(pool.QueryRow(ctx, `INSERT INTO contractors (company_id, name) VALUES ($1, 'Maria Dev (Lisbon)') RETURNING id`, companyID).Scan(&contractorID))
	must(pool.QueryRow(ctx, `INSERT INTO contractor_payments (company_id, contractor_id, invoice_no, service_date, gross_amount, ewt_base, ewt_rate, ewt_amount, net_amount, atc_code, status)
		VALUES ($1, $2, 'INV-7', '2026-10-31', 10000, 10000, 0.05, 500, 9500, 'WI010', 'approved') RETURNING id`, companyID, contractorID).Scan(&paymentID))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(auth.ContextKeyCompanyID, companyID)
		c.Set(auth.ContextKeyUserID, int64(0))
		c.Next()
	})
	g := r.Group("/contractors")
	h.registerWise(g)
	do := func(method, path, body string) map[string]any {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		out["_code"] = float64(w.Code)
		return out
	}
	q := do("GET", fmt.Sprintf("/contractors/payments/%d/wise", paymentID), "")
	if d := q["data"].(map[string]any); d["blocker"] != "link this contractor to a Wise recipient first" {
		t.Fatalf("no account: %+v", d)
	}
	do("PUT", fmt.Sprintf("/contractors/%d/payout-account", contractorID), `{"recipient_id":555,"currency":"eur"}`)
	q = do("GET", fmt.Sprintf("/contractors/payments/%d/wise", paymentID), "")
	quote := q["data"].(map[string]any)["quote"].(map[string]any)
	if quote["id"] != "q-1" || quote["targetCurrency"] != "EUR" || quote["sourceAmount"].(float64) != 9500 {
		t.Fatalf("quote %+v", quote)
	}
	if out := do("POST", fmt.Sprintf("/contractors/payments/%d/wise", paymentID), `{"quote_id":"q-1"}`); out["_code"].(float64) != 400 {
		t.Error("pay without confirm accepted")
	}
	out := do("POST", fmt.Sprintf("/contractors/payments/%d/wise", paymentID), `{"quote_id":"q-1","confirm":true,"target_amount":169.9,"rate":0.01793}`)
	if out["_code"].(float64) != 200 || f.funded != 1 {
		t.Fatalf("pay: %+v", out)
	}
	var status, ref string
	_ = pool.QueryRow(ctx, `SELECT status, paid_reference FROM contractor_payments WHERE id = $1`, paymentID).Scan(&status, &ref)
	if status != "paid" || ref != "wise:9001" {
		t.Errorf("payment %s %s", status, ref)
	}
	if out := do("POST", fmt.Sprintf("/contractors/payments/%d/wise", paymentID), `{"quote_id":"q-1","confirm":true}`); out["_code"].(float64) != 400 || len(f.transfers) != 1 {
		t.Error("second payment must be refused")
	}
	q = do("GET", fmt.Sprintf("/contractors/payments/%d/wise", paymentID), "")
	if tr := q["data"].(map[string]any)["transfer"].(map[string]any); tr["status"] != "outgoing_payment_sent" || !strings.Contains(fmt.Sprint(tr["transfer_id"]), "9001") {
		t.Errorf("transfer status %+v", tr)
	}
}
