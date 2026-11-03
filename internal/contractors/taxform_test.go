package contractors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
)

func TestContractorTaxForm(t *testing.T) {
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
	h := NewHandler(q, slog.New(slog.NewTextHandler(io.Discard, nil)), "HalaPeople")
	h.SetAppURL("https://people.example.com/")
	var cid, kid int64
	sfx := fmt.Sprint(time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name, country) VALUES ($1, 'PHL') RETURNING id`, "TaxForm Co "+sfx).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid) //nolint:errcheck
	if err := pool.QueryRow(ctx, `INSERT INTO contractors (company_id, kind, name, tin, address) VALUES ($1, 'individual', 'Lia Tan', '', '') RETURNING id`, cid).Scan(&kid); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	prot := r.Group("/api")
	prot.Use(func(c *gin.Context) { c.Set(auth.ContextKeyCompanyID, cid); c.Set(auth.ContextKeyRole, auth.RoleAdmin) })
	pass := func(c *gin.Context) { c.Next() }
	h.RegisterTaxFormRoutes(prot, r.Group("/pub"), pass, pass)
	call := func(method, path string, body any) (int, map[string]any) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, rd)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	code, out := call("POST", fmt.Sprintf("/api/contractors/%d/tax-form", kid), nil)
	if code != 200 {
		t.Fatalf("create link: %d %v", code, out)
	}
	url := out["data"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(url, "https://people.example.com/contractor-form/") {
		t.Fatalf("url %s", url)
	}
	token := url[strings.LastIndex(url, "/")+1:]
	if code, out := call("GET", "/pub/public/contractor-form/"+token, nil); code != 200 || out["data"].(map[string]any)["contractor"] != "Lia Tan" {
		t.Fatalf("open link: %d %v", code, out)
	}
	form := map[string]any{"tax_country": "PH", "entity_type": "individual", "address": "12 Mabini St, Makati", "signer_name": "Lia Tan", "agree": true,
		"bank_name": "BDO", "bank_account_no": "001234567890", "bank_account_name": "Lia Tan"}
	if code, _ := call("POST", "/pub/public/contractor-form/"+token, form); code != 400 {
		t.Errorf("PH resident without TIN accepted: %d", code)
	}
	form["tin"] = "123-456-789-000"
	form["agree"] = false
	if code, _ := call("POST", "/pub/public/contractor-form/"+token, form); code != 400 {
		t.Errorf("submitted without accepting the declaration: %d", code)
	}
	form["agree"] = true
	if code, out := call("POST", "/pub/public/contractor-form/"+token, form); code != 200 {
		t.Fatalf("submit: %d %v", code, out)
	}
	var tin, addr, bank string
	_ = pool.QueryRow(ctx, `SELECT tin, address, bank_account_no FROM contractors WHERE id = $1`, kid).Scan(&tin, &addr, &bank)
	if tin != "123-456-789-000" || addr != "12 Mabini St, Makati" || bank != "001234567890" {
		t.Errorf("contractor not updated: %q %q %q", tin, addr, bank)
	}
	if code, _ := call("POST", "/pub/public/contractor-form/"+token, form); code != 410 {
		t.Errorf("link reused: %d", code)
	}
	code, out = call("GET", fmt.Sprintf("/api/contractors/%d/tax-form", kid), nil)
	d := out["data"].(map[string]any)
	if code != 200 || d["status"] != "submitted" || d["signer_name"] != "Lia Tan" || !strings.Contains(d["declaration"].(string), "the Philippines") || d["token_hash"] != "" {
		t.Fatalf("status: %d %v", code, d)
	}
	// A new link revokes nothing already signed, and an unknown token is gone.
	if code, _ := call("GET", "/pub/public/contractor-form/nope", nil); code != 410 {
		t.Errorf("unknown token: %d", code)
	}
}
