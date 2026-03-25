# Onboarding Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add guided onboarding checklists for H5 employees and Desktop admins, plus action-oriented empty states across both frontends.

**Architecture:** New `onboarding_progress` table with JSONB step tracking. New `internal/onboarding_checklist/` handler package (separate from existing `internal/onboarding/` HR workflows). Reusable `EmptyState.vue` components for both NaiveUI (Desktop) and Vant (H5).

**Tech Stack:** Go 1.25 (Gin+sqlc+pgx), PostgreSQL 16, Vue 3 + TypeScript, NaiveUI (Desktop), Vant (H5 Mobile)

**Spec:** `docs/superpowers/specs/2026-03-25-onboarding-redesign-design.md`

---

## File Structure

### Backend (new files)
| File | Responsibility |
|------|---------------|
| `db/migrations/00085_onboarding_checklist.sql` | Create `onboarding_progress` table |
| `db/query/onboarding_checklist.sql` | sqlc queries for CRUD + auto-detect |
| `internal/onboarding_checklist/handler.go` | HTTP handlers (get progress, complete step, dismiss) |
| `internal/onboarding_checklist/routes.go` | Route registration |
| `internal/onboarding_checklist/handler_test.go` | Unit tests |

### Backend (modify)
| File | Change |
|------|--------|
| `internal/app/bootstrap.go` | Wire new handler |

### H5 Mobile Frontend (new files)
| File | Responsibility |
|------|---------------|
| `frontend-mobile/src/components/OnboardingChecklist.vue` | 5-step checklist card |
| `frontend-mobile/src/components/EmptyState.vue` | Reusable empty state |

### H5 Mobile Frontend (modify)
| File | Change |
|------|--------|
| `frontend-mobile/src/api/client.ts` | Add `onboardingChecklistAPI` |
| `frontend-mobile/src/views/HomeView.vue` | Mount OnboardingChecklist |
| `frontend-mobile/src/views/LeaveView.vue` | Add onboarding step trigger + EmptyState |
| `frontend-mobile/src/views/PayslipsView.vue` | Add onboarding step trigger + EmptyState |
| `frontend-mobile/src/views/AttendanceView.vue` | Replace empty state with EmptyState |
| `frontend-mobile/src/views/NotificationsView.vue` | Replace empty state with EmptyState |
| `frontend-mobile/src/i18n/en.ts` | Add onboarding + empty state keys |
| `frontend-mobile/src/i18n/zh.ts` | Add onboarding + empty state keys |

### Desktop Frontend (new files)
| File | Responsibility |
|------|---------------|
| `frontend/src/components/GettingStartedChecklist.vue` | 7-step admin checklist card |
| `frontend/src/components/EmptyState.vue` | Reusable empty state |

### Desktop Frontend (modify)
| File | Change |
|------|--------|
| `frontend/src/api/client.ts` | Add `onboardingChecklistAPI` |
| `frontend/src/views/DashboardView.vue` | Replace "Get Started" card |
| `frontend/src/i18n/en.ts` | Add onboarding + empty state keys |
| `frontend/src/i18n/zh.ts` | Add onboarding + empty state keys |
| ~15 view files | Replace bare `NEmpty` with `EmptyState` |

---

## Task 1: Database Migration

**Files:**
- Create: `db/migrations/00085_onboarding_checklist.sql`

- [ ] **Step 1: Create migration file**

```sql
-- +goose Up
CREATE TABLE onboarding_progress (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona      VARCHAR(20) NOT NULL DEFAULT 'employee',
    steps        JSONB NOT NULL DEFAULT '{}',
    dismissed    BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, user_id, persona)
);

CREATE INDEX idx_onboarding_progress_company ON onboarding_progress(company_id);

-- +goose Down
DROP TABLE IF EXISTS onboarding_progress;
```

- [ ] **Step 2: Commit**

```bash
git add db/migrations/00085_onboarding_checklist.sql
git commit -m "feat(onboarding-checklist): add migration 00085"
```

---

## Task 2: sqlc Queries

**Files:**
- Create: `db/query/onboarding_checklist.sql`

- [ ] **Step 1: Write sqlc queries**

```sql
-- name: GetOnboardingChecklist :one
SELECT * FROM onboarding_progress
WHERE company_id = $1 AND user_id = $2 AND persona = $3;

-- name: UpsertOnboardingChecklist :one
INSERT INTO onboarding_progress (company_id, user_id, persona, steps)
VALUES ($1, $2, $3, $4)
ON CONFLICT (company_id, user_id, persona)
DO UPDATE SET steps = $4, updated_at = NOW()
RETURNING *;

-- name: DismissOnboardingChecklist :exec
INSERT INTO onboarding_progress (company_id, user_id, persona, steps, dismissed)
VALUES ($1, $2, $3, '{}', TRUE)
ON CONFLICT (company_id, user_id, persona)
DO UPDATE SET dismissed = TRUE, updated_at = NOW();

-- name: CompleteOnboardingChecklist :exec
UPDATE onboarding_progress
SET completed_at = NOW(), updated_at = NOW()
WHERE company_id = $1 AND user_id = $2 AND persona = $3;

-- name: CheckCompanyInfoComplete :one
SELECT (legal_name IS NOT NULL AND tin IS NOT NULL)::boolean AS complete
FROM companies WHERE id = $1;

-- name: CountEmployeesByCompany :one
SELECT COUNT(*) FROM employees WHERE company_id = $1;

-- name: CountDepartmentsByCompany :one
SELECT COUNT(*) FROM departments WHERE company_id = $1;

-- name: CountPositionsByCompany :one
SELECT COUNT(*) FROM positions WHERE company_id = $1;

-- name: CountNonStatutoryLeaveTypes :one
SELECT COUNT(*) FROM leave_types WHERE company_id = $1 AND is_statutory = false;

-- name: CountScheduleTemplates :one
SELECT COUNT(*) FROM schedule_templates WHERE company_id = $1;

-- name: CountSalaryStructures :one
SELECT COUNT(*) FROM salary_structures WHERE company_id = $1;

-- name: CountCompletedPayrollRuns :one
SELECT COUNT(*) FROM payroll_runs WHERE company_id = $1 AND status = 'completed';
```

- [ ] **Step 2: Generate sqlc code**

Run: `~/go/bin/sqlc generate`
Expected: No errors, new files generated in `internal/store/`

- [ ] **Step 3: Commit**

```bash
git add db/query/onboarding_checklist.sql internal/store/
git commit -m "feat(onboarding-checklist): add sqlc queries"
```

