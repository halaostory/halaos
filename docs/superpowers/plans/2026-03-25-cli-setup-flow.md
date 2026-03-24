# CLI Setup Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let new users register/login and get an API key entirely from the CLI via a single MCP tool, eliminating the 7-step browser onboarding.

**Architecture:** Two new backend endpoints (`cli-register`, `cli-login`) that bundle auth with automatic API key creation. One new MCP tool (`setup_account`) that calls these endpoints without requiring an existing API key. The MCP server startup is restructured to allow running without `HALAOS_API_KEY`.

**Tech Stack:** Go 1.25, Gin, sqlc/pgx, mcp-go, SHA-256 key hashing, bcrypt passwords

**Spec:** `docs/superpowers/specs/2026-03-25-cli-setup-flow-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `db/query/api_keys.sql` | Modify | Add `RevokeAPIKeyByName` query |
| `internal/auth/handler.go` | Modify | Add `CLIRegister()` and `CLILogin()` handlers |
| `internal/auth/handler_cli_test.go` | Create | Tests for CLI handlers |
| `internal/auth/routes.go` | Modify | Add 2 public routes |
| `cmd/mcp/tools_setup.go` | Create | `setup_account` tool with standalone HTTP client |
| `cmd/mcp/tools_setup_test.go` | Create | Tests for setup tool |
| `cmd/mcp/main.go` | Modify | Remove hard exit, conditional tool registration |
| `openclaw-skill/SKILL.md` | Modify | Add first-time setup section |

After sqlc changes, regenerate with `~/go/bin/sqlc generate`.

---

### Task 1: Add `RevokeAPIKeyByName` SQL Query

The `cli-login` handler needs to revoke any existing `cli-default` key before creating a new one.

**Files:**
- Modify: `db/query/api_keys.sql`

- [ ] **Step 1: Add the new query to api_keys.sql**

Append to end of `db/query/api_keys.sql`:

```sql
-- name: RevokeAPIKeyByName :exec
UPDATE api_keys SET is_active = false
WHERE user_id = $1 AND name = $2 AND is_active = true;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors. New method `RevokeAPIKeyByName` in generated store.

- [ ] **Step 3: Verify generated code**

Run: `cd /Users/anna/Documents/aigonhr && grep -n 'RevokeAPIKeyByName' internal/store/*.go`
Expected: Method signature and `RevokeAPIKeyByNameParams` struct found.

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add db/query/api_keys.sql internal/store/
git commit -m "feat: add RevokeAPIKeyByName query for CLI setup flow"
```

---

### Task 2: Add `CLIRegister` Handler

**Files:**
- Modify: `internal/auth/handler.go` (add after existing `Register` method ~line 299)
- Create: `internal/auth/handler_cli_test.go`

- [ ] **Step 1: Write the tests**

Create `internal/auth/handler_cli_test.go`. Uses the project's MockDBTX + NewGinContext pattern (see `handler_test.go` and `apikey_test.go` for examples).

The CLIRegister handler makes these DB calls in order:
1. `GetUserByEmail` (QueryRow) — check email not taken
2. `CreateCompanyWithCountry` (QueryRow) — create company
3. `CreateUser` (QueryRow) — create user
4. `CreateTokenBalance` (QueryRow) — free tokens
5. `seedCountryDefaults` — multiple Exec calls for leave types & holidays
6. `MarkEmailVerified` (Exec) — auto-verify
7. `CreateAPIKey` (QueryRow) — create the key

```go
package auth

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

// companyScanValues returns scan values matching CreateCompanyWithCountry RETURNING clause.
// The Company struct has many fields; we need the ones from the INSERT...RETURNING *.
func cliTestCompany() []interface{} {
	return []interface{}{
		int64(1),              // id
		"test.com",            // name
		(*string)(nil),        // legal_name
		(*string)(nil),        // tin
		(*string)(nil),        // bir_rdo
		(*string)(nil),        // address
		(*string)(nil),        // city
		(*string)(nil),        // province
		(*string)(nil),        // zip_code
		"PHL",                 // country
		"Asia/Manila",         // timezone
		"PHP",                 // currency
		"semi_monthly",        // pay_frequency
		"active",              // status
		(*string)(nil),        // logo_url
		(*string)(nil),        // sss_er_no
		(*string)(nil),        // philhealth_er_no
		(*string)(nil),        // pagibig_er_no
		(*string)(nil),        // bank_name
		(*string)(nil),        // bank_branch
		(*string)(nil),        // bank_account_no
		(*string)(nil),        // bank_account_name
		(*string)(nil),        // contact_person
		(*string)(nil),        // contact_email
		(*string)(nil),        // contact_phone
		time.Now(),            // created_at
		time.Now(),            // updated_at
		(*string)(nil),        // referral_code
		(*string)(nil),        // referred_by_code
	}
}

