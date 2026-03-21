# HalaOS Product Completeness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the unfinished backend infrastructure (SSO, company switching, logout), deploy Finance Agent Phase 1, and extend Finance i18n to all internal pages.

**Architecture:** Four independent workstreams. WS1 (HR auth endpoints) creates the foundation for WS2 (Finance SSO token). WS3 (agent deployment) and WS4 (i18n) are fully independent. HR uses Go+Gin+sqlc with `testutil.MockDBTX` test pattern. Finance uses the same stack with UUID-based IDs.

**Tech Stack:** Go 1.25/1.26, Gin, sqlc (pgx/v5), PostgreSQL, Redis, Vue3+TS, vue-i18n

**Spec:** `docs/superpowers/specs/2026-03-21-halaos-product-completeness-design.md`

---

## File Structure

### AIGoNHR (HR) — New/Modified Files

| File | Responsibility |
|------|---------------|
| `db/migrations/00081_user_companies.sql` | Create `user_companies` join table + seed |
| `db/query/user_companies.sql` | sqlc queries for company membership |
| `internal/integration/sso.go` | Add `FinanceToHRClaims` + `ValidateFinanceToken()` |
| `internal/auth/handler.go` | Add Redis + SSOService deps; SSOLogin, SwitchCompany, Logout methods |
| `internal/auth/routes.go` | Register new routes: `/sso`, `/switch-company`, `/logout` |
| `internal/company/handler.go` | Add `ListUserCompanies` method |
| `internal/company/routes.go` | Register `GET /companies` route |
| `internal/app/bootstrap.go` | Wire Redis + SSOService into AuthHandler |
| `internal/auth/handler_test.go` | Tests for new auth endpoints |
| `internal/integration/sso_test.go` | Tests for FinanceToHRClaims validation |

### AIStarlight-Go (Finance Backend) — New/Modified Files

| File | Responsibility |
|------|---------------|
| `internal/handler/integration_handler.go` | Add `GetHRSSOToken` method |
| `internal/handler/router.go` | Register `GET /integrations/hr/sso-token` route |

### AIStarlight (Finance Frontend) — Modified Files

| File | Responsibility |
|------|---------------|
| `frontend/src/locales/en.ts` | Add ~800+ view-specific translation keys |
| `frontend/src/locales/zh.ts` | Add ~800+ view-specific Chinese translations |
| `frontend/src/views/*.vue` (38 files) | Replace hardcoded strings with `t()` calls |

---

## Workstream 3: Deploy Finance Agent Phase 1 (Do First)

### Task 1: Deploy and Verify Agent Phase 1

**Server:** `34.124.185.43` (ssh aistarlight-gce, user tonypk25)

- [ ] **Step 1: Pull latest code**

```bash
ssh aistarlight-gce "sudo -u anna bash -c 'cd /home/anna/aistarlight-go && git pull origin main'"
```

- [ ] **Step 2: Run migration**

```bash
ssh aistarlight-gce "sudo bash -c 'cd /home/anna/aistarlight-go && docker compose -f docker-compose.prod.yml run --rm migrate'"
```

Verify:
```bash
ssh aistarlight-gce "sudo bash -c 'cd /home/anna/aistarlight-go && docker compose -f docker-compose.prod.yml exec postgres psql -U aistarlight -d aistarlight -c \"SELECT column_name, data_type FROM information_schema.columns WHERE table_name = '\''action_plans'\'' ORDER BY ordinal_position;\"'"
```
Expected: Table with columns `id`, `thread_id`, `agent_id`, `company_id`, `user_id`, `tool_name`, `tool_args`, `summary`, `impact`, `status`, `result`, `error_message`, `created_at`, `updated_at`, `confirmed_at`, `executed_at`.

- [ ] **Step 3: Rebuild API + Worker containers**

```bash
ssh aistarlight-gce "sudo bash -c 'cd /home/anna/aistarlight-go && docker compose -f docker-compose.prod.yml up -d --build api worker'"
```

- [ ] **Step 4: Restart nginx**

```bash
ssh aistarlight-gce "sudo bash -c 'cd /home/anna/aistarlight-go && docker compose -f docker-compose.prod.yml restart nginx'"
```

- [ ] **Step 5: Verify API health**

```bash
curl -s https://finance.halaos.com/health | jq .
```
Expected: `{"status": "ok"}` or similar.

- [ ] **Step 6: Verify agent endpoints respond**

```bash
curl -sI https://finance.halaos.com/api/v1/agents
```
Expected: 401 (auth required, not 404 — confirms route is registered).

---

## Workstream 1: HR Backend Auth Endpoints

### Task 2: Database Migration — user_companies Table

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Create: `db/migrations/00081_user_companies.sql`

- [ ] **Step 1: Create migration file**

```sql
-- +goose Up
CREATE TABLE user_companies (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    role       VARCHAR(20) NOT NULL DEFAULT 'employee',
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, company_id)
);

CREATE INDEX idx_user_companies_company ON user_companies(company_id);

-- Seed from existing user→company relationships
INSERT INTO user_companies (user_id, company_id, role)
SELECT id, company_id, role FROM users WHERE company_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS user_companies;
```

- [ ] **Step 2: Verify migration number**

Run: `ls db/migrations/ | tail -3`
Expected: `00081_user_companies.sql` is the newest file. If a higher number exists, renumber.

- [ ] **Step 3: Commit**

