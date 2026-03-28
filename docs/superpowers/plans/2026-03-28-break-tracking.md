# Break Tracking & Monthly Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add mid-work break clock-in/out (meal, bathroom, rest, leave_post) with company-configurable overtime rules and a monthly Excel report matching the 23-column format.

**Architecture:** Two new tables (`break_policies`, `break_logs`) with a new `internal/breaks` handler package. Bot gets inline keyboard break flow via new `SendWithKeyboard` interface method. Frontend adds break section to existing AttendanceView. CLI gets `hr break` subcommands. Monthly report uses `excelize` for .xlsx generation.

**Tech Stack:** Go 1.25 (Gin, sqlc, pgx/v5, excelize), Vue3+TS+NaiveUI, Cobra CLI, Telegram Bot API inline keyboards.

---

## File Structure

### New Files (Backend)
| File | Responsibility |
|------|---------------|
| `db/migrations/00088_break_tracking.sql` | Create break_policies + break_logs tables |
| `db/query/breaks.sql` | sqlc queries for break_logs + break_policies |
| `internal/breaks/handler.go` | Handler struct + constructor |
| `internal/breaks/routes.go` | RegisterRoutes for break endpoints |
| `internal/breaks/handler_break.go` | Start/end/list/active break endpoints |
| `internal/breaks/handler_policy.go` | Break policy CRUD (admin) |
| `internal/breaks/handler_report.go` | Monthly Excel report generation |
| `internal/breaks/handler_test.go` | Unit tests for break handlers |
| `internal/breaks/handler_policy_test.go` | Unit tests for policy handlers |
| `internal/breaks/handler_report_test.go` | Unit tests for report handler |
| `internal/bot/handler_break.go` | Bot break command + callback handlers |
| `internal/bot/handler_break_test.go` | Bot break handler tests |

### New Files (Frontend)
| File | Responsibility |
|------|---------------|
| (none — all changes go into existing files) | |

### New Files (CLI)
| File | Responsibility |
|------|---------------|
| `cmd/hr_break.go` | Break start/end/list/active CLI commands |
| `cmd/hr_break_policy.go` | Break policy list/set CLI commands |
| `cmd/hr_report.go` | Monthly report download CLI command |
| `cmd/hr_break_test.go` | Break CLI tests |
| `cmd/hr_break_policy_test.go` | Break policy CLI tests |
| `cmd/hr_report_test.go` | Report CLI tests |

### Modified Files
| File | Change |
|------|--------|
| `internal/app/bootstrap.go` | Wire breaks handler |
| `internal/bot/interfaces.go` | Add `SendWithKeyboard` to MessageSender |
| `internal/bot/telegram/sender.go` | Implement `SendWithKeyboard` |
| `internal/bot/dispatcher.go` | Add break commands + callback routing |
| `frontend/src/api/client.ts` | Add breakAPI methods |
| `frontend/src/views/AttendanceView.vue` | Add break section UI |
| `frontend/src/views/SettingsView.vue` | Add break policy settings (or new tab) |
| `frontend/src/i18n/en.ts` | Break-related English translations |
| `frontend/src/i18n/zh.ts` | Break-related Chinese translations |
| `go.mod` / `go.sum` | Add excelize dependency |

---

## Task 1: Database Migration

**Files:**
- Create: `db/migrations/00088_break_tracking.sql`

- [ ] **Step 1: Create the migration file**

```sql
-- +goose Up

-- Break policies: company-level per-break-type overtime thresholds
CREATE TABLE break_policies (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    break_type      VARCHAR(20) NOT NULL,
    max_minutes     INT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, break_type)
);

-- Break logs: one row per break session
CREATE TABLE break_logs (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id),
    employee_id       BIGINT NOT NULL REFERENCES employees(id),
    attendance_log_id BIGINT NOT NULL REFERENCES attendance_logs(id),
    break_type        VARCHAR(20) NOT NULL CHECK (break_type IN ('meal','bathroom','rest','leave_post')),
    start_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_at            TIMESTAMPTZ,
    duration_minutes  INT,
    overtime_minutes  INT DEFAULT 0,
    note              TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_break_logs_attendance ON break_logs(attendance_log_id);
CREATE INDEX idx_break_logs_employee_date ON break_logs(employee_id, start_at DESC);
CREATE INDEX idx_break_logs_company_period ON break_logs(company_id, start_at);

-- Seed default break policies for all existing companies
INSERT INTO break_policies (company_id, break_type, max_minutes)
SELECT c.id, bt.break_type, bt.max_minutes
FROM companies c
CROSS JOIN (VALUES
    ('meal', 30),
    ('bathroom', 5),
    ('rest', 0),
    ('leave_post', 0)
) AS bt(break_type, max_minutes)
ON CONFLICT (company_id, break_type) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS break_logs;
DROP TABLE IF EXISTS break_policies;
```

**Note**: For new companies created after this migration, add a `SeedBreakPolicies` function in the company creation handler to insert the 4 default policies.

- [ ] **Step 2: Verify migration applies locally**

Run: `cd /Users/anna/Documents/aigonhr && goose -dir db/migrations postgres "$DATABASE_URL" up`
Expected: Migration 00088 applied successfully.

- [ ] **Step 3: Commit**

```bash
git add db/migrations/00088_break_tracking.sql
git commit -m "feat: add break_policies and break_logs tables (migration 00088)"
```

---

## Task 2: sqlc Queries

**Files:**
- Create: `db/query/breaks.sql`

- [ ] **Step 1: Write all break sqlc queries**

```sql
-- name: CreateBreakLog :one
INSERT INTO break_logs (company_id, employee_id, attendance_log_id, break_type, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: EndBreakLog :one
UPDATE break_logs SET
    end_at = NOW(),
    duration_minutes = EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60,
    overtime_minutes = CASE
        WHEN $2::int > 0 AND EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60 > $2::int
        THEN EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60 - $2::int
        ELSE 0
    END
WHERE id = $1 AND end_at IS NULL
RETURNING *;

-- name: GetActiveBreak :one
SELECT * FROM break_logs
WHERE employee_id = $1 AND company_id = $2 AND end_at IS NULL
ORDER BY start_at DESC LIMIT 1;

-- name: ListBreaksByAttendance :many
SELECT * FROM break_logs
WHERE attendance_log_id = $1
ORDER BY start_at;

-- name: ListBreaksByDate :many
SELECT * FROM break_logs
WHERE employee_id = $1 AND company_id = $2
    AND start_at >= $3 AND start_at < $4
ORDER BY start_at;

-- name: GetMonthlyBreakSummary :many
SELECT
    bl.employee_id,
    bl.break_type,
    COUNT(*)::int as total_count,
    COALESCE(SUM(bl.duration_minutes), 0)::int as total_minutes,
    COALESCE(SUM(bl.overtime_minutes), 0)::int as total_overtime_minutes,
    COUNT(CASE WHEN bl.overtime_minutes > 0 THEN 1 END)::int as overtime_count
FROM break_logs bl
WHERE bl.company_id = $1
    AND bl.start_at >= $2 AND bl.start_at < $3
    AND bl.end_at IS NOT NULL
GROUP BY bl.employee_id, bl.break_type;

-- name: ListBreakPolicies :many
SELECT * FROM break_policies
WHERE company_id = $1 AND is_active = true;

-- name: UpsertBreakPolicy :one
INSERT INTO break_policies (company_id, break_type, max_minutes)
VALUES ($1, $2, $3)
ON CONFLICT (company_id, break_type)
DO UPDATE SET max_minutes = $3, updated_at = NOW()
RETURNING *;

-- name: GetBreakPolicy :one
SELECT * FROM break_policies
WHERE company_id = $1 AND break_type = $2 AND is_active = true;
```

- [ ] **Step 2: Generate sqlc code**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors. New file `internal/store/breaks.sql.go` generated.

- [ ] **Step 3: Verify generated code compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add db/query/breaks.sql internal/store/breaks.sql.go
git commit -m "feat: add sqlc queries for break_logs and break_policies"
```

---

## Task 3: Backend Break Handler — Core Structure

**Files:**
- Create: `internal/breaks/handler.go`
- Create: `internal/breaks/routes.go`

- [ ] **Step 1: Write the handler struct and constructor**

`internal/breaks/handler.go`:
```go
package breaks

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/store"
)

// Handler manages break tracking endpoints.
type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	rdb     *redis.Client
}

// NewHandler creates a break tracking handler.
func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, rdb *redis.Client) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, rdb: rdb}
}
```

- [ ] **Step 2: Write the routes file**

`internal/breaks/routes.go`:
```go
package breaks

import (
	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/auth"
)

// RegisterRoutes registers break tracking endpoints.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	// Employee break actions
	protected.POST("/attendance/breaks/start", h.StartBreak)
	protected.POST("/attendance/breaks/end", h.EndBreak)
	protected.GET("/attendance/breaks", h.ListBreaks)
	protected.GET("/attendance/breaks/active", h.GetActiveBreak)

	// Admin break policy management
	protected.GET("/attendance/break-policies", auth.ManagerOrAbove(), h.ListPolicies)
	protected.PUT("/attendance/break-policies", auth.AdminOnly(), h.UpsertPolicies)

	// Monthly report (manager+)
	protected.GET("/attendance/report/monthly", auth.ManagerOrAbove(), h.MonthlyReport)
}
```

- [ ] **Step 3: Verify it compiles (stubs will be needed — add in next tasks)**

We'll add the handler methods in the next tasks. For now, just verify the structure compiles by adding placeholder stubs:

`internal/breaks/handler_break.go`:
```go
package breaks

