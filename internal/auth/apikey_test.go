package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

func TestGenerateAPIKey_Format(t *testing.T) {
	key := generateAPIKey()
	if !strings.HasPrefix(key, "halaos_") {
		t.Errorf("key should start with halaos_, got %q", key[:7])
	}
	if len(key) != 47 {
		t.Errorf("key should be 47 chars (halaos_ + 40 hex), got %d", len(key))
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := generateAPIKey()
		if seen[key] {
			t.Fatalf("duplicate key generated on iteration %d", i)
		}
		seen[key] = true
	}
}

func TestAPIKeyPrefix(t *testing.T) {
	key := "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	prefix := apiKeyPrefix(key)
	if prefix != "halaos_a1b2c3" {
		t.Errorf("expected halaos_a1b2c3, got %q", prefix)
	}
}

func TestAPIKeyPrefix_ShortKey(t *testing.T) {
	prefix := apiKeyPrefix("short")
	if prefix != "short" {
		t.Errorf("expected short, got %q", prefix)
	}
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	key := "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	h1 := hashAPIKey(key)
	h2 := hashAPIKey(key)
	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(h1))
	}
}

func TestHashAPIKey_DifferentKeys(t *testing.T) {
	h1 := hashAPIKey("halaos_aaaa")
	h2 := hashAPIKey("halaos_bbbb")
	if h1 == h2 {
		t.Error("different keys should produce different hashes")
	}
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)

	key := generateAPIKey()

	// Mock GetAPIKeyByHash :one — return valid result
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1),             // ak.id
		int64(1),             // ak.user_id
		int64(1),             // ak.company_id
		"halaos_a1b",         // ak.prefix
		"test-key",           // ak.name
		true,                 // ak.is_active
		pgtype.Timestamptz{}, // ak.last_used_at
		time.Now(),           // ak.created_at (time.Time, not pgtype)
		"admin",              // u.role
		"admin@test.com",     // u.email
	))
	// Mock TouchAPIKeyLastUsed :exec — async, best-effort
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+key)

	middleware := APIKeyMiddleware(queries)
	middleware(c)

	// Give goroutine time to run TouchAPIKeyLastUsed
	time.Sleep(10 * time.Millisecond)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("expected success, got 401")
	}
	if v, _ := c.Get(ContextKeyUserID); v != int64(1) {
		t.Errorf("expected user_id=1, got %v", v)
	}
	if v, _ := c.Get(ContextKeyCompanyID); v != int64(1) {
		t.Errorf("expected company_id=1, got %v", v)
	}
}

func TestAPIKeyMiddleware_NonAPIKeyToken_Passthrough(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.jwt-token")

	middleware := APIKeyMiddleware(queries)
	middleware(c)

	// Should NOT set user_id (passthrough to JWT middleware)
	_, exists := c.Get(ContextKeyUserID)
	if exists {
		t.Error("non-API-key token should not set user_id")
	}
}

func TestAPIKeyMiddleware_InvalidKey_401(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)

	// Mock GetAPIKeyByHash :one — return not found error
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer halaos_invalid_key_that_does_not_exist_1234567")

	middleware := APIKeyMiddleware(queries)
	middleware(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_NoHeader_Passthrough(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	// No Authorization header

	middleware := APIKeyMiddleware(queries)
	middleware(c)

	// Should pass through without error
	if w.Code == http.StatusUnauthorized {
		t.Error("no header should passthrough, not 401")
	}
}

// --- Handler Tests ---

func TestCreateAPIKey_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock CreateAPIKey :one — returns the created row
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1),              // id
		"halaos_a1b2c3",      // prefix
		"My Key",             // name
		true,                 // is_active
		pgtype.Timestamptz{}, // last_used_at
		time.Now(),           // created_at (time.Time)
	))

	c, w := testutil.NewGinContext("POST", "/api-keys", map[string]string{"name": "My Key"}, adminAuth)
	h.CreateAPIKey(c)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	data := body["data"].(map[string]interface{})
	key, ok := data["key"].(string)
	if !ok || !strings.HasPrefix(key, "halaos_") {
		t.Errorf("response should include raw key starting with halaos_, got %v", data["key"])
	}
}

func TestCreateAPIKey_MissingName(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock CreateAPIKey :one — should use "default" as name
	mockDB.OnQueryRow(testutil.NewRow(
		int64(2),              // id
		"halaos_x9y8z7",      // prefix
		"default",            // name (auto-filled)
		true,                 // is_active
		pgtype.Timestamptz{}, // last_used_at
		time.Now(),           // created_at
	))

	c, w := testutil.NewGinContext("POST", "/api-keys", map[string]string{}, adminAuth)
	h.CreateAPIKey(c)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListAPIKeys_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock ListAPIKeysByUser :many — return 1 row
	mockDB.OnQuery(testutil.NewRows([][]interface{}{
		{int64(1), "halaos_a1b2c3", "Key 1", true, pgtype.Timestamptz{}, time.Now()},
	}), nil)

	c, w := testutil.NewGinContext("GET", "/api-keys", nil, adminAuth)
	h.ListAPIKeys(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRevokeAPIKey_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock RevokeAPIKey :exec
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	c, w := testutil.NewGinContextWithParams("DELETE", "/api-keys/1",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)
	h.RevokeAPIKey(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRevokeAPIKey_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("DELETE", "/api-keys/abc",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.RevokeAPIKey(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