```bash
git add db/migrations/00081_user_companies.sql
git commit -m "feat: add user_companies join table migration"
```

---

### Task 3: sqlc Queries for user_companies

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Create: `db/query/user_companies.sql`

- [ ] **Step 1: Create query file**

```sql
-- name: GetUserCompanies :many
SELECT c.id, c.name, c.country, c.timezone, c.currency, c.logo_url
FROM user_companies uc
JOIN companies c ON c.id = uc.company_id
WHERE uc.user_id = $1
ORDER BY c.name;

-- name: GetUserCompanyMembership :one
SELECT user_id, company_id, role, joined_at
FROM user_companies
WHERE user_id = $1 AND company_id = $2;

-- name: UpdateUserActiveCompany :exec
UPDATE users SET company_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: InsertUserCompany :exec
INSERT INTO user_companies (user_id, company_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, company_id) DO NOTHING;
```

- [ ] **Step 2: Run sqlc generate**

Run: `~/go/bin/sqlc generate`
Expected: No errors. New methods appear in `internal/store/user_companies.sql.go`.

- [ ] **Step 3: Verify generated code compiles**

Run: `go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add db/query/user_companies.sql internal/store/
git commit -m "feat: add sqlc queries for user_companies"
```

---

### Task 4: FinanceToHRClaims + ValidateFinanceToken

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Modify: `internal/integration/sso.go`
- Create: `internal/integration/sso_test.go` (if not exists, else modify)

**Context:** The existing `CrossAppClaims` handles HR→Finance direction. We need a new struct for Finance→HR direction. The two structs use `iss` field to distinguish direction: `iss="aigonhr"` vs `iss="aistarlight"`.

- [ ] **Step 1: Write the test**

File: `internal/integration/sso_test.go`

```go
package integration

import (
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestValidateFinanceToken_Valid(t *testing.T) {
    secret := "test-secret-32-chars-long-padded!"
    svc := NewSSOService(secret)

    // Simulate what Finance backend would generate
    claims := &FinanceToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aistarlight",
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ID:        "sso-test-123",
        },
        Email:            "user@test.com",
        FinanceCompanyID: "550e8400-e29b-41d4-a716-446655440000",
        FinanceUserID:    "660e8400-e29b-41d4-a716-446655440001",
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenStr, err := token.SignedString([]byte(secret))
    require.NoError(t, err)

    result, err := svc.ValidateFinanceToken(tokenStr)
    require.NoError(t, err)
    assert.Equal(t, "user@test.com", result.Email)
    assert.Equal(t, "aistarlight", result.Issuer)
    assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result.FinanceCompanyID)
    assert.Equal(t, "660e8400-e29b-41d4-a716-446655440001", result.FinanceUserID)
}

func TestValidateFinanceToken_WrongIssuer(t *testing.T) {
    secret := "test-secret-32-chars-long-padded!"
    svc := NewSSOService(secret)

    claims := &FinanceToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aigonhr", // wrong issuer
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
        },
        Email: "user@test.com",
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenStr, _ := token.SignedString([]byte(secret))

    _, err := svc.ValidateFinanceToken(tokenStr)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "issuer")
}

func TestValidateFinanceToken_Expired(t *testing.T) {
    secret := "test-secret-32-chars-long-padded!"
    svc := NewSSOService(secret)

    claims := &FinanceToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aistarlight",
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
        },
        Email: "user@test.com",
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenStr, _ := token.SignedString([]byte(secret))

    _, err := svc.ValidateFinanceToken(tokenStr)
    assert.Error(t, err)
}

func TestValidateFinanceToken_EmptySecret(t *testing.T) {
    svc := NewSSOService("")
    _, err := svc.ValidateFinanceToken("some-token")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not configured")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/integration/ -run TestValidateFinanceToken -v`
Expected: FAIL — `FinanceToHRClaims` and `ValidateFinanceToken` not defined.

- [ ] **Step 3: Implement FinanceToHRClaims + ValidateFinanceToken**

Add to `internal/integration/sso.go` after the existing `CrossAppClaims`:

```go
// FinanceToHRClaims represents a cross-app SSO token from Finance→HR.
// Direction is identified by iss="aistarlight" (vs iss="aigonhr" for HR→Finance).
type FinanceToHRClaims struct {
	jwt.RegisteredClaims
	Email            string `json:"email"`
	FinanceCompanyID string `json:"finance_company_id"`
	FinanceUserID    string `json:"finance_user_id"`
}

// ValidateFinanceToken parses and validates a Finance→HR SSO JWT.
func (s *SSOService) ValidateFinanceToken(tokenStr string) (*FinanceToHRClaims, error) {
	if s.secret == "" {
		return nil, fmt.Errorf("INTEGRATION_JWT_SECRET not configured")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &FinanceToHRClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Finance SSO token: %w", err)
	}

	claims, ok := token.Claims.(*FinanceToHRClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Issuer != "aistarlight" {
		return nil, fmt.Errorf("invalid issuer: expected aistarlight, got %s", claims.Issuer)
	}

	return claims, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/integration/ -run TestValidateFinanceToken -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/integration/sso.go internal/integration/sso_test.go
git commit -m "feat: add FinanceToHRClaims and ValidateFinanceToken for bidirectional SSO"
```

---

### Task 5: Auth Handler — Add Dependencies + SSOLogin

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Modify: `internal/auth/handler.go`
- Modify: `internal/auth/routes.go`
- Modify: `internal/auth/handler_test.go`