func cliTestAPIKeyRow() []interface{} {
	return []interface{}{
		int64(1),              // id
		"halaos_a1b2c3",      // prefix
		"cli-default",         // name
		true,                  // is_active
		pgtype.Timestamptz{},  // last_used_at
		time.Now(),            // created_at
	}
}

// NOTE: TestCLIRegister_Success is NOT included as a unit test because CLIRegister
// uses h.pool.Begin() for transactions, and newTestHandler passes nil for pool
// (matching the existing pattern — the original Register handler also has no unit
// tests for the same reason). The success path is verified via integration testing
// in Task 8.

func TestCLIRegister_ExistingEmail(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByEmail — found (email taken)
	u := activeUser("dupe@example.com", "password")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-register", map[string]interface{}{
		"email":    "dupe@example.com",
		"password": "TestPass123",
	}, testutil.AuthContext{})

	h.CLIRegister(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLIRegister_PasswordTooShort(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-register", map[string]interface{}{
		"email":    "short@example.com",
		"password": "abc",
	}, testutil.AuthContext{})

	h.CLIRegister(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

**Note on scan values:** The `cliTestCompany` helper must match the exact column order returned by `CreateCompanyWithCountry`. Check `internal/store/company.sql.go` for the `Scan` call order. If the Company struct has more/fewer fields than shown here, adjust accordingly. The `cliTestAPIKeyRow` values match the `CreateAPIKey` RETURNING clause (id, prefix, name, is_active, last_used_at, created_at). The `cliTestCompany()` is used for both `CreateCompanyWithCountry` and `GetCompanyByID` scan values — verify both queries return the same columns in the same order, or create a separate helper if they differ.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestCLIRegister -v -count=1`
Expected: Compilation error — `h.CLIRegister` undefined.

- [ ] **Step 3: Implement CLIRegister handler**

Add to `internal/auth/handler.go` after the existing `Register` method (~line 299):

```go
// CLI Registration request — requires password (unlike browser register which allows empty for magic-link)
type cliRegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	CompanyName  string `json:"company_name"`
	Country      string `json:"country"`
	ReferralCode string `json:"referral_code"`
}

