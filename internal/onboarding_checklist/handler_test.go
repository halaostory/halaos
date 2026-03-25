package onboarding_checklist

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

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
func onboardingRow(id, companyID, userID int64, persona string, steps json.RawMessage, dismissed bool) []interface{} {
	return []interface{}{
		id, companyID, userID, persona, steps, dismissed,
		pgtype.Timestamptz{},  // completed_at (null)
		time.Now(),            // created_at
		time.Now(),            // updated_at
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
		json.RawMessage(`{"profile":{"done":false},"first_clock":{"done":false},"view_leave":{"done":false},"view_payslip":{"done":false},"ai_chat":{"done":false}}`),
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
		json.RawMessage(`{"profile":{"done":false},"first_clock":{"done":false},"view_leave":{"done":false},"view_payslip":{"done":false},"ai_chat":{"done":false}}`),
		false,
	)...))
	// Upsert updated steps (QueryRow for UpsertOnboardingChecklist)
	mockDB.OnQueryRow(testutil.NewRow(onboardingRow(
		1, 1, 10, "employee",
		json.RawMessage(`{"profile":{"done":true},"first_clock":{"done":false},"view_leave":{"done":false},"view_payslip":{"done":false},"ai_chat":{"done":false}}`),
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