import "github.com/gin-gonic/gin"

func (h *Handler) StartBreak(c *gin.Context)    {}
func (h *Handler) EndBreak(c *gin.Context)      {}
func (h *Handler) ListBreaks(c *gin.Context)     {}
func (h *Handler) GetActiveBreak(c *gin.Context) {}
```

`internal/breaks/handler_policy.go`:
```go
package breaks

import "github.com/gin-gonic/gin"

func (h *Handler) ListPolicies(c *gin.Context)  {}
func (h *Handler) UpsertPolicies(c *gin.Context) {}
```

`internal/breaks/handler_report.go`:
```go
package breaks

import "github.com/gin-gonic/gin"

func (h *Handler) MonthlyReport(c *gin.Context) {}
```

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Wire handler in bootstrap.go**

Modify `internal/app/bootstrap.go`. After the attendance handler line (line ~244), add:

```go
breaksHandler := breaks.NewHandler(a.Queries, a.Pool, a.Logger, a.Redis)
```

And after `attendanceHandler.RegisterRoutes(protected)` (line ~364), add:

```go
breaksHandler.RegisterRoutes(protected)
```

Add the import:
```go
"github.com/tonypk/aigonhr/internal/breaks"
```

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/breaks/ internal/app/bootstrap.go
git commit -m "feat: scaffold breaks handler with routes and bootstrap wiring"
```

---

## Task 4: Backend — StartBreak Endpoint (TDD)

**Files:**
- Modify: `internal/breaks/handler_break.go`
- Create: `internal/breaks/handler_test.go`

- [ ] **Step 1: Write the failing test for StartBreak**

`internal/breaks/handler_test.go`:
```go
package breaks

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger, nil)
}

func TestStartBreak_NotClockedIn(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID returns employee
	mockDB.OnQueryRow(testutil.NewRow(store.Employee{ID: 10, CompanyID: 1}))
	// GetOpenAttendance returns error (not clocked in)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "meal",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartBreak_AlreadyOnBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID
	mockDB.OnQueryRow(testutil.NewRow(store.Employee{ID: 10, CompanyID: 1}))
	// GetOpenAttendance succeeds
	mockDB.OnQueryRow(testutil.NewRow(store.AttendanceLog{ID: 100, EmployeeID: 10, CompanyID: 1}))
	// GetActiveBreak returns existing break (already on break)
	mockDB.OnQueryRow(testutil.NewRow(store.BreakLog{ID: 5, BreakType: "meal"}))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "bathroom",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartBreak_InvalidType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "invalid",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -run TestStartBreak -v`
Expected: FAIL (stub handler returns 200 with empty body)

- [ ] **Step 3: Implement StartBreak**

Replace `internal/breaks/handler_break.go`:
```go
package breaks

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

var validBreakTypes = map[string]bool{
	"meal": true, "bathroom": true, "rest": true, "leave_post": true,
}

type startBreakRequest struct {
	BreakType string `json:"break_type" binding:"required"`
	Note      string `json:"note"`
}

func (h *Handler) StartBreak(c *gin.Context) {
	var req startBreakRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "break_type is required")
		return
	}

	if !validBreakTypes[req.BreakType] {
		response.BadRequest(c, "Invalid break type. Must be: meal, bathroom, rest, leave_post")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	// Must have open attendance
	att, err := h.queries.GetOpenAttendance(c.Request.Context(), store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.BadRequest(c, "Must clock in before starting a break")
		return
	}

	// No active break allowed
	activeBreak, err := h.queries.GetActiveBreak(c.Request.Context(), store.GetActiveBreakParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err == nil && activeBreak.ID > 0 {
		response.BadRequest(c, "Already on break (type: "+activeBreak.BreakType+"). End it first.")
		return
	}

	var notePtr *string
	if req.Note != "" {
		notePtr = &req.Note
	}

	breakLog, err := h.queries.CreateBreakLog(c.Request.Context(), store.CreateBreakLogParams{
		CompanyID:       companyID,
		EmployeeID:      emp.ID,
		AttendanceLogID: att.ID,
		BreakType:       req.BreakType,
		Note:            notePtr,
	})
	if err != nil {
		h.logger.Error("failed to create break log", "error", err)
		response.InternalError(c, "Failed to start break")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": breakLog})
}

func (h *Handler) EndBreak(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	activeBreak, err := h.queries.GetActiveBreak(c.Request.Context(), store.GetActiveBreakParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.BadRequest(c, "No active break to end")
		return
	}

	// Look up policy for overtime calculation
	var maxMinutes int32
	policy, err := h.queries.GetBreakPolicy(c.Request.Context(), store.GetBreakPolicyParams{
		CompanyID: companyID,
		BreakType: activeBreak.BreakType,
	})
	if err == nil {
		maxMinutes = policy.MaxMinutes
	}

	ended, err := h.queries.EndBreakLog(c.Request.Context(), store.EndBreakLogParams{
		ID:         activeBreak.ID,
		MaxMinutes: maxMinutes,
	})
	if err != nil {
		h.logger.Error("failed to end break", "error", err)
		response.InternalError(c, "Failed to end break")
		return
	}

	response.OK(c, ended)
}

func (h *Handler) ListBreaks(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	dateStr := c.DefaultQuery("date", "")
	start, end, err := parseDateRange(dateStr)
	if err != nil {
		response.BadRequest(c, "Invalid date format")
		return
	}

	breaks, err := h.queries.ListBreaksByDate(c.Request.Context(), store.ListBreaksByDateParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
		StartAt:    start,
		StartAt_2:  end,
	})
	if err != nil && err != pgx.ErrNoRows {
		h.logger.Error("failed to list breaks", "error", err)
		response.InternalError(c, "Failed to list breaks")
		return
	}

	response.OK(c, breaks)
}

func (h *Handler) GetActiveBreak(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	activeBreak, err := h.queries.GetActiveBreak(c.Request.Context(), store.GetActiveBreakParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.OK(c, nil)
		return
	}

	response.OK(c, activeBreak)
}
```

Also add a helper file `internal/breaks/helpers.go`:
```go
package breaks

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func parseDateRange(dateStr string) (pgtype.Timestamptz, pgtype.Timestamptz, error) {
	var start, end pgtype.Timestamptz

	loc := time.UTC
	if dateStr == "" {
		now := time.Now().In(loc)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		start = pgtype.Timestamptz{Time: today, Valid: true}
		end = pgtype.Timestamptz{Time: today.Add(24 * time.Hour), Valid: true}
		return start, end, nil
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return start, end, err
	}

	start = pgtype.Timestamptz{Time: t, Valid: true}
	end = pgtype.Timestamptz{Time: t.Add(24 * time.Hour), Valid: true}
	return start, end, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -run TestStartBreak -v`
Expected: All 3 tests PASS.

Note: The exact mock row patterns depend on the generated sqlc types. You may need to adjust `testutil.NewRow(store.BreakLog{...})` fields to match the generated `BreakLog` model. Check `internal/store/models.go` for the exact struct after sqlc generate.

- [ ] **Step 5: Commit**

```bash
git add internal/breaks/
git commit -m "feat: implement StartBreak, EndBreak, ListBreaks, GetActiveBreak endpoints"
```

---

## Task 5: Backend — EndBreak Tests

**Files:**
- Modify: `internal/breaks/handler_test.go`

- [ ] **Step 1: Add EndBreak tests**

Append to `internal/breaks/handler_test.go`:
```go
func TestEndBreak_NoActiveBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID
	mockDB.OnQueryRow(testutil.NewRow(store.Employee{ID: 10, CompanyID: 1}))
	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/end", nil, adminAuth)

	h.EndBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetActiveBreak_None(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID
	mockDB.OnQueryRow(testutil.NewRow(store.Employee{ID: 10, CompanyID: 1}))
	// GetActiveBreak fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContext("GET", "/attendance/breaks/active", nil, adminAuth)

	h.GetActiveBreak(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -v`
Expected: All tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/breaks/handler_test.go
git commit -m "test: add EndBreak and GetActiveBreak tests"
```

---

## Task 6: Backend — Break Policy Handler (TDD)

**Files:**
- Modify: `internal/breaks/handler_policy.go`
- Create: `internal/breaks/handler_policy_test.go`

- [ ] **Step 1: Write failing tests for policy endpoints**

`internal/breaks/handler_policy_test.go`:
```go
package breaks

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

func TestListPolicies_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// ListBreakPolicies returns empty
	mockDB.OnQuery(testutil.NewRows(nil))

	c, w := testutil.NewGinContext("GET", "/attendance/break-policies", nil, adminAuth)

	h.ListPolicies(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpsertPolicies_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// 4 UpsertBreakPolicy calls
	for i := 0; i < 4; i++ {
		mockDB.OnQueryRow(testutil.NewRow(store.BreakPolicy{ID: int64(i + 1), CompanyID: 1}))
	}

	body := gin.H{
		"policies": []gin.H{
			{"break_type": "meal", "max_minutes": 30},
			{"break_type": "bathroom", "max_minutes": 5},
			{"break_type": "rest", "max_minutes": 0},
			{"break_type": "leave_post", "max_minutes": 0},
		},
	}

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", body, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpsertPolicies_InvalidType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	body := gin.H{
		"policies": []gin.H{
			{"break_type": "nap", "max_minutes": 30},
		},
	}

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", body, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -run TestListPolicies -v`
Expected: FAIL (stubs are empty)

