# OpenClaw API Key Authentication — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add API Key authentication so all 150+ existing HalaOS endpoints are accessible via `Authorization: Bearer halaos_...` tokens for OpenClaw integration.

**Architecture:** A new `APIKeyMiddleware` is inserted before the existing `JWTMiddleware` in the middleware chain. If the Bearer token starts with `halaos_`, it authenticates via SHA-256 hash lookup in the `api_keys` table and sets the same context keys (`user_id`, `role`, `company_id`) as JWT. All existing handlers work without modification.

**Tech Stack:** Go 1.25, Gin, sqlc (pgx/v5), PostgreSQL, SHA-256 hashing

**Spec:** `docs/superpowers/specs/2026-03-24-openclaw-api-key-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `db/migrations/00082_api_keys.sql` | Create | Table schema + indexes |
| `db/query/api_keys.sql` | Create | 5 sqlc queries |
| `internal/store/` (generated) | Regenerate | sqlc output |
| `internal/auth/apikey.go` | Create | Key generation, hashing, middleware, 3 CRUD handlers (~150 LOC) |
| `internal/auth/apikey_test.go` | Create | Unit tests for key gen, hash, middleware, CRUD handlers |
| `internal/auth/middleware.go` | Modify | Add early return at line 18 (if user_id already set) |
| `internal/auth/routes.go` | Modify | Add 3 API Key routes to protected group |
| `internal/app/bootstrap.go` | Modify | Insert APIKeyMiddleware at line 344 (before JWTMiddleware) |

---

### Task 1: Database Migration

**Files:**
- Create: `db/migrations/00082_api_keys.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- db/migrations/00082_api_keys.sql

CREATE TABLE IF NOT EXISTS api_keys (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id   BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    prefix       VARCHAR(20) NOT NULL,
    key_hash     VARCHAR(64) NOT NULL,
    name         VARCHAR(100) NOT NULL DEFAULT 'default',
    is_active    BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id) WHERE is_active = true;
```

- [ ] **Step 2: Commit**

```bash
git add db/migrations/00082_api_keys.sql
git commit -m "feat: add api_keys table migration (00082)"
```

---

### Task 2: sqlc Queries

**Files:**
- Create: `db/query/api_keys.sql`
- Regenerate: `internal/store/` (via `sqlc generate`)

- [ ] **Step 1: Write the sqlc query file**

```sql
-- db/query/api_keys.sql

-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, company_id, prefix, key_hash, name)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, prefix, name, is_active, last_used_at, created_at;

-- name: GetAPIKeyByHash :one
SELECT ak.id, ak.user_id, ak.company_id, ak.prefix, ak.name,
       ak.is_active, ak.last_used_at, ak.created_at, u.role, u.email
FROM api_keys ak
JOIN users u ON u.id = ak.user_id
WHERE ak.key_hash = $1 AND ak.is_active = true;

-- name: ListAPIKeysByUser :many
SELECT id, prefix, name, is_active, last_used_at, created_at
FROM api_keys
WHERE user_id = $1 AND is_active = true
ORDER BY created_at DESC;

-- name: RevokeAPIKey :exec
UPDATE api_keys SET is_active = false WHERE id = $1 AND user_id = $2;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;
```

- [ ] **Step 2: Run sqlc generate**

```bash
~/go/bin/sqlc generate
```

Expected: No errors, new file `internal/store/api_keys.sql.go` generated.

- [ ] **Step 3: Verify generated code compiles**

```bash
go build ./internal/store/...
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add db/query/api_keys.sql internal/store/
git commit -m "feat: add sqlc queries for api_keys"
```

---

### Task 3: API Key Utility Functions + Tests (TDD)

**Files:**
- Create: `internal/auth/apikey.go`
- Create: `internal/auth/apikey_test.go`

- [ ] **Step 1: Write failing tests for utility functions**

Create `internal/auth/apikey_test.go`:

```go
package auth