**Context:** The existing `Handler` struct has `{queries, pool, jwt, email, logger}`. We need to add `redis *redis.Client` and `sso *integration.SSOService`. The `SSOLogin` endpoint is public (no auth required — user doesn't have HR token yet).

- [ ] **Step 1: Write the test for SSOLogin**

Add to `internal/auth/handler_test.go`:

```go
func TestSSOLogin_Success(t *testing.T) {
    mockDB := testutil.NewMockDBTX()
    h := newTestHandler(mockDB)

    // Set up SSO service
    secret := "test-secret-32-chars-long-padded!"
    h.sso = integration.NewSSOService(secret)

    // Create a valid Finance→HR SSO token
    claims := &integration.FinanceToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aistarlight",
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ID:        "sso-test-1",
        },
        Email:            "user@test.com",
        FinanceCompanyID: "550e8400-e29b-41d4-a716-446655440000",
        FinanceUserID:    "660e8400-e29b-41d4-a716-446655440001",
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    ssoToken, _ := token.SignedString([]byte(secret))

    // Mock: GetUserByEmail returns active, verified user
    mockDB.OnQueryRow(testutil.NewRow(
        int64(10),      // id
        "user@test.com", // email
        "$2a$10$hash",   // hashed_password
        "Jane",          // first_name
        "Doe",           // last_name
        "employee",      // role
        int64(1),        // company_id
        true,            // email_verified
        "active",        // status
        (*string)(nil),  // avatar_url
    ))

    // Mock: GetCompanyByID for enriching response
    mockDB.OnQueryRow(testutil.NewRow(
        int64(1), "Test Co", "PHL", "Asia/Manila", "PHP", (*string)(nil),
    ))

    // Mock: UpdateLastLogin
    mockDB.OnExecSuccess()

    body := fmt.Sprintf(`{"sso_token":"%s"}`, ssoToken)
    c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", body, testutil.AuthContext{})
    h.SSOLogin(c)

    assert.Equal(t, 200, w.Code)
    data := testutil.ResponseBody(w)
    assert.NotEmpty(t, data["token"])
    assert.NotEmpty(t, data["refresh_token"])
}

func TestSSOLogin_UserNotFound(t *testing.T) {
    mockDB := testutil.NewMockDBTX()
    h := newTestHandler(mockDB)

    secret := "test-secret-32-chars-long-padded!"
    h.sso = integration.NewSSOService(secret)

    claims := &integration.FinanceToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aistarlight",
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
            ID:        "sso-test-2",
        },
        Email: "nobody@test.com",
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    ssoToken, _ := token.SignedString([]byte(secret))

    // Mock: GetUserByEmail returns no rows
    mockDB.OnQueryRowError(pgx.ErrNoRows)

    body := fmt.Sprintf(`{"sso_token":"%s"}`, ssoToken)
    c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", body, testutil.AuthContext{})
    h.SSOLogin(c)

    assert.Equal(t, 403, w.Code)
}

func TestSSOLogin_InactiveUser(t *testing.T) {
    mockDB := testutil.NewMockDBTX()
    h := newTestHandler(mockDB)

    secret := "test-secret-32-chars-long-padded!"
    h.sso = integration.NewSSOService(secret)

    claims := &integration.FinanceToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aistarlight",
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
            ID:        "sso-test-3",
        },
        Email: "user@test.com",
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    ssoToken, _ := token.SignedString([]byte(secret))

    // Mock: GetUserByEmail returns inactive user
    mockDB.OnQueryRow(testutil.NewRow(
        int64(10), "user@test.com", "$2a$10$hash", "Jane", "Doe",
        "employee", int64(1), true, "inactive", (*string)(nil),
    ))

    body := fmt.Sprintf(`{"sso_token":"%s"}`, ssoToken)
    c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", body, testutil.AuthContext{})
    h.SSOLogin(c)

    assert.Equal(t, 403, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestSSOLogin -v`
Expected: FAIL — `h.sso` field doesn't exist, `SSOLogin` method not defined.

- [ ] **Step 3: Update Handler struct + constructor**

In `internal/auth/handler.go`, modify the struct and constructor:

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/tonypk/aigonhr/internal/integration"
)

type Handler struct {
    queries *store.Queries
    pool    *pgxpool.Pool
    jwt     *JWTService
    email   *email.Service
    logger  *slog.Logger
    redis   *redis.Client
    sso     *integration.SSOService
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, jwt *JWTService, emailSvc *email.Service, logger *slog.Logger, rdb *redis.Client, sso *integration.SSOService) *Handler {
    return &Handler{
        queries: queries,
        pool:    pool,
        jwt:     jwt,
        email:   emailSvc,
        logger:  logger,
        redis:   rdb,
        sso:     sso,
    }
}
```

Update `newTestHandler` in test file to match:
```go
func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
    queries := store.New(mockDB)
    jwt := NewJWTService("test-secret-key-for-unit-tests", time.Hour, 24*time.Hour)
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    return NewHandler(queries, nil, jwt, nil, logger, nil, nil)
}
```

- [ ] **Step 4: Implement SSOLogin method**

Add to `internal/auth/handler.go`:

```go
type ssoLoginRequest struct {
    SSOToken string `json:"sso_token" binding:"required"`
}