- [ ] **Step 3: Implement policy handlers**

Replace `internal/breaks/handler_policy.go`:
```go
package breaks

import (
	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListPolicies(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	policies, err := h.queries.ListBreakPolicies(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("failed to list break policies", "error", err)
		response.InternalError(c, "Failed to list break policies")
		return
	}

	response.OK(c, policies)
}

type policyItem struct {
	BreakType  string `json:"break_type" binding:"required"`
	MaxMinutes int32  `json:"max_minutes"`
}

type upsertPoliciesRequest struct {
	Policies []policyItem `json:"policies" binding:"required,dive"`
}

func (h *Handler) UpsertPolicies(c *gin.Context) {
	var req upsertPoliciesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	for _, p := range req.Policies {
		if !validBreakTypes[p.BreakType] {
			response.BadRequest(c, "Invalid break type: "+p.BreakType)
			return
		}
	}

	results := make([]store.BreakPolicy, 0, len(req.Policies))
	for _, p := range req.Policies {
		policy, err := h.queries.UpsertBreakPolicy(c.Request.Context(), store.UpsertBreakPolicyParams{
			CompanyID:  companyID,
			BreakType:  p.BreakType,
			MaxMinutes: p.MaxMinutes,
		})
		if err != nil {
			h.logger.Error("failed to upsert break policy", "error", err, "type", p.BreakType)
			response.InternalError(c, "Failed to save break policy")
			return
		}
		results = append(results, policy)
	}

	response.OK(c, results)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/breaks/handler_policy.go internal/breaks/handler_policy_test.go
git commit -m "feat: implement break policy list and upsert endpoints"
```

---

## Task 7: Backend — Monthly Excel Report (TDD)

**Files:**
- Modify: `internal/breaks/handler_report.go`
- Create: `internal/breaks/handler_report_test.go`

- [ ] **Step 1: Add excelize dependency**

Run: `cd /Users/anna/Documents/aigonhr && go get github.com/xuri/excelize/v2`
Expected: Dependency added to go.mod/go.sum.

- [ ] **Step 2: Write the failing test**

`internal/breaks/handler_report_test.go`:
```go
package breaks

import (
	"net/http"
	"testing"

	"github.com/tonypk/aigonhr/internal/testutil"
)

func TestMonthlyReport_MissingParams(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("GET", "/attendance/report/monthly", nil, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_InvalidYear(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("GET", "/attendance/report/monthly?year=abc&month=2", nil, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -run TestMonthlyReport -v`
Expected: FAIL (stub handler returns 200)

- [ ] **Step 4: Implement the monthly report handler**

Replace `internal/breaks/handler_report.go`:
```go
package breaks

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// breakTypeLabels maps break_type to Chinese labels.
var breakTypeLabels = map[string]string{
	"meal": "吃饭", "bathroom": "上厕所", "rest": "休息", "leave_post": "中途离岗",
}

func (h *Handler) MonthlyReport(c *gin.Context) {
	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		response.BadRequest(c, "year and month are required")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		response.BadRequest(c, "Invalid year")
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		response.BadRequest(c, "Invalid month")
		return
	}

	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	// Date range for the month
	startTime := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.AddDate(0, 1, 0)
	start := pgtype.Timestamptz{Time: startTime, Valid: true}
	end := pgtype.Timestamptz{Time: endTime, Valid: true}

	// Get attendance report data (existing query)
	attendanceData, err := h.queries.GetAttendanceReport(ctx, store.GetAttendanceReportParams{
		CompanyID: companyID,
		StartAt:   start,
		StartAt_2: end,
	})
	if err != nil {
		h.logger.Error("failed to get attendance report", "error", err)
		response.InternalError(c, "Failed to generate report")
		return
	}

	// Get break summary data
	breakData, err := h.queries.GetMonthlyBreakSummary(ctx, store.GetMonthlyBreakSummaryParams{
		CompanyID: companyID,
		StartAt:   start,
		StartAt_2: end,
	})
	if err != nil {
		h.logger.Error("failed to get break summary", "error", err)
		response.InternalError(c, "Failed to generate report")
		return
	}

	// Get telegram display names via bot_user_links
	botLinks, _ := h.queries.ListBotUserLinksByCompany(ctx, companyID)

	// Build break map: employee_id -> break_type -> summary
	breakMap := make(map[int64]map[string]*breakSummaryData)
	for _, b := range breakData {
		if _, ok := breakMap[b.EmployeeID]; !ok {
			breakMap[b.EmployeeID] = make(map[string]*breakSummaryData)
		}
		breakMap[b.EmployeeID][b.BreakType] = &breakSummaryData{
			Count:           b.TotalCount,
			TotalMinutes:    b.TotalMinutes,
			OvertimeMinutes: b.TotalOvertimeMinutes,
			OvertimeCount:   b.OvertimeCount,
		}
	}

	// Build bot link map: user_id -> platform_user_id
	// Then we need employee_id -> platform_user_id, so look up employees by user
	botLinkMap := make(map[int64]string) // user_id -> platform_user_id
	for _, link := range botLinks {
		if link.PlatformUserID != nil {
			botLinkMap[link.UserID] = *link.PlatformUserID
		}
	}
	// Build employee_id -> user_id map from attendance data (employees table has user_id)
	empUserMap := make(map[int64]int64) // employee_id -> user_id
	employees, _ := h.queries.ListActiveEmployees(ctx, companyID)
	for _, emp := range employees {
		if emp.UserID != nil {
			empUserMap[emp.ID] = *emp.UserID
		}
	}

	// Generate Excel
	f := excelize.NewFile()
	sheet := "月报"
	f.SetSheetName("Sheet1", sheet)

	// Headers (A-W)
	headers := []string{
		"用户昵称", "用户标识", "工作天数", "工作时间总计", "纯工作时间总计",
		"迟到天数", "迟到总时长", "早退天数", "早退总时长",
		"吃饭总次数", "吃饭总用时", "吃饭总超时",
		"上厕所总次数", "上厕所总用时", "上厕所总超时",
		"休息总次数", "休息总用时",
		"中途离岗总次数", "中途离岗总用时",
		"所有次数总计", "所有用时总计", "所有超时总计", "手动惩罚",
	}
	for i, h := range headers {
		cell := fmt.Sprintf("%s1", string(rune('A'+i)))
		f.SetCellValue(sheet, cell, h)
	}

	// Style header row bold
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	f.SetRowStyle(sheet, 1, 1, style)

	dash := "——"

	for rowIdx, att := range attendanceData {
		row := rowIdx + 2
		empBreaks := breakMap[att.EmployeeID]

		// A: 用户昵称 (employee name)
		name := att.FirstName + " " + att.LastName
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), name)

		// B: 用户标识 (Telegram ID) — map employee_id -> user_id -> platform_user_id
		userID := empUserMap[att.EmployeeID]
		botID := botLinkMap[userID]
		if botID != "" {
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), botID)
		} else {
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), dash)
		}

		// C: 工作天数
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), att.DaysWorked)

		// D: 工作时间总计
		workHrs := float64(att.TotalWorkHours) // numeric stored as string
		if workHrs > 0 {
			f.SetCellValue(sheet, fmt.Sprintf("D%d", row), fmt.Sprintf("%.1f 小时", workHrs))
		} else {
			f.SetCellValue(sheet, fmt.Sprintf("D%d", row), dash)
		}

		// Compute total break minutes for E
		var totalBreakMinutes int32
		for _, bt := range []string{"meal", "bathroom", "rest", "leave_post"} {
			if bs, ok := empBreaks[bt]; ok {
				totalBreakMinutes += bs.TotalMinutes
			}
		}

		// E: 纯工作时间总计 (work hours minus break time)
		pureHrs := workHrs - float64(totalBreakMinutes)/60.0
		if pureHrs > 0 {
			f.SetCellValue(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("%.1f 小时", pureHrs))
		} else {
			f.SetCellValue(sheet, fmt.Sprintf("E%d", row), dash)
		}

		// F: 迟到天数
		setIntOrDash(f, sheet, row, 'F', int32(att.LateCount))

		// G: 迟到总时长
		setMinutesOrDash(f, sheet, row, 'G', int32(att.TotalLateMinutes))

		// H: 早退天数
		setIntOrDash(f, sheet, row, 'H', int32(att.UndertimeCount))

		// I: 早退总时长
		setMinutesOrDash(f, sheet, row, 'I', int32(att.TotalUndertimeMinutes))

		// J-L: 吃饭 (meal)
		writeBreakColumns(f, sheet, row, 'J', empBreaks["meal"])

		// M-O: 上厕所 (bathroom)
		writeBreakColumns(f, sheet, row, 'M', empBreaks["bathroom"])

		// P-Q: 休息 (rest) — no overtime column
		writeBreakColumnsNoOT(f, sheet, row, 'P', empBreaks["rest"])

		// R-S: 中途离岗 (leave_post) — no overtime column
		writeBreakColumnsNoOT(f, sheet, row, 'R', empBreaks["leave_post"])

		// T: 所有次数总计
		var totalCount int32
		for _, bt := range []string{"meal", "bathroom", "rest", "leave_post"} {
			if bs, ok := empBreaks[bt]; ok {
				totalCount += bs.Count
			}
		}
		setIntOrDash(f, sheet, row, 'T', totalCount)

		// U: 所有用时总计
		setHoursOrDash(f, sheet, row, 'U', totalBreakMinutes)

		// V: 所有超时总计
		var totalOT int32
		for _, bt := range []string{"meal", "bathroom"} {
			if bs, ok := empBreaks[bt]; ok {
				totalOT += bs.OvertimeMinutes
			}
		}
		setMinutesOrDash(f, sheet, row, 'V', totalOT)

		// W: 手动惩罚
		f.SetCellValue(sheet, fmt.Sprintf("W%d", row), dash)
	}

	// Set column widths
	for i := 0; i < 23; i++ {
		col := string(rune('A' + i))
		f.SetColWidth(sheet, col, col, 14)
	}

	// Write to response
	filename := fmt.Sprintf("monthly_break_report_%d_%02d.xlsx", year, month)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := f.Write(c.Writer); err != nil {
		h.logger.Error("failed to write Excel file", "error", err)
		return
	}
	c.Status(http.StatusOK)
}

func setIntOrDash(f *excelize.File, sheet string, row int, col rune, val int32) {
	cell := fmt.Sprintf("%c%d", col, row)
	if val > 0 {
		f.SetCellValue(sheet, cell, val)
	} else {
		f.SetCellValue(sheet, cell, "——")
	}
}

func setMinutesOrDash(f *excelize.File, sheet string, row int, col rune, minutes int32) {
	cell := fmt.Sprintf("%c%d", col, row)
	if minutes > 0 {
		f.SetCellValue(sheet, cell, fmt.Sprintf("%d 分钟", minutes))
	} else {
		f.SetCellValue(sheet, cell, "——")
	}
}

func setHoursOrDash(f *excelize.File, sheet string, row int, col rune, minutes int32) {
	cell := fmt.Sprintf("%c%d", col, row)
	if minutes > 0 {
		f.SetCellValue(sheet, cell, fmt.Sprintf("%.1f 小时", float64(minutes)/60.0))
	} else {
		f.SetCellValue(sheet, cell, "——")
	}
}

type breakSummaryData struct {
	Count           int32
	TotalMinutes    int32
	OvertimeMinutes int32
	OvertimeCount   int32
}

func writeBreakColumns(f *excelize.File, sheet string, row int, startCol rune, bs *breakSummaryData) {
	if bs == nil {
		bs = &breakSummaryReport{}
	}
	// Count
	setIntOrDash(f, sheet, row, startCol, bs.Count)
	// Total time (hours)
	setHoursOrDash(f, sheet, row, startCol+1, bs.TotalMinutes)
	// Overtime
	cell := fmt.Sprintf("%c%d", startCol+2, row)
	if bs.OvertimeMinutes > 0 {
		f.SetCellValue(sheet, cell, fmt.Sprintf("%d 分钟（共 %d 次）", bs.OvertimeMinutes, bs.OvertimeCount))
	} else {
		f.SetCellValue(sheet, cell, "——")
	}
}

func writeBreakColumnsNoOT(f *excelize.File, sheet string, row int, startCol rune, bs *breakSummaryData) {
	if bs == nil {
		bs = &breakSummaryReport{}
	}
	setIntOrDash(f, sheet, row, startCol, bs.Count)
	setHoursOrDash(f, sheet, row, startCol+1, bs.TotalMinutes)
}
```

