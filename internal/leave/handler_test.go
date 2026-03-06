package leave

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

func leaveTypeScanValues() []interface{} {
	return []interface{}{
		int64(1),            // ID
		int64(1),            // CompanyID
		"VL",                // Code
		"Vacation Leave",    // Name
		true,                // IsPaid
		pgtype.Numeric{},    // DefaultDays
		false,               // IsConvertible
		false,               // RequiresAttachment
		int32(0),            // MinDaysNotice
		"annual",            // AccrualType
		(*string)(nil),      // GenderSpecific
		false,               // IsStatutory
		true,                // IsActive
		time.Now(),          // CreatedAt
		pgtype.Numeric{},    // MaxCarryover
		(*int32)(nil),       // CarryoverExpiryMonths
	}
}

// --- ListTypes ---

func TestListTypes_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{leaveTypeScanValues()}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/types", nil, adminAuth)
	h.ListTypes(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTypes_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/types", nil, adminAuth)
	h.ListTypes(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListTypes_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/types", nil, adminAuth)
	h.ListTypes(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateType ---

func TestCreateType_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(leaveTypeScanValues()...))

	c, w := testutil.NewGinContext("POST", "/leave/types", gin.H{
		"code": "VL",
		"name": "Vacation Leave",
	}, adminAuth)

	h.CreateType(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateType_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/leave/types", gin.H{
		// missing required code and name
	}, adminAuth)

	h.CreateType(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateType_Conflict(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("unique violation")))

	c, w := testutil.NewGinContext("POST", "/leave/types", gin.H{
		"code": "VL",
		"name": "Vacation Leave",
	}, adminAuth)

	h.CreateType(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetBalances ---

func TestGetBalances_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID returns error
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/balances", nil, adminAuth)
	h.GetBalances(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateRequest ---

func TestCreateRequest_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/leave/requests", gin.H{
		// missing required fields
	}, adminAuth)

	h.CreateRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateRequest_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContext("POST", "/leave/requests", gin.H{
		"leave_type_id": 1,
		"start_date":    "2025-06-01",
		"end_date":      "2025-06-03",
		"days":          "3",
	}, adminAuth)

	h.CreateRequest(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListRequests ---

func TestListRequests_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/requests", nil, adminAuth)
	h.ListRequests(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ApproveRequest ---

func TestApproveRequest_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/leave/requests/abc/approve",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)

	h.ApproveRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- RejectRequest ---

func TestRejectRequest_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/leave/requests/abc/reject",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)

	h.RejectRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListRequests Success ---

func TestListRequests_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQueryRow(testutil.NewRow(int64(0))) // CountLeaveRequests

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/requests", nil, adminAuth)
	h.ListRequests(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CancelRequest ---

func TestCancelRequest_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/leave/requests/abc/cancel",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)

	h.CancelRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetBalances Success ---

func TestGetBalances_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID returns an employee
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))
	// ListLeaveBalances returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/balances", nil, adminAuth)
	h.GetBalances(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetBalances_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))
	// ListLeaveBalances fails
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/leave/balances", nil, adminAuth)
	h.GetBalances(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateRequest date validation ---

func TestCreateRequest_InvalidStartDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))

	c, w := testutil.NewGinContext("POST", "/leave/requests", gin.H{
		"leave_type_id": 1,
		"start_date":    "not-a-date",
		"end_date":      "2025-06-03",
		"days":          "3",
	}, adminAuth)

	h.CreateRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateRequest_InvalidEndDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))

	c, w := testutil.NewGinContext("POST", "/leave/requests", gin.H{
		"leave_type_id": 1,
		"start_date":    "2025-06-01",
		"end_date":      "not-a-date",
		"days":          "3",
	}, adminAuth)

	h.CreateRequest(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
