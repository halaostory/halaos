package importexport

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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

// --- Utility functions ---

func TestNumStr(t *testing.T) {
	tests := []struct {
		name string
		n    pgtype.Numeric
		want string
	}{
		{"invalid", pgtype.Numeric{}, "0.00"},
		{"valid", mustNumeric("123.45"), "123.45"},
		{"zero", mustNumeric("0"), "0.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numStr(tt.n)
			if got != tt.want {
				t.Fatalf("numStr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNumFloat(t *testing.T) {
	tests := []struct {
		name string
		n    pgtype.Numeric
		want float64
	}{
		{"invalid", pgtype.Numeric{}, 0},
		{"valid", mustNumeric("42.5"), 42.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numFloat(tt.n)
			if got != tt.want {
				t.Fatalf("numFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntPtrStr(t *testing.T) {
	if got := intPtrStr(nil); got != "0" {
		t.Fatalf("expected '0' for nil, got %q", got)
	}
	v := int32(15)
	if got := intPtrStr(&v); got != "15" {
		t.Fatalf("expected '15', got %q", got)
	}
}

// --- ExportAttendanceCSV ---

func TestExportAttendanceCSV_MissingParams(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithQuery("GET", "/export/attendance/csv", nil, adminAuth)
	h.ExportAttendanceCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportAttendanceCSV_InvalidStartDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"start": {"bad"}, "end": {"2026-01-31"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/export/attendance/csv", q, adminAuth)
	h.ExportAttendanceCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportAttendanceCSV_InvalidEndDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"start": {"2026-01-01"}, "end": {"bad"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/export/attendance/csv", q, adminAuth)
	h.ExportAttendanceCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportAttendanceCSV_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	q := url.Values{"start": {"2026-01-01"}, "end": {"2026-01-31"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/export/attendance/csv", q, adminAuth)
	h.ExportAttendanceCSV(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ExportLeaveBalancesCSV ---

func TestExportLeaveBalancesCSV_InvalidYear(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"abc"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/export/leave-balances/csv", q, adminAuth)
	h.ExportLeaveBalancesCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportLeaveBalancesCSV_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	q := url.Values{"year": {"2026"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/export/leave-balances/csv", q, adminAuth)
	h.ExportLeaveBalancesCSV(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ImportEmployeesCSV ---

func TestImportEmployeesCSV_NoFile(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/import/employees/csv", nil, adminAuth)
	h.ImportEmployeesCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportEmployeesCSV_MissingColumns(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := newGinContextWithCSV("file", "bad_header.csv", "name,email\nJohn,john@test.com\n", adminAuth)
	h.ImportEmployeesCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- PreviewImportCSV ---

func TestPreviewImportCSV_NoFile(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/import/employees/preview", nil, adminAuth)
	h.PreviewImportCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPreviewImportCSV_MissingColumns(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := newGinContextWithCSV("file", "bad.csv", "name,email\nJohn,test@test.com\n", adminAuth)
	h.PreviewImportCSV(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Helpers ---

func mustNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(s)
	return n
}

func newGinContextWithCSV(fieldName, filename, content string, ac testutil.AuthContext) (*gin.Context, *httptest.ResponseRecorder) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, filename)
	_, _ = part.Write([]byte(content))
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req

	c.Set("user_id", ac.UserID)
	c.Set("email", ac.Email)
	c.Set("role", ac.Role)
	c.Set("company_id", ac.CompanyID)

	return c, w
}
