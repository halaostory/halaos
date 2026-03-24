package payroll

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

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
	return NewHandler(queries, nil, logger)
}

// --- ListCycles ---

func TestListCycles_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/payroll/cycles", nil, adminAuth)
	h.ListCycles(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListCycles_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/payroll/cycles", nil, adminAuth)
	h.ListCycles(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateCycle ---

func TestCreateCycle_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/payroll/cycles", gin.H{
		// missing required fields
	}, adminAuth)

	h.CreateCycle(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCycle_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContext("POST", "/payroll/cycles", gin.H{
		"name":         "Jan 2025 1st Half",
		"period_start": "2025-01-01",
		"period_end":   "2025-01-15",
		"pay_date":     "2025-01-20",
	}, adminAuth)

	h.CreateCycle(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- RunPayroll ---

func TestRunPayroll_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/payroll/run", gin.H{
		// missing cycle_id
	}, adminAuth)

	h.RunPayroll(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ApproveCycle ---

func TestApproveCycle_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// IsPayrollCycleLocked returns error (not found)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	// ApprovePayrollCycle fails
	mockDB.OnExec(testutil.ZeroCommandTag(), fmt.Errorf("not found"))

	c, w := testutil.NewGinContextWithParams("POST", "/payroll/cycles/999/approve",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)

	h.ApproveCycle(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListPayslips ---

func TestListPayslips_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContextWithQuery("GET", "/payroll/payslips", nil, adminAuth)
	h.ListPayslips(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetPayslip ---

func TestGetPayslip_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/payslips/invalid",
		gin.Params{{Key: "id", Value: "not-a-uuid"}}, nil, adminAuth)

	h.GetPayslip(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetPayslip_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/payslips/550e8400-e29b-41d4-a716-446655440000",
		gin.Params{{Key: "id", Value: "550e8400-e29b-41d4-a716-446655440000"}}, nil, adminAuth)

	h.GetPayslip(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListPayrollItems ---

func TestListPayrollItems_RunNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetPayrollRun returns error (not found)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/runs/1/items",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.ListPayrollItems(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListPayrollItems_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetPayrollRun succeeds
	mockDB.OnQueryRow(testutil.NewRow(payrollRunScanValues()...))
	// ListPayrollItems query fails
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/runs/1/items",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.ListPayrollItems(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DownloadPayslipPDF ---

func TestDownloadPayslipPDF_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/payslips/invalid/pdf",
		gin.Params{{Key: "id", Value: "not-a-uuid"}}, nil, adminAuth)

	h.DownloadPayslipPDF(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// payrollRunScanValues returns values matching the PayrollRun scan order (14 fields).
func payrollRunScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	userID := int64(1)
	return []interface{}{
		int64(1),                  // ID
		int64(1),                  // CompanyID
		int64(1),                  // CycleID
		"regular",                 // RunType
		int32(1),                  // RunNumber
		int32(0),                  // TotalEmployees
		pgtype.Numeric{},          // TotalGross
		pgtype.Numeric{},          // TotalDeductions
		pgtype.Numeric{},          // TotalNet
		"completed",               // Status
		(*string)(nil),            // ErrorMessage
		&userID,                   // InitiatedBy
		pgtype.Timestamptz{},      // CompletedAt
		now,                       // CreatedAt
	}
}

// payrollCycleScanValues returns values matching the PayrollCycle scan order (16 fields).
func payrollCycleScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	userID := int64(1)
	return []interface{}{
		int64(1),                  // ID
		int64(1),                  // CompanyID
		"Jan 2025 1st Half",      // Name
		now,                       // PeriodStart
		now.AddDate(0, 0, 14),    // PeriodEnd
		now.AddDate(0, 0, 19),    // PayDate
		"regular",                 // CycleType
		"draft",                   // Status
		&userID,                   // CreatedBy
		(*int64)(nil),             // ApprovedBy
		pgtype.Timestamptz{},      // ApprovedAt
		now,                       // CreatedAt
		now,                       // UpdatedAt
		false,                     // IsLocked
		pgtype.Timestamptz{},      // LockedAt
		(*int64)(nil),             // LockedBy
	}
}

// --- CreateCycle Success ---

func TestCreateCycle_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(payrollCycleScanValues()...))

	c, w := testutil.NewGinContext("POST", "/payroll/cycles", gin.H{
		"name":         "Jan 2025 1st Half",
		"period_start": "2025-01-01",
		"period_end":   "2025-01-15",
		"pay_date":     "2025-01-20",
	}, adminAuth)

	h.CreateCycle(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListPayrollItems Success ---

func TestListPayrollItems_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetPayrollRun succeeds
	mockDB.OnQueryRow(testutil.NewRow(payrollRunScanValues()...))
	// ListPayrollItems returns empty result
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/runs/1/items",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.ListPayrollItems(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- RunPayroll cycle locked ---

func TestRunPayroll_CycleLocked(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// IsPayrollCycleLocked returns true
	mockDB.OnQueryRow(testutil.NewRow(true))

	c, w := testutil.NewGinContext("POST", "/payroll/run", gin.H{
		"cycle_id": 1,
	}, adminAuth)

	h.RunPayroll(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ApproveCycle locked ---

func TestApproveCycle_Locked(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// IsPayrollCycleLocked returns true
	mockDB.OnQueryRow(testutil.NewRow(true))

	c, w := testutil.NewGinContextWithParams("POST", "/payroll/cycles/1/approve",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.ApproveCycle(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DownloadPayslipPDF employee not found ---

func TestDownloadPayslipPDF_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContextWithParams("GET", "/payroll/payslips/550e8400-e29b-41d4-a716-446655440000/pdf",
		gin.Params{{Key: "id", Value: "550e8400-e29b-41d4-a716-446655440000"}}, nil, adminAuth)

	h.DownloadPayslipPDF(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
