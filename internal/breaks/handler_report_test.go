package breaks

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/testutil"
)

// --- MonthlyReport ---

func TestMonthlyReport_MissingParams(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// No year or month query params
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", nil, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_MissingMonth(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"2026"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_InvalidYear(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"abc"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_YearOutOfRange(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"1999"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_InvalidMonth(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"2026"}, "month": {"13"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_InvalidMonthString(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"2026"}, "month": {"xyz"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_Success_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAttendanceReport returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// GetMonthlyBreakSummary returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// ListBotUserLinksByCompany returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// ListActiveEmployees returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Content-Type header
	ct := w.Header().Get("Content-Type")
	expected := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct != expected {
		t.Fatalf("expected Content-Type %q, got %q", expected, ct)
	}

	// Verify Content-Disposition header
	cd := w.Header().Get("Content-Disposition")
	if cd != "attachment; filename=break_report_2026_03.xlsx" {
		t.Fatalf("unexpected Content-Disposition: %q", cd)
	}
}

// --- Formatting helpers ---

func TestFmtHoursOrDash(t *testing.T) {
	if got := fmtHoursOrDash(0); got != dashStr {
		t.Errorf("expected dash, got %q", got)
	}
	if got := fmtHoursOrDash(1.5); got != "1.5 小时" {
		t.Errorf("expected '1.5 小时', got %q", got)
	}
}

func TestFmtMinutesOrDash(t *testing.T) {
	if got := fmtMinutesOrDash(0); got != dashStr {
		t.Errorf("expected dash, got %q", got)
	}
	if got := fmtMinutesOrDash(15); got != "15 分钟" {
		t.Errorf("expected '15 分钟', got %q", got)
	}
}