import (
	"strings"
	"testing"
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
	if prefix != "halaos_a1b2c" {
		t.Errorf("expected halaos_a1b2c, got %q", prefix)
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
```

- [ ] **Step 2: Run tests — they should fail**

```bash
go test ./internal/auth/ -run TestGenerateAPIKey -v
go test ./internal/auth/ -run TestAPIKeyPrefix -v
go test ./internal/auth/ -run TestHashAPIKey -v
```

Expected: FAIL — functions not defined.

- [ ] **Step 3: Implement utility functions**

Create `internal/auth/apikey.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

const apiKeyPrefixStr = "halaos_"

// generateAPIKey returns "halaos_" + 40 random hex chars (20 bytes).
func generateAPIKey() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return apiKeyPrefixStr + hex.EncodeToString(b)
}

// apiKeyPrefix returns the first 13 chars for display (e.g. "halaos_a1b2c3").
func apiKeyPrefix(key string) string {
	if len(key) <= 13 {
		return key
	}
	return key[:13]
}

// hashAPIKey returns the SHA-256 hex digest of the key.
func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 4: Run tests — they should pass**

```bash
go test ./internal/auth/ -run "TestGenerateAPIKey|TestAPIKeyPrefix|TestHashAPIKey" -v
```

Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/apikey.go internal/auth/apikey_test.go
git commit -m "feat: add API key generation and hashing utilities"
```

---

### Task 4: API Key Middleware + Tests (TDD)

**Files:**
- Modify: `internal/auth/apikey.go` (add middleware)
- Modify: `internal/auth/apikey_test.go` (add middleware tests)
- Modify: `internal/auth/middleware.go:17-18` (add early return)

- [ ] **Step 1: Write failing test for middleware — valid API key**

Add to `internal/auth/apikey_test.go`:

```go
func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)

	key := generateAPIKey()
	hash := hashAPIKey(key)

	// Mock GetAPIKeyByHash :one — return valid result
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1),     // ak.id
		int64(1),     // ak.user_id
		int64(1),     // ak.company_id
		"halaos_a1b", // ak.prefix
		"test-key",   // ak.name
		true,         // ak.is_active
		pgtype.Timestamptz{}, // ak.last_used_at
		pgtype.Timestamptz{Valid: true, Time: time.Now()}, // ak.created_at
		"admin",      // u.role
		"admin@test.com", // u.email
	))
	// Mock TouchAPIKeyLastUsed :exec — async, best-effort
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer "+key)

	called := false
	middleware := APIKeyMiddleware(queries)
	c.Set("_gin_handler_chain", []gin.HandlerFunc{
		middleware,
		func(c *gin.Context) { called = true },
	})
	middleware(c)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("expected success, got 401")
	}
	if v, _ := c.Get(ContextKeyUserID); v != int64(1) {
		t.Errorf("expected user_id=1, got %v", v)
	}
	if v, _ := c.Get(ContextKeyCompanyID); v != int64(1) {
		t.Errorf("expected company_id=1, got %v", v)
	}

	_ = hash // used in middleware internally
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
```

Also add imports at top of test file:

```go
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
```

- [ ] **Step 2: Run tests — they should fail**

```bash
go test ./internal/auth/ -run "TestAPIKeyMiddleware" -v
```

Expected: FAIL — `APIKeyMiddleware` not defined.

- [ ] **Step 3: Implement APIKeyMiddleware**

Add to `internal/auth/apikey.go`:

```go
import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/store"
)