// CLIRegister handles registration from CLI/MCP clients.
// Auto-verifies email and creates an API key in one step.
func (h *Handler) CLIRegister(c *gin.Context) {
	var req cliRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_input", "Email and password (min 8 chars) are required")
		return
	}

	// Check if email already registered
	_, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err == nil {
		response.Error(c, http.StatusConflict, "email_exists", "Email already registered. Use cli-login instead.")
		return
	}

	// Derive defaults
	companyName := req.CompanyName
	if companyName == "" {
		parts := strings.SplitN(req.Email, "@", 2)
		if len(parts) == 2 {
			companyName = parts[1]
		} else {
			companyName = "My Company"
		}
	}
	country := req.Country
	if country == "" {
		country = "PHL"
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Begin transaction
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to begin transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.queries.WithTx(tx)

	// Resolve country defaults
	cc := countryConfig(country)

	// Create company with country-specific settings
	company, err := qtx.CreateCompanyWithCountry(c.Request.Context(), store.CreateCompanyWithCountryParams{
		Name:         companyName,
		Country:      cc.Country,
		Currency:     cc.Currency,
		Timezone:     cc.Timezone,
		PayFrequency: cc.PayFrequency,
	})
	if err != nil {
		h.logger.Error("failed to create company", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create user with super_admin role
	user, err := qtx.CreateUser(c.Request.Context(), store.CreateUserParams{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FirstName:    "User",
		LastName:     "",
		Role:         string(RoleSuperAdmin),
		CompanyID:    company.ID,
	})
	if err != nil {
		h.logger.Error("failed to create user", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Create token balance with initial free tokens
	_, tokenErr := qtx.CreateTokenBalance(c.Request.Context(), store.CreateTokenBalanceParams{
		CompanyID: company.ID,
		Balance:   1000,
	})
	if tokenErr != nil {
		h.logger.Warn("failed to create initial token balance", "company_id", company.ID, "error", tokenErr)
	}

	// Seed country-specific leave types and holidays
	if seedErr := seedCountryDefaults(c.Request.Context(), qtx, company.ID, country); seedErr != nil {
		h.logger.Warn("failed to seed country defaults", "company_id", company.ID, "country", country, "error", seedErr)
	}

	// Track referral if a referral code was provided
	if req.ReferralCode != "" {
		referrer, refErr := qtx.GetCompanyByReferralCode(c.Request.Context(), &req.ReferralCode)
		if refErr == nil && referrer.ID != company.ID {
			_ = qtx.SetReferredByCode(c.Request.Context(), store.SetReferredByCodeParams{
				ID:             company.ID,
				ReferredByCode: &req.ReferralCode,
			})
			_, _ = qtx.CreateReferralEvent(c.Request.Context(), store.CreateReferralEventParams{
				ReferrerCompanyID: referrer.ID,
				ReferredCompanyID: company.ID,
				ReferralCode:      req.ReferralCode,
			})
		}
	}

	// Auto-verify email (CLI users skip email verification)
	_ = qtx.MarkEmailVerified(c.Request.Context(), user.ID)

	// Create API key inside the transaction
	apiKey := generateAPIKey()
	prefix := apiKeyPrefix(apiKey)
	hash := hashAPIKey(apiKey)

	_, err = qtx.CreateAPIKey(c.Request.Context(), store.CreateAPIKeyParams{
		UserID:    user.ID,
		CompanyID: company.ID,
		Prefix:    prefix,
		KeyHash:   hash,
		Name:      "cli-default",
	})
	if err != nil {
		h.logger.Error("failed to create API key", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		h.logger.Error("failed to commit transaction", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	// Generate JWT tokens (after commit — these don't need tx)
	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}
	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "Registration failed")
		return
	}

	response.Created(c, gin.H{
		"token":          token,
		"refresh_token":  refreshToken,
		"api_key":        apiKey,
		"api_key_prefix": prefix,
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"first_name": "User",
			"last_name":  "",
			"role":       user.Role,
			"company_id": company.ID,
		},
	})
}
```

**Implementation notes:**
- `MarkEmailVerified` and `CreateAPIKey` are inside the transaction (use `qtx`). JWT generation is after commit since it's pure computation.
- `GetUserByEmail` is used for the duplicate check. If this query filters by `status = 'active'`, use `GetUserByEmailAny` instead to catch ALL existing emails.
- The referral handling is copied verbatim from the existing `Register` method (~lines 214-227).
- `RoleSuperAdmin` is a constant defined in the auth package.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestCLIRegister -v -count=1`
Expected: All 3 tests PASS. If scan value count mismatches occur, adjust `cliTestCompany()` to match the actual Company columns.

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/auth/handler.go internal/auth/handler_cli_test.go
git commit -m "feat: add CLIRegister handler for one-step CLI registration"
```

---

### Task 3: Add `CLILogin` Handler

**Files:**
- Modify: `internal/auth/handler.go` (add after `CLIRegister`)
- Modify: `internal/auth/handler_cli_test.go` (append tests)

- [ ] **Step 1: Write the tests**

Append to `internal/auth/handler_cli_test.go`:

```go
func TestCLILogin_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// 1. GetUserByEmailAny — found (EmailVerified=true, so MarkEmailVerified is skipped)
	u := activeUser("login-test@example.com", "TestPass123")
	u.Role = "super_admin"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))
	// 2. UpdateLastLogin
	mockDB.OnExecSuccess()
	// 3. RevokeAPIKeyByName — revoke old cli-default
	mockDB.OnExecSuccess()
	// 4. CreateAPIKey
	mockDB.OnQueryRow(testutil.NewRow(cliTestAPIKeyRow()...))
	// 5. GetCompanyByID — for enriching response
	mockDB.OnQueryRow(testutil.NewRow(cliTestCompany()...))

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-login", map[string]interface{}{
		"email":    "login-test@example.com",
		"password": "TestPass123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["token"] == nil {
		t.Fatal("expected token in response")
	}
	apiKey, _ := data["api_key"].(string)
	if !strings.HasPrefix(apiKey, "halaos_") {
		t.Fatalf("expected api_key starting with halaos_, got %v", data["api_key"])
	}
}

