package holiday

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

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
	return NewHandler(queries, nil, logger)
}

// holidayScanValues returns values matching the Holiday scan order (8 fields).
func holidayScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{
		int64(1),      // ID
		int64(1),      // CompanyID
		"New Year",    // Name
		now,           // HolidayDate
		"regular",     // HolidayType
		int32(2025),   // Year
		true,          // IsNationwide
		now,           // CreatedAt
	}
}

func TestList_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/holidays", nil, adminAuth)
	h.List(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestList_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/holidays", nil, adminAuth)
	h.List(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/holidays", gin.H{}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(holidayScanValues()...))
	c, w := testutil.NewGinContext("POST", "/holidays", gin.H{
		"name":         "New Year",
		"holiday_date": "2025-01-01",
		"holiday_type": "regular",
	}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_InvalidDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/holidays", gin.H{
		"name":         "New Year",
		"holiday_date": "not-a-date",
		"holiday_type": "regular",
	}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))
	c, w := testutil.NewGinContext("POST", "/holidays", gin.H{
		"name":         "New Year",
		"holiday_date": "2025-01-01",
		"holiday_type": "regular",
	}, adminAuth)
	h.Create(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDelete_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExecSuccess()
	c, w := testutil.NewGinContextWithParams("DELETE", "/holidays/1",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.Delete(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDelete_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnExec(testutil.ZeroCommandTag(), fmt.Errorf("not found"))
	c, w := testutil.NewGinContextWithParams("DELETE", "/holidays/999",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)
	h.Delete(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
