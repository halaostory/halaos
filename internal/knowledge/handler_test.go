package knowledge

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

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

func TestSearch_MissingQuery(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithQuery("GET", "/knowledge/search", nil, adminAuth)
	h.Search(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// Both primary search AND ILIKE fallback will fail, so all queries error.
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/knowledge/search", url.Values{
		"q": {"test"},
	}, adminAuth)
	h.Search(c)
	// With ILIKE fallback, a primary DB error still returns 200 with empty results
	// if the fallback also fails. The handler gracefully returns empty results.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGet_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithParams("GET", "/knowledge/1",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.Get(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestList_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/knowledge", nil, adminAuth)
	h.List(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