func (h *Handler) SSOLogin(c *gin.Context) {
    var req ssoLoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "sso_token is required")
        return
    }

    if h.sso == nil {
        response.InternalError(c, "SSO not configured")
        return
    }

    claims, err := h.sso.ValidateFinanceToken(req.SSOToken)
    if err != nil {
        h.logger.Warn("SSO token validation failed", "error", err)
        response.Unauthorized(c, "invalid or expired SSO token")
        return
    }

    user, err := h.queries.GetUserByEmail(c.Request.Context(), claims.Email)
    if err != nil {
        response.Forbidden(c, "no HR account for this email")
        return
    }

    if user.Status != "active" {
        response.Forbidden(c, "account is not active")
        return
    }

    if !user.EmailVerified {
        response.Forbidden(c, "email not verified")
        return
    }

    token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
    if err != nil {
        response.InternalError(c, "failed to generate token")
        return
    }

    refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
    if err != nil {
        response.InternalError(c, "failed to generate refresh token")
        return
    }

    // Update last login
    _ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

    // Enrich with company info
    company, _ := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)

    resp := authResponse{
        Token:        token,
        RefreshToken: refreshToken,
        User: userResponse{
            ID:        user.ID,
            Email:     user.Email,
            FirstName: user.FirstName,
            LastName:  user.LastName,
            Role:      user.Role,
            CompanyID: user.CompanyID,
            AvatarUrl: user.AvatarUrl,
        },
    }
    if company.ID != 0 {
        resp.User.CompanyCountry = company.Country
        resp.User.CompanyCurrency = company.Currency
        resp.User.CompanyTimezone = company.Timezone
    }

    response.OK(c, resp)
}
```

- [ ] **Step 5: Register the route**

Add to `internal/auth/routes.go` in the `auth` (public) group:

```go
auth.POST("/sso", loginLimiter, h.SSOLogin)
```

- [ ] **Step 6: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestSSOLogin -v`
Expected: All 3 tests PASS. Note: If mock expectations differ from actual DB calls, adjust mock setup.

- [ ] **Step 7: Run full auth tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v`
Expected: All existing tests still pass (constructor change is backward-compatible if bootstrap is updated simultaneously).

- [ ] **Step 8: Commit**

```bash
git add internal/auth/handler.go internal/auth/routes.go internal/auth/handler_test.go
git commit -m "feat: add SSO login endpoint for Finance→HR cross-app authentication"
```

---

### Task 6: List User Companies Endpoint

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Modify: `internal/company/handler.go`
- Modify: `internal/company/routes.go`

- [ ] **Step 1: Implement ListUserCompanies**

Add to `internal/company/handler.go`:

```go
func (h *Handler) ListUserCompanies(c *gin.Context) {
    userID := auth.GetUserID(c)

    companies, err := h.queries.GetUserCompanies(c.Request.Context(), userID)
    if err != nil {
        h.logger.Error("failed to list user companies", "error", err, "user_id", userID)
        response.InternalError(c, "failed to load companies")
        return
    }

    response.OK(c, companies)
}
```

- [ ] **Step 2: Register the route**

Add to `internal/company/routes.go` inside `RegisterRoutes`:

```go
protected.GET("/companies", h.ListUserCompanies)
```

- [ ] **Step 3: Build and verify**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/company/handler.go internal/company/routes.go
git commit -m "feat: add GET /companies endpoint for multi-company support"
```

---

### Task 7: Switch Company Endpoint

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Modify: `internal/auth/handler.go`
- Modify: `internal/auth/routes.go`
- Modify: `internal/auth/handler_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/auth/handler_test.go`:

```go
func TestSwitchCompany_Success(t *testing.T) {
    mockDB := testutil.NewMockDBTX()
    h := newTestHandler(mockDB)

    // Mock: GetUserCompanyMembership — user belongs to target company
    mockDB.OnQueryRow(testutil.NewRow(
        int64(1), int64(2), "admin", time.Now(),
    ))
    // Mock: UpdateUserActiveCompany
    mockDB.OnExecSuccess()
    // Mock: GetCompanyByID for response enrichment
    mockDB.OnQueryRow(testutil.NewRow(
        int64(2), "Other Co", "SGP", "Asia/Singapore", "SGD", (*string)(nil),
    ))

    ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
    c, w := testutil.NewGinContext("POST", "/api/v1/auth/switch-company", `{"company_id":2}`, ac)
    h.SwitchCompany(c)

    assert.Equal(t, 200, w.Code)
    data := testutil.ResponseBody(w)
    assert.NotEmpty(t, data["token"])
}

func TestSwitchCompany_NotMember(t *testing.T) {
    mockDB := testutil.NewMockDBTX()
    h := newTestHandler(mockDB)

    // Mock: GetUserCompanyMembership — no rows
    mockDB.OnQueryRowError(pgx.ErrNoRows)

    ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
    c, w := testutil.NewGinContext("POST", "/api/v1/auth/switch-company", `{"company_id":999}`, ac)
    h.SwitchCompany(c)

    assert.Equal(t, 403, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestSwitchCompany -v`
Expected: FAIL — `SwitchCompany` not defined.

- [ ] **Step 3: Implement SwitchCompany**

Add to `internal/auth/handler.go`:

```go
type switchCompanyRequest struct {
    CompanyID int64 `json:"company_id" binding:"required"`
}

func (h *Handler) SwitchCompany(c *gin.Context) {
    var req switchCompanyRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "company_id is required")
        return
    }

    userID := GetUserID(c)
    email := GetEmail(c)
    ctx := c.Request.Context()

    // Use transaction for atomic membership check + company update
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        response.InternalError(c, "failed to start transaction")
        return
    }
    defer tx.Rollback(ctx)

    qtx := h.queries.WithTx(tx)

    // Verify membership
    membership, err := qtx.GetUserCompanyMembership(ctx, store.GetUserCompanyMembershipParams{
        UserID:    userID,
        CompanyID: req.CompanyID,
    })
    if err != nil {
        response.Forbidden(c, "access denied")
        return
    }

    // Update active company
    if err := qtx.UpdateUserActiveCompany(ctx, store.UpdateUserActiveCompanyParams{
        ID:        userID,
        CompanyID: req.CompanyID,
    }); err != nil {
        response.InternalError(c, "failed to switch company")
        return
    }

    if err := tx.Commit(ctx); err != nil {
        response.InternalError(c, "failed to commit")
        return
    }

    // Issue new tokens with updated company + role
    role := Role(membership.Role)
    token, err := h.jwt.GenerateToken(userID, email, role, req.CompanyID)
    if err != nil {
        response.InternalError(c, "failed to generate token")
        return
    }
    refreshToken, err := h.jwt.GenerateRefreshToken(userID, email, role, req.CompanyID)
    if err != nil {
        response.InternalError(c, "failed to generate refresh token")
        return
    }

    company, _ := h.queries.GetCompanyByID(ctx, req.CompanyID)

    resp := authResponse{
        Token:        token,
        RefreshToken: refreshToken,
        User: userResponse{
            ID:        userID,
            Email:     email,
            Role:      string(role),
            CompanyID: req.CompanyID,
        },
    }
    if company.ID != 0 {
        resp.User.CompanyCountry = company.Country
        resp.User.CompanyCurrency = company.Currency
        resp.User.CompanyTimezone = company.Timezone
    }

    response.OK(c, resp)
}
```

- [ ] **Step 4: Register the route**

Add to `internal/auth/routes.go` in the protected group:

```go
protected.POST("/auth/switch-company", h.SwitchCompany)
```

- [ ] **Step 5: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestSwitchCompany -v`
Expected: Both tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/handler.go internal/auth/routes.go internal/auth/handler_test.go
git commit -m "feat: add switch-company endpoint for multi-tenant support"
```

---

### Task 8: Logout Endpoint

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Modify: `internal/auth/handler.go`
- Modify: `internal/auth/routes.go`
- Modify: `internal/auth/handler_test.go`

- [ ] **Step 1: Write the test**

Add to `internal/auth/handler_test.go`:

```go
func TestLogout_Success(t *testing.T) {
    mockDB := testutil.NewMockDBTX()
    h := newTestHandler(mockDB)

    // Set up a real Redis for this test (or use miniredis)
    // For unit test without Redis, just verify the handler doesn't panic with nil Redis
    ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
    c, w := testutil.NewGinContext("POST", "/api/v1/auth/logout", `{"refresh_token":"some-token"}`, ac)
    h.Logout(c)

    // With nil Redis, logout should still return 204 (graceful degradation)
    assert.Equal(t, 204, w.Code)
}
```

- [ ] **Step 2: Implement Logout**

Add to `internal/auth/handler.go`:

```go
import (
    "crypto/sha256"
    "encoding/hex"
)

type logoutRequest struct {
    RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Logout(c *gin.Context) {
    var req logoutRequest
    _ = c.ShouldBindJSON(&req) // optional body

    if req.RefreshToken != "" && h.redis != nil {
        // Parse refresh token to get expiry (don't fully validate — just extract exp)
        parser := jwt.NewParser()
        claims := &Claims{}
        // Parse without validation to get expiry
        _, _, err := parser.ParseUnverified(req.RefreshToken, claims)
        if err == nil && claims.ExpiresAt != nil {
            ttl := time.Until(claims.ExpiresAt.Time)
            if ttl > 0 {
                hash := sha256.Sum256([]byte(req.RefreshToken))
                key := "blacklist:refresh:" + hex.EncodeToString(hash[:])
                h.redis.Set(c.Request.Context(), key, "1", ttl)
            }
        }
    }

    c.Status(204)
}
```

- [ ] **Step 3: Add blacklist check to Refresh handler**

In the existing `Refresh` method in `internal/auth/handler.go`, add at the beginning (before `h.jwt.ValidateToken`):

```go
// Check refresh token blacklist
if h.redis != nil {
    hash := sha256.Sum256([]byte(req.RefreshToken))
    key := "blacklist:refresh:" + hex.EncodeToString(hash[:])
    if h.redis.Exists(c.Request.Context(), key).Val() > 0 {
        response.Unauthorized(c, "token has been revoked")
        return
    }
}
```

- [ ] **Step 4: Register the route**

Add to `internal/auth/routes.go`:

```go
protected.POST("/auth/logout", h.Logout)
```

- [ ] **Step 5: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v`
Expected: All tests pass (existing + new).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/handler.go internal/auth/routes.go internal/auth/handler_test.go
git commit -m "feat: add logout endpoint with refresh token blacklisting"
```

---

### Task 9: Bootstrap Wiring

**Project:** `/Users/anna/Documents/aigonhr`

**Files:**
- Modify: `internal/app/bootstrap.go`