// APIKeyMiddleware checks for "halaos_" prefixed Bearer tokens.
// If found and valid, sets user context. Otherwise falls through to next middleware (JWT).
func APIKeyMiddleware(queries *store.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" || !strings.HasPrefix(token, apiKeyPrefixStr) {
			c.Next()
			return
		}

		hash := hashAPIKey(token)
		ak, err := queries.GetAPIKeyByHash(c.Request.Context(), hash)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "invalid_api_key", "message": "Invalid or revoked API key"},
			})
			return
		}

		c.Set(ContextKeyUserID, ak.UserID)
		c.Set(ContextKeyEmail, ak.Email)
		c.Set(ContextKeyRole, Role(ak.Role))
		c.Set(ContextKeyCompanyID, ak.CompanyID)

		// Async update last_used_at (best-effort)
		go func() {
			_ = queries.TouchAPIKeyLastUsed(c.Request.Context(), ak.ID)
		}()

		c.Next()
	}
}
```

- [ ] **Step 4: Modify JWTMiddleware — add early return**

In `internal/auth/middleware.go`, add early return at line 18 (inside the returned func, before `extractBearerToken`):

```go
func JWTMiddleware(jwtSvc *JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip JWT validation if already authenticated (e.g. by APIKeyMiddleware)
		if _, exists := c.Get(ContextKeyUserID); exists {
			c.Next()
			return
		}

		tokenStr := extractBearerToken(c)
		// ... rest unchanged
```

- [ ] **Step 5: Run tests — they should pass**

```bash
go test ./internal/auth/ -run "TestAPIKeyMiddleware" -v
```

Expected: All 4 middleware tests PASS.

- [ ] **Step 6: Run existing auth tests to verify no regression**

```bash
go test ./internal/auth/ -v
```

Expected: All existing tests still PASS (middleware early return does not affect JWT flow).

- [ ] **Step 7: Commit**

```bash
git add internal/auth/apikey.go internal/auth/apikey_test.go internal/auth/middleware.go
git commit -m "feat: add APIKeyMiddleware with JWT fallthrough"
```

---

### Task 5: API Key CRUD Handlers + Tests (TDD)

**Files:**
- Modify: `internal/auth/apikey.go` (add 3 handlers)
- Modify: `internal/auth/apikey_test.go` (add handler tests)

- [ ] **Step 1: Write failing tests for CRUD handlers**

Add to `internal/auth/apikey_test.go`:

```go
func TestCreateAPIKey_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock CreateAPIKey :one — returns the created row
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1),     // id
		"halaos_a1b", // prefix
		"My Key",     // name
		true,         // is_active
		pgtype.Timestamptz{}, // last_used_at
		pgtype.Timestamptz{Valid: true, Time: time.Now()}, // created_at
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
		int64(2),       // id
		"halaos_x9y8z", // prefix
		"default",      // name (auto-filled)
		true,           // is_active
		pgtype.Timestamptz{}, // last_used_at
		pgtype.Timestamptz{Valid: true, Time: time.Now()}, // created_at
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
		{int64(1), "halaos_a1b", "Key 1", true, pgtype.Timestamptz{}, pgtype.Timestamptz{Valid: true, Time: time.Now()}},
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
```

- [ ] **Step 2: Run tests — they should fail**

```bash
go test ./internal/auth/ -run "TestCreateAPIKey|TestListAPIKeys|TestRevokeAPIKey" -v
```

Expected: FAIL — handlers not defined.

- [ ] **Step 3: Implement CRUD handlers**

Add to `internal/auth/apikey.go`:

```go
import (
	"strconv"

	"github.com/tonypk/aigonhr/pkg/response"
)

type createAPIKeyRequest struct {
	Name string `json:"name"`
}

// CreateAPIKey handles POST /api-keys
func (h *Handler) CreateAPIKey(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body — use default name
	}
	if req.Name == "" {
		req.Name = "default"
	}

	key := generateAPIKey()
	prefix := apiKeyPrefix(key)
	hash := hashAPIKey(key)

	userID := GetUserID(c)
	companyID := GetCompanyID(c)

	row, err := h.queries.CreateAPIKey(c.Request.Context(), store.CreateAPIKeyParams{
		UserID:    userID,
		CompanyID: companyID,
		Prefix:    prefix,
		KeyHash:   hash,
		Name:      req.Name,
	})
	if err != nil {
		h.logger.Error("failed to create API key", "error", err)
		response.InternalError(c, "Failed to create API key")
		return
	}

	response.Created(c, gin.H{
		"id":         row.ID,
		"prefix":     row.Prefix,
		"name":       row.Name,
		"key":        key, // Raw key — shown only once
		"created_at": row.CreatedAt.Time,
	})
}

// ListAPIKeys handles GET /api-keys
func (h *Handler) ListAPIKeys(c *gin.Context) {
	userID := GetUserID(c)

	keys, err := h.queries.ListAPIKeysByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list API keys", "error", err)
		response.InternalError(c, "Failed to list API keys")
		return
	}

	result := make([]gin.H, len(keys))
	for i, k := range keys {
		result[i] = gin.H{
			"id":           k.ID,
			"prefix":       k.Prefix,
			"name":         k.Name,
			"created_at":   k.CreatedAt.Time,
			"last_used_at": nil,
		}
		if k.LastUsedAt.Valid {
			result[i]["last_used_at"] = k.LastUsedAt.Time
		}
	}

	response.OK(c, result)
}