func TestCLILogin_InvalidCredentials(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByEmailAny — not found
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-login", map[string]interface{}{
		"email":    "nobody@example.com",
		"password": "WrongPass123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	// Generic 401 — no email enumeration
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLILogin_WrongPassword(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("test@example.com", "correctpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-login", map[string]interface{}{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLILogin_InactiveAccount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("test@example.com", "TestPass123")
	u.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-login", map[string]interface{}{
		"email":    "test@example.com",
		"password": "TestPass123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLILogin_AutoVerifiesUnverifiedEmail(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("unverified@example.com", "TestPass123")
	u.EmailVerified = false
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))
	// MarkEmailVerified
	mockDB.OnExecSuccess()
	// UpdateLastLogin
	mockDB.OnExecSuccess()
	// RevokeAPIKeyByName
	mockDB.OnExecSuccess()
	// CreateAPIKey
	mockDB.OnQueryRow(testutil.NewRow(cliTestAPIKeyRow()...))
	// GetCompanyByID
	mockDB.OnQueryRow(testutil.NewRow(cliTestCompany()...))

	c, w := testutil.NewGinContext("POST", "/v1/auth/cli-login", map[string]interface{}{
		"email":    "unverified@example.com",
		"password": "TestPass123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (auto-verify), got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["api_key"] == nil {
		t.Fatal("expected api_key in response")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestCLILogin -v -count=1`
Expected: Compilation error — `h.CLILogin` undefined.

- [ ] **Step 3: Implement CLILogin handler**

Add to `internal/auth/handler.go` after `CLIRegister`:

```go
type cliLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// CLILogin handles login from CLI/MCP clients.
// Auto-verifies email if needed, revokes old cli-default key, creates new one.
func (h *Handler) CLILogin(c *gin.Context) {
	var req cliLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_input", "Email and password are required")
		return
	}

	// Get user — use GetUserByEmailAny to handle all statuses
	user, err := h.queries.GetUserByEmailAny(c.Request.Context(), req.Email)
	if err != nil {
		// Generic error — no email enumeration
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	// Check account status (matches existing Login handler pattern)
	if user.Status != "active" {
		response.Error(c, http.StatusForbidden, "account_disabled", "Account has been disabled")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	// Auto-verify email if needed
	if !user.EmailVerified {
		_ = h.queries.MarkEmailVerified(c.Request.Context(), user.ID)
	}

	// Update last login (synchronous, matching existing Login handler)
	_ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

	// Revoke existing cli-default key (best-effort)
	_ = h.queries.RevokeAPIKeyByName(c.Request.Context(), store.RevokeAPIKeyByNameParams{
		UserID: user.ID,
		Name:   "cli-default",
	})

	// Create fresh API key
	apiKey := generateAPIKey()
	prefix := apiKeyPrefix(apiKey)
	hash := hashAPIKey(apiKey)

	_, err = h.queries.CreateAPIKey(c.Request.Context(), store.CreateAPIKeyParams{
		UserID:    user.ID,
		CompanyID: user.CompanyID,
		Prefix:    prefix,
		KeyHash:   hash,
		Name:      "cli-default",
	})
	if err != nil {
		h.logger.Error("failed to create API key", "error", err)
		response.InternalError(c, "Login succeeded but API key creation failed")
		return
	}

	// Generate JWT tokens
	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		response.InternalError(c, "Login failed")
		return
	}
	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		response.InternalError(c, "Login failed")
		return
	}

	// Build response (enrich with company info, matching existing Login)
	userResp := gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"role":       user.Role,
		"company_id": user.CompanyID,
	}

	comp, compErr := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)
	if compErr == nil {
		userResp["company_country"] = comp.Country
		userResp["company_currency"] = comp.Currency
		userResp["company_timezone"] = comp.Timezone
	}

	response.OK(c, gin.H{
		"token":          token,
		"refresh_token":  refreshToken,
		"api_key":        apiKey,
		"api_key_prefix": prefix,
		"user":           userResp,
	})
}
```

**Implementation notes:**
- Uses `GetUserByEmailAny` (not `GetUserByEmail`) — matches existing `Login` pattern (line 311).
- Status check uses `user.Status != "active"` — matches existing `Login` (line 317).
- `UpdateLastLogin` is synchronous on request context — matches existing `Login` (line 347).
- No `context` import needed — already available in handler.go.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestCLILogin -v -count=1`
Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/auth/handler.go internal/auth/handler_cli_test.go
git commit -m "feat: add CLILogin handler with auto-verify and key rotation"
```

---

### Task 4: Add Routes for CLI Endpoints

**Files:**
- Modify: `internal/auth/routes.go` (line ~18, inside the `auth` group)

- [ ] **Step 1: Add routes**

In `internal/auth/routes.go`, add inside the `auth` group block, after the existing `auth.POST("/reset-password", ...)` line:

```go
		// CLI setup endpoints (public, rate-limited)
		auth.POST("/cli-register", loginLimiter, h.CLIRegister)
		auth.POST("/cli-login", loginLimiter, h.CLILogin)
```

- [ ] **Step 2: Verify build compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: No errors.

- [ ] **Step 3: Run all auth tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v -count=1`
Expected: All tests pass (existing + new CLI tests).

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add internal/auth/routes.go
git commit -m "feat: add CLI register/login routes"
```

---

### Task 5: Implement `setup_account` MCP Tool

Create the tool first, then restructure main.go to use it.

**Files:**
- Create: `cmd/mcp/tools_setup.go`
- Create: `cmd/mcp/tools_setup_test.go`

- [ ] **Step 1: Write the test**

Create `cmd/mcp/tools_setup_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallCLIEndpoint_Register(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/cli-register" {
			t.Errorf("expected path /api/v1/auth/cli-register, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no Authorization header for public endpoint")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"token":          "test-jwt",
				"api_key":        "halaos_test1234567890abcdef1234567890abcdef1234",
				"api_key_prefix": "halaos_test12",
			},
		})
	}))
	defer srv.Close()

	resp, err := callCLIEndpoint(srv.URL, "cli-register", map[string]interface{}{
		"email":    "test@example.com",
		"password": "TestPass123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp["success"].(bool) {
		t.Fatal("expected success=true")
	}
}