---

## Task 3: Backend Handler + Routes + Tests

**Files:**
- Create: `internal/onboarding_checklist/handler.go`
- Create: `internal/onboarding_checklist/routes.go`
- Create: `internal/onboarding_checklist/handler_test.go`

- [ ] **Step 1: Create handler**

```go
package onboarding_checklist

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Valid step keys per persona.
var validSteps = map[string][]string{
	"employee": {"profile", "first_clock", "view_leave", "view_payslip", "ai_chat"},
	"admin":    {"company_info", "departments", "import_employees", "leave_policies", "schedules", "payroll_config", "first_payroll"},
}

type stepState struct {
	Done   bool   `json:"done"`
	DoneAt string `json:"done_at,omitempty"`
}

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// GetMyProgress returns the current user's onboarding checklist state.
func (h *Handler) GetMyProgress(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	persona := c.DefaultQuery("persona", "employee")

	row, err := h.queries.GetOnboardingChecklist(c.Request.Context(), store.GetOnboardingChecklistParams{
		CompanyID: companyID,
		UserID:    userID,
		Persona:   persona,
	})
	if err == pgx.ErrNoRows {
		// Auto-create with all steps as not done
		steps := buildInitialSteps(persona)
		stepsJSON, _ := json.Marshal(steps)
		row, err = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
			CompanyID: companyID,
			UserID:    userID,
			Persona:   persona,
			Steps:     stepsJSON,
		})
		if err != nil {
			h.logger.Error("failed to create onboarding progress", "error", err)
			response.InternalError(c, "Failed to create onboarding progress")
			return
		}
	} else if err != nil {
		h.logger.Error("failed to get onboarding progress", "error", err)
		response.InternalError(c, "Failed to get onboarding progress")
		return
	}

	// For admin persona, auto-detect step completion from DB state
	if persona == "admin" {
		steps := h.autoDetectAdminSteps(c, companyID, row.Steps)
		if steps != nil {
			stepsJSON, _ := json.Marshal(steps)
			row, _ = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
				CompanyID: companyID,
				UserID:    userID,
				Persona:   persona,
				Steps:     stepsJSON,
			})
		}
	}

	response.OK(c, row)
}

// CompleteStep marks a single step as done.
func (h *Handler) CompleteStep(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Step    string `json:"step" binding:"required"`
		Persona string `json:"persona"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Persona == "" {
		req.Persona = "employee"
	}

	// Validate step key
	keys, ok := validSteps[req.Persona]
	if !ok {
		response.BadRequest(c, "invalid persona")
		return
	}
	valid := false
	for _, k := range keys {
		if k == req.Step {
			valid = true
			break
		}
	}
	if !valid {
		response.BadRequest(c, "invalid step key")
		return
	}

	// Get or create progress
	row, err := h.queries.GetOnboardingChecklist(c.Request.Context(), store.GetOnboardingChecklistParams{
		CompanyID: companyID, UserID: userID, Persona: req.Persona,
	})
	if err == pgx.ErrNoRows {
		steps := buildInitialSteps(req.Persona)
		stepsJSON, _ := json.Marshal(steps)
		row, err = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
			CompanyID: companyID, UserID: userID, Persona: req.Persona, Steps: stepsJSON,
		})
	}
	if err != nil {
		response.InternalError(c, "Failed to get onboarding progress")
		return
	}

	// Parse steps, mark done (idempotent)
	var steps map[string]stepState
	if err := json.Unmarshal(row.Steps, &steps); err != nil {
		steps = buildInitialSteps(req.Persona)
	}
	s := steps[req.Step]
	if !s.Done {
		s.Done = true
		s.DoneAt = time.Now().UTC().Format(time.RFC3339)
		steps[req.Step] = s
	}

	stepsJSON, _ := json.Marshal(steps)
	row, err = h.queries.UpsertOnboardingChecklist(c.Request.Context(), store.UpsertOnboardingChecklistParams{
		CompanyID: companyID, UserID: userID, Persona: req.Persona, Steps: stepsJSON,
	})
	if err != nil {
		response.InternalError(c, "Failed to update step")
		return
	}

	// Check if all steps done → mark completed
	allDone := true
	if err := json.Unmarshal(row.Steps, &steps); err == nil {
		for _, k := range validSteps[req.Persona] {
			if !steps[k].Done {
				allDone = false
				break
			}
		}
	}
	if allDone && row.CompletedAt.Time.IsZero() {
		_ = h.queries.CompleteOnboardingChecklist(c.Request.Context(), store.CompleteOnboardingChecklistParams{
			CompanyID: companyID, UserID: userID, Persona: req.Persona,
		})
	}

	response.OK(c, row)
}

// Dismiss hides the checklist for the user.
func (h *Handler) Dismiss(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	persona := c.DefaultQuery("persona", "employee")

	err := h.queries.DismissOnboardingChecklist(c.Request.Context(), store.DismissOnboardingChecklistParams{
		CompanyID: companyID, UserID: userID, Persona: persona,
	})
	if err != nil {
		response.InternalError(c, "Failed to dismiss")
		return
	}
	response.OK(c, gin.H{"dismissed": true})
}

func buildInitialSteps(persona string) map[string]stepState {
	steps := make(map[string]stepState)
	for _, k := range validSteps[persona] {
		steps[k] = stepState{Done: false}
	}
	return steps
}