- [ ] **Step 5: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -run TestMonthlyReport -v`
Expected: Tests PASS.

- [ ] **Step 6: Verify full build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 7: Commit**

```bash
git add internal/breaks/handler_report.go internal/breaks/handler_report_test.go go.mod go.sum
git commit -m "feat: implement monthly Excel report with 23-column break tracking format"
```

---

## Task 8: Add ListBotUserLinksByCompany Query

The monthly report needs to look up Telegram display names. Add this query if it doesn't exist.

**Files:**
- Modify: `db/query/bot.sql`

- [ ] **Step 1: Check if the query already exists**

Run: `cd /Users/anna/Documents/aigonhr && grep -n "ListBotUserLinksByCompany" db/query/bot.sql`

If it doesn't exist, add to `db/query/bot.sql`:

```sql
-- name: ListBotUserLinksByCompany :many
SELECT * FROM bot_user_links
WHERE company_id = $1 AND verified_at IS NOT NULL;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors.

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add db/query/bot.sql internal/store/
git commit -m "feat: add ListBotUserLinksByCompany query for monthly report"
```

---

## Task 9: Bot — Add SendWithKeyboard to MessageSender Interface

**Files:**
- Modify: `internal/bot/interfaces.go`
- Modify: `internal/bot/telegram/sender.go`

- [ ] **Step 1: Add InlineButton type and SendWithKeyboard to interfaces.go**

Add to `internal/bot/interfaces.go`:

```go
// InlineButton represents an inline keyboard button (platform-agnostic).
type InlineButton struct {
	Text         string
	CallbackData string
}

// Add to MessageSender interface:
// SendWithKeyboard(ctx context.Context, chatID string, text string, buttons [][]InlineButton) error
```

The full updated interface:
```go
type MessageSender interface {
	SendText(ctx context.Context, chatID string, text string) error
	SendMarkdown(ctx context.Context, chatID string, markdown string) error
	SendDraftConfirmation(ctx context.Context, chatID string, text string, draftID string) error
	EditMessage(ctx context.Context, chatID string, messageID int, text string) error
	AnswerCallback(ctx context.Context, callbackID string, text string) error
	SendWithKeyboard(ctx context.Context, chatID string, text string, buttons [][]InlineButton) error
}
```

- [ ] **Step 2: Implement in Telegram sender**

Add to `internal/bot/telegram/sender.go`:

```go
func (s *Sender) SendWithKeyboard(ctx context.Context, chatID string, text string, buttons [][]bot.InlineButton) error {
	var rows [][]InlineKeyboardButton
	for _, row := range buttons {
		var kbRow []InlineKeyboardButton
		for _, btn := range row {
			kbRow = append(kbRow, InlineKeyboardButton{
				Text:         btn.Text,
				CallbackData: btn.CallbackData,
			})
		}
		rows = append(rows, kbRow)
	}

	keyboard := InlineKeyboardMarkup{InlineKeyboard: rows}

	return s.send(ctx, "sendMessage", map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": keyboard,
	})
}
```

Add the import for `bot` package:
```go
import (
	// ...
	bot "github.com/tonypk/aigonhr/internal/bot"
)
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/bot/interfaces.go internal/bot/telegram/sender.go
git commit -m "feat: add SendWithKeyboard to MessageSender interface for inline keyboards"
```

---

## Task 10: Bot — Break Command and Callback Handlers

**Files:**
- Create: `internal/bot/handler_break.go`
- Modify: `internal/bot/dispatcher.go`

- [ ] **Step 1: Create bot break handler**

`internal/bot/handler_break.go`:
```go
package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/tonypk/aigonhr/internal/store"
)

var breakTypeLabel = map[string]string{
	"meal": "吃饭", "bathroom": "上厕所", "rest": "休息", "leave_post": "中途离岗",
}

func (d *Dispatcher) handleBreakStart(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	// Check if already on break
	activeBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err == nil && activeBreak.ID > 0 {
		label := breakTypeLabel[activeBreak.BreakType]
		sender.SendWithKeyboard(ctx, msg.ChatID,
			fmt.Sprintf("⏳ 你正在休息中: %s\n开始时间: %s", label, activeBreak.StartAt.Time.Format("15:04")),
			[][]InlineButton{
				{{Text: "结束休息", CallbackData: "bk:end"}},
			},
		)
		return
	}

	// Check if clocked in
	_, err = d.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.SendText(ctx, msg.ChatID, "❌ 请先打卡上班")
		return
	}

	// Show break type selection keyboard
	sender.SendWithKeyboard(ctx, msg.ChatID, "选择休息类型:", [][]InlineButton{
		{
			{Text: "🍽 吃饭", CallbackData: "bk:meal"},
			{Text: "🚻 上厕所", CallbackData: "bk:bathroom"},
		},
		{
			{Text: "😌 休息", CallbackData: "bk:rest"},
			{Text: "🚪 中途离岗", CallbackData: "bk:leave_post"},
		},
	})
}

func (d *Dispatcher) handleBreakEnd(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	d.endActiveBreak(ctx, identity, msg.ChatID, sender)
}

func (d *Dispatcher) handleBreakStatus(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	activeBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.SendText(ctx, msg.ChatID, "✅ 没有进行中的休息")
		return
	}

	label := breakTypeLabel[activeBreak.BreakType]
	sender.SendText(ctx, msg.ChatID,
		fmt.Sprintf("⏳ 当前休息: %s\n开始时间: %s", label, activeBreak.StartAt.Time.Format("15:04")))
}

func (d *Dispatcher) handleBreakTypeCallback(ctx context.Context, cb CallbackQuery, identity *UserIdentity, breakType string, sender MessageSender) {
	// Verify clocked in
	att, err := d.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.AnswerCallback(ctx, cb.ID, "请先打卡上班")
		return
	}

	// Check no active break
	existingBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err == nil && existingBreak.ID > 0 {
		sender.AnswerCallback(ctx, cb.ID, "已有进行中的休息")
		return
	}

	// Create break log
	breakLog, err := d.queries.CreateBreakLog(ctx, store.CreateBreakLogParams{
		CompanyID:       identity.CompanyID,
		EmployeeID:      identity.EmployeeID,
		AttendanceLogID: att.ID,
		BreakType:       breakType,
	})
	if err != nil {
		d.logger.Error("bot: failed to create break", "error", err)
		sender.AnswerCallback(ctx, cb.ID, "创建休息记录失败")
		return
	}

	label := breakTypeLabel[breakType]
	sender.AnswerCallback(ctx, cb.ID, "✅ 开始休息")
	sender.EditMessage(ctx, cb.ChatID, cb.MessageID,
		fmt.Sprintf("✅ 开始休息: %s\n⏰ 开始时间: %s", label, breakLog.StartAt.Time.Format("15:04")))

	// Send end button
	sender.SendWithKeyboard(ctx, cb.ChatID,
		"休息进行中，完成后点击结束:",
		[][]InlineButton{
			{{Text: "结束休息", CallbackData: "bk:end"}},
		},
	)
}

