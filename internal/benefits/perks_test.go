package benefits

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
)

func TestPerksFlow(t *testing.T) {
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
	h := NewHandler(store.New(pool), pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var cid, other int64
	sfx := fmt.Sprint(time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Perk Co "+sfx).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Other Co "+sfx).Scan(&other); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id IN ($1, $2)`, cid, other) //nolint:errcheck

	gin.SetMode(gin.TestMode)
	router := func(company int64, role auth.Role) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(auth.ContextKeyCompanyID, company); c.Set(auth.ContextKeyRole, role) })
		g := r.Group("")
		h.RegisterRoutes(g)
		return r
	}
	admin, emp, stranger := router(cid, auth.RoleAdmin), router(cid, auth.RoleEmployee), router(other, auth.RoleEmployee)
	call := func(r *gin.Engine, method, path string, body any) (int, map[string]any) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, rd))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	if code, _ := call(admin, "POST", "/benefits/perks", map[string]any{"title": "Gym", "url": "javascript:alert(1)"}); code != 400 {
		t.Fatalf("non-http link accepted: %d", code)
	}
	if code, _ := call(emp, "POST", "/benefits/perks", map[string]any{"title": "Gym"}); code != 403 {
		t.Fatalf("employee created a perk: %d", code)
	}
	code, out := call(admin, "POST", "/benefits/perks", map[string]any{"title": "Gym 20% off", "partner": "FitHub", "category": "wellness", "offer": "20% off", "code": "HALA20", "url": "https://fithub.example/join"})
	if code != 201 {
		t.Fatalf("create: %d %v", code, out)
	}
	id := int64(out["data"].(map[string]any)["id"].(float64))
	_, _ = call(admin, "POST", "/benefits/perks", map[string]any{"title": "Old deal", "expires_on": "2020-01-01"})

	_, out = call(emp, "GET", "/benefits/perks", nil)
	list := out["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("employee must see only the live perk: %v", list)
	}
	p := list[0].(map[string]any)
	if _, leaked := p["code"]; leaked || p["has_code"] != true {
		t.Fatalf("code must be hidden until claimed: %v", p)
	}
	_, out = call(admin, "GET", "/benefits/perks", nil)
	if len(out["data"].([]any)) != 2 {
		t.Fatalf("admin sees all perks: %v", out["data"])
	}
	if code, _ := call(stranger, "POST", fmt.Sprintf("/benefits/perks/%d/claim", id), nil); code != 404 {
		t.Fatalf("other company claimed: %d", code)
	}
	code, out = call(emp, "POST", fmt.Sprintf("/benefits/perks/%d/claim", id), nil)
	if code != 200 || out["data"].(map[string]any)["code"] != "HALA20" {
		t.Fatalf("claim: %d %v", code, out)
	}
	_, out = call(admin, "GET", "/benefits/perks", nil)
	for _, x := range out["data"].([]any) {
		if m := x.(map[string]any); m["title"] == "Gym 20% off" && m["claims"] != 1.0 {
			t.Fatalf("claims not counted: %v", m)
		}
	}
	if code, _ := call(admin, "PUT", fmt.Sprintf("/benefits/perks/%d", id), map[string]any{"title": "Gym", "active": false}); code != 200 {
		t.Fatal("update failed")
	}
	if code, _ := call(emp, "POST", fmt.Sprintf("/benefits/perks/%d/claim", id), nil); code != 404 {
		t.Fatalf("inactive perk claimed: %d", code)
	}
}
