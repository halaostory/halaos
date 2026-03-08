package analytics

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

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

// analyticsSummaryScanValues returns values in exact scan order for GetAnalyticsSummaryRow
// (5 fields: ActiveEmployees, SeparatedEmployees, NewHiresThisMonth, ProbationaryCount, AvgTenureYears).
func analyticsSummaryScanValues() []interface{} {
	return []interface{}{
		int64(50),            // ActiveEmployees
		int64(5),             // SeparatedEmployees
		int64(3),             // NewHiresThisMonth
		int64(8),             // ProbationaryCount
		pgtype.Numeric{},     // AvgTenureYears
	}
}

// headcountTrendScanValues returns values for GetHeadcountTrendRow
// (4 fields: Month, ActiveCount, SeparatedCount, TotalCount).
func headcountTrendScanValues() []interface{} {
	return []interface{}{
		"2025-06",  // Month
		int64(45),  // ActiveCount
		int64(2),   // SeparatedCount
		int64(47),  // TotalCount
	}
}

// turnoverStatsScanValues returns values for GetTurnoverStatsRow
// (3 fields: Month, Separations, ActiveCount).
func turnoverStatsScanValues() []interface{} {
	return []interface{}{
		"2025-06",  // Month
		int64(2),   // Separations
		int64(48),  // ActiveCount
	}
}

// departmentCostScanValues returns values for GetDepartmentCostAnalysisRow
// (3 fields: DepartmentName, EmployeeCount, TotalSalaryCost).
func departmentCostScanValues() []interface{} {
	return []interface{}{
		"Engineering", // DepartmentName
		int64(15),     // EmployeeCount
		int64(750000), // TotalSalaryCost (interface{})
	}
}

// employmentTypeScanValues returns values for GetEmploymentTypeBreakdownRow
// (2 fields: EmploymentType, Count).
func employmentTypeScanValues() []interface{} {
	return []interface{}{
		"regular",  // EmploymentType
		int64(40),  // Count
	}
}

// attendancePatternScanValues returns values for GetAttendancePatternsRow
// (4 fields: DayOfWeek, AvgHours, AvgLateMinutes, TotalRecords).
func attendancePatternScanValues() []interface{} {
	return []interface{}{
		int32(1),         // DayOfWeek (Monday)
		pgtype.Numeric{}, // AvgHours
		pgtype.Numeric{}, // AvgLateMinutes
		int64(100),       // TotalRecords
	}
}

// leaveUtilizationScanValues returns values for GetLeaveUtilizationRow
// (3 fields: LeaveType, TotalRequests, TotalDaysUsed).
func leaveUtilizationScanValues() []interface{} {
	return []interface{}{
		"Vacation Leave", // LeaveType
		int64(25),        // TotalRequests
		int64(50),        // TotalDaysUsed (interface{})
	}
}

// --- GetSummary Tests ---