**Context:** The `AuthHandler` constructor now requires `*redis.Client` and `*integration.SSOService`. Both are already available in bootstrap: `a.Redis` (line ~120) and `acctSSO` (line ~250). The challenge is that `authHandler` is constructed at line ~236 and `acctSSO` at line ~250. We need to reorder or construct SSO service earlier.

- [ ] **Step 1: Update bootstrap wiring**

In `internal/app/bootstrap.go`, move SSO service construction before auth handler:

```go
// Before auth handler construction, create SSO service
acctSSO := integration.NewSSOService(a.Cfg.Integration.JWTSecret)

// Update auth handler construction to include Redis and SSO
authHandler := auth.NewHandler(a.Queries, a.Pool, jwtSvc, a.Resend, a.Logger, a.Redis, acctSSO)
```

Remove the later duplicate `acctSSO` construction (original line ~250) — it's now created earlier. The `acctHandler` that uses `acctSSO` still works since it's constructed after.

- [ ] **Step 2: Build to verify**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: Build succeeds. All constructor calls match new signatures.

- [ ] **Step 3: Run all tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./... 2>&1 | tail -20`
Expected: All tests pass. If any test creates `auth.NewHandler` with old signature, update it.

- [ ] **Step 4: Commit**

```bash
git add internal/app/bootstrap.go
git commit -m "feat: wire Redis and SSO service into auth handler"
```

---

## Workstream 2: Finance→HR SSO Token Endpoint

### Task 10: Finance GetHRSSOToken Endpoint

**Project:** `/Users/anna/Documents/aistarlight-go`

**Files:**
- Modify: `internal/handler/integration_handler.go`
- Modify: `internal/handler/router.go`

**Context:** `IntegrationHandler` currently only has `q *sqlc.Queries`. It needs access to `INTEGRATION_JWT_SECRET` to sign SSO tokens. The simplest approach is adding a `jwtSecret string` field.

- [ ] **Step 1: Update IntegrationHandler struct**

In `internal/handler/integration_handler.go`:

```go
type IntegrationHandler struct {
    q         *sqlc.Queries
    jwtSecret string
}

func NewIntegrationHandler(q *sqlc.Queries, jwtSecret string) *IntegrationHandler {
    return &IntegrationHandler{q: q, jwtSecret: jwtSecret}
}
```

- [ ] **Step 2: Add FinanceToHRClaims and GetHRSSOToken**

Add to `internal/handler/integration_handler.go`:

```go
import (
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/tonypk/aistarlight-go/internal/middleware"
)

type financeToHRClaims struct {
    jwt.RegisteredClaims
    Email            string `json:"email"`
    FinanceCompanyID string `json:"finance_company_id"`
    FinanceUserID    string `json:"finance_user_id"`
}

func (h *IntegrationHandler) GetHRSSOToken(c *gin.Context) {
    if h.jwtSecret == "" {
        response.InternalError(c, "SSO integration not configured")
        return
    }

    companyID := middleware.GetCompanyID(c)
    userID := middleware.GetUserID(c)
    // middleware.GetEmail() doesn't exist — extract email from context directly
    emailVal, _ := c.Get("email")
    email, _ := emailVal.(string)

    // Check that an active HR integration source exists for this company
    source, err := h.q.GetIntegrationSource(c.Request.Context(), sqlc.GetIntegrationSourceParams{
        CompanyID:    companyID,
        SourceSystem: "aigonhr",
    })
    if err != nil {
        response.NotFound(c, "no HR integration configured for this company")
        return
    }
    if source.Status != "active" {
        response.NotFound(c, "HR integration is not active")
        return
    }

    now := time.Now()
    claims := &financeToHRClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "aistarlight",
            ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(now),
            ID:        fmt.Sprintf("sso-%s-%d", userID.String(), now.UnixMilli()),
        },
        Email:            email,
        FinanceCompanyID: companyID.String(),
        FinanceUserID:    userID.String(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenStr, err := token.SignedString([]byte(h.jwtSecret))
    if err != nil {
        response.InternalError(c, "failed to generate SSO token")
        return
    }

    targetURL := os.Getenv("HR_BASE_URL")
    if targetURL == "" {
        targetURL = "https://hr.halaos.com"
    }

    response.OK(c, gin.H{
        "sso_token":  tokenStr,
        "target_url": targetURL,
    })
}
```

- [ ] **Step 3: Register route in router.go**

In the integration routes section of `internal/handler/router.go`, add inside the `if rt.Integration != nil` block:

```go
integrations.GET("/hr/sso-token", rt.Integration.GetHRSSOToken)
```

- [ ] **Step 4: Update NewIntegrationHandler call**

Find where `NewIntegrationHandler` is called (likely in `cmd/api/main.go` or router setup) and add the JWT secret parameter:

```go
integrationHandler := handler.NewIntegrationHandler(queries, cfg.Integration.JWTSecret)
```

- [ ] **Step 5: Build and verify**

Run: `cd /Users/anna/Documents/aistarlight-go && go build ./...`
Expected: Build succeeds.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/integration_handler.go internal/handler/router.go cmd/api/main.go
git commit -m "feat: add GET /integrations/hr/sso-token for Finance→HR SSO"
```

---

## Workstream 4: Finance i18n Phase 2

### Task 11: i18n Batch 1 — Data & Transactions

**Project:** `/Users/anna/Documents/aistarlight`

