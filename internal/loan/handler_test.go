package loan

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger, nil)
}

func TestListLoanTypes_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/loans/types", nil, adminAuth)
	h.ListLoanTypes(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLoanTypes_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/loans/types", nil, adminAuth)
	h.ListLoanTypes(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateLoanType_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/loans/types", gin.H{}, adminAuth)
	h.CreateLoanType(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListLoans ---

func TestListLoans_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/loans", nil, adminAuth)
	h.ListLoans(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLoans_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/loans", nil, adminAuth)
	h.ListLoans(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListMyLoans ---

func TestListMyLoans_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithQuery("GET", "/loans/my", nil, adminAuth)
	h.ListMyLoans(c)
	// ListMyLoans returns 200 with empty array when employee not found
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetLoan ---

func TestGetLoan_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("GET", "/loans/abc",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.GetLoan(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLoan_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithParams("GET", "/loans/999",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)
	h.GetLoan(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ApplyLoan ---

func TestApplyLoan_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/loans", gin.H{}, adminAuth)
	h.ApplyLoan(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestApplyLoan_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContext("POST", "/loans", gin.H{
		"loan_type_id": 1,
		"amount":       10000,
		"term_months":  12,
		"start_date":   "2025-01-01",
	}, adminAuth)
	h.ApplyLoan(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestApplyLoan_InvalidStartDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetEmployeeByUserID succeeds
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))
	c, w := testutil.NewGinContext("POST", "/loans", gin.H{
		"loan_type_id": 1,
		"amount":       10000,
		"term_months":  12,
		"start_date":   "not-a-date",
	}, adminAuth)
	h.ApplyLoan(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ApproveLoan ---

func TestApproveLoan_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("POST", "/loans/abc/approve",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.ApproveLoan(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CancelLoan ---

func TestCancelLoan_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("POST", "/loans/abc/cancel",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.CancelLoan(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelLoan_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithParams("POST", "/loans/999/cancel",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)
	h.CancelLoan(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- RecordPayment ---

func TestRecordPayment_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("POST", "/loans/1/payments",
		gin.Params{{Key: "id", Value: "1"}}, gin.H{}, adminAuth)
	h.RecordPayment(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRecordPayment_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("POST", "/loans/abc/payments",
		gin.Params{{Key: "id", Value: "abc"}}, gin.H{}, adminAuth)
	h.RecordPayment(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRecordPayment_InvalidPaymentDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("POST", "/loans/1/payments",
		gin.Params{{Key: "id", Value: "1"}}, gin.H{
			"amount":       1000,
			"payment_date": "not-a-date",
		}, adminAuth)
	h.RecordPayment(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
