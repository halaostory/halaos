package breaks

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
	return NewHandler(queries, nil, logger, nil)
}

// attendanceLogScanValues returns mock scan values matching the AttendanceLog scan order (27 fields).
func attendanceLogScanValues() []interface{} {
	return []interface{}{
		int64(100),                // ID
		int64(1),                  // CompanyID
		int64(1),                  // EmployeeID
		pgtype.Timestamptz{Time: time.Now(), Valid: true}, // ClockInAt
		pgtype.Timestamptz{},     // ClockOutAt
		"web",                     // ClockInSource
		(*string)(nil),            // ClockOutSource
		pgtype.Numeric{},          // ClockInLat
		pgtype.Numeric{},          // ClockInLng
		pgtype.Numeric{},          // ClockOutLat
		pgtype.Numeric{},          // ClockOutLng
		(*string)(nil),            // ClockInNote
		(*string)(nil),            // ClockOutNote
		pgtype.Numeric{},          // WorkHours
		pgtype.Numeric{},          // OvertimeHours
		(*int32)(nil),             // LateMinutes
		(*int32)(nil),             // UndertimeMinutes
		"open",                    // Status
		false,                     // IsCorrected
		(*int64)(nil),             // CorrectedBy
		time.Now(),                // CreatedAt
		time.Now(),                // UpdatedAt
		(*int64)(nil),             // ClockInGeofenceID
		(*string)(nil),            // ClockInGeofenceStatus
		(*int64)(nil),             // ClockOutGeofenceID
		(*string)(nil),            // ClockOutGeofenceStatus
	}
}

// breakLogScanValues returns mock scan values matching the BreakLog scan order (11 fields).
func breakLogScanValues(breakType string) []interface{} {
	return []interface{}{
		int64(200),                // ID
		int64(1),                  // CompanyID
		int64(1),                  // EmployeeID
		int64(100),                // AttendanceLogID
		breakType,                 // BreakType
		time.Now(),                // StartAt
		pgtype.Timestamptz{},      // EndAt
		(*int32)(nil),             // DurationMinutes
		(*int32)(nil),             // OvertimeMinutes
		(*string)(nil),            // Note
		time.Now(),                // CreatedAt
	}
}

// --- StartBreak ---

func TestStartBreak_InvalidType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Employee found
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "smoking",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartBreak_NotClockedIn(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetOpenAttendance fails (not clocked in)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no open record")))

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

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetOpenAttendance succeeds
	mockDB.OnQueryRow(testutil.NewRow(attendanceLogScanValues()...))

	// GetActiveBreak succeeds (already on break)
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues("meal")...))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "rest",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartBreak_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Send invalid JSON (missing required break_type)
	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{}, adminAuth)
	h.StartBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- EndBreak ---

func TestEndBreak_NoActiveBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no active break")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/end", nil, adminAuth)

	h.EndBreak(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEndBreak_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/end", nil, adminAuth)

	h.EndBreak(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetActiveBreak ---

func TestGetActiveBreak_None(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no active break")))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks/active", nil, adminAuth)

	h.GetActiveBreak(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetActiveBreak_NoEmployee(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks/active", nil, adminAuth)

	h.GetActiveBreak(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListBreaks ---

func TestListBreaks_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks", nil, adminAuth)

	h.ListBreaks(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListBreaks_InvalidDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks?date=not-a-date", nil, adminAuth)

	h.ListBreaks(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