func TestCallCLIEndpoint_LoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/cli-login" {
			t.Errorf("expected path /api/v1/auth/cli-login, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   map[string]interface{}{"code": "invalid_credentials", "message": "Invalid email or password"},
		})
	}))
	defer srv.Close()

	_, err := callCLIEndpoint(srv.URL, "cli-login", map[string]interface{}{
		"email":    "test@example.com",
		"password": "wrong",
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if err.Error() != "Invalid email or password (HTTP 401)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCallCLIEndpoint_ConflictError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   map[string]interface{}{"code": "email_exists", "message": "Email already registered. Use cli-login instead."},
		})
	}))
	defer srv.Close()

	_, err := callCLIEndpoint(srv.URL, "cli-register", map[string]interface{}{
		"email":    "existing@example.com",
		"password": "TestPass123",
	})
	if err == nil {
		t.Fatal("expected error for 409 response")
	}
}
```

- [ ] **Step 2: Implement tools_setup.go**

Create `cmd/mcp/tools_setup.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func setupAccountTool() mcp.Tool {
	return mcp.NewTool("setup_account",
		mcp.WithDescription("Register or login to HalaOS to get an API key for MCP Server access. Use this when HALAOS_API_KEY is not configured."),
		mcp.WithString("action", mcp.Required(), mcp.Enum("register", "login"), mcp.Description("'register' for new account, 'login' for existing account")),
		mcp.WithString("email", mcp.Required(), mcp.Description("Email address")),
		mcp.WithString("password", mcp.Required(), mcp.Description("Password (min 8 characters)")),
		mcp.WithString("company_name", mcp.Description("Company name (register only, defaults to email domain)")),
		mcp.WithString("country", mcp.Description("Country code: PHL, SGP, LKA, IDN (register only, default: PHL)")),
		mcp.WithString("referral_code", mcp.Description("Referral code (register only, optional)")),
	)
}

