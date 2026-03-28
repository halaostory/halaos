package breaks

import (
	"net/http"
	"net/url"
	"testing"

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
