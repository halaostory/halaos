package virtualoffice

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

var empAuth = testutil.AuthContext{
	UserID: 10, Email: "emp@test.com", Role: auth.RoleEmployee, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger, nil)
}

// voConfigScanValues returns scan values for VirtualOfficeConfig in sqlc scan order:
// company_id, template, created_at, updated_at
func voConfigScanValues(companyID int64, template string) []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{companyID, template, now, now}
}

func TestUpdateConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// UpsertVirtualOfficeConfig scans: company_id, template, created_at, updated_at
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "medium")...))
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{"template": "medium"}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfig_InvalidTemplate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{"template": "huge"}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfig_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/config", nil, adminAuth)
	h.GetConfig(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/config", nil, adminAuth)
	h.GetConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfig_MissingTemplate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignSeat_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetEmployeeByID returns error
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(99),
		"zone":        "desk-a",
		"seat_x":      int32(2),
		"seat_y":      int32(2),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignSeat_InactiveEmployee(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// Return an inactive employee
	emp := testutil.FixtureEmployee()
	emp.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(1),
		"zone":        "desk-a",
		"seat_x":      int32(2),
		"seat_y":      int32(2),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAutoAssign_NotConfigured(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetVirtualOfficeConfig returns no rows
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/auto", nil, adminAuth)
	h.AutoAssign(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAutoAssign_NoUnassigned(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetVirtualOfficeConfig returns config
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	// ListUnassignedActiveEmployees returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/auto", nil, adminAuth)
	h.AutoAssign(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSnapshot_NotConfigured(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/snapshot", nil, empAuth)
	h.GetSnapshot(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