// RevokeAPIKey handles DELETE /api-keys/:id
func (h *Handler) RevokeAPIKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	userID := GetUserID(c)

	if err := h.queries.RevokeAPIKey(c.Request.Context(), store.RevokeAPIKeyParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		h.logger.Error("failed to revoke API key", "error", err)
		response.InternalError(c, "Failed to revoke API key")
		return
	}

	response.OK(c, gin.H{"revoked": true})
}
```

- [ ] **Step 4: Run tests — they should pass**

```bash
go test ./internal/auth/ -run "TestCreateAPIKey|TestListAPIKeys|TestRevokeAPIKey" -v
```

Expected: All 5 handler tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/apikey.go internal/auth/apikey_test.go
git commit -m "feat: add API key CRUD handlers (create, list, revoke)"
```

---

### Task 6: Route Registration + Bootstrap Wiring

**Files:**
- Modify: `internal/auth/routes.go:19` (add 3 routes after protected auth routes)
- Modify: `internal/app/bootstrap.go:343-344` (insert APIKeyMiddleware)

- [ ] **Step 1: Add API Key routes to routes.go**

In `internal/auth/routes.go`, after line 25 (`protected.POST("/auth/logout", h.Logout)`), add:

```go
	// API Key management
	protected.POST("/api-keys", h.CreateAPIKey)
	protected.GET("/api-keys", h.ListAPIKeys)
	protected.DELETE("/api-keys/:id", h.RevokeAPIKey)
```

- [ ] **Step 2: Add APIKeyMiddleware to bootstrap.go**

In `internal/app/bootstrap.go`, change lines 343-345 from:

```go
	protected := api.Group("")
	protected.Use(auth.JWTMiddleware(jwtSvc))
	protected.Use(a.Limiter.APIMiddleware())
```

To:

```go
	protected := api.Group("")
	protected.Use(auth.APIKeyMiddleware(a.Queries))
	protected.Use(auth.JWTMiddleware(jwtSvc))
	protected.Use(a.Limiter.APIMiddleware())
```

- [ ] **Step 3: Verify full project compiles**

```bash
go build ./...
```

Expected: No errors.

- [ ] **Step 4: Run all auth tests**

```bash
go test ./internal/auth/ -v
```

Expected: All tests PASS (existing + new).

- [ ] **Step 5: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: All 258+ tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/routes.go internal/app/bootstrap.go
git commit -m "feat: wire APIKeyMiddleware into protected route group"
```

---

### Task 7: Update SKILL.md with Correct API Key Prefix

**Files:**
- Modify: `openclaw-skill/SKILL.md`

- [ ] **Step 1: Update SKILL.md**

Update the `primaryEnv` and examples to use `HALAOS_API_KEY` consistently. The API key format is now confirmed as `halaos_` + 40 hex chars. No code changes needed — just verify the SKILL.md references match.

- [ ] **Step 2: Publish updated skill**

```bash
clawhub publish openclaw-skill --slug halaos-hr --version 1.1.0 --changelog "API Key authentication now implemented — all endpoints accessible via Bearer halaos_... tokens"
```

- [ ] **Step 3: Commit**

```bash
git add openclaw-skill/
git commit -m "chore: bump skill to 1.1.0 with API key auth implemented"
```

---

### Task 8: Deploy to Server

**Files:** None (deployment only)

- [ ] **Step 1: Push all changes**

```bash
git push
```

CI/CD will auto-deploy via GitHub Actions.

- [ ] **Step 2: Run migration on server**

SSH to server and verify migration runs:

```bash
ssh aigonhr
cd /home/ubuntu/aigonhr
# Migration runs automatically on container startup via goose
docker compose -f docker-compose.deploy.yml logs api | grep -i "00082\|api_key\|migration"
```

- [ ] **Step 3: Verify API Key creation works**

```bash
# Login to get JWT token
TOKEN=$(curl -s https://hr.halaos.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@demo.com","password":"Admin123abc"}' | jq -r '.data.token // .token')

# Create an API key
curl -s https://hr.halaos.com/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"OpenClaw Test"}' | jq .
```

Expected: 201 with `halaos_...` key in response.

- [ ] **Step 4: Verify API Key auth works**

```bash
# Use the API key to access an endpoint
API_KEY="<key from step 3>"
curl -s https://hr.halaos.com/api/v1/employees \
  -H "Authorization: Bearer $API_KEY" | jq .
```

Expected: 200 with employee list (same as JWT auth would return).

- [ ] **Step 5: Verify JWT auth still works**

```bash
curl -s https://hr.halaos.com/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: 200 with user info (no regression).