func (d *Dispatcher) handleBreakEndCallback(ctx context.Context, cb CallbackQuery, identity *UserIdentity, sender MessageSender) {
	d.endActiveBreak(ctx, identity, cb.ChatID, sender)
	sender.AnswerCallback(ctx, cb.ID, "休息已结束")
}

func (d *Dispatcher) endActiveBreak(ctx context.Context, identity *UserIdentity, chatID string, sender MessageSender) {
	activeBreak, err := d.queries.GetActiveBreak(ctx, store.GetActiveBreakParams{
		EmployeeID: identity.EmployeeID,
		CompanyID:  identity.CompanyID,
	})
	if err != nil {
		sender.SendText(ctx, chatID, "❌ 没有进行中的休息")
		return
	}

	// Get policy for overtime calc
	var maxMinutes int32
	policy, err := d.queries.GetBreakPolicy(ctx, store.GetBreakPolicyParams{
		CompanyID: identity.CompanyID,
		BreakType: activeBreak.BreakType,
	})
	if err == nil {
		maxMinutes = policy.MaxMinutes
	}

	ended, err := d.queries.EndBreakLog(ctx, store.EndBreakLogParams{
		ID:         activeBreak.ID,
		MaxMinutes: maxMinutes,
	})
	if err != nil {
		d.logger.Error("bot: failed to end break", "error", err)
		sender.SendText(ctx, chatID, "❌ 结束休息失败")
		return
	}

	label := breakTypeLabel[ended.BreakType]
	dur := ended.DurationMinutes

	var durVal int32
	if dur != nil {
		durVal = *dur
	}

	var otVal int32
	if ended.OvertimeMinutes != nil {
		otVal = *ended.OvertimeMinutes
	}

	if otVal > 0 {
		sender.SendText(ctx, chatID,
			fmt.Sprintf("⚠️ 休息结束: %s\n⏱ 时长: %d 分钟（超时 %d 分钟）", label, durVal, otVal))
	} else {
		sender.SendText(ctx, chatID,
			fmt.Sprintf("✅ 休息结束: %s\n⏱ 时长: %d 分钟", label, durVal))
	}
}
```

- [ ] **Step 2: Add break commands to dispatcher**

Modify `internal/bot/dispatcher.go`:

In `dispatchCommand` switch, add before the `default` case:

```go
	case "break":
		d.handleBreakStart(ctx, msg, identity, sender)
	case "break_end":
		d.handleBreakEnd(ctx, msg, identity, sender)
	case "break_status":
		d.handleBreakStatus(ctx, msg, identity, sender)
```

In `HandleCallback`, refactor to handle break callbacks. Before the `action := data[:2]` line, add a check for the `bk:` prefix:

```go
	// Handle break callbacks (prefix "bk:")
	if strings.HasPrefix(data, "bk:") {
		breakAction := data[3:]
		if breakAction == "end" {
			d.handleBreakEndCallback(ctx, cb, identity, sender)
		} else {
			// breakAction is the break type (meal, bathroom, rest, leave_post)
			d.handleBreakTypeCallback(ctx, cb, identity, breakAction, sender)
		}
		return
	}
```

Also add `"strings"` to imports if not already present.

Update the help text in `handleHelp` to include break commands:

```go
func (d *Dispatcher) handleHelp(ctx context.Context, msg IncomingMessage, sender MessageSender) {
	help := `Available commands:
/balance - Check leave balances
/payslip - View latest payslip
/clock - Clock in/out (share location)
/break - Start a break (meal, bathroom, rest, leave post)
/break_end - End current break
/break_status - Check active break
/leave <description> - Request leave via AI
/new - Start a new conversation
/help - Show this help

Or just type a message to chat with the AI assistant.`
	sender.SendText(ctx, msg.ChatID, help)
}
```

Update the tutorial text in `handleStart` to include break commands — add these lines to the `📋 *Quick Commands*` section:

```
/break — Start a break (meal, bathroom, rest, leave post)
/break_end — End current break
/break_status — Check if you're on break
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/bot/handler_break.go internal/bot/dispatcher.go
git commit -m "feat: add Telegram bot break commands with inline keyboard flow"
```

---

## Task 11: Bot — Break Handler Tests

**Files:**
- Create: `internal/bot/handler_break_test.go`

- [ ] **Step 1: Write bot break handler tests**

`internal/bot/handler_break_test.go`:
```go
package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

type mockSender struct {
	texts     []string
	keyboards int
	callbacks []string
	edits     []string
}

func (m *mockSender) SendText(_ context.Context, _ string, text string) error {
	m.texts = append(m.texts, text)
	return nil
}
func (m *mockSender) SendMarkdown(_ context.Context, _ string, _ string) error { return nil }
func (m *mockSender) SendDraftConfirmation(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (m *mockSender) EditMessage(_ context.Context, _ string, _ int, text string) error {
	m.edits = append(m.edits, text)
	return nil
}
func (m *mockSender) AnswerCallback(_ context.Context, _ string, text string) error {
	m.callbacks = append(m.callbacks, text)
	return nil
}
func (m *mockSender) SendWithKeyboard(_ context.Context, _ string, _ string, _ [][]InlineButton) error {
	m.keyboards++
	return nil
}

func TestHandleBreakStart_NotClockedIn(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	// GetOpenAttendance fails (not clocked in)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "❌ 请先打卡上班" {
		t.Fatalf("expected not clocked in message, got: %v", sender.texts)
	}
}

func TestHandleBreakStart_ShowsKeyboard(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	// GetOpenAttendance succeeds
	mockDB.OnQueryRow(testutil.NewRow(store.AttendanceLog{ID: 100}))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	if sender.keyboards != 1 {
		t.Fatalf("expected keyboard to be sent, got %d keyboards", sender.keyboards)
	}
}

func TestHandleBreakStart_AlreadyOnBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak succeeds (already on break)
	mockDB.OnQueryRow(testutil.NewRow(store.BreakLog{ID: 5, BreakType: "meal"}))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	// Should show "end break" keyboard instead
	if sender.keyboards != 1 {
		t.Fatalf("expected end-break keyboard, got %d keyboards", sender.keyboards)
	}
}

func TestHandleBreakEnd_NoActiveBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakEnd(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "❌ 没有进行中的休息" {
		t.Fatalf("expected no active break message, got: %v", sender.texts)
	}
}