**Files:**
- Modify: `frontend/src/locales/en.ts` — add keys for upload, receipt, transactions, vendors, classification, tags, approvals, mapping
- Modify: `frontend/src/locales/zh.ts` — add Chinese translations
- Modify: 8 view files: `UploadView.vue`, `ReceiptUploadView.vue`, `TransactionsView.vue`, `TransactionClassificationView.vue`, `VendorView.vue`, `TagsView.vue`, `ApprovalsView.vue`, `MappingView.vue`

- [ ] **Step 1: Read each view file and extract all hardcoded strings**

For each view, find: page titles, table headers, button labels, form labels, placeholders, empty states, error messages, tooltips, status labels, confirmation dialogs.

- [ ] **Step 2: Add translation keys to en.ts**

Add new namespaces to `en.ts`. Follow the existing pattern (`moduleName.category.key`). Example structure:

```typescript
upload: {
    title: 'Upload Data',
    dropzone: 'Drop files here or click to upload',
    // ... all strings from UploadView.vue
},
receipt: {
    title: 'Receipt Scanner',
    // ... all strings from ReceiptUploadView.vue
},
transactions: {
    title: 'Transactions',
    // ... all strings from TransactionsView.vue
},
// ... etc for each view
```

- [ ] **Step 3: Add Chinese translations to zh.ts**

Mirror every key added to `en.ts` with Chinese translations in `zh.ts`.

- [ ] **Step 4: Update each view file**

For each view:
1. Add `import { useI18n } from 'vue-i18n'` to `<script setup>`
2. Add `const { t } = useI18n()`
3. Replace all hardcoded strings with `t('namespace.key')` calls

- [ ] **Step 5: Verify**

Run: `cd /Users/anna/Documents/aistarlight/frontend && npx vite build`
Expected: Build succeeds with no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/locales/en.ts frontend/src/locales/zh.ts frontend/src/views/UploadView.vue frontend/src/views/ReceiptUploadView.vue frontend/src/views/TransactionsView.vue frontend/src/views/TransactionClassificationView.vue frontend/src/views/VendorView.vue frontend/src/views/TagsView.vue frontend/src/views/ApprovalsView.vue frontend/src/views/MappingView.vue
git commit -m "feat: add i18n for Data & Transactions views"
```

---

### Task 12: i18n Batch 2 — Accounting

**Project:** `/Users/anna/Documents/aistarlight`

**Files:**
- Modify: `frontend/src/locales/en.ts`, `frontend/src/locales/zh.ts`
- Modify: 4 views: `ChartOfAccountsView.vue`, `JournalEntriesView.vue`, `GeneralLedgerView.vue`, `FinancialStatementsView.vue`

Follow same process as Task 11: extract strings, add to both locale files, update views with `t()` calls.

- [ ] **Step 1: Extract strings from all 4 views**
- [ ] **Step 2: Add en.ts keys** (namespaces: `coa`, `journal`, `generalLedger`, `statements`)
- [ ] **Step 3: Add zh.ts Chinese translations**
- [ ] **Step 4: Update views with t() calls**
- [ ] **Step 5: Build verify**: `cd /Users/anna/Documents/aistarlight/frontend && npx vite build`
- [ ] **Step 6: Commit**: `git commit -m "feat: add i18n for Accounting views"`

---

### Task 13: i18n Batch 3 — Tax & Filing

**Project:** `/Users/anna/Documents/aistarlight`

**Files:**
- Modify: `frontend/src/locales/en.ts`, `frontend/src/locales/zh.ts`
- Modify: 9 views: `TaxPrepView.vue`, `ReportView.vue`, `ReportEditView.vue`, `FormRouterView.vue`, `FilingCalendarView.vue`, `PenaltyCalculatorView.vue`, `WithholdingView.vue`, `CASComplianceView.vue`, `TaxBridgeView.vue`

Follow same process as Task 11.

- [ ] **Step 1: Extract strings from all 9 views**
- [ ] **Step 2: Add en.ts keys** (namespaces: `taxPrep`, `reports`, `reportEdit`, `formRouter`, `filingCalendar`, `penaltyCalc`, `withholding`, `casCompliance`, `taxBridge`)
- [ ] **Step 3: Add zh.ts Chinese translations**
- [ ] **Step 4: Update views with t() calls**
- [ ] **Step 5: Build verify**
- [ ] **Step 6: Commit**: `git commit -m "feat: add i18n for Tax & Filing views"`

---

### Task 14: i18n Batch 4 — Reconciliation + More

**Project:** `/Users/anna/Documents/aistarlight`

**Files:**
- Modify: `frontend/src/locales/en.ts`, `frontend/src/locales/zh.ts`
- Modify: 9 views: `ReconciliationView.vue`, `BankReconciliationView.vue`, `ChatView.vue`, `KnowledgeView.vue`, `LearningInsightsView.vue`, `VendorPoliciesView.vue`, `SpendingDashboardView.vue`, `PeriodComparisonView.vue`, `InvoiceView.vue`

Follow same process as Task 11.

- [ ] **Step 1: Extract strings from all 9 views**
- [ ] **Step 2: Add en.ts keys**
- [ ] **Step 3: Add zh.ts Chinese translations**
- [ ] **Step 4: Update views with t() calls**
- [ ] **Step 5: Build verify**
- [ ] **Step 6: Commit**: `git commit -m "feat: add i18n for Reconciliation and More views"`

---

### Task 15: i18n Batch 5 — System + Dashboard

**Project:** `/Users/anna/Documents/aistarlight`

**Files:**
- Modify: `frontend/src/locales/en.ts`, `frontend/src/locales/zh.ts`
- Modify: 9 views: `SettingsView.vue`, `MemoryView.vue`, `GuideView.vue`, `OrgDashboardView.vue`, `OrgManageView.vue`, `IntegrationView.vue`, `GLMappingView.vue`, `DashboardView.vue`

Follow same process as Task 11.

- [ ] **Step 1: Extract strings from all views**
- [ ] **Step 2: Add en.ts keys**
- [ ] **Step 3: Add zh.ts Chinese translations**
- [ ] **Step 4: Update views with t() calls**
- [ ] **Step 5: Build verify**
- [ ] **Step 6: Commit**: `git commit -m "feat: add i18n for System and Dashboard views"`

---

### Task 16: i18n Key Parity Verification + Final Build

**Project:** `/Users/anna/Documents/aistarlight`

- [ ] **Step 1: Verify en/zh key parity**

Compare key structures between en.ts and zh.ts using grep:
```bash
cd /Users/anna/Documents/aistarlight/frontend/src/locales
# Extract all leaf keys from both files and diff
grep -oP "^\s+\w+:" en.ts | sort > /tmp/en-keys.txt
grep -oP "^\s+\w+:" zh.ts | sort > /tmp/zh-keys.txt
diff /tmp/en-keys.txt /tmp/zh-keys.txt
```
Expected: No differences. If differences found, fix missing keys.

Also run the build to catch any `t()` calls referencing non-existent keys (vue-i18n warns in dev mode):
```bash
cd /Users/anna/Documents/aistarlight/frontend && npx vite build
```

- [ ] **Step 2: Fix any missing keys**

- [ ] **Step 3: Final build**

Run: `cd /Users/anna/Documents/aistarlight/frontend && npx vite build`
Expected: Build succeeds.

- [ ] **Step 4: Commit any fixes**

```bash
git add frontend/src/locales/
git commit -m "fix: ensure en/zh locale key parity"
```

---

## Final: Deploy + End-to-End Verification

### Task 17: Deploy All Changes

**Note:** WS3 (Agent Phase 1) was already deployed in Task 1. This task deploys WS1 (HR auth), WS2 (Finance SSO), and WS4 (i18n).

- [ ] **Step 1: Push HR repo**

```bash
cd /Users/anna/Documents/aigonhr && git push origin main
```

- [ ] **Step 2: Push Finance backend repo**

```bash
cd /Users/anna/Documents/aistarlight-go && git push origin main
```

- [ ] **Step 3: Push Finance frontend repo**

```bash
cd /Users/anna/Documents/aistarlight && git push origin main
```

- [ ] **Step 4: Deploy HR**

```bash
# Build frontend
cd /Users/anna/Documents/aigonhr/frontend && npx vite build
# Tarball + scp + rebuild
tar czf /tmp/aigonhr-dist.tar.gz dist
scp /tmp/aigonhr-dist.tar.gz aigonhr:/tmp/
ssh aigonhr "cd /home/ubuntu/aigonhr/frontend && rm -rf dist && tar xzf /tmp/aigonhr-dist.tar.gz"

