package dashboard

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

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

func TestGetStats_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// CountEmployees
	mockDB.OnQueryRow(testutil.NewRow(int64(10)))
	// GetTodayAttendanceSummary
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// CountLeaveRequests
	mockDB.OnQueryRow(testutil.NewRow(int64(3)))
	// CountOvertimeRequests
	mockDB.OnQueryRow(testutil.NewRow(int64(1)))

	c, w := testutil.NewGinContextWithQuery("GET", "/dashboard/stats", nil, adminAuth)
	h.GetStats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetAttendance_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/dashboard/attendance", nil, adminAuth)
	h.GetAttendance(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