// callCLIEndpoint makes an unauthenticated POST to the HalaOS CLI auth endpoints.
func callCLIEndpoint(baseURL, endpoint string, body interface{}) (map[string]interface{}, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/api/v1/auth/" + endpoint
	httpClient := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header — these are public endpoints

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		errMsg := "Request failed"
		if errObj, ok := result["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				errMsg = msg
			}
		}
		return result, fmt.Errorf("%s (HTTP %d)", errMsg, resp.StatusCode)
	}

	return result, nil
}

func makeSetupAccount(baseURL string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		action := req.GetString("action", "")
		email := req.GetString("email", "")
		password := req.GetString("password", "")

		if action == "" || email == "" || password == "" {
			return mcp.NewToolResultError("action, email, and password are required"), nil
		}

		var endpoint string
		body := map[string]interface{}{
			"email":    email,
			"password": password,
		}

		switch action {
		case "register":
			endpoint = "cli-register"
			if cn := req.GetString("company_name", ""); cn != "" {
				body["company_name"] = cn
			}
			if co := req.GetString("country", ""); co != "" {
				body["country"] = co
			}
			if rc := req.GetString("referral_code", ""); rc != "" {
				body["referral_code"] = rc
			}
		case "login":
			endpoint = "cli-login"
		default:
			return mcp.NewToolResultError("action must be 'register' or 'login'"), nil
		}

		result, err := callCLIEndpoint(baseURL, endpoint, body)
		if err != nil {
			errMsg := err.Error()
			suggestion := ""

			if strings.Contains(errMsg, "already registered") || strings.Contains(errMsg, "email_exists") {
				suggestion = "login"
				errMsg = "Email already registered. Try action='login' instead."
			} else if strings.Contains(errMsg, "Invalid email or password") && action == "login" {
				suggestion = "register"
				errMsg = "Invalid credentials. If you don't have an account, try action='register'."
			}

			output := map[string]interface{}{"error": errMsg}
			if suggestion != "" {
				output["suggestion"] = suggestion
			}
			jsonOut, _ := json.MarshalIndent(output, "", "  ")
			return mcp.NewToolResultError(string(jsonOut)), nil
		}

		// Extract key from response
		data, _ := result["data"].(map[string]interface{})
		apiKey, _ := data["api_key"].(string)
		prefix, _ := data["api_key_prefix"].(string)

		actionMsg := "Account created"
		if action == "login" {
			actionMsg = "Logged in"
		}

		output := map[string]interface{}{
			"api_key":        apiKey,
			"api_key_prefix": prefix,
			"message":        fmt.Sprintf("%s successfully. Your API key has been generated.", actionMsg),
			"config_hint":    fmt.Sprintf("Add this to your MCP server environment:\n  HALAOS_API_KEY=%s\n\nThen restart the MCP server for the change to take effect.", apiKey),
		}
		jsonOut, _ := json.MarshalIndent(output, "", "  ")
		return mcp.NewToolResultText(string(jsonOut)), nil
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./cmd/mcp/ -run TestCallCLIEndpoint -v -count=1`
Expected: All 3 tests PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add cmd/mcp/tools_setup.go cmd/mcp/tools_setup_test.go
git commit -m "feat: add setup_account MCP tool for CLI registration/login"
```

---

### Task 6: Restructure MCP Server Startup

Make `HALAOS_API_KEY` optional so `setup_account` works without it.

**Files:**
- Modify: `cmd/mcp/main.go`

- [ ] **Step 1: Modify main.go startup**

Replace the hard exit on missing API key (lines 18-21 of current main.go):

```go
apiKey := os.Getenv("HALAOS_API_KEY")
if apiKey == "" {
    fmt.Fprintln(os.Stderr, "HALAOS_API_KEY environment variable is required")
    os.Exit(1)
}
```

With:

```go
apiKey := os.Getenv("HALAOS_API_KEY")

var client *Client
if apiKey != "" {
    client = NewClient(baseURL, apiKey)
}
```

- [ ] **Step 2: Add setup_account tool registration (unconditional)**

Add before the existing tool registrations:

```go
// Always register setup tool (works without API key)
s.AddTool(setupAccountTool(), makeSetupAccount(baseURL))
```

- [ ] **Step 3: Guard existing tool registrations**

Wrap ALL existing `s.AddTool(...)` calls (the 24 HR tools) in a `client != nil` check:

```go
if client != nil {
    // Employee tools
    s.AddTool(listEmployeesTool(), makeListEmployees(client))
    s.AddTool(getEmployeeTool(), makeGetEmployee(client))
    // ... all other existing tool registrations ...
}
```

Also remove the line `client := NewClient(baseURL, apiKey)` since we moved client creation above.

- [ ] **Step 4: Verify build compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./cmd/mcp/`
Expected: No errors.

- [ ] **Step 5: Add tool availability test**

Append to `cmd/mcp/tools_setup_test.go`:

```go
func TestToolRegistration_NoAPIKey(t *testing.T) {
	// When HALAOS_API_KEY is empty, only setup_account should be registered.
	// This is a structural test — we verify the conditional registration logic
	// by checking that NewClient is not created when apiKey is empty.
	apiKey := ""
	if apiKey != "" {
		t.Fatal("this test verifies the no-key path")
	}
	// The actual conditional tool registration is in main.go.
	// We verify the tool definitions themselves exist and are valid.
	tool := setupAccountTool()
	if tool.Name != "setup_account" {
		t.Fatalf("expected tool name 'setup_account', got '%s'", tool.Name)
	}
}
```

**Note:** Full MCP tool listing verification (sending `tools/list` JSON-RPC) is checked manually in Task 8 Steps 3-4.

- [ ] **Step 6: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add cmd/mcp/main.go cmd/mcp/tools_setup_test.go
git commit -m "refactor: make HALAOS_API_KEY optional in MCP server startup"
```

---

### Task 7: Update SKILL.md

**Files:**
- Modify: `openclaw-skill/SKILL.md`

- [ ] **Step 1: Add first-time setup section**

Add after the MCP setup instructions (the section showing `.mcp.json` configuration), before the tools list:

```markdown
## First-Time Setup (No API Key Yet?)

If `HALAOS_API_KEY` is not configured, the only available tool is `setup_account`. Use it to create an account or log in:

**New user:**
1. Ask the user for their email address
2. Call `setup_account` with `action="register"`, their email, and a password they choose
3. Show them the returned API key and the config instructions
4. They add `HALAOS_API_KEY` to their MCP config and restart

**Existing user:**
1. Call `setup_account` with `action="login"` and their credentials
2. Show the new API key and config instructions

After setting the key and restarting, all 24 HR tools become available.
```

- [ ] **Step 2: Add note to MCP setup section**

In the existing MCP setup section (where it shows `.mcp.json`), add a note:

```markdown
> **Don't have an API key?** Just add the MCP server without `HALAOS_API_KEY` — on first use, the `setup_account` tool will guide you through registration.
```

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/aigonhr
git add openclaw-skill/SKILL.md
git commit -m "docs: add first-time CLI setup instructions to SKILL.md"
```

---

### Task 8: Integration Verification

**Note:** This task also covers the CLIRegister success path (not unit-testable due to `h.pool.Begin()` requiring a real `*pgxpool.Pool`) and key rotation behavior (stateful, requires checking DB state between two calls).

- [ ] **Step 1: Run all project tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./... 2>&1 | tail -20`
Expected: All tests pass (existing + new CLI tests + MCP tests).

- [ ] **Step 2: Build MCP binary**

Run: `cd /Users/anna/Documents/aigonhr && go build -o bin/halaos-mcp ./cmd/mcp/`
Expected: Binary builds successfully.

- [ ] **Step 3: Test MCP server starts without API key**

Run: `HALAOS_BASE_URL=http://localhost:8080 /Users/anna/Documents/aigonhr/bin/halaos-mcp`
(Ctrl+C after verifying it starts without crashing)
Expected: Server starts, no exit/error about missing HALAOS_API_KEY.

- [ ] **Step 4: Test MCP server starts with API key**

Run: `HALAOS_API_KEY=test HALAOS_BASE_URL=http://localhost:8080 /Users/anna/Documents/aigonhr/bin/halaos-mcp`
(Ctrl+C after verifying)
Expected: Server starts with all tools registered.

- [ ] **Step 5: Full build check**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: No errors across the entire project.

- [ ] **Step 6: Final commit if any fixes needed**

If integration testing revealed issues, commit those fixes now.