func TestGetSummary_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(analyticsSummaryScanValues()...))

	c, w := testutil.NewGinContext("GET", "/analytics/summary", nil, adminAuth)
	h.GetSummary(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSummary_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContext("GET", "/analytics/summary", nil, adminAuth)
	h.GetSummary(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetHeadcountTrend Tests ---

func TestGetHeadcountTrend_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{headcountTrendScanValues()}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/headcount-trend", nil, adminAuth)
	h.GetHeadcountTrend(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetHeadcountTrend_WithMonthsParam(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{headcountTrendScanValues()}), nil)

	q := url.Values{"months": []string{"6"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/headcount-trend", q, adminAuth)
	h.GetHeadcountTrend(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetHeadcountTrend_InvalidMonthsParam(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Invalid months param defaults to 12
	mockDB.OnQuery(testutil.NewRows([][]interface{}{headcountTrendScanValues()}), nil)

	q := url.Values{"months": []string{"abc"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/headcount-trend", q, adminAuth)
	h.GetHeadcountTrend(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetHeadcountTrend_WithStartDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{headcountTrendScanValues()}), nil)

	q := url.Values{"start_date": []string{"2025-01-01"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/headcount-trend", q, adminAuth)
	h.GetHeadcountTrend(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetHeadcountTrend_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/headcount-trend", nil, adminAuth)
	h.GetHeadcountTrend(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetHeadcountTrend_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/headcount-trend", nil, adminAuth)
	h.GetHeadcountTrend(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetTurnoverStats Tests ---

func TestGetTurnoverStats_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{turnoverStatsScanValues()}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/turnover", nil, adminAuth)
	h.GetTurnoverStats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTurnoverStats_WithStartDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{turnoverStatsScanValues()}), nil)

	q := url.Values{"start_date": []string{"2025-03-01"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/turnover", q, adminAuth)
	h.GetTurnoverStats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTurnoverStats_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/turnover", nil, adminAuth)
	h.GetTurnoverStats(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetDepartmentCosts Tests ---

func TestGetDepartmentCosts_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{departmentCostScanValues()}), nil)

	c, w := testutil.NewGinContext("GET", "/analytics/department-costs", nil, adminAuth)
	h.GetDepartmentCosts(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDepartmentCosts_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContext("GET", "/analytics/department-costs", nil, adminAuth)
	h.GetDepartmentCosts(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDepartmentCosts_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContext("GET", "/analytics/department-costs", nil, adminAuth)
	h.GetDepartmentCosts(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetAttendancePatterns Tests ---

func TestGetAttendancePatterns_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{attendancePatternScanValues()}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/attendance-patterns", nil, adminAuth)
	h.GetAttendancePatterns(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetAttendancePatterns_WithStartDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{attendancePatternScanValues()}), nil)

	q := url.Values{"start_date": []string{"2025-01-01"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/attendance-patterns", q, adminAuth)
	h.GetAttendancePatterns(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetAttendancePatterns_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/attendance-patterns", nil, adminAuth)
	h.GetAttendancePatterns(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetEmploymentTypeBreakdown Tests ---

func TestGetEmploymentTypeBreakdown_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{employmentTypeScanValues()}), nil)

	c, w := testutil.NewGinContext("GET", "/analytics/employment-types", nil, adminAuth)
	h.GetEmploymentTypeBreakdown(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetEmploymentTypeBreakdown_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContext("GET", "/analytics/employment-types", nil, adminAuth)
	h.GetEmploymentTypeBreakdown(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetLeaveUtilization Tests ---

func TestGetLeaveUtilization_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{leaveUtilizationScanValues()}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/leave-utilization", nil, adminAuth)
	h.GetLeaveUtilization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLeaveUtilization_WithYearParam(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewRows([][]interface{}{leaveUtilizationScanValues()}), nil)

	q := url.Values{"year": []string{"2024"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/leave-utilization", q, adminAuth)
	h.GetLeaveUtilization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLeaveUtilization_InvalidYearParam(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Invalid year defaults to current year
	mockDB.OnQuery(testutil.NewRows([][]interface{}{leaveUtilizationScanValues()}), nil)

	q := url.Values{"year": []string{"abc"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/leave-utilization", q, adminAuth)
	h.GetLeaveUtilization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLeaveUtilization_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/leave-utilization", nil, adminAuth)
	h.GetLeaveUtilization(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ExportCSV Tests ---

func TestExportCSV_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAnalyticsSummary (QueryRow)
	mockDB.OnQueryRow(testutil.NewRow(analyticsSummaryScanValues()...))
	// GetHeadcountTrend (Query)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{headcountTrendScanValues()}), nil)
	// GetTurnoverStats (Query)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{turnoverStatsScanValues()}), nil)
	// GetDepartmentCostAnalysis (Query)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{departmentCostScanValues()}), nil)
	// GetEmploymentTypeBreakdown (Query)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{employmentTypeScanValues()}), nil)
	// GetLeaveUtilization (Query)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{leaveUtilizationScanValues()}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/export", nil, adminAuth)
	h.ExportCSV(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Fatalf("expected Content-Type text/csv, got %q", contentType)
	}

	disposition := w.Header().Get("Content-Disposition")
	if disposition == "" {
		t.Fatal("expected Content-Disposition header")
	}

	body := w.Body.String()
	if body == "" {
		t.Fatal("expected non-empty CSV body")
	}
}

func TestExportCSV_AllDBErrors(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// All queries fail - ExportCSV should still return 200 (writes partial CSV)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("summary failed")))
	mockDB.OnQuery(nil, fmt.Errorf("trend failed"))
	mockDB.OnQuery(nil, fmt.Errorf("turnover failed"))
	mockDB.OnQuery(nil, fmt.Errorf("costs failed"))
	mockDB.OnQuery(nil, fmt.Errorf("types failed"))
	mockDB.OnQuery(nil, fmt.Errorf("leave failed"))

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/export", nil, adminAuth)
	h.ExportCSV(c)

	// ExportCSV streams to response, so status is 200 regardless of DB errors
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportCSV_EmptyData(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetAnalyticsSummary succeeds
	mockDB.OnQueryRow(testutil.NewRow(analyticsSummaryScanValues()...))
	// All :many queries return empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil) // headcount trend
	mockDB.OnQuery(testutil.NewEmptyRows(), nil) // turnover
	mockDB.OnQuery(testutil.NewEmptyRows(), nil) // dept costs
	mockDB.OnQuery(testutil.NewEmptyRows(), nil) // employment types
	mockDB.OnQuery(testutil.NewEmptyRows(), nil) // leave utilization

	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/export", nil, adminAuth)
	h.ExportCSV(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	// Should at least have the summary section
	if body == "" {
		t.Fatal("expected non-empty CSV body with summary section")
	}
}

func TestExportCSV_WithYearParam(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(analyticsSummaryScanValues()...))
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	q := url.Values{"year": []string{"2024"}}
	c, w := testutil.NewGinContextWithQuery("GET", "/analytics/export", q, adminAuth)
	h.ExportCSV(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- parseStartDate Tests ---

func TestParseStartDate_ValidDate(t *testing.T) {
	q := url.Values{"start_date": []string{"2025-03-15"}}
	c, _ := testutil.NewGinContextWithQuery("GET", "/test", q, adminAuth)

	fallback := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	result := parseStartDate(c, fallback)

	expected := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseStartDate_InvalidDate_ReturnsFallback(t *testing.T) {
	q := url.Values{"start_date": []string{"not-a-date"}}
	c, _ := testutil.NewGinContextWithQuery("GET", "/test", q, adminAuth)

	fallback := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	result := parseStartDate(c, fallback)

	if !result.Equal(fallback) {
		t.Fatalf("expected fallback %v, got %v", fallback, result)
	}
}

func TestParseStartDate_NoParam_ReturnsFallback(t *testing.T) {
	c, _ := testutil.NewGinContextWithQuery("GET", "/test", nil, adminAuth)

	fallback := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	result := parseStartDate(c, fallback)

	if !result.Equal(fallback) {
		t.Fatalf("expected fallback %v, got %v", fallback, result)
	}
}
