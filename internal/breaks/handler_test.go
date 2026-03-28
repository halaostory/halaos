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

// --- StartBreak happy path ---

func TestStartBreak_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetOpenAttendance succeeds (employee is clocked in)
	mockDB.OnQueryRow(testutil.NewRow(attendanceLogScanValues()...))

	// GetActiveBreak fails (no active break — good)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	// CreateBreakLog succeeds
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues("meal")...))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "meal",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartBreak_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "meal",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartBreak_CreateFails(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetOpenAttendance succeeds
	mockDB.OnQueryRow(testutil.NewRow(attendanceLogScanValues()...))

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	// CreateBreakLog fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/start", gin.H{
		"break_type": "rest",
	}, adminAuth)

	h.StartBreak(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- EndBreak happy paths ---

func TestEndBreak_SuccessWithPolicy(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetActiveBreak succeeds (has active break)
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues("meal")...))

	// GetBreakPolicy succeeds
	mockDB.OnQueryRow(testutil.NewRow(breakPolicyScanValues("meal", 30)...))

	// EndBreakLog succeeds — return a completed break log with duration
	dur := int32(25)
	ot := int32(0)
	endedBreak := []interface{}{
		int64(200),
		int64(1),
		int64(1),
		int64(100),
		"meal",
		time.Now().Add(-25 * time.Minute),
		pgtype.Timestamptz{Time: time.Now(), Valid: true},
		&dur,
		&ot,
		(*string)(nil),
		time.Now(),
	}
	mockDB.OnQueryRow(testutil.NewRow(endedBreak...))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/end", nil, adminAuth)

	h.EndBreak(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEndBreak_SuccessNoPolicy(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetActiveBreak succeeds
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues("rest")...))

	// GetBreakPolicy fails (no policy for this type)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	// EndBreakLog succeeds — maxMinutes stays 0
	dur := int32(10)
	endedBreak := []interface{}{
		int64(200),
		int64(1),
		int64(1),
		int64(100),
		"rest",
		time.Now().Add(-10 * time.Minute),
		pgtype.Timestamptz{Time: time.Now(), Valid: true},
		&dur,
		(*int32)(nil),
		(*string)(nil),
		time.Now(),
	}
	mockDB.OnQueryRow(testutil.NewRow(endedBreak...))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/end", nil, adminAuth)

	h.EndBreak(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEndBreak_EndBreakLogFails(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetActiveBreak succeeds
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues("meal")...))

	// GetBreakPolicy succeeds
	mockDB.OnQueryRow(testutil.NewRow(breakPolicyScanValues("meal", 30)...))

	// EndBreakLog fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContext("POST", "/attendance/breaks/end", nil, adminAuth)

	h.EndBreak(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetActiveBreak success ---

func TestGetActiveBreak_Found(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// GetActiveBreak succeeds (has active break)
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues("bathroom")...))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks/active", nil, adminAuth)

	h.GetActiveBreak(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListBreaks happy paths ---

func TestListBreaks_SuccessWithDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// ListBreaksByDate returns a list of break logs
	rows := [][]interface{}{
		breakLogScanValues("meal"),
		breakLogScanValues("bathroom"),
	}
	mockDB.OnQuery(testutil.NewRows(rows), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks?date=2026-03-28", nil, adminAuth)

	h.ListBreaks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListBreaks_SuccessNoDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// ListBreaksByDate returns empty (no breaks today)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks", nil, adminAuth)

	h.ListBreaks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListBreaks_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID succeeds
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	// ListBreaksByDate fails
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/breaks?date=2026-03-28", nil, adminAuth)

	h.ListBreaks(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- parseDateRange ---

func TestParseDateRange_ValidDate(t *testing.T) {
	from, to, err := parseDateRange("2026-03-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if !from.Equal(expected) {
		t.Errorf("from = %v, want %v", from, expected)
	}
	if !to.Equal(expected.Add(24 * time.Hour)) {
		t.Errorf("to = %v, want %v", to, expected.Add(24*time.Hour))
	}
}

func TestParseDateRange_EmptyDefaults(t *testing.T) {
	from, to, err := parseDateRange("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if !from.Equal(today) {
		t.Errorf("from = %v, want %v", from, today)
	}
	if !to.Equal(today.Add(24 * time.Hour)) {
		t.Errorf("to = %v, want %v", to, today.Add(24*time.Hour))
	}
}

func TestParseDateRange_Invalid(t *testing.T) {
	_, _, err := parseDateRange("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

// --- RegisterRoutes ---

func TestRegisterRoutes(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api")
	h.RegisterRoutes(group)

	// Verify routes are registered by checking the routes list
	routes := r.Routes()
	expectedPaths := map[string]string{
		"POST /api/attendance/breaks/start":     "",
		"POST /api/attendance/breaks/end":       "",
		"GET /api/attendance/breaks":            "",
		"GET /api/attendance/breaks/active":     "",
		"GET /api/attendance/break-policies":    "",
		"PUT /api/attendance/break-policies":    "",
		"GET /api/attendance/report/monthly":    "",
	}
	for _, route := range routes {
		key := route.Method + " " + route.Path
		delete(expectedPaths, key)
	}
	if len(expectedPaths) > 0 {
		for k := range expectedPaths {
			t.Errorf("expected route not found: %s", k)
		}
	}
}