func TestHandleBreakStatus_NoBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStatus(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "✅ 没有进行中的休息" {
		t.Fatalf("expected no break message, got: %v", sender.texts)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/bot/ -run TestHandleBreak -v`
Expected: All tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/bot/handler_break_test.go
git commit -m "test: add bot break handler tests"
```

---

## Task 12: Frontend — API Client Break Methods

**Files:**
- Modify: `frontend/src/api/client.ts`

- [ ] **Step 1: Add breakAPI methods to client.ts**

Find the `attendanceAPI` export in `frontend/src/api/client.ts` and add a new `breakAPI` export after it:

```typescript
export const breakAPI = {
  startBreak: (data: { break_type: string; note?: string }) =>
    post("/v1/attendance/breaks/start", data),
  endBreak: () =>
    post("/v1/attendance/breaks/end", {}),
  listBreaks: (params?: Record<string, string>) =>
    get("/v1/attendance/breaks", params),
  getActiveBreak: () =>
    get("/v1/attendance/breaks/active"),
  listPolicies: () =>
    get("/v1/attendance/break-policies"),
  upsertPolicies: (data: { policies: Array<{ break_type: string; max_minutes: number }> }) =>
    put("/v1/attendance/break-policies", data),
  downloadMonthlyReport: (year: number, month: number) =>
    api(`/v1/attendance/report/monthly?year=${year}&month=${month}`, {
      method: "GET",
      responseType: "blob",
    }),
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/api/client.ts
git commit -m "feat: add breakAPI methods to frontend API client"
```

---

## Task 13: Frontend — i18n Break Translations

**Files:**
- Modify: `frontend/src/i18n/en.ts`
- Modify: `frontend/src/i18n/zh.ts`

- [ ] **Step 1: Add English break translations**

Find the `attendance:` section in `en.ts` and add a `break:` subsection after it:

```typescript
break: {
  title: "Break Tracking",
  startBreak: "Start Break",
  endBreak: "End Break",
  selectType: "Select break type",
  meal: "Meal",
  bathroom: "Bathroom",
  rest: "Rest",
  leavePost: "Leave Post",
  duration: "Duration",
  overtime: "Overtime",
  noActiveBreak: "No active break",
  activeBreak: "Active Break",
  breakStarted: "Break started",
  breakEnded: "Break ended",
  mustClockIn: "Must clock in before starting a break",
  alreadyOnBreak: "Already on break. End it first.",
  todayBreaks: "Today's Breaks",
  type: "Type",
  start: "Start",
  end: "End",
  note: "Note",
  policies: "Break Policies",
  maxMinutes: "Max Minutes (0 = no limit)",
  savePolicies: "Save Policies",
  policiesSaved: "Break policies saved",
  monthlyReport: "Monthly Break Report",
  downloadReport: "Download Report",
  minutes: "min",
  hours: "hrs",
},
```

- [ ] **Step 2: Add Chinese break translations**

Find the `attendance:` section in `zh.ts` and add a `break:` subsection after it:

```typescript
break: {
  title: "休息打卡",
  startBreak: "开始休息",
  endBreak: "结束休息",
  selectType: "选择休息类型",
  meal: "吃饭",
  bathroom: "上厕所",
  rest: "休息",
  leavePost: "中途离岗",
  duration: "时长",
  overtime: "超时",
  noActiveBreak: "没有进行中的休息",
  activeBreak: "进行中的休息",
  breakStarted: "休息已开始",
  breakEnded: "休息已结束",
  mustClockIn: "请先打卡上班再开始休息",
  alreadyOnBreak: "已有进行中的休息，请先结束",
  todayBreaks: "今日休息记录",
  type: "类型",
  start: "开始时间",
  end: "结束时间",
  note: "备注",
  policies: "休息规则",
  maxMinutes: "最大分钟数（0 = 不限制）",
  savePolicies: "保存规则",
  policiesSaved: "休息规则已保存",
  monthlyReport: "月度休息报告",
  downloadReport: "下载报告",
  minutes: "分钟",
  hours: "小时",
},
```

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/i18n/en.ts frontend/src/i18n/zh.ts
git commit -m "feat: add break tracking i18n translations (en + zh)"
```

---

## Task 14: Frontend — Break Section in AttendanceView

**Files:**
- Modify: `frontend/src/views/AttendanceView.vue`

- [ ] **Step 1: Add break state and API calls**

Add to the `<script setup>` section of `AttendanceView.vue`:

```typescript
import { onUnmounted } from 'vue'
import { breakAPI } from '../api/client'

// Break state
const activeBreak = ref<Record<string, unknown> | null>(null)
const todayBreaks = ref<Record<string, unknown>[]>([])
const breakLoading = ref(false)
const breakTimer = ref('')
let breakInterval: ReturnType<typeof setInterval> | null = null

const breakTypes = [
  { key: 'meal', icon: '🍽' },
  { key: 'bathroom', icon: '🚻' },
  { key: 'rest', icon: '😌' },
  { key: 'leave_post', icon: '🚪' },
]

async function loadBreakState() {
  try {
    const res = await breakAPI.getActiveBreak() as { data?: Record<string, unknown> }
    activeBreak.value = (res.data || res) as Record<string, unknown> | null
    if (activeBreak.value) {
      startBreakTimer()
    }
  } catch {
    activeBreak.value = null
  }
  try {
    const res = await breakAPI.listBreaks() as { data?: Record<string, unknown>[] }
    todayBreaks.value = (res.data || res || []) as Record<string, unknown>[]
  } catch {
    todayBreaks.value = []
  }
}

async function startBreak(breakType: string) {
  breakLoading.value = true
  try {
    await breakAPI.startBreak({ break_type: breakType })
    message.success(t('break.breakStarted'))
    await loadBreakState()
  } catch {
    message.error(t('break.mustClockIn'))
  } finally {
    breakLoading.value = false
  }
}

async function endBreak() {
  breakLoading.value = true
  try {
    await breakAPI.endBreak()
    message.success(t('break.breakEnded'))
    stopBreakTimer()
    await loadBreakState()
  } catch {
    message.error(t('common.failed'))
  } finally {
    breakLoading.value = false
  }
}

function startBreakTimer() {
  stopBreakTimer()
  updateBreakTimer()
  breakInterval = setInterval(updateBreakTimer, 1000)
}

function stopBreakTimer() {
  if (breakInterval) {
    clearInterval(breakInterval)
    breakInterval = null
  }
  breakTimer.value = ''
}

function updateBreakTimer() {
  if (!activeBreak.value?.start_at) return
  const start = new Date(activeBreak.value.start_at as string).getTime()
  const elapsed = Math.floor((Date.now() - start) / 1000)
  const mins = Math.floor(elapsed / 60)
  const secs = elapsed % 60
  breakTimer.value = `${mins}:${secs.toString().padStart(2, '0')}`
}

// Cleanup timer on unmount
onUnmounted(() => stopBreakTimer())

const breakColumns: DataTableColumns = [
  { title: () => t('break.type'), key: 'break_type', width: 100,
    render: (row) => t('break.' + (row.break_type === 'leave_post' ? 'leavePost' : row.break_type as string)) },
  { title: () => t('break.start'), key: 'start_at', width: 80,
    render: (row) => fmtTime(row.start_at) },
  { title: () => t('break.end'), key: 'end_at', width: 80,
    render: (row) => fmtTime(row.end_at) },
  { title: () => t('break.duration'), key: 'duration_minutes', width: 80,
    render: (row) => row.duration_minutes ? `${row.duration_minutes} ${t('break.minutes')}` : '-' },
  { title: () => t('break.overtime'), key: 'overtime_minutes', width: 80,
    render: (row) => {
      const ot = row.overtime_minutes as number
      return ot > 0 ? h(NTag, { type: 'error', size: 'small' }, () => `${ot} ${t('break.minutes')}`) : '-'
    }},
]
```

Add to `onMounted`:
```typescript
onMounted(async () => {
  // ... existing clock-in state loading ...
  if (clockedIn.value) {
    loadBreakState()
  }
})
```

Also call `loadBreakState()` after successful clock-in.

- [ ] **Step 2: Add break UI to template**

Add after the existing clock-in/out section in the template:

```vue
<!-- Break Tracking (visible when clocked in) -->
<NCard v-if="clockedIn" :title="t('break.title')" style="margin-top: 16px;">
  <!-- Active break display -->
  <div v-if="activeBreak" style="text-align: center; margin-bottom: 16px;">
    <NTag type="warning" size="large">
      {{ t('break.activeBreak') }}: {{ t('break.' + (activeBreak.break_type === 'leave_post' ? 'leavePost' : activeBreak.break_type)) }}
    </NTag>
    <div style="font-size: 32px; font-weight: bold; margin: 8px 0;">
      {{ breakTimer }}
    </div>
    <NButton type="error" :loading="breakLoading" @click="endBreak">
      {{ t('break.endBreak') }}
    </NButton>
  </div>

  <!-- Break type selection (when no active break) -->
  <NSpace v-else justify="center" style="margin-bottom: 16px;">
    <NButton
      v-for="bt in breakTypes"
      :key="bt.key"
      :loading="breakLoading"
      @click="startBreak(bt.key)"
    >
      {{ bt.icon }} {{ t('break.' + (bt.key === 'leave_post' ? 'leavePost' : bt.key)) }}
    </NButton>
  </NSpace>

  <!-- Today's break log -->
  <NDataTable
    v-if="todayBreaks.length > 0"
    :columns="breakColumns"
    :data="todayBreaks"
    :row-key="(row: any) => row.id"
    size="small"
    style="margin-top: 8px;"
  />
  <div v-else style="text-align: center; color: #999; padding: 16px;">
    {{ t('break.noActiveBreak') }}
  </div>
</NCard>
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/views/AttendanceView.vue
git commit -m "feat: add break tracking UI to attendance page"
```

---

## Task 15: Frontend — Admin Break Policy Settings

**Files:**
- Modify: `frontend/src/views/SettingsView.vue`

- [ ] **Step 1: Add break policy section to settings page**

Add to SettingsView.vue `<script setup>`:

```typescript
import { breakAPI } from '../api/client'

// Break policies
const breakPolicies = ref([
  { break_type: 'meal', max_minutes: 30 },
  { break_type: 'bathroom', max_minutes: 5 },
  { break_type: 'rest', max_minutes: 0 },
  { break_type: 'leave_post', max_minutes: 0 },
])
const policyLoading = ref(false)

async function loadBreakPolicies() {
  try {
    const res = await breakAPI.listPolicies() as { data?: Array<{ break_type: string; max_minutes: number }> }
    const data = (res.data || res) as Array<{ break_type: string; max_minutes: number }>
    if (Array.isArray(data) && data.length > 0) {
      for (const policy of data) {
        const existing = breakPolicies.value.find(p => p.break_type === policy.break_type)
        if (existing) {
          existing.max_minutes = policy.max_minutes
        }
      }
    }
  } catch {
    console.error('Failed to load break policies')
  }
}

async function saveBreakPolicies() {
  policyLoading.value = true
  try {
    await breakAPI.upsertPolicies({ policies: breakPolicies.value })
    message.success(t('break.policiesSaved'))
  } catch {
    message.error(t('common.saveFailed'))
  } finally {
    policyLoading.value = false
  }
}

onMounted(async () => {
  // ... existing onMounted logic ...
  if (authStore.isAdmin) {
    loadBreakPolicies()
  }
})
```

Add to the template (admin-only section):

```vue
<!-- Break Policies (Admin only) -->
<NCard v-if="authStore.isAdmin" :title="t('break.policies')" style="margin-top: 16px;">
  <NForm label-placement="left" label-width="160px">
    <NFormItem v-for="policy in breakPolicies" :key="policy.break_type"
      :label="t('break.' + (policy.break_type === 'leave_post' ? 'leavePost' : policy.break_type))">
      <NInputNumber
        v-model:value="policy.max_minutes"
        :min="0"
        :placeholder="t('break.maxMinutes')"
        style="width: 200px;"
      />
      <span style="margin-left: 8px; color: #999;">{{ t('break.minutes') }}</span>
    </NFormItem>
  </NForm>
  <template #footer>
    <NSpace justify="end">
      <NButton type="primary" :loading="policyLoading" @click="saveBreakPolicies">
        {{ t('break.savePolicies') }}
      </NButton>
    </NSpace>
  </template>
</NCard>
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/views/SettingsView.vue
git commit -m "feat: add break policy settings for admin"
```

---

## Task 16: Frontend — Monthly Report Download

**Files:**
- Modify: `frontend/src/views/AttendanceView.vue` (or a separate report section)

- [ ] **Step 1: Add report download section**

Add to AttendanceView.vue `<script setup>` (visible for managers):

```typescript
const reportYear = ref(new Date().getFullYear())
const reportMonth = ref(new Date().getMonth() + 1)
const reportLoading = ref(false)

async function downloadMonthlyReport() {
  reportLoading.value = true
  try {
    const res = await breakAPI.downloadMonthlyReport(reportYear.value, reportMonth.value)
    const blob = new Blob([res as BlobPart], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `monthly_break_report_${reportYear.value}_${String(reportMonth.value).padStart(2, '0')}.xlsx`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    message.error(t('common.failed'))
  } finally {
    reportLoading.value = false
  }
}
```

Add to template (after break section, visible for managers):

```vue
<!-- Monthly Report (Manager+) -->
<NCard v-if="authStore.isManager" :title="t('break.monthlyReport')" style="margin-top: 16px;">
  <NSpace>
    <NInputNumber v-model:value="reportYear" :min="2020" :max="2100" />
    <NInputNumber v-model:value="reportMonth" :min="1" :max="12" />
    <NButton type="primary" :loading="reportLoading" @click="downloadMonthlyReport">
      {{ t('break.downloadReport') }}
    </NButton>
  </NSpace>
</NCard>
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add frontend/src/views/AttendanceView.vue
git commit -m "feat: add monthly break report download for managers"
```

---

## Task 17: CLI — Break Commands

**Files:**
- Create: `cmd/hr_break.go` (in halaos-cli)
- Create: `cmd/hr_break_test.go` (in halaos-cli)

- [ ] **Step 1: Write failing tests**

`/Users/anna/Documents/halaos-cli/cmd/hr_break_test.go`:
```go
package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestBreakStart(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"POST /api/v1/attendance/breaks/start": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, map[string]any{"id": 1, "break_type": "meal", "start_at": "2026-03-28T14:00:00Z"})
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()
	outputFormat = ""

	breakStartCmd.Flags().Set("type", "meal")
	defer breakStartCmd.Flags().Set("type", "")

	out := captureStdout(func() {
		err := breakStartCmd.RunE(breakStartCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "Break started") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestBreakEnd(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"POST /api/v1/attendance/breaks/end": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, map[string]any{
				"id": 1, "break_type": "meal", "duration_minutes": 28, "overtime_minutes": 0,
			})
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()

	out := captureStdout(func() {
		err := breakEndCmd.RunE(breakEndCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "Break ended") {
		t.Errorf("expected success message, got: %s", out)
	}
	if !strings.Contains(out, "28") {
		t.Errorf("expected duration in output, got: %s", out)
	}
}

func TestBreakList(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"GET /api/v1/attendance/breaks": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []map[string]any{
				{"id": 1, "break_type": "meal", "start_at": "2026-03-28T14:00:00Z",
					"end_at": "2026-03-28T14:28:00Z", "duration_minutes": 28, "overtime_minutes": 0},
			})
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()
	outputFormat = ""

	out := captureStdout(func() {
		err := breakListCmd.RunE(breakListCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "meal") {
		t.Errorf("expected meal break type, got: %s", out)
	}
}

func TestBreakActive(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"GET /api/v1/attendance/breaks/active": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, map[string]any{
				"id": 1, "break_type": "bathroom", "start_at": "2026-03-28T14:00:00Z",
			})
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()

	out := captureStdout(func() {
		err := breakActiveCmd.RunE(breakActiveCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "bathroom") {
		t.Errorf("expected bathroom break type, got: %s", out)
	}
}

func TestBreakActive_None(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"GET /api/v1/attendance/breaks/active": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, nil)
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()

	out := captureStdout(func() {
		err := breakActiveCmd.RunE(breakActiveCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "No active break") {
		t.Errorf("expected no active break message, got: %s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/halaos-cli && go test ./cmd/ -run TestBreak -v`
Expected: FAIL (commands don't exist yet)

- [ ] **Step 3: Implement break CLI commands**

`/Users/anna/Documents/halaos-cli/cmd/hr_break.go`:
```go
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tonypk/halaos-cli/internal/output"
)

func init() {
	hrCmd.AddCommand(breakCmd)
	breakCmd.AddCommand(breakStartCmd)
	breakCmd.AddCommand(breakEndCmd)
	breakCmd.AddCommand(breakListCmd)
	breakCmd.AddCommand(breakActiveCmd)

	breakStartCmd.Flags().String("type", "", "Break type: meal, bathroom, rest, leave_post")
	breakStartCmd.Flags().String("note", "", "Optional note")
	breakStartCmd.MarkFlagRequired("type")

	breakListCmd.Flags().String("date", "", "Date (YYYY-MM-DD), defaults to today")
}

var breakCmd = &cobra.Command{
	Use:   "break",
	Short: "Break tracking",
}

var breakStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a break",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		breakType, _ := cmd.Flags().GetString("type")
		note, _ := cmd.Flags().GetString("note")

		body := map[string]any{"break_type": breakType}
		if note != "" {
			body["note"] = note
		}

		data, err := c.Post("/api/v1/attendance/breaks/start", body)
		if err != nil {
			return err
		}

		format := getOutputFormat()
		if format == "json" {
			output.Print(format, nil, nil, data)
			return nil
		}

		var result struct {
			BreakType string `json:"break_type"`
			StartAt   string `json:"start_at"`
		}
		json.Unmarshal(data, &result)
		fmt.Printf("Break started: %s at %s\n", result.BreakType, result.StartAt)
		return nil
	},
}

var breakEndCmd = &cobra.Command{
	Use:   "end",
	Short: "End current break",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		data, err := c.Post("/api/v1/attendance/breaks/end", map[string]any{})
		if err != nil {
			return err
		}

		format := getOutputFormat()
		if format == "json" {
			output.Print(format, nil, nil, data)
			return nil
		}

		var result struct {
			BreakType       string `json:"break_type"`
			DurationMinutes *int   `json:"duration_minutes"`
			OvertimeMinutes *int   `json:"overtime_minutes"`
		}
		json.Unmarshal(data, &result)

		dur := 0
		if result.DurationMinutes != nil {
			dur = *result.DurationMinutes
		}
		ot := 0
		if result.OvertimeMinutes != nil {
			ot = *result.OvertimeMinutes
		}

		fmt.Printf("Break ended: %s\nDuration: %d minutes\n", result.BreakType, dur)
		if ot > 0 {
			fmt.Printf("Overtime: %d minutes\n", ot)
		}
		return nil
	},
}

var breakListCmd = &cobra.Command{
	Use:   "list",
	Short: "List today's breaks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		q := map[string]string{}
		if v, _ := cmd.Flags().GetString("date"); v != "" {
			q["date"] = v
		}

		data, err := c.Get("/api/v1/attendance/breaks", q)
		if err != nil {
			return err
		}

		format := getOutputFormat()
		if format == "json" {
			output.Print(format, nil, nil, data)
			return nil
		}

		var breaks []struct {
			BreakType       string `json:"break_type"`
			StartAt         string `json:"start_at"`
			EndAt           *string `json:"end_at"`
			DurationMinutes *int   `json:"duration_minutes"`
			OvertimeMinutes *int   `json:"overtime_minutes"`
		}
		json.Unmarshal(data, &breaks)

		headers := []string{"TYPE", "START", "END", "DURATION", "OVERTIME"}
		var rows [][]string
		for _, b := range breaks {
			endAt := "-"
			if b.EndAt != nil {
				endAt = *b.EndAt
			}
			dur := "-"
			if b.DurationMinutes != nil {
				dur = fmt.Sprintf("%d min", *b.DurationMinutes)
			}
			ot := "-"
			if b.OvertimeMinutes != nil && *b.OvertimeMinutes > 0 {
				ot = fmt.Sprintf("%d min", *b.OvertimeMinutes)
			}
			rows = append(rows, []string{b.BreakType, b.StartAt, endAt, dur, ot})
		}

		output.Print(format, headers, rows, data)
		return nil
	},
}

var breakActiveCmd = &cobra.Command{
	Use:   "active",
	Short: "Check active break",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		data, err := c.Get("/api/v1/attendance/breaks/active", nil)
		if err != nil {
			return err
		}

		format := getOutputFormat()
		if format == "json" {
			output.Print(format, nil, nil, data)
			return nil
		}

		if string(data) == "null" || len(data) == 0 {
			fmt.Println("No active break.")
			return nil
		}

		var result struct {
			BreakType string `json:"break_type"`
			StartAt   string `json:"start_at"`
		}
		json.Unmarshal(data, &result)
		fmt.Printf("Active break: %s (started at %s)\n", result.BreakType, result.StartAt)
		return nil
	},
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/anna/Documents/halaos-cli && go test ./cmd/ -run TestBreak -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/halaos-cli
git add cmd/hr_break.go cmd/hr_break_test.go
git commit -m "feat: add break start/end/list/active CLI commands"
```

---

## Task 18: CLI — Break Policy and Report Commands

**Files:**
- Create: `cmd/hr_break_policy.go` (in halaos-cli)
- Create: `cmd/hr_report.go` (in halaos-cli)
- Create: `cmd/hr_break_policy_test.go`
- Create: `cmd/hr_report_test.go`

- [ ] **Step 1: Write failing tests for break policy CLI**

`/Users/anna/Documents/halaos-cli/cmd/hr_break_policy_test.go`:
```go
package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestBreakPolicyList(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"GET /api/v1/attendance/break-policies": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []map[string]any{
				{"break_type": "meal", "max_minutes": 30},
				{"break_type": "bathroom", "max_minutes": 5},
			})
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()
	outputFormat = ""

	out := captureStdout(func() {
		err := breakPolicyListCmd.RunE(breakPolicyListCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "meal") {
		t.Errorf("expected meal policy, got: %s", out)
	}
	if !strings.Contains(out, "30") {
		t.Errorf("expected 30 min, got: %s", out)
	}
}

func TestBreakPolicySet(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"PUT /api/v1/attendance/break-policies": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			policies := body["policies"].([]any)
			if len(policies) != 1 {
				t.Errorf("expected 1 policy, got %d", len(policies))
			}
			jsonOK(w, []map[string]any{{"break_type": "meal", "max_minutes": 45}})
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()

	breakPolicySetCmd.Flags().Set("type", "meal")
	breakPolicySetCmd.Flags().Set("max-minutes", "45")
	defer func() {
		breakPolicySetCmd.Flags().Set("type", "")
		breakPolicySetCmd.Flags().Set("max-minutes", "0")
	}()

	out := captureStdout(func() {
		err := breakPolicySetCmd.RunE(breakPolicySetCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "saved") || !strings.Contains(out, "meal") {
		t.Errorf("expected success message, got: %s", out)
	}
}
```

- [ ] **Step 2: Implement break policy CLI**

`/Users/anna/Documents/halaos-cli/cmd/hr_break_policy.go`:
```go
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tonypk/halaos-cli/internal/output"
)

func init() {
	hrCmd.AddCommand(breakPolicyCmd)
	breakPolicyCmd.AddCommand(breakPolicyListCmd)
	breakPolicyCmd.AddCommand(breakPolicySetCmd)

	breakPolicySetCmd.Flags().String("type", "", "Break type: meal, bathroom, rest, leave_post")
	breakPolicySetCmd.Flags().Int("max-minutes", 0, "Max minutes (0 = no limit)")
	breakPolicySetCmd.MarkFlagRequired("type")
}

var breakPolicyCmd = &cobra.Command{
	Use:   "break-policy",
	Short: "Break policy management",
}

var breakPolicyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List break policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		data, err := c.Get("/api/v1/attendance/break-policies", nil)
		if err != nil {
			return err
		}

		format := getOutputFormat()
		if format == "json" {
			output.Print(format, nil, nil, data)
			return nil
		}

		var policies []struct {
			BreakType  string `json:"break_type"`
			MaxMinutes int    `json:"max_minutes"`
			IsActive   bool   `json:"is_active"`
		}
		json.Unmarshal(data, &policies)

		headers := []string{"TYPE", "MAX MINUTES", "ACTIVE"}
		var rows [][]string
		for _, p := range policies {
			maxStr := "no limit"
			if p.MaxMinutes > 0 {
				maxStr = fmt.Sprintf("%d", p.MaxMinutes)
			}
			rows = append(rows, []string{p.BreakType, maxStr, fmt.Sprintf("%v", p.IsActive)})
		}

		output.Print(format, headers, rows, data)
		return nil
	},
}

var breakPolicySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set break policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		breakType, _ := cmd.Flags().GetString("type")
		maxMinutes, _ := cmd.Flags().GetInt("max-minutes")

		body := map[string]any{
			"policies": []map[string]any{
				{"break_type": breakType, "max_minutes": maxMinutes},
			},
		}

		_, err := c.Put("/api/v1/attendance/break-policies", body)
		if err != nil {
			return err
		}

		fmt.Printf("Break policy saved: %s (max: %d minutes)\n", breakType, maxMinutes)
		return nil
	},
}
```

- [ ] **Step 3: Write report CLI test and implementation**

`/Users/anna/Documents/halaos-cli/cmd/hr_report_test.go`:
```go
package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestReportMonthly(t *testing.T) {
	srv := mockAPI(map[string]http.HandlerFunc{
		"GET /api/v1/attendance/report/monthly": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("year") != "2026" {
				t.Errorf("expected year=2026, got %s", r.URL.Query().Get("year"))
			}
			if r.URL.Query().Get("month") != "2" {
				t.Errorf("expected month=2, got %s", r.URL.Query().Get("month"))
			}
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Write([]byte("fake-xlsx-content"))
		},
	})
	defer srv.Close()
	cleanup := setupTestConfig(srv.URL, srv.URL)
	defer cleanup()

	reportMonthlyCmd.Flags().Set("year", "2026")
	reportMonthlyCmd.Flags().Set("month", "2")
	reportMonthlyCmd.Flags().Set("out", "/tmp/test_report.xlsx")
	defer func() {
		reportMonthlyCmd.Flags().Set("year", "0")
		reportMonthlyCmd.Flags().Set("month", "0")
		reportMonthlyCmd.Flags().Set("out", "")
	}()

	out := captureStdout(func() {
		err := reportMonthlyCmd.RunE(reportMonthlyCmd, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "Report saved") {
		t.Errorf("expected success message, got: %s", out)
	}
}
```

`/Users/anna/Documents/halaos-cli/cmd/hr_report.go`:
```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	hrCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(reportMonthlyCmd)

	reportMonthlyCmd.Flags().Int("year", 0, "Report year")
	reportMonthlyCmd.Flags().Int("month", 0, "Report month (1-12)")
	reportMonthlyCmd.Flags().String("out", "", "Output file path (default: report_YYYY_MM.xlsx)")
	reportMonthlyCmd.MarkFlagRequired("year")
	reportMonthlyCmd.MarkFlagRequired("month")
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Report generation",
}

var reportMonthlyCmd = &cobra.Command{
	Use:   "monthly",
	Short: "Generate monthly break report",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := hrClient()
		year, _ := cmd.Flags().GetInt("year")
		month, _ := cmd.Flags().GetInt("month")
		outPath, _ := cmd.Flags().GetString("out")

		if outPath == "" {
			outPath = fmt.Sprintf("report_%d_%02d.xlsx", year, month)
		}

		q := map[string]string{
			"year":  fmt.Sprintf("%d", year),
			"month": fmt.Sprintf("%d", month),
		}

		err := c.Download("/api/v1/attendance/report/monthly", q, outPath)
		if err != nil {
			return err
		}

		fmt.Printf("Report saved to %s\n", outPath)
		return nil
	},
}
```

- [ ] **Step 4: Run all CLI tests**

Run: `cd /Users/anna/Documents/halaos-cli && go test ./cmd/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/halaos-cli
git add cmd/hr_break_policy.go cmd/hr_break_policy_test.go cmd/hr_report.go cmd/hr_report_test.go
git commit -m "feat: add break-policy and report monthly CLI commands"
```

---

## Task 19: Backend — Full Test Coverage Pass

**Files:**
- Modify: `internal/breaks/handler_test.go`
- Modify: `internal/breaks/handler_policy_test.go`
- Modify: `internal/breaks/handler_report_test.go`

- [ ] **Step 1: Run coverage check**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -coverprofile=/tmp/breaks-coverage.out -covermode=atomic`
Run: `go tool cover -func=/tmp/breaks-coverage.out | grep total`

Target: 80%+ coverage for breaks package.

- [ ] **Step 2: Add missing tests to reach 80%+ coverage**

Based on coverage output, add tests for uncovered paths:
- `ListBreaks` with valid date parameter
- `ListBreaks` with invalid date parameter
- `EndBreak` success path (requires mock DB to return active break + end successfully)
- `GetActiveBreak` with active break
- `MonthlyReport` with valid params but empty data
- Any uncovered error paths

- [ ] **Step 3: Re-run coverage**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/breaks/ -coverprofile=/tmp/breaks-coverage.out -covermode=atomic && go tool cover -func=/tmp/breaks-coverage.out | grep total`
Expected: 80%+ coverage.

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/breaks/
git commit -m "test: improve break handler test coverage to 80%+"
```

---

## Task 20: Integration Verification

- [ ] **Step 1: Run full backend test suite**

Run: `cd /Users/anna/Documents/aigonhr && go test ./... -count=1`
Expected: All tests pass.

- [ ] **Step 2: Run full CLI test suite**

Run: `cd /Users/anna/Documents/halaos-cli && go test ./... -count=1`
Expected: All tests pass.

- [ ] **Step 3: Build backend binary**

Run: `cd /Users/anna/Documents/aigonhr && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api ./cmd/api`
Expected: Build succeeds.

- [ ] **Step 4: Build frontend**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 5: Run CLI build**

Run: `cd /Users/anna/Documents/halaos-cli && go build -o bin/halaos ./`
Expected: Build succeeds.

- [ ] **Step 6: Final commit (if any remaining changes)**

```bash
cd /Users/anna/Documents/aigonhr
git status
# If clean, this step is a no-op
```