func (h *Handler) autoDetectAdminSteps(c *gin.Context, companyID int64, currentSteps []byte) map[string]stepState {
	var steps map[string]stepState
	if err := json.Unmarshal(currentSteps, &steps); err != nil {
		steps = buildInitialSteps("admin")
	}

	changed := false
	now := time.Now().UTC().Format(time.RFC3339)
	ctx := c.Request.Context()

	// Step 1: company_info — companies.legal_name IS NOT NULL AND tin IS NOT NULL
	infoComplete, err := h.queries.CheckCompanyInfoComplete(ctx, companyID)
	if err == nil && infoComplete && !steps["company_info"].Done {
		steps["company_info"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 2: departments — count > 0 AND positions count > 0
	deptCount, err := h.queries.CountDepartmentsByCompany(ctx, companyID)
	if err == nil && deptCount > 0 {
		posCount, err2 := h.queries.CountPositionsByCompany(ctx, companyID)
		if err2 == nil && posCount > 0 && !steps["departments"].Done {
			steps["departments"] = stepState{Done: true, DoneAt: now}
			changed = true
		}
	}

	// Step 3: import_employees — count > 1
	empCount, err := h.queries.CountEmployeesByCompany(ctx, companyID)
	if err == nil && empCount > 1 && !steps["import_employees"].Done {
		steps["import_employees"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 4: leave_policies
	ltCount, err := h.queries.CountNonStatutoryLeaveTypes(ctx, companyID)
	if err == nil && ltCount > 0 && !steps["leave_policies"].Done {
		steps["leave_policies"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 5: schedules
	stCount, err := h.queries.CountScheduleTemplates(ctx, companyID)
	if err == nil && stCount > 0 && !steps["schedules"].Done {
		steps["schedules"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 6: payroll_config
	ssCount, err := h.queries.CountSalaryStructures(ctx, companyID)
	if err == nil && ssCount > 0 && !steps["payroll_config"].Done {
		steps["payroll_config"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	// Step 7: first_payroll
	prCount, err := h.queries.CountCompletedPayrollRuns(ctx, companyID)
	if err == nil && prCount > 0 && !steps["first_payroll"].Done {
		steps["first_payroll"] = stepState{Done: true, DoneAt: now}
		changed = true
	}

	if changed {
		return steps
	}
	return nil
}
```

- [ ] **Step 2: Create routes**

```go
package onboarding_checklist

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/onboarding-checklist/my-progress", h.GetMyProgress)
	protected.POST("/onboarding-checklist/complete-step", h.CompleteStep)
	protected.POST("/onboarding-checklist/dismiss", h.Dismiss)
}
```

- [ ] **Step 3: Write tests**

```go
package onboarding_checklist

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

var employeeAuth = testutil.DefaultEmployee
var adminAuth = testutil.DefaultAdmin

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger)
}

// onboardingRow builds a mock row matching onboarding_progress columns:
// id, company_id, user_id, persona, steps, dismissed, completed_at, created_at, updated_at
func onboardingRow(id, companyID, userID int64, persona string, steps []byte, dismissed bool) []interface{} {
	return []interface{}{
		id, companyID, userID, persona, steps, dismissed,
		pgtype.Timestamptz{}, // completed_at (null)
		pgtype.Timestamptz{Valid: true}, // created_at
		pgtype.Timestamptz{Valid: true}, // updated_at
	}
}

func TestGetMyProgress_NewUser(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// First QueryRow returns pgx.ErrNoRows (GetOnboardingChecklist finds no row)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// Second QueryRow returns the newly upserted row
	mockDB.OnQueryRow(testutil.NewRow(onboardingRow(
		1, 1, 10, "employee",
		[]byte(`{"profile":{"done":false},"first_clock":{"done":false},"view_leave":{"done":false},"view_payslip":{"done":false},"ai_chat":{"done":false}}`),
		false,
	)...))
	c, w := testutil.NewGinContextWithQuery("GET", "/onboarding-checklist/my-progress",
		url.Values{"persona": {"employee"}}, employeeAuth)
	h.GetMyProgress(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCompleteStep_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// Get existing progress (QueryRow for GetOnboardingChecklist)
	mockDB.OnQueryRow(testutil.NewRow(onboardingRow(
		1, 1, 10, "employee",
		[]byte(`{"profile":{"done":false},"first_clock":{"done":false},"view_leave":{"done":false},"view_payslip":{"done":false},"ai_chat":{"done":false}}`),
		false,
	)...))
	// Upsert updated steps (QueryRow for UpsertOnboardingChecklist)
	mockDB.OnQueryRow(testutil.NewRow(onboardingRow(
		1, 1, 10, "employee",
		[]byte(`{"profile":{"done":true},"first_clock":{"done":false},"view_leave":{"done":false},"view_payslip":{"done":false},"ai_chat":{"done":false}}`),
		false,
	)...))
	c, w := testutil.NewGinContext("POST", "/onboarding-checklist/complete-step",
		gin.H{"step": "profile", "persona": "employee"}, employeeAuth)
	h.CompleteStep(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCompleteStep_InvalidKey(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/onboarding-checklist/complete-step",
		gin.H{"step": "nonexistent", "persona": "employee"}, employeeAuth)
	h.CompleteStep(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDismiss_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// DismissOnboardingChecklist is :exec → uses Exec
	mockDB.OnExecSuccess()
	c, w := testutil.NewGinContextWithQuery("POST", "/onboarding-checklist/dismiss",
		url.Values{"persona": {"employee"}}, employeeAuth)
	h.Dismiss(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Note:** The `onboardingRow` helper returns column values matching the exact scan order of `SELECT * FROM onboarding_progress`. After running `sqlc generate`, check the generated struct field order and adjust if needed (e.g., `pgtype.Timestamptz` vs `time.Time` for `created_at`/`updated_at`).

- [ ] **Step 4: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/onboarding_checklist/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/onboarding_checklist/
git commit -m "feat(onboarding-checklist): add handler, routes, and tests"
```

---

## Task 4: Wire Handler into Bootstrap

**Files:**
- Modify: `internal/app/bootstrap.go`

- [ ] **Step 1: Add import and handler creation**

In `bootstrap.go`, add alongside existing handler creation (near line 256):
```go
import onboardingChecklist "github.com/tonypk/aigonhr/internal/onboarding_checklist"
```

Create handler (near line 256, after `onboardingHandler`):
```go
onboardingChecklistHandler := onboardingChecklist.NewHandler(a.Queries, a.Pool, a.Logger)
```

Register routes (near line 359, after `onboardingHandler.RegisterRoutes(protected)`):
```go
onboardingChecklistHandler.RegisterRoutes(protected)
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./cmd/api`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add internal/app/bootstrap.go
git commit -m "feat(onboarding-checklist): wire handler into bootstrap"
```

---

## Task 5: H5 Mobile — API Client + OnboardingChecklist Component

**Files:**
- Modify: `frontend-mobile/src/api/client.ts`
- Create: `frontend-mobile/src/components/OnboardingChecklist.vue`
- Modify: `frontend-mobile/src/i18n/en.ts`
- Modify: `frontend-mobile/src/i18n/zh.ts`

- [ ] **Step 1: Add API client**

In `frontend-mobile/src/api/client.ts`, add after existing exports:
```typescript
export const onboardingChecklistAPI = {
  getProgress: (persona = "employee") =>
    get("/v1/onboarding-checklist/my-progress", { persona }),
  completeStep: (step: string, persona = "employee") =>
    post("/v1/onboarding-checklist/complete-step", { step, persona }),
  dismiss: (persona = "employee") =>
    post("/v1/onboarding-checklist/dismiss", { persona }),
};
```

- [ ] **Step 2: Add i18n keys**

In `frontend-mobile/src/i18n/en.ts`, add `onboarding` section:
```typescript
onboarding: {
  welcome: 'Welcome, {name}!',
  completeSteps: 'Complete these steps to get started',
  skip: 'Skip',
  completed: '{n} / {total} completed',
  steps: {
    profile: 'Complete your profile',
    profileDesc: 'Name, photo, emergency contact',
    firstClock: 'Clock in for the first time',
    firstClockDesc: 'Tap the clock button to record attendance',
    viewLeave: 'Learn how to request leave',
    viewLeaveDesc: 'View leave balance and file a request',
    viewPayslip: 'Check your payslip',
    viewPayslipDesc: 'View salary breakdown and deductions',
    aiChat: 'Meet the AI Assistant',
    aiChatDesc: 'Ask questions about HR policies anytime',
  },
  allDone: 'All done! You\'re all set.',
},
```

In `frontend-mobile/src/i18n/zh.ts`, add corresponding Chinese keys:
```typescript
onboarding: {
  welcome: '欢迎，{name}！',
  completeSteps: '完成以下步骤开始使用',
  skip: '跳过',
  completed: '{n} / {total} 已完成',
  steps: {
    profile: '完善个人资料',
    profileDesc: '姓名、头像、紧急联系人',
    firstClock: '首次打卡',
    firstClockDesc: '点击打卡按钮记录考勤',
    viewLeave: '了解请假流程',
    viewLeaveDesc: '查看假期余额并提交请假申请',
    viewPayslip: '查看工资单',
    viewPayslipDesc: '查看薪资明细和扣款',
    aiChat: '认识AI助手',
    aiChatDesc: '随时咨询HR政策问题',
  },
  allDone: '全部完成！准备就绪。',
},
```

- [ ] **Step 3: Create OnboardingChecklist component**

Create `frontend-mobile/src/components/OnboardingChecklist.vue`:
```vue
<template>
  <div v-if="visible" class="onboarding-wrap">
    <!-- Welcome header -->
    <div class="onboarding-header">
      <div class="onboarding-skip" @click="handleDismiss">{{ t('onboarding.skip') }} ×</div>
      <div class="onboarding-title">{{ t('onboarding.welcome', { name: firstName }) }}</div>
      <div class="onboarding-subtitle">{{ t('onboarding.completeSteps') }}</div>
      <div class="onboarding-progress">
        <div class="onboarding-progress-bar">
          <div class="onboarding-progress-fill" :style="{ width: progressPct + '%' }" />
        </div>
        <div class="onboarding-progress-text">{{ t('onboarding.completed', { n: doneCount, total: steps.length }) }}</div>
      </div>
    </div>

    <!-- Steps -->
    <div class="onboarding-steps">
      <div
        v-for="(step, i) in steps"
        :key="step.key"
        class="onboarding-step"
        :class="{ done: step.done, current: !step.done && i === nextIndex }"
        @click="goToStep(step)"
      >
        <div class="step-circle" :class="{ done: step.done, current: !step.done && i === nextIndex }">
          <span v-if="step.done">✓</span>
          <span v-else>{{ i + 1 }}</span>
        </div>
        <div class="step-content">
          <div class="step-title" :class="{ done: step.done }">{{ step.title }}</div>
          <div class="step-desc">{{ step.desc }}</div>
        </div>
        <div v-if="!step.done" class="step-arrow">›</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { onboardingChecklistAPI } from '../api/client'
import type { ApiResponse } from '../types'

interface StepDef {
  key: string
  title: string
  desc: string
  route: string
  done: boolean
}

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()

const visible = ref(false)
const stepData = ref<Record<string, { done: boolean; done_at?: string }>>({})

const firstName = computed(() => auth.user?.first_name || '')

const steps = computed<StepDef[]>(() => [
  { key: 'profile', title: t('onboarding.steps.profile'), desc: t('onboarding.steps.profileDesc'), route: 'profile', done: stepData.value.profile?.done ?? false },
  { key: 'first_clock', title: t('onboarding.steps.firstClock'), desc: t('onboarding.steps.firstClockDesc'), route: 'attendance', done: stepData.value.first_clock?.done ?? false },
  { key: 'view_leave', title: t('onboarding.steps.viewLeave'), desc: t('onboarding.steps.viewLeaveDesc'), route: 'leave', done: stepData.value.view_leave?.done ?? false },
  { key: 'view_payslip', title: t('onboarding.steps.viewPayslip'), desc: t('onboarding.steps.viewPayslipDesc'), route: 'payslips', done: stepData.value.view_payslip?.done ?? false },
  { key: 'ai_chat', title: t('onboarding.steps.aiChat'), desc: t('onboarding.steps.aiChatDesc'), route: 'ai-chat', done: stepData.value.ai_chat?.done ?? false },
])

const doneCount = computed(() => steps.value.filter(s => s.done).length)
const progressPct = computed(() => (doneCount.value / steps.value.length) * 100)
const nextIndex = computed(() => steps.value.findIndex(s => !s.done))

async function loadProgress() {
  try {
    const res = await onboardingChecklistAPI.getProgress('employee') as ApiResponse<{
      steps: Record<string, { done: boolean; done_at?: string }>
      dismissed: boolean
      completed_at: string | null
    }>
    const data = res.data ?? (res as any)
    if (data.dismissed || data.completed_at) {
      visible.value = false
      return
    }
    stepData.value = data.steps || {}
    visible.value = true
    // TODO: When all steps done, show brief celebration animation then auto-hide after 2s
  } catch {
    visible.value = false
  }
}

function goToStep(step: StepDef) {
  if (!step.done) {
    router.push({ name: step.route })
  }
}

async function handleDismiss() {
  try {
    await onboardingChecklistAPI.dismiss('employee')
  } catch { /* ignore */ }
  visible.value = false
}

onMounted(loadProgress)

defineExpose({ loadProgress })
</script>

<style scoped>
.onboarding-wrap { margin-bottom: 16px; }
.onboarding-header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 16px;
  padding: 20px;
  color: white;
  position: relative;
  margin-bottom: 12px;
}
.onboarding-skip {
  position: absolute; top: 12px; right: 12px;
  font-size: 12px; opacity: 0.8; cursor: pointer;
}
.onboarding-title { font-size: 18px; font-weight: 700; margin-bottom: 4px; }
.onboarding-subtitle { font-size: 13px; opacity: 0.9; margin-bottom: 16px; }
.onboarding-progress-bar {
  background: rgba(255,255,255,0.25); border-radius: 8px; height: 6px; margin-bottom: 6px;
}
.onboarding-progress-fill {
  background: white; border-radius: 8px; height: 6px; transition: width 0.3s;
}
.onboarding-progress-text { font-size: 11px; opacity: 0.8; }
.onboarding-steps {
  background: white; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}
.onboarding-step {
  display: flex; align-items: center; padding: 14px 16px;
  border-bottom: 1px solid #f0f0f0; cursor: pointer;
}
.onboarding-step:last-child { border-bottom: none; }
.onboarding-step.done { opacity: 0.6; }
.onboarding-step.current { background: #f0f7ff; }
.step-circle {
  width: 24px; height: 24px; border-radius: 50%;
  border: 2px solid #ddd; color: #bbb;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; flex-shrink: 0;
}
.step-circle.done { background: #18a058; color: white; border-color: #18a058; }
.step-circle.current { background: #2563eb; color: white; border-color: #2563eb; }
.step-content { margin-left: 12px; flex: 1; }
.step-title { font-size: 14px; font-weight: 500; color: #333; }
.step-title.done { text-decoration: line-through; color: #999; }
.onboarding-step.current .step-title { font-weight: 600; color: #2563eb; }
.step-desc { font-size: 12px; color: #999; margin-top: 2px; }
.step-arrow { font-size: 18px; color: #ccc; }
.onboarding-step.current .step-arrow { color: #2563eb; }
</style>
```

- [ ] **Step 4: Build H5 to verify no errors**

Run: `cd /Users/anna/Documents/aigonhr/frontend-mobile && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend-mobile/src/api/client.ts frontend-mobile/src/components/OnboardingChecklist.vue frontend-mobile/src/i18n/
git commit -m "feat(onboarding-checklist): add H5 mobile checklist component and API"
```

---

## Task 6: H5 Mobile — HomeView Integration + Page-Visit Triggers

**Files:**
- Modify: `frontend-mobile/src/views/HomeView.vue`
- Modify: `frontend-mobile/src/views/LeaveView.vue`
- Modify: `frontend-mobile/src/views/PayslipsView.vue`

- [ ] **Step 1: Add OnboardingChecklist to HomeView**

In `HomeView.vue` template, add right after `<PullRefresh>` opens (as the first child inside the pull-refresh content area):
```vue
<OnboardingChecklist />
```

In script, add import:
```typescript
import OnboardingChecklist from '../components/OnboardingChecklist.vue'
```

- [ ] **Step 2: Add page-visit triggers to LeaveView and PayslipsView**

In `LeaveView.vue`, add to the `onMounted` block:
```typescript
import { onboardingChecklistAPI } from '../api/client'

onMounted(() => {
  onboardingChecklistAPI.completeStep('view_leave').catch(() => {})
  // ... existing loadData()
})
```

In `PayslipsView.vue`, add similarly:
```typescript
import { onboardingChecklistAPI } from '../api/client'

onMounted(() => {
  onboardingChecklistAPI.completeStep('view_payslip').catch(() => {})
  // ... existing loadData()
})
```

- [ ] **Step 3: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr/frontend-mobile && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend-mobile/src/views/HomeView.vue frontend-mobile/src/views/LeaveView.vue frontend-mobile/src/views/PayslipsView.vue
git commit -m "feat(onboarding-checklist): integrate H5 checklist into HomeView with page triggers"
```

---

## Task 7: Desktop — API Client + GettingStartedChecklist Component

**Files:**
- Modify: `frontend/src/api/client.ts`
- Create: `frontend/src/components/GettingStartedChecklist.vue`
- Modify: `frontend/src/i18n/en.ts`
- Modify: `frontend/src/i18n/zh.ts`

- [ ] **Step 1: Add API client**

In `frontend/src/api/client.ts`, add after existing exports:
```typescript
export const onboardingChecklistAPI = {
  getProgress: (persona = "admin") =>
    get("/v1/onboarding-checklist/my-progress", { persona }),
  completeStep: (step: string, persona = "admin") =>
    post("/v1/onboarding-checklist/complete-step", { step, persona }),
  dismiss: (persona = "admin") =>
    post("/v1/onboarding-checklist/dismiss", { persona }),
}
```

- [ ] **Step 2: Add i18n keys to en.ts and zh.ts**

In `frontend/src/i18n/en.ts`, add `gettingStarted` section:
```typescript
gettingStarted: {
  title: 'Getting Started with HalaOS',
  subtitle: 'Complete these steps to set up your HR system',
  completed: '{n} / {total} completed',
  dismiss: 'Dismiss',
  goTo: 'Go to {feature}',
  allDone: 'Setup complete! Your HR system is ready.',
  steps: {
    companyInfo: 'Company Information',
    companyInfoDesc: 'Set up company name, TIN, address',
    departments: 'Departments & Positions',
    departmentsDesc: 'Create your org structure',
    importEmployees: 'Import Employees',
    importEmployeesDesc: 'Add your team via CSV or one by one',
    leavePolicies: 'Configure Leave Policies',
    leavePoliciesDesc: 'Set leave types, balances, approval rules',
    schedules: 'Set Up Work Schedules',
    schedulesDesc: 'Define shifts, work hours, rest days',
    payrollConfig: 'Configure Payroll',
    payrollConfigDesc: 'Set pay periods, salary structure, taxes',
    firstPayroll: 'Run First Payroll',
    firstPayrollDesc: 'Process your first payroll cycle',
  },
  links: {
    companyInfo: 'Settings',
    departments: 'Departments',
    importEmployees: 'Employees',
    leavePolicies: 'Settings',
    schedules: 'Schedules',
    payrollConfig: 'Salary Config',
    firstPayroll: 'Payroll',
  },
},
```

In `frontend/src/i18n/zh.ts`, add corresponding:
```typescript
gettingStarted: {
  title: '开始使用 HalaOS',
  subtitle: '完成以下步骤来设置您的HR系统',
  completed: '{n} / {total} 已完成',
  dismiss: '关闭',
  goTo: '前往{feature}',
  allDone: '设置完成！您的HR系统已准备就绪。',
  steps: {
    companyInfo: '公司信息',
    companyInfoDesc: '设置公司名称、TIN、地址',
    departments: '部门和职位',
    departmentsDesc: '创建组织架构',
    importEmployees: '导入员工',
    importEmployeesDesc: '通过CSV或逐个添加团队成员',
    leavePolicies: '配置假期政策',
    leavePoliciesDesc: '设置假期类型、额度、审批规则',
    schedules: '设置排班',
    schedulesDesc: '定义班次、工时、休息日',
    payrollConfig: '配置薪资',
    payrollConfigDesc: '设置薪资周期、薪资结构、税务',
    firstPayroll: '运行首次发薪',
    firstPayrollDesc: '处理第一次薪资周期',
  },
  links: {
    companyInfo: '设置',
    departments: '部门',
    importEmployees: '员工',
    leavePolicies: '设置',
    schedules: '排班',
    payrollConfig: '薪资配置',
    firstPayroll: '薪资',
  },
},
```

- [ ] **Step 3: Create GettingStartedChecklist component**

Create `frontend/src/components/GettingStartedChecklist.vue`:
```vue
<template>
  <NCard v-if="visible" style="margin-bottom: 20px;">
    <div class="gs-header">
      <div>
        <div class="gs-title">{{ t('gettingStarted.title') }}</div>
        <div class="gs-subtitle">{{ t('gettingStarted.subtitle') }}</div>
      </div>
      <div class="gs-header-right">
        <span class="gs-counter">{{ t('gettingStarted.completed', { n: doneCount, total: steps.length }) }}</span>
        <NButton text size="small" @click="handleDismiss">{{ t('gettingStarted.dismiss') }} ×</NButton>
      </div>
    </div>
    <div class="gs-progress">
      <div class="gs-progress-fill" :style="{ width: progressPct + '%' }" />
    </div>
    <div class="gs-grid">
      <div
        v-for="(step, i) in steps"
        :key="step.key"
        class="gs-step"
        :class="{ done: step.done, current: !step.done && i === nextIndex }"
        @click="goToStep(step)"
      >
        <div class="gs-circle" :class="{ done: step.done, current: !step.done && i === nextIndex }">
          <span v-if="step.done">✓</span>
          <span v-else>{{ i + 1 }}</span>
        </div>
        <div class="gs-content">
          <div class="gs-step-title" :class="{ done: step.done }">{{ step.title }}</div>
          <div class="gs-step-desc">{{ step.desc }}</div>
          <div v-if="!step.done && i === nextIndex" class="gs-step-link">
            {{ t('gettingStarted.goTo', { feature: step.linkLabel }) }} →
          </div>
        </div>
      </div>
    </div>
  </NCard>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NCard, NButton } from 'naive-ui'
import { onboardingChecklistAPI } from '../api/client'

const { t } = useI18n()
const router = useRouter()

const visible = ref(false)
const stepData = ref<Record<string, { done: boolean; done_at?: string }>>({})

interface StepDef {
  key: string; title: string; desc: string; route: string; linkLabel: string; done: boolean
}

const steps = computed<StepDef[]>(() => [
  { key: 'company_info', title: t('gettingStarted.steps.companyInfo'), desc: t('gettingStarted.steps.companyInfoDesc'), route: 'settings', linkLabel: t('gettingStarted.links.companyInfo'), done: stepData.value.company_info?.done ?? false },
  { key: 'departments', title: t('gettingStarted.steps.departments'), desc: t('gettingStarted.steps.departmentsDesc'), route: 'departments', linkLabel: t('gettingStarted.links.departments'), done: stepData.value.departments?.done ?? false },
  { key: 'import_employees', title: t('gettingStarted.steps.importEmployees'), desc: t('gettingStarted.steps.importEmployeesDesc'), route: 'employees', linkLabel: t('gettingStarted.links.importEmployees'), done: stepData.value.import_employees?.done ?? false },
  { key: 'leave_policies', title: t('gettingStarted.steps.leavePolicies'), desc: t('gettingStarted.steps.leavePoliciesDesc'), route: 'settings', linkLabel: t('gettingStarted.links.leavePolicies'), done: stepData.value.leave_policies?.done ?? false },
  { key: 'schedules', title: t('gettingStarted.steps.schedules'), desc: t('gettingStarted.steps.schedulesDesc'), route: 'schedules', linkLabel: t('gettingStarted.links.schedules'), done: stepData.value.schedules?.done ?? false },
  { key: 'payroll_config', title: t('gettingStarted.steps.payrollConfig'), desc: t('gettingStarted.steps.payrollConfigDesc'), route: 'salary', linkLabel: t('gettingStarted.links.payrollConfig'), done: stepData.value.payroll_config?.done ?? false },
  { key: 'first_payroll', title: t('gettingStarted.steps.firstPayroll'), desc: t('gettingStarted.steps.firstPayrollDesc'), route: 'payroll', linkLabel: t('gettingStarted.links.firstPayroll'), done: stepData.value.first_payroll?.done ?? false },
])

const doneCount = computed(() => steps.value.filter(s => s.done).length)
const progressPct = computed(() => (doneCount.value / steps.value.length) * 100)
const nextIndex = computed(() => steps.value.findIndex(s => !s.done))

async function loadProgress() {
  try {
    const res = await onboardingChecklistAPI.getProgress('admin') as { data?: any }
    const data = (res.data || res) as any
    if (data.dismissed || data.completed_at) {
      visible.value = false
      return
    }
    stepData.value = data.steps || {}
    visible.value = true
  } catch {
    visible.value = false
  }
}

function goToStep(step: StepDef) {
  if (!step.done) {
    router.push({ name: step.route })
  }
}

async function handleDismiss() {
  try { await onboardingChecklistAPI.dismiss('admin') } catch { /* ignore */ }
  visible.value = false
}

onMounted(loadProgress)
</script>

<style scoped>
.gs-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.gs-title { font-size: 18px; font-weight: 700; color: #111; }
.gs-subtitle { font-size: 13px; color: #888; margin-top: 4px; }
.gs-header-right { display: flex; align-items: center; gap: 12px; }
.gs-counter { font-size: 13px; color: #2563eb; font-weight: 600; }
.gs-progress { background: #f0f0f0; border-radius: 8px; height: 6px; margin-bottom: 20px; }
.gs-progress-fill { background: linear-gradient(90deg, #2563eb, #7c3aed); border-radius: 8px; height: 6px; transition: width 0.3s; }
.gs-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
@media (max-width: 768px) { .gs-grid { grid-template-columns: 1fr; } }
.gs-step {
  display: flex; align-items: flex-start; padding: 14px;
  border-radius: 10px; background: #fafafa; border: 1px solid #eee; cursor: pointer;
  transition: background 0.15s;
}
.gs-step:hover { background: #f5f5f5; }
.gs-step.done { background: #f8faf8; border-color: #d4edda; }
.gs-step.current { background: #eff6ff; border-color: #bfdbfe; }
.gs-circle {
  width: 28px; height: 28px; border-radius: 50%;
  border: 2px solid #ddd; color: #bbb;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; flex-shrink: 0; margin-top: 2px;
}
.gs-circle.done { background: #18a058; color: white; border-color: #18a058; }
.gs-circle.current { background: #2563eb; color: white; border-color: #2563eb; }
.gs-content { margin-left: 12px; }
.gs-step-title { font-size: 14px; font-weight: 600; color: #333; }
.gs-step-title.done { text-decoration: line-through; color: #999; }
.gs-step.current .gs-step-title { color: #2563eb; }
.gs-step-desc { font-size: 12px; color: #999; margin-top: 2px; }
.gs-step-link { font-size: 11px; color: #2563eb; margin-top: 6px; font-weight: 500; }
</style>
```

- [ ] **Step 4: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/components/GettingStartedChecklist.vue frontend/src/i18n/
git commit -m "feat(onboarding-checklist): add Desktop admin checklist component and API"
```

---

## Task 8: Desktop — DashboardView Integration

**Files:**
- Modify: `frontend/src/views/DashboardView.vue`

- [ ] **Step 1: Replace "Get Started" card with GettingStartedChecklist**

Import the component:
```typescript
import GettingStartedChecklist from '../components/GettingStartedChecklist.vue'
```

Replace the existing "Getting Started CTA" block (around lines 323-343) with:
```vue
<GettingStartedChecklist v-if="auth.isAdmin" />
```

Remove the old `NCard v-if="totalEmployees === 0 && auth.isAdmin"` block entirely.

- [ ] **Step 2: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/DashboardView.vue
git commit -m "feat(onboarding-checklist): integrate admin checklist into DashboardView"
```

---

## Task 9: Desktop — EmptyState Component

**Files:**
- Create: `frontend/src/components/EmptyState.vue`

- [ ] **Step 1: Create reusable EmptyState component**

Note: This uses a custom div layout instead of wrapping NaiveUI's `NEmpty`. This is an intentional simplification — the custom layout with gradient icon circle provides better visual design than `NEmpty`'s default rendering. If dark mode support is added later, add theme-aware CSS variables.

```vue
<template>
  <div class="empty-state">
    <div class="empty-state-icon">{{ icon }}</div>
    <div class="empty-state-title">{{ title }}</div>
    <div class="empty-state-desc">{{ description }}</div>
    <NSpace v-if="primaryAction || secondaryAction" style="margin-top: 16px;">
      <NButton v-if="primaryAction" type="primary" @click="primaryAction.handler">
        {{ primaryAction.label }}
      </NButton>
      <NButton v-if="secondaryAction" @click="secondaryAction.handler">
        {{ secondaryAction.label }}
      </NButton>
    </NSpace>
  </div>
</template>

<script setup lang="ts">
import { NButton, NSpace } from 'naive-ui'

defineProps<{
  icon: string
  title: string
  description: string
  primaryAction?: { label: string; handler: () => void }
  secondaryAction?: { label: string; handler: () => void }
}>()
</script>

<style scoped>
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; padding: 48px 24px; text-align: center;
}
.empty-state-icon {
  width: 80px; height: 80px; border-radius: 50%;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  display: flex; align-items: center; justify-content: center;
  font-size: 36px; margin-bottom: 16px;
}
.empty-state-title {
  font-size: 16px; font-weight: 600; color: #333; margin-bottom: 6px;
}
.empty-state-desc {
  font-size: 13px; color: #888; max-width: 320px; line-height: 1.5;
}
</style>
```

- [ ] **Step 2: Add empty state i18n keys to en.ts**

```typescript
emptyState: {
  leaves: { title: 'No leave requests yet', desc: 'File a request and your manager will be notified automatically.', cta: 'Request Leave' },
  attendance: { title: 'No attendance records', desc: 'Clock in to start tracking your work hours.', cta: 'Clock In Now' },
  payslips: { title: 'No payslips available', desc: 'Payslips are generated after each payroll run. Check back after the next pay period.' },
  employees: { title: 'No employees yet', desc: 'Add your team to start managing attendance, leave, and payroll.', cta: 'Add Employee', cta2: 'Import CSV' },
  approvals: { title: 'All caught up!', desc: 'No pending approvals. When employees request leave or overtime, they\'ll appear here.' },
  notifications: { title: 'No notifications', desc: 'You\'ll be notified about approvals, announcements, and important updates.' },
  loans: { title: 'No active loans', desc: 'Need a salary advance or company loan? Apply here.', cta: 'Apply for Loan' },
  expenses: { title: 'No expense claims', desc: 'Submit receipts for reimbursement.', cta: 'File Expense' },
  training: { title: 'No training courses', desc: 'Check back soon for assigned courses and certifications.' },
  overtime: { title: 'No overtime records', desc: 'When you work extra hours, file an overtime request here.', cta: 'File OT Request' },
  schedules: { title: 'No schedules yet', desc: 'Create work schedules to manage shifts and hours.' },
  announcements: { title: 'No announcements', desc: 'Company announcements will appear here.' },
  directory: { title: 'No employees found', desc: 'Your team directory will populate as employees are added.' },
},
```

Add corresponding Chinese translations to `zh.ts`.

- [ ] **Step 3: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/EmptyState.vue frontend/src/i18n/
git commit -m "feat(empty-states): add reusable EmptyState component with i18n"
```

---

## Task 10: Desktop — Replace NEmpty in High-Traffic Views

**Files to modify** (replace `NEmpty` with `EmptyState`):
- `frontend/src/views/ApprovalsView.vue`
- `frontend/src/views/AttendanceView.vue`
- `frontend/src/views/NotificationsView.vue`
- `frontend/src/views/SchedulesView.vue`
- `frontend/src/views/LeaveCalendarView.vue`
- `frontend/src/views/AnnouncementsView.vue`
- `frontend/src/views/DirectoryView.vue`
- `frontend/src/views/PayrollView.vue`

- [ ] **Step 1: Update each view**

For each file, follow this pattern:

1. Import `EmptyState`:
```typescript
import EmptyState from '../components/EmptyState.vue'
```

2. Replace bare `<NEmpty :description="..." />` with:
```vue
<EmptyState
  icon="✅"
  :title="t('emptyState.approvals.title')"
  :description="t('emptyState.approvals.desc')"
/>
```

Apply appropriate icon, title, and description per the spec's empty state table. Add `primaryAction` where a CTA is specified (e.g., leaves, attendance, loans).

- [ ] **Step 2: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/
git commit -m "feat(empty-states): replace NEmpty with EmptyState in high-traffic views"
```

---

## Task 11: H5 Mobile — EmptyState Component + View Updates

**Files:**
- Create: `frontend-mobile/src/components/EmptyState.vue`
- Modify: `frontend-mobile/src/views/NotificationsView.vue`
- Modify: `frontend-mobile/src/views/LeaveView.vue`
- Modify: `frontend-mobile/src/views/AttendanceView.vue`
- Modify: `frontend-mobile/src/views/PayslipsView.vue`
- Modify: `frontend-mobile/src/i18n/en.ts`
- Modify: `frontend-mobile/src/i18n/zh.ts`

- [ ] **Step 1: Create H5 EmptyState component**

Note: This uses plain divs with Vant button instead of wrapping `van-empty`. This is an intentional simplification — the custom layout provides better UX than van-empty's default rendering. Vant uses auto-import via `unplugin-vue-components`, so `<van-button>` works without explicit imports.

```vue
<template>
  <div class="empty-state">
    <div class="empty-state-icon">{{ icon }}</div>
    <div class="empty-state-title">{{ title }}</div>
    <div class="empty-state-desc">{{ description }}</div>
    <div v-if="primaryAction" class="empty-state-actions">
      <van-button type="primary" size="small" round @click="primaryAction.handler">
        {{ primaryAction.label }}
      </van-button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  icon: string
  title: string
  description: string
  primaryAction?: { label: string; handler: () => void }
}>()
</script>

<style scoped>
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  padding: 40px 24px; text-align: center;
}
.empty-state-icon {
  width: 64px; height: 64px; border-radius: 50%;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  display: flex; align-items: center; justify-content: center;
  font-size: 28px; margin-bottom: 12px;
}
.empty-state-title { font-size: 15px; font-weight: 600; color: #333; margin-bottom: 4px; }
.empty-state-desc { font-size: 13px; color: #888; max-width: 260px; line-height: 1.5; }
.empty-state-actions { margin-top: 16px; }
</style>
```

- [ ] **Step 2: Add H5 empty state i18n keys**

Add `emptyState` section to `frontend-mobile/src/i18n/en.ts` and `zh.ts` (covers: leaves, attendance, payslips, notifications):

```typescript
// en.ts
emptyState: {
  leaves: { title: 'No leave requests yet', desc: 'File a request and your manager will be notified automatically.', cta: 'Request Leave' },
  attendance: { title: 'No attendance records', desc: 'Clock in to start tracking your work hours.', cta: 'Clock In Now' },
  payslips: { title: 'No payslips available', desc: 'Payslips are generated after each payroll run. Check back after the next pay period.' },
  notifications: { title: 'No notifications', desc: "You'll be notified about approvals, announcements, and important updates." },
},

// zh.ts
emptyState: {
  leaves: { title: '暂无请假记录', desc: '提交请假申请，您的主管将收到通知。', cta: '申请请假' },
  attendance: { title: '暂无考勤记录', desc: '打卡开始记录工时。', cta: '立即打卡' },
  payslips: { title: '暂无工资单', desc: '工资单在每次发薪后生成，请在下个发薪周期后查看。' },
  notifications: { title: '暂无通知', desc: '审批、公告和重要更新将在此显示。' },
},
```

- [ ] **Step 3: Update H5 views to use EmptyState**

In each file, import `EmptyState` and replace existing `van-empty` or bare empty state markup:

**`NotificationsView.vue`:**
```vue
<EmptyState icon="🔔" :title="t('emptyState.notifications.title')" :description="t('emptyState.notifications.desc')" />
```

**`LeaveView.vue`:** Replace the empty list state with:
```vue
<EmptyState icon="🏖️" :title="t('emptyState.leaves.title')" :description="t('emptyState.leaves.desc')"
  :primaryAction="{ label: t('emptyState.leaves.cta'), handler: () => router.push({ name: 'leave-apply' }) }" />
```

**`AttendanceView.vue`:** Replace the empty state with:
```vue
<EmptyState icon="⏰" :title="t('emptyState.attendance.title')" :description="t('emptyState.attendance.desc')" />
```

**`PayslipsView.vue`:** Replace the empty state with:
```vue
<EmptyState icon="💰" :title="t('emptyState.payslips.title')" :description="t('emptyState.payslips.desc')" />
```

- [ ] **Step 4: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr/frontend-mobile && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend-mobile/src/components/EmptyState.vue frontend-mobile/src/views/ frontend-mobile/src/i18n/
git commit -m "feat(empty-states): add H5 EmptyState component and update views"
```

---

## Task 12: Deploy and Verify

- [ ] **Step 1: Run backend tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/onboarding_checklist/ -v`
Expected: All tests pass

- [ ] **Step 2: Build Go API binary**

Run: `cd /Users/anna/Documents/aigonhr && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api ./cmd/api`
Expected: Binary created at `bin/api`

- [ ] **Step 3: Build both frontends**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Run: `cd /Users/anna/Documents/aigonhr/frontend-mobile && npm run build && cp -r dist ../frontend/mobile-dist`

- [ ] **Step 4: Deploy to server**

```bash
scp bin/api aigonhr:/home/ubuntu/aigonhr/bin/
scp -r frontend/dist aigonhr:/home/ubuntu/aigonhr/frontend/
scp -r frontend/mobile-dist aigonhr:/home/ubuntu/aigonhr/frontend/
ssh aigonhr "cd /home/ubuntu/aigonhr && docker compose -f docker-compose.deploy.yml up -d --build"
```

- [ ] **Step 5: Run migration on server**

```bash
ssh aigonhr "cd /home/ubuntu/aigonhr && docker compose -f docker-compose.deploy.yml exec api ./api migrate"
```

- [ ] **Step 6: Verify in browser**

1. Desktop: Login as admin → Dashboard should show Getting Started checklist
2. H5: Login as employee at `/m/` → HomeView should show onboarding checklist
3. Navigate to a page with no data → Should see EmptyState component instead of bare "No Data"

- [ ] **Step 7: Final commit**

```bash
git add -A
git commit -m "feat(onboarding-redesign): complete implementation with checklists and empty states"
```
