package employee

import (
	"context"
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
	"github.com/halaostory/halaos/pkg/response"
)

func TestRecordAccess(t *testing.T) {
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
	h := &Handler{queries: store.New(pool), pool: pool, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	sfx := fmt.Sprint(time.Now().UnixNano())
	var cid, uSelf, eSelf, eOther int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Acc Co "+sfx).Scan(&cid))
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid)         //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM users WHERE company_id = $1`, cid)     //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM employees WHERE company_id = $1`, cid) //nolint:errcheck
	must(pool.QueryRow(ctx, `INSERT INTO users (company_id, email, password_hash, first_name, last_name, role) VALUES ($1, $2, 'x', 'A', 'B', 'employee') RETURNING id`, cid, "self"+sfx+"@t.test").Scan(&uSelf))
	must(pool.QueryRow(ctx, `INSERT INTO employees (company_id, user_id, employee_no, first_name, last_name, hire_date) VALUES ($1, $2, 'S', 'Self', 'One', '2026-01-01') RETURNING id`, cid, uSelf).Scan(&eSelf))
	must(pool.QueryRow(ctx, `INSERT INTO employees (company_id, employee_no, first_name, last_name, hire_date) VALUES ($1, 'O', 'Other', 'Two', '2026-01-01') RETURNING id`, cid).Scan(&eOther))

	gin.SetMode(gin.TestMode)
	status := func(role auth.Role, uid, emp int64, private bool) int {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(auth.ContextKeyCompanyID, cid)
			c.Set(auth.ContextKeyUserID, uid)
			c.Set(auth.ContextKeyRole, role)
		})
		r.GET("/employees/:id", h.recordAccess(private), func(c *gin.Context) { response.OK(c, "ok") })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/employees/%d", emp), nil))
		return w.Code
	}
	cases := []struct {
		name    string
		role    auth.Role
		uid     int64
		emp     int64
		private bool
		want    int
	}{
		{"admin private", auth.RoleAdmin, 1, eOther, accessPrivate, 200},
		{"manager basic", auth.RoleManager, 1, eOther, accessBasic, 200},
		{"manager private", auth.RoleManager, 1, eOther, accessPrivate, 403},
		{"self private", auth.RoleEmployee, uSelf, eSelf, accessPrivate, 200},
		{"other basic", auth.RoleEmployee, uSelf, eOther, accessBasic, 403},
		{"other private", auth.RoleEmployee, uSelf, eOther, accessPrivate, 403},
		{"no employee record", auth.RoleEmployee, 999999999, eOther, accessBasic, 403},
	}
	for _, tc := range cases {
		if got := status(tc.role, tc.uid, tc.emp, tc.private); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
