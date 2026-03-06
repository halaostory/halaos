package selfservice

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

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

func TestGetMyInfo_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeFullInfo returns error -> returns nil (graceful)
	mockDB.OnQueryRow(testutil.NewErrorRow(nil)) // nil error but scan may fail

	c, w := testutil.NewGinContextWithQuery("GET", "/self/info", nil, adminAuth)
	h.GetMyInfo(c)

	// Should return 200 regardless (graceful handler)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetMyTeam_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetEmployeeByUserID fails
	mockDB.OnQueryRow(testutil.NewErrorRow(nil))

	c, w := testutil.NewGinContextWithQuery("GET", "/self/team", nil, adminAuth)
	h.GetMyTeam(c)

	// Returns 200 with defaults on not found
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
