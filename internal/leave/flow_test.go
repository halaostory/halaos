package leave

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
	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/store"
)

func TestLeaveRequestRules(t *testing.T) {
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(store.New(pool), pool, logger, email.NewSender(config.SMTPConfig{}, logger))
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	sfx := fmt.Sprint(time.Now().UnixNano())
	var cid, uMgr, uRep, uOther, eMgr, eRep, eOther, ltID int64
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Leave Co "+sfx).Scan(&cid))
	defer func() {
		for _, q := range []string{`DELETE FROM approval_workflows WHERE company_id = $1`, `DELETE FROM hr_events WHERE company_id = $1`, `DELETE FROM notifications WHERE company_id = $1`,
			`DELETE FROM leave_requests WHERE company_id = $1`, `DELETE FROM leave_balances WHERE company_id = $1`, `DELETE FROM holidays WHERE company_id = $1`,
			`DELETE FROM leave_types WHERE company_id = $1`, `DELETE FROM employees WHERE company_id = $1`, `DELETE FROM users WHERE company_id = $1`, `DELETE FROM companies WHERE id = $1`} {
			_, _ = pool.Exec(ctx, q, cid)
		}
	}()
	user := func(name string, role string) int64 {
		var id int64
		must(pool.QueryRow(ctx, `INSERT INTO users (company_id, email, password_hash, first_name, last_name, role) VALUES ($1, $2, 'x', $3, 'T', $4) RETURNING id`, cid, name+sfx+"@t.test", name, role).Scan(&id))
		return id
	}
	emp := func(uid int64, no string, mgr *int64) int64 {
		var id int64
		must(pool.QueryRow(ctx, `INSERT INTO employees (company_id, user_id, employee_no, first_name, last_name, hire_date, manager_id) VALUES ($1, $2, $3, $3, 'T', '2026-01-01', $4) RETURNING id`, cid, uid, no, mgr).Scan(&id))
		return id
	}
	uMgr, uRep, uOther = user("mgr", "manager"), user("rep", "employee"), user("oth", "employee")
	eMgr = emp(uMgr, "MGR", nil)
	eRep = emp(uRep, "REP", &eMgr)
	eOther = emp(uOther, "OTH", nil)
	must(pool.QueryRow(ctx, `INSERT INTO leave_types (company_id, code, name, is_paid, default_days) VALUES ($1, 'VL', 'Vacation', true, 5) RETURNING id`, cid).Scan(&ltID))
	for _, e := range []int64{eMgr, eRep, eOther} {
		_, err := pool.Exec(ctx, `INSERT INTO leave_balances (company_id, employee_id, leave_type_id, year, earned, used, carried, adjusted) VALUES ($1, $2, $3, 2027, 5, 0, 0, 0)`, cid, e, ltID)
		must(err)
	}
	// Wed 2027-01-06 is a company holiday.
	_, err = pool.Exec(ctx, `INSERT INTO holidays (company_id, name, holiday_date, holiday_type, year) VALUES ($1, 'Test day', '2027-01-06', 'regular', 2027)`, cid)
	must(err)

	gin.SetMode(gin.TestMode)
	call := func(uid int64, role auth.Role, method, path string, body any) (int, map[string]any) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(auth.ContextKeyCompanyID, cid)
			c.Set(auth.ContextKeyUserID, uid)
			c.Set(auth.ContextKeyRole, role)
		})
		h.RegisterRoutes(r.Group(""))
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewReader(b)))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	req := func(uid int64, start, end, days string) (int, map[string]any) {
		return call(uid, auth.RoleEmployee, "POST", "/leaves/requests", map[string]any{"leave_type_id": ltID, "start_date": start, "end_date": end, "days": days})
	}
	// Mon 4 .. Sun 10 Jan 2027: 5 weekdays minus the holiday = 4, whatever the client says.
	code, out := req(uRep, "2027-01-04", "2027-01-10", "1")
	if code != 201 || out["data"].(map[string]any)["days"] != 4.0 {
		t.Fatalf("days must be counted by the server: %d %v", code, out)
	}
	id := int64(out["data"].(map[string]any)["id"].(float64))
	if code, _ := req(uRep, "2027-01-08", "2027-01-08", ""); code != 409 {
		t.Errorf("overlap accepted: %d", code)
	}
	if code, _ := req(uRep, "2027-02-01", "2027-02-12", ""); code != 400 {
		t.Errorf("10 days with 5 left accepted: %d", code)
	}
	if code, _ := req(uRep, "2027-01-09", "2027-01-10", ""); code != 400 {
		t.Errorf("weekend-only request accepted: %d", code)
	}
	if code, _ := req(uRep, "2027-01-20", "2027-01-18", ""); code != 400 {
		t.Errorf("end before start accepted: %d", code)
	}
	if code, out := req(uRep, "2027-03-01", "2027-03-01", "0.5"); code != 201 || out["data"].(map[string]any)["days"] != 0.5 {
		t.Errorf("half day: %d %v", code, out)
	}
	// Decisions: not your own, only your reports'.
	code, mine := req(uMgr, "2027-01-11", "2027-01-11", "")
	if code != 201 {
		t.Fatalf("manager's own request: %d %v", code, mine)
	}
	mineID := int64(mine["data"].(map[string]any)["id"].(float64))
	if code, _ := call(uMgr, auth.RoleManager, "POST", fmt.Sprintf("/leaves/requests/%d/approve", mineID), nil); code != 403 {
		t.Errorf("manager approved own leave: %d", code)
	}
	_, oth := req(uOther, "2027-01-12", "2027-01-12", "")
	othID := int64(oth["data"].(map[string]any)["id"].(float64))
	if code, _ := call(uMgr, auth.RoleManager, "POST", fmt.Sprintf("/leaves/requests/%d/approve", othID), nil); code != 403 {
		t.Errorf("manager approved a non-report: %d", code)
	}
	if code, out := call(uMgr, auth.RoleManager, "POST", fmt.Sprintf("/leaves/requests/%d/approve", id), nil); code != 200 {
		t.Fatalf("manager approves report: %d %v", code, out)
	}
	var stale int
	must(pool.QueryRow(ctx, `SELECT COUNT(*) FROM approval_workflows WHERE entity_type = 'leave_request' AND entity_id = $1 AND status = 'pending'`, id).Scan(&stale))
	if stale != 0 {
		t.Errorf("approved leave still pending in the approvals inbox (%d)", stale)
	}
	var used float64
	must(pool.QueryRow(ctx, `SELECT used::float8 FROM leave_balances WHERE employee_id = $1 AND leave_type_id = $2 AND year = 2027`, eRep, ltID).Scan(&used))
	if used != 4 {
		t.Errorf("balance used = %v, want 4", used)
	}
	// An admin without an employee record can reject.
	if code, out := call(999999999, auth.RoleAdmin, "POST", fmt.Sprintf("/leaves/requests/%d/reject", othID), map[string]any{"reason": "no"}); code != 200 {
		t.Errorf("admin reject: %d %v", code, out)
	}
}