# Run migration on HR server
ssh aigonhr "cd /home/ubuntu/aigonhr && sudo docker compose -f docker-compose.deploy.yml run --rm migrate"

# Rebuild API (includes new auth endpoints)
ssh aigonhr "cd /home/ubuntu/aigonhr && sudo docker compose -f docker-compose.deploy.yml up -d --build api frontend"
```

- [ ] **Step 5: Deploy Finance backend**

```bash
# Build Go binaries
cd /Users/anna/Documents/aistarlight-go && make build-linux

# Push binaries and deploy
git add -f aistarlight-api aistarlight-worker
git commit -m "build: update linux binaries"
git push origin main

ssh aistarlight-gce "sudo -u anna bash -c 'cd /home/anna/aistarlight-go && git pull origin main'"
ssh aistarlight-gce "sudo bash -c 'cd /home/anna/aistarlight-go && docker compose -f docker-compose.prod.yml up -d --build api worker && docker compose -f docker-compose.prod.yml restart nginx'"
```

- [ ] **Step 6: Deploy Finance frontend**

```bash
ssh aistarlight-gce "sudo -u anna bash -c 'cd /home/anna/aistarlight && git pull origin main'"
ssh aistarlight-gce "sudo bash -c 'cd /home/anna/aistarlight-go && docker compose -f docker-compose.prod.yml up -d --build frontend && docker compose -f docker-compose.prod.yml restart nginx'"
```

- [ ] **Step 7: Verify both sites**

```bash
curl -sI https://hr.halaos.com/login | head -3
curl -sI https://finance.halaos.com/login | head -3
```
Expected: Both return HTTP/2 200.

- [ ] **Step 8: End-to-end SSO test**

1. Login to HR at `https://hr.halaos.com/login`
2. Click "Finance" in sidebar → should SSO into Finance
3. Login to Finance at `https://finance.halaos.com/login`
4. Click "HR" in sidebar → should SSO into HR
5. Test company switching on HR (if user has multiple companies)
6. Test logout on HR → verify cannot refresh after logout
7. Test locale switching on Finance → all pages should show Chinese
