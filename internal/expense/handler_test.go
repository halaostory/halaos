package expense

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
	return NewHandler(queries, nil, logger)
}

func TestListCategories_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/expenses/categories", nil, adminAuth)
	h.ListCategories(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListCategories_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/expenses/categories", nil, adminAuth)
	h.ListCategories(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCategory_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/expenses/categories", gin.H{}, adminAuth)
	h.CreateCategory(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetSummary ---

func TestGetSummary_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))
	c, w := testutil.NewGinContextWithQuery("GET", "/expenses/summary", nil, adminAuth)
	h.GetSummary(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- List ---

func TestList_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQueryRow(testutil.NewRow(int64(0))) // CountExpenseClaims
	c, w := testutil.NewGinContextWithQuery("GET", "/expenses", nil, adminAuth)
	h.List(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestList_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/expenses", nil, adminAuth)
	h.List(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Get ---

func TestGet_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithParams("GET", "/expenses/999",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)
	h.Get(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListMy ---

func TestListMy_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithQuery("GET", "/expenses/my", nil, adminAuth)
	h.ListMy(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (empty list), got %d: %s", w.Code, w.Body.String())
	}
}

// --- Create ---

func TestCreate_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))
	c, w := testutil.NewGinContext("POST", "/expenses", gin.H{}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContext("POST", "/expenses", gin.H{
		"category_id":  1,
		"description":  "Taxi",
		"amount":       500,
		"expense_date": "2025-06-15",
	}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_InvalidExpenseDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(testutil.FixtureEmployee())...))
	c, w := testutil.NewGinContext("POST", "/expenses", gin.H{
		"category_id":  1,
		"description":  "Taxi",
		"amount":       500,
		"expense_date": "not-a-date",
	}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Submit ---

func TestSubmit_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithParams("POST", "/expenses/1/submit",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.Submit(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Approve ---

func TestApprove_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	c, w := testutil.NewGinContextWithParams("POST", "/expenses/1/approve",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.Approve(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