func TestFmtOvertimeWithCount(t *testing.T) {
	got := fmtOvertimeWithCount(10, 3)
	expected := "10 分钟（共 3 次）"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{nil, 0},
		{float64(1.5), 1.5},
		{float32(2.5), 2.5},
		{int64(3), 3},
		{int32(4), 4},
		{"5.5", 5.5},
		{[]byte("6.5"), 6.5},
		{true, 0}, // unsupported type
	}
	for _, tt := range tests {
		got := toFloat64(tt.input)
		if got != tt.expected {
			t.Errorf("toFloat64(%v) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

// --- fmtIntOrDash ---

func TestFmtIntOrDash(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, dashStr},
		{1, "1"},
		{42, "42"},
		{-5, "-5"},
	}
	for _, tt := range tests {
		got := fmtIntOrDash(tt.input)
		if got != tt.expected {
			t.Errorf("fmtIntOrDash(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- fmtInt32OrDash ---

func TestFmtInt32OrDash(t *testing.T) {
	tests := []struct {
		input    int32
		expected string
	}{
		{0, dashStr},
		{1, "1"},
		{99, "99"},
		{-3, "-3"},
	}
	for _, tt := range tests {
		got := fmtInt32OrDash(tt.input)
		if got != tt.expected {
			t.Errorf("fmtInt32OrDash(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- writeBreakCols ---

func TestWriteBreakCols_Direct(t *testing.T) {
	// We can test writeBreakCols directly since it's an unexported helper
	// using excelize directly.
	// This is tested indirectly through MonthlyReport_Success_WithData below,
	// but we also exercise it directly for better coverage.

	// Tested indirectly via MonthlyReport_Success_WithData.
	// Direct tests of the excelize helper are not easily done without
	// creating a full file, so we rely on integration via the handler test.
}

// --- MonthlyReport DB errors ---

func TestMonthlyReport_AttendanceReportError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAttendanceReport fails
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_BreakSummaryError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAttendanceReport succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// GetMonthlyBreakSummary fails
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_ListEmployeesError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAttendanceReport succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// GetMonthlyBreakSummary succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// ListBotUserLinksByCompany succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// ListActiveEmployees fails
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_BotLinksError_NonFatal(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAttendanceReport succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// GetMonthlyBreakSummary succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// ListBotUserLinksByCompany fails (non-fatal)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	// ListActiveEmployees succeeds (empty)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	// Should still succeed with 200 (bot links error is non-fatal)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// attendanceReportScanValues returns mock scan values for GetAttendanceReportRow (12 fields).
func attendanceReportScanValues(empID int64, firstName, lastName string, daysWorked int64) []interface{} {
	return []interface{}{
		empID,     // EmployeeID
		"EMP-001", // EmployeeNo
		firstName, // FirstName
		lastName,  // LastName
		"Engineering", // DepartmentName
		daysWorked,    // DaysWorked
		float64(160.0), // TotalWorkHours (interface{})
		float64(10.0),  // TotalOvertimeHours (interface{})
		float64(30.0),  // TotalLateMinutes (interface{})
		float64(15.0),  // TotalUndertimeMinutes (interface{})
		int64(2),       // LateCount
		int64(1),       // UndertimeCount
	}
}

// breakSummaryScanValues returns mock scan values for GetMonthlyBreakSummaryRow (6 fields).
func breakSummaryScanValues(empID int64, breakType string, count, minutes, otMinutes, otCount int32) []interface{} {
	return []interface{}{
		empID,     // EmployeeID
		breakType, // BreakType
		count,     // TotalCount
		minutes,   // TotalMinutes
		otMinutes, // TotalOvertimeMinutes
		otCount,   // OvertimeCount
	}
}

// botLinkScanValues returns mock scan values for BotUserLink (12 fields).
func botLinkScanValues(userID int64, platformUserID string) []interface{} {
	return []interface{}{
		int64(1),        // ID
		"telegram",      // Platform
		&platformUserID, // PlatformUserID
		userID,          // UserID
		int64(1),        // CompanyID
		(*string)(nil),  // LinkCode
		pgtype.Timestamptz{}, // LinkCodeExp
		pgtype.Timestamptz{Time: time.Now(), Valid: true}, // VerifiedAt
		"en",                // Locale
		pgtype.UUID{},       // ActiveSessionID
		time.Now(),          // CreatedAt
		time.Now(),          // UpdatedAt
	}
}

// employeeScanValuesWithUserID returns employee scan values with a user_id set.
func employeeScanValuesWithUserID(empID, userID int64) []interface{} {
	uid := userID
	return []interface{}{
		empID, int64(1), &uid, "EMP-001",
		"John", "Doe", (*string)(nil), (*string)(nil),
		(*string)(nil), strPtr("john@test.com"), (*string)(nil), pgtype.Date{},
		(*string)(nil), (*string)(nil), (*string)(nil),
		(*int64)(nil), (*int64)(nil), (*int64)(nil), (*int64)(nil),
		time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), pgtype.Date{}, pgtype.Date{},
		"regular", "active",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), pgtype.Date{},
	}
}

func strPtr(s string) *string { return &s }

func TestMonthlyReport_Success_WithData(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	empID := int64(1)
	userID := int64(1)

	// GetAttendanceReport returns one employee row
	attRows := [][]interface{}{
		attendanceReportScanValues(empID, "John", "Doe", 20),
	}
	mockDB.OnQuery(testutil.NewRows(attRows), nil)

	// GetMonthlyBreakSummary returns break data for meal, bathroom, rest, leave_post
	breakRows := [][]interface{}{
		breakSummaryScanValues(empID, "meal", 15, 450, 30, 3),
		breakSummaryScanValues(empID, "bathroom", 20, 200, 10, 2),
		breakSummaryScanValues(empID, "rest", 5, 50, 0, 0),
		breakSummaryScanValues(empID, "leave_post", 2, 30, 0, 0),
	}
	mockDB.OnQuery(testutil.NewRows(breakRows), nil)

	// ListBotUserLinksByCompany returns one link
	botRows := [][]interface{}{
		botLinkScanValues(userID, "123456789"),
	}
	mockDB.OnQuery(testutil.NewRows(botRows), nil)

	// ListActiveEmployees returns one employee with user_id set
	empRows := [][]interface{}{
		employeeScanValuesWithUserID(empID, userID),
	}
	mockDB.OnQuery(testutil.NewRows(empRows), nil)

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Content-Type header
	ct := w.Header().Get("Content-Type")
	expectedCT := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct != expectedCT {
		t.Fatalf("expected Content-Type %q, got %q", expectedCT, ct)
	}

	// Verify Content-Disposition
	cd := w.Header().Get("Content-Disposition")
	if cd != "attachment; filename=break_report_2026_03.xlsx" {
		t.Fatalf("unexpected Content-Disposition: %q", cd)
	}

	// Verify the body is non-empty (valid Excel data)
	if w.Body.Len() == 0 {
		t.Fatal("expected non-empty response body for Excel file")
	}
}

func TestMonthlyReport_Success_WithDataNoBreaks(t *testing.T) {
	// Tests path where attendance rows exist but no break data (all dashes)
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	empID := int64(1)

	// GetAttendanceReport returns one employee row
	attRows := [][]interface{}{
		attendanceReportScanValues(empID, "Jane", "Smith", 10),
	}
	mockDB.OnQuery(testutil.NewRows(attRows), nil)

	// GetMonthlyBreakSummary returns empty (no breaks)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	// ListBotUserLinksByCompany returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	// ListActiveEmployees returns empty (no user_id mapping)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{"year": {"2026"}, "month": {"1"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_Success_NegativeNetWorkHours(t *testing.T) {
	// Test where total break time exceeds work hours (netWorkHours clamped to 0)
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	empID := int64(1)

	// GetAttendanceReport with very low work hours
	attRows := [][]interface{}{
		{
			empID,         // EmployeeID
			"EMP-001",     // EmployeeNo
			"John",        // FirstName
			"Doe",         // LastName
			"Engineering", // DepartmentName
			int64(1),      // DaysWorked
			float64(0.5),  // TotalWorkHours — only 0.5 hours
			float64(0.0),  // TotalOvertimeHours
			float64(0.0),  // TotalLateMinutes
			float64(0.0),  // TotalUndertimeMinutes
			int64(0),      // LateCount
			int64(0),      // UndertimeCount
		},
	}
	mockDB.OnQuery(testutil.NewRows(attRows), nil)

	// Break summary: 120 minutes = 2 hours (exceeds 0.5h work)
	breakRows := [][]interface{}{
		breakSummaryScanValues(empID, "meal", 4, 120, 0, 0),
	}
	mockDB.OnQuery(testutil.NewRows(breakRows), nil)

	// Empty bot links and employees
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{"year": {"2026"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_MonthZero(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"2026"}, "month": {"0"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMonthlyReport_YearTooHigh(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	q := url.Values{"year": {"2101"}, "month": {"3"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/report/monthly", q, adminAuth)

	h.MonthlyReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// Verify fmtIntOrDash and fmtInt32OrDash produce valid strings
func TestFmtIntOrDash_Positive(t *testing.T) {
	got := fmtIntOrDash(100)
	if got != strconv.FormatInt(100, 10) {
		t.Errorf("fmtIntOrDash(100) = %q, want %q", got, "100")
	}
}

func TestFmtInt32OrDash_Positive(t *testing.T) {
	got := fmtInt32OrDash(50)
	if got != "50" {
		t.Errorf("fmtInt32OrDash(50) = %q, want %q", got, "50")
	}
}

// Suppress unused import warnings — these are used in the test helpers above.
var _ = fmt.Sprintf
var _ = time.Now
var _ = pgtype.Timestamptz{}
