# Auth Flow Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve HalaOS registration/login experience with forgot password, resend verification, magic link expiry UX, jurisdiction persistence, setup wizard mobile layout, and semantic error codes.

**Architecture:** Backend adds 7 new sqlc queries + 2 new endpoints (forgot-password, reset-password) + updates Login/VerifyEmail handlers to use semantic error codes. Frontend adds 2 new views (ForgotPasswordView, ResetPasswordView) + updates LoginView/VerifyEmailView/RegisterView/SetupWizardView for better UX. All changes are in the auth module; no other handlers touched.

**Tech Stack:** Go 1.25 (Gin + sqlc + pgx/v5), Vue3 + TypeScript + NaiveUI, PostgreSQL, Resend.com email

**Spec:** `docs/superpowers/specs/2026-03-24-auth-flow-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `db/migrations/00083_password_reset.sql` | Create | Add reset_token columns to users table |
| `db/query/auth.sql` | Modify | Add 7 new queries |
| `internal/store/` | Regenerate | sqlc generate |
| `internal/testutil/fixtures.go` | Modify | Update UserScanValues for 2 new fields |
| `internal/auth/handler.go` | Modify | Add ForgotPassword + ResetPassword handlers, update Login + VerifyEmail error codes |
| `internal/auth/handler_test.go` | Modify | Add 10 new tests |
| `internal/auth/routes.go` | Modify | Add 2 public routes |
| `internal/email/resend.go` | Modify | Add SendPasswordResetEmail method |
| `frontend/src/api/client.ts` | Modify | Add forgotPassword + resetPassword API calls |
| `frontend/src/router/index.ts` | Modify | Add 2 new routes |
| `frontend/src/views/ForgotPasswordView.vue` | Create | Forgot password form + success state |
| `frontend/src/views/ResetPasswordView.vue` | Create | Reset password form + error state |
| `frontend/src/views/LoginView.vue` | Modify | Add forgot password link + resend verification UI + jurisdiction persistence |
| `frontend/src/views/RegisterView.vue` | Modify | Jurisdiction persistence + fix TS error |
| `frontend/src/views/VerifyEmailView.vue` | Modify | Handle token_expired/token_invalid/already_verified |
| `frontend/src/views/SetupWizardView.vue` | Modify | Responsive grid for mobile |

---

## Task 1: Database Migration + sqlc Queries

**Files:**
- Create: `db/migrations/00083_password_reset.sql`
- Modify: `db/query/auth.sql`
- Regenerate: `internal/store/` (sqlc generate)
- Modify: `internal/testutil/fixtures.go`

- [ ] **Step 1: Create the migration file**

Create `db/migrations/00083_password_reset.sql`:
```sql
-- +goose Up
ALTER TABLE users ADD COLUMN reset_token VARCHAR(100);
ALTER TABLE users ADD COLUMN reset_token_expires_at TIMESTAMPTZ;

CREATE INDEX idx_users_reset_token ON users(reset_token) WHERE reset_token IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_reset_token;
ALTER TABLE users DROP COLUMN IF EXISTS reset_token_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS reset_token;
```

- [ ] **Step 2: Add 7 new queries to `db/query/auth.sql`**

Append to `db/query/auth.sql`:
```sql
-- name: SetResetToken :exec
UPDATE users SET reset_token = $2, reset_token_expires_at = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetUserByResetToken :one
SELECT * FROM users WHERE reset_token = $1 AND reset_token_expires_at > NOW();

-- name: GetUserByResetTokenAny :one
SELECT * FROM users WHERE reset_token = $1;

-- name: ClearResetToken :exec
UPDATE users SET reset_token = NULL, reset_token_expires_at = NULL, updated_at = NOW()
WHERE id = $1;

-- name: ResetUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1;

-- name: GetUserByEmailAny :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByVerificationTokenAny :one
SELECT * FROM users WHERE verification_token = $1;
```

- [ ] **Step 3: Run sqlc generate**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors. New files generated in `internal/store/`.

Verify the User struct now has `ResetToken *string` and `ResetTokenExpiresAt pgtype.Timestamptz` fields:
Run: `grep -A2 'ResetToken' internal/store/models.go`
Expected: Two new fields in User struct.

- [ ] **Step 4: Update testutil fixtures for new User fields**

The `UserScanValues` function in `internal/testutil/fixtures.go` must include the 2 new fields in the exact column order that sqlc generates. After `VerificationTokenExpiresAt`, add `ResetToken` and `ResetTokenExpiresAt`.

Update `FixtureUser()` — add to the struct:
```go
ResetToken:          nil,
ResetTokenExpiresAt: pgtype.Timestamptz{},
```

Update `UserScanValues(u store.User)` — append to the return slice:
```go
u.ResetToken,
u.ResetTokenExpiresAt,
```

**Important:** The field order in `UserScanValues` MUST match the column order in the `users` table as sqlc generates it. Check `internal/store/models.go` to confirm the exact field order after sqlc generate. The new fields should appear after `VerificationTokenExpiresAt`.

**Also update `activeUser()` in `internal/auth/handler_test.go`** — this is a separate helper in the auth package (not testutil). It constructs a `store.User` literal and MUST include the new fields or tests will panic with "staticrow: expected 18 dest, got 16":

```go
// In handler_test.go, activeUser() function — add after VerificationTokenExpiresAt:
ResetToken:          nil,
ResetTokenExpiresAt: pgtype.Timestamptz{},
```

Also update `userScanValues()` in handler_test.go if it exists as a wrapper around `testutil.UserScanValues` — it should still work since it delegates to testutil. Verify both helpers produce 18 values.

- [ ] **Step 5: Verify existing tests still pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v -count=1 2>&1 | tail -30`
Expected: All existing tests pass (the fixture update maintains compatibility).

If tests fail with "staticrow: expected N dest, got M" — the scan count changed. Check `internal/store/models.go` for the exact User struct field count and update `UserScanValues` accordingly.

- [ ] **Step 6: Commit**

```bash
git add db/migrations/00083_password_reset.sql db/query/auth.sql internal/store/ internal/testutil/fixtures.go internal/auth/handler_test.go
git commit -m "feat(auth): add password reset migration + 7 new sqlc queries

- Add reset_token + reset_token_expires_at columns to users table
- Add queries: SetResetToken, GetUserByResetToken, GetUserByResetTokenAny,
  ClearResetToken, ResetUserPassword, GetUserByEmailAny,
  GetUserByVerificationTokenAny
- Update testutil fixtures + handler_test.go activeUser() for new User fields"
```

---

## Task 2: Email Template — SendPasswordResetEmail

**Files:**
- Modify: `internal/email/resend.go`

- [ ] **Step 1: Add SendPasswordResetEmail method**

Add to `internal/email/resend.go` (after `SendVerificationEmail`):

```go
// SendPasswordResetEmail sends a password reset link to the user.
func (s *Service) SendPasswordResetEmail(toEmail, firstName, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, token)

	subject := "Reset your HalaOS password"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; color: #1a1a2e;">
  <div style="text-align: center; margin-bottom: 32px;">
    <h1 style="color: #4f46e5; font-size: 28px; margin: 0;">HalaOS</h1>
    <p style="color: #64748b; font-size: 14px;">Unified HR &amp; Accounting Platform</p>
  </div>
  <h2 style="font-size: 20px; margin-bottom: 16px;">Hi %s,</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    We received a request to reset your password. Click the button below to choose a new password.
  </p>
  <div style="text-align: center; margin: 32px 0;">
    <a href="%s" style="display: inline-block; padding: 14px 32px; background: #4f46e5; color: #fff; text-decoration: none; border-radius: 8px; font-size: 16px; font-weight: 600;">
      Reset Password
    </a>
  </div>
  <p style="font-size: 14px; color: #64748b;">
    Or copy and paste this link: <br>
    <a href="%s" style="color: #4f46e5; word-break: break-all;">%s</a>
  </p>
  <p style="font-size: 14px; color: #94a3b8; margin-top: 32px;">
    This link expires in 1 hour. If you didn't request a password reset, you can safely ignore this email.
  </p>
  <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 32px 0;">
  <p style="font-size: 12px; color: #94a3b8; text-align: center;">
    HalaOS &mdash; HR, Payroll &amp; Tax Compliance
  </p>
</body>
</html>`, firstName, resetURL, resetURL, resetURL)

	if s.client == nil {
		s.logger.Info("email service not configured, logging password reset email",
			"to", toEmail,
			"subject", subject,
			"reset_url", resetURL,
		)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	sent, err := s.client.Emails.Send(params)
	if err != nil {
		s.logger.Error("failed to send password reset email", "to", toEmail, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("password reset email sent", "to", toEmail, "email_id", sent.Id)
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./internal/email/`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/email/resend.go
git commit -m "feat(email): add SendPasswordResetEmail template

1-hour expiry, same branding as verification email"
```

---

## Task 3: Backend — ForgotPassword + ResetPassword Handlers (TDD)

**Files:**
- Modify: `internal/auth/handler.go` (add 2 new handlers + request types)
- Modify: `internal/auth/handler_test.go` (add 6 tests)
- Modify: `internal/auth/routes.go` (add 2 routes)

- [ ] **Step 1: Add request types to handler.go**

Add after existing request types (around line 111):
```go
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}
```

- [ ] **Step 2: Write tests for ForgotPassword handler**

Add to `internal/auth/handler_test.go`:

```go
func TestForgotPassword_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "password123")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmail
	mockDB.OnExecSuccess()                                     // SetResetToken

	c, w := testutil.NewGinContext("POST", "/auth/forgot-password", gin.H{
		"email": "admin@test.com",
	}, testutil.AuthContext{})

	h.ForgotPassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	if body["success"] != true {
		t.Fatal("expected success: true")
	}
}

func TestForgotPassword_EmailNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByEmail returns no row
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/auth/forgot-password", gin.H{
		"email": "nobody@test.com",
	}, testutil.AuthContext{})

	h.ForgotPassword(c)

	// Should still return 200 (prevent email enumeration)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestForgotPassword -v -count=1 2>&1 | tail -10`
Expected: FAIL — `h.ForgotPassword` undefined.

- [ ] **Step 4: Implement ForgotPassword handler**

Add to `internal/auth/handler.go`:

```go
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Valid email is required")
		return
	}

	// Always return success to prevent email enumeration
	successMsg := gin.H{"message": "If an account exists, a reset link has been sent"}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		response.OK(c, successMsg)
		return
	}

	token, err := generateVerificationToken()
	if err != nil {
		h.logger.Error("failed to generate reset token", "error", err)
		response.OK(c, successMsg)
		return
	}

	expiresAt := pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true}
	err = h.queries.SetResetToken(c.Request.Context(), store.SetResetTokenParams{
		ID:                  user.ID,
		ResetToken:          &token,
		ResetTokenExpiresAt: expiresAt,
	})
	if err != nil {
		h.logger.Error("failed to set reset token", "error", err)
		response.OK(c, successMsg)
		return
	}

	if h.email != nil && h.email.IsEnabled() {
		go func() {
			if err := h.email.SendPasswordResetEmail(user.Email, user.FirstName, token); err != nil {
				h.logger.Error("failed to send password reset email", "email", user.Email, "error", err)
			}
		}()
	}

	response.OK(c, successMsg)
}
```

**Important:** Add `"time"` and `"github.com/jackc/pgx/v5/pgtype"` to imports if not already present. Also ensure `"github.com/tonypk/aigonhr/internal/store"` is imported.

- [ ] **Step 5: Run ForgotPassword tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run TestForgotPassword -v -count=1 2>&1 | tail -15`
Expected: PASS for both tests.

- [ ] **Step 6: Write tests for ResetPassword handler**

Add to `internal/auth/handler_test.go`:

```go
func TestResetPassword_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "oldpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByResetToken
	mockDB.OnExecSuccess()                                     // ResetUserPassword
	mockDB.OnExecSuccess()                                     // ClearResetToken
	mockDB.OnExecSuccess()                                     // UpdateLastLogin
	// GetCompanyByID for login enrichment
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/auth/reset-password", gin.H{
		"token":    "valid-token-123",
		"password": "newPassword123",
	}, testutil.AuthContext{})

	h.ResetPassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["token"] == nil {
		t.Fatal("expected JWT token in response")
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByResetToken fails (no match)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// GetUserByResetTokenAny also fails (token never existed)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/auth/reset-password", gin.H{
		"token":    "nonexistent-token",
		"password": "newPassword123",
	}, testutil.AuthContext{})

	h.ResetPassword(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["code"] != "token_invalid" {
		t.Fatalf("expected error code token_invalid, got %v", errObj["code"])
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByResetToken fails (expired)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// GetUserByResetTokenAny succeeds (token exists but expired)
	u := activeUser("admin@test.com", "oldpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/auth/reset-password", gin.H{
		"token":    "expired-token",
		"password": "newPassword123",
	}, testutil.AuthContext{})

	h.ResetPassword(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["code"] != "token_expired" {
		t.Fatalf("expected error code token_expired, got %v", errObj["code"])
	}
}

func TestResetPassword_WeakPassword(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/auth/reset-password", gin.H{
		"token":    "valid-token",
		"password": "short",
	}, testutil.AuthContext{})

	h.ResetPassword(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 7: Implement ResetPassword handler**

Add to `internal/auth/handler.go`:

```go
func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Token and password (min 8 characters) are required")
		return
	}

	// Look up by reset token with expiry check
	user, err := h.queries.GetUserByResetToken(c.Request.Context(), &req.Token)
	if err != nil {
		// Differentiate expired vs invalid
		_, err2 := h.queries.GetUserByResetTokenAny(c.Request.Context(), &req.Token)
		if err2 != nil {
			response.Error(c, http.StatusBadRequest, "token_invalid", "This reset link is invalid")
			return
		}
		response.Error(c, http.StatusBadRequest, "token_expired", "This reset link has expired")
		return
	}

	// Hash new password
	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		response.InternalError(c, "Password reset failed")
		return
	}

	// Update password
	err = h.queries.ResetUserPassword(c.Request.Context(), store.ResetUserPasswordParams{
		ID:           user.ID,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		h.logger.Error("failed to reset password", "error", err)
		response.InternalError(c, "Password reset failed")
		return
	}

	// Clear reset token
	err = h.queries.ClearResetToken(c.Request.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to clear reset token", "error", err)
		// Non-fatal — password was already reset
	}

	// Auto-login: generate tokens
	token, err := h.jwt.GenerateToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate token after reset", "error", err)
		response.InternalError(c, "Password reset succeeded but login failed")
		return
	}

	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID, user.Email, Role(user.Role), user.CompanyID)
	if err != nil {
		h.logger.Error("failed to generate refresh token after reset", "error", err)
		response.InternalError(c, "Password reset succeeded but login failed")
		return
	}

	_ = h.queries.UpdateLastLogin(c.Request.Context(), user.ID)

	loginResp := userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		CompanyID: user.CompanyID,
	}

	comp, compErr := h.queries.GetCompanyByID(c.Request.Context(), user.CompanyID)
	if compErr == nil {
		loginResp.CompanyCountry = comp.Country
		loginResp.CompanyCurrency = comp.Currency
		loginResp.CompanyTimezone = comp.Timezone
	}

	response.OK(c, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         loginResp,
	})
}
```

- [ ] **Step 8: Add routes**

In `internal/auth/routes.go`, add to the public auth group (after the `auth.POST("/sso"...)` line):
```go
auth.POST("/forgot-password", loginLimiter, h.ForgotPassword)
auth.POST("/reset-password", loginLimiter, h.ResetPassword)
```

- [ ] **Step 9: Run all new tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run "TestForgotPassword|TestResetPassword" -v -count=1 2>&1 | tail -20`
Expected: All 6 tests pass.

**Note on SetResetTokenParams:** After sqlc generate, check the exact param struct name. It may be `SetResetTokenParams` with fields `ID int64`, `ResetToken *string`, `ResetTokenExpiresAt pgtype.Timestamptz`. Check `internal/store/auth.sql.go` for exact types. Same for `ResetUserPasswordParams`, `GetUserByResetTokenAny` parameter type (`*string`).

- [ ] **Step 10: Run full auth tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v -count=1 2>&1 | tail -30`
Expected: All tests pass (existing + new).

- [ ] **Step 11: Commit**

```bash
git add internal/auth/handler.go internal/auth/handler_test.go internal/auth/routes.go
git commit -m "feat(auth): add forgot-password + reset-password endpoints

- POST /auth/forgot-password — always 200 (anti-enumeration)
- POST /auth/reset-password — validates token, resets password, auto-login
- Distinguishes expired vs invalid tokens
- 6 new unit tests"
```

---

## Task 4: Backend — Semantic Error Codes for Login + VerifyEmail (TDD)

**Files:**
- Modify: `internal/auth/handler.go` (Login handler lines 304-371, VerifyEmail handler lines 645-698)
- Modify: `internal/auth/handler_test.go` (add/update 4 tests)

- [ ] **Step 1: Write tests for Login semantic error codes**

Add to `internal/auth/handler_test.go`:

```go
func TestLogin_EmailNotVerified_ReturnsSemanticCode(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "password123")
	u.EmailVerified = false
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny (returns user regardless of status)

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "admin@test.com", "password": "password123",
	}, testutil.AuthContext{})

	h.Login(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["code"] != "email_not_verified" {
		t.Fatalf("expected error code email_not_verified, got %v", errObj["code"])
	}
}

func TestLogin_AccountDisabled_ReturnsSemanticCode(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "password123")
	u.Status = "disabled"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny (returns user regardless of status)

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "admin@test.com", "password": "password123",
	}, testutil.AuthContext{})

	h.Login(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["code"] != "account_disabled" {
		t.Fatalf("expected error code account_disabled, got %v", errObj["code"])
	}
}
```

- [ ] **Step 2: Write tests for VerifyEmail semantic error codes**

Add to `internal/auth/handler_test.go`:

```go
func TestVerifyEmail_TokenExpired_ReturnsSemanticCode(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByVerificationToken fails (expired)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	// GetUserByVerificationTokenAny succeeds (token exists but expired)
	u := activeUser("admin@test.com", "password123")
	u.EmailVerified = false
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContextWithQuery("GET", "/auth/verify-email",
		url.Values{"token": {"expired-token"}}, testutil.AuthContext{})

	h.VerifyEmail(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["code"] != "token_expired" {
		t.Fatalf("expected error code token_expired, got %v", errObj["code"])
	}
}

func TestVerifyEmail_AlreadyVerified_ReturnsSuccess(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByVerificationToken succeeds
	u := activeUser("admin@test.com", "password123")
	u.EmailVerified = true // already verified
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContextWithQuery("GET", "/auth/verify-email",
		url.Values{"token": {"some-token"}}, testutil.AuthContext{})

	h.VerifyEmail(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	if body["success"] != true {
		t.Fatal("expected success: true")
	}
	data, _ := body["data"].(map[string]interface{})
	if data["status"] != "already_verified" {
		t.Fatalf("expected status already_verified, got %v", data["status"])
	}
}
```

**Add `"net/url"` to imports in handler_test.go if not already present.**

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run "TestLogin_EmailNotVerified_Returns|TestLogin_AccountDisabled|TestVerifyEmail_Token|TestVerifyEmail_Already" -v -count=1 2>&1 | tail -20`
Expected: FAIL — existing handlers don't produce semantic codes yet.

- [ ] **Step 4: Update Login handler to use GetUserByEmailAny + semantic codes**

In `internal/auth/handler.go`, modify the Login handler (lines 304-371):

**Change line 311:** Replace `GetUserByEmail` with `GetUserByEmailAny`:
```go
// Before:
user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
// After:
user, err := h.queries.GetUserByEmailAny(c.Request.Context(), req.Email)
```

**Change line 313:** Use semantic error code:
```go
// Before:
response.Unauthorized(c, "Invalid email or password")
// After:
response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
```

**Change line 318:** Use semantic error code:
```go
// Before:
response.Forbidden(c, "Account is not active")
// After:
response.Error(c, http.StatusForbidden, "account_disabled", "Account has been disabled")
```

**Change line 323:** Use semantic error code:
```go
// Before:
response.Unauthorized(c, "Invalid email or password")
// After:
response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
```

**Change line 328:** Use semantic error code:
```go
// Before:
response.Forbidden(c, "Please verify your email address before logging in")
// After:
response.Error(c, http.StatusForbidden, "email_not_verified", "Please verify your email address before logging in")
```

- [ ] **Step 5: Update VerifyEmail handler for semantic error codes**

In `internal/auth/handler.go`, modify the VerifyEmail handler (lines 645-698):

**Replace lines 653-657** (the error block after `GetUserByVerificationToken`):
```go
// Before:
user, err := h.queries.GetUserByVerificationToken(c.Request.Context(), &token)
if err != nil {
    response.BadRequest(c, "Invalid or expired verification token")
    return
}

// After:
user, err := h.queries.GetUserByVerificationToken(c.Request.Context(), &token)
if err != nil {
    // Check if token exists but is expired
    expiredUser, err2 := h.queries.GetUserByVerificationTokenAny(c.Request.Context(), &token)
    if err2 != nil {
        response.Error(c, http.StatusBadRequest, "token_invalid", "This verification link is invalid")
        return
    }
    if expiredUser.EmailVerified {
        response.OK(c, gin.H{"status": "already_verified", "message": "Email already verified"})
        return
    }
    response.Error(c, http.StatusBadRequest, "token_expired", "This verification link has expired")
    return
}
```

**Replace lines 659-662** (the already-verified check):
```go
// Before:
if user.EmailVerified {
    response.OK(c, gin.H{"message": "Email already verified"})
    return
}

// After:
if user.EmailVerified {
    response.OK(c, gin.H{"status": "already_verified", "message": "Email already verified"})
    return
}
```

- [ ] **Step 6: Run semantic code tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -run "TestLogin_EmailNotVerified_Returns|TestLogin_AccountDisabled|TestVerifyEmail_Token|TestVerifyEmail_Already" -v -count=1 2>&1 | tail -20`
Expected: All 4 tests pass.

- [ ] **Step 7: Run full auth test suite**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v -count=1 2>&1 | tail -30`
Expected: All tests pass. The existing `TestLogin_InactiveAccount` (line 146) WILL need updating because the query changed from `GetUserByEmail` (which filters `status = 'active'`) to `GetUserByEmailAny`. Update `TestLogin_InactiveAccount` to:

```go
func TestLogin_InactiveAccount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "password123")
	u.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny returns user regardless of status

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "admin@test.com", "password": "password123",
	}, testutil.AuthContext{})

	h.Login(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	body := testutil.ResponseBody(w)
	errObj, _ := body["error"].(map[string]interface{})
	if errObj["code"] != "account_disabled" {
		t.Fatalf("expected error code account_disabled, got %v", errObj["code"])
	}
}
```

- [ ] **Step 8: Commit**

```bash
git add internal/auth/handler.go internal/auth/handler_test.go
git commit -m "feat(auth): semantic error codes for Login + VerifyEmail

Login: invalid_credentials, account_disabled, email_not_verified
VerifyEmail: token_expired, token_invalid, already_verified
Uses GetUserByEmailAny to detect disabled accounts"
```

---

## Task 5: Frontend — API Client + Router + ForgotPasswordView + ResetPasswordView

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/router/index.ts`
- Create: `frontend/src/views/ForgotPasswordView.vue`
- Create: `frontend/src/views/ResetPasswordView.vue`

- [ ] **Step 1: Add API methods to client.ts**

In `frontend/src/api/client.ts`, add to the `authAPI` object (after `ssoLogin`):

```ts
forgotPassword: (email: string) =>
  post("/v1/auth/forgot-password", { email }),
resetPassword: (token: string, password: string) =>
  post("/v1/auth/reset-password", { token, password }),
```

- [ ] **Step 2: Add routes to router/index.ts**

In `frontend/src/router/index.ts`, add after the `verify-email` route (in the guest section):

```ts
{
  path: '/forgot-password',
  name: 'forgot-password',
  component: () => import('../views/ForgotPasswordView.vue'),
  meta: { title: 'Forgot Password' },
},
{
  path: '/reset-password',
  name: 'reset-password',
  component: () => import('../views/ResetPasswordView.vue'),
  meta: { title: 'Reset Password' },
},
```

- [ ] **Step 3: Create ForgotPasswordView.vue**

Create `frontend/src/views/ForgotPasswordView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { NForm, NFormItem, NInput, NButton, NAlert, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { authAPI } from '../api/client'

const formRef = ref<FormInst | null>(null)
const email = ref('')
const loading = ref(false)
const sent = ref(false)
const message = useMessage()

const rules: FormRules = {
  email: [
    { required: true, message: 'Email is required', trigger: ['blur', 'input'] },
    { type: 'email', message: 'Please enter a valid email', trigger: ['blur'] },
  ],
}

async function handleSubmit() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    await authAPI.forgotPassword(email.value)
    sent.value = true
  } catch (err: any) {
    message.error(err.data?.error?.message || 'Something went wrong. Please try again.')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-wrapper">
    <div class="auth-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
        <h2>Reset your password</h2>
      </div>

      <template v-if="!sent">
        <p class="auth-description">
          Enter your email address and we'll send you a link to reset your password.
        </p>
        <NForm ref="formRef" :model="{ email }" :rules="rules">
          <NFormItem path="email" label="Email">
            <NInput
              v-model:value="email"
              placeholder="email@company.com"
              :input-props="{ type: 'email' }"
              @keyup.enter="handleSubmit"
            />
          </NFormItem>
          <NButton
            type="primary"
            block
            :loading="loading"
            @click.prevent="handleSubmit"
          >
            Send Reset Link
          </NButton>
        </NForm>
      </template>

      <template v-else>
        <NAlert type="success" title="Check your email" style="margin-bottom: 16px;">
          If an account exists for <strong>{{ email }}</strong>, we've sent a password reset link.
          The link expires in 1 hour.
        </NAlert>
        <p class="auth-description">
          Didn't receive the email? Check your spam folder or
          <a href="#" @click.prevent="sent = false; loading = false">try again</a>.
        </p>
      </template>

      <div class="auth-footer">
        <router-link to="/login">Back to Login</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
}
.auth-card {
  width: 420px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
.brand-header {
  text-align: center;
  margin-bottom: 24px;
}
.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 16px;
}
.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #fff;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 800;
}
.logo-text {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}
.brand-header h2 {
  font-size: 16px;
  font-weight: 500;
  color: #64748b;
  margin: 0;
}
.auth-description {
  font-size: 14px;
  color: #64748b;
  margin-bottom: 20px;
  line-height: 1.5;
}
.auth-description a {
  color: #2563eb;
  text-decoration: none;
  font-weight: 600;
}
.auth-footer {
  text-align: center;
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
  font-size: 14px;
}
.auth-footer a {
  color: #2563eb;
  font-weight: 600;
  text-decoration: none;
}
</style>
```

- [ ] **Step 4: Create ResetPasswordView.vue**

Create `frontend/src/views/ResetPasswordView.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NForm, NFormItem, NInput, NButton, NAlert, useMessage } from 'naive-ui'
import type { FormRules, FormInst } from 'naive-ui'
import { authAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()

const formRef = ref<FormInst | null>(null)
const form = ref({ password: '', confirmPassword: '' })
const loading = ref(false)
const success = ref(false)
const errorCode = ref<string | null>(null)
const token = ref('')

onMounted(() => {
  token.value = (route.query.token as string) || ''
  if (!token.value) {
    errorCode.value = 'token_invalid'
  }
})

const rules: FormRules = {
  password: [
    { required: true, message: 'Password is required', trigger: ['blur', 'input'] },
    { min: 8, message: 'Password must be at least 8 characters', trigger: ['blur'] },
  ],
  confirmPassword: [
    { required: true, message: 'Please confirm your password', trigger: ['blur', 'input'] },
    {
      validator: (_rule: unknown, value: string) => {
        return value === form.value.password || new Error('Passwords do not match')
      },
      trigger: ['blur'],
    },
  ],
}

async function handleReset() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    const res = await authAPI.resetPassword(token.value, form.value.password)
    const data = res.data || res
    // Auto-login with returned tokens
    if (data.token) {
      localStorage.setItem('access_token', data.token)
      localStorage.setItem('refresh_token', data.refresh_token)
      auth.setUser(data.user)
    }
    success.value = true
    message.success('Password reset successful!')
    setTimeout(() => router.push('/dashboard'), 1500)
  } catch (err: any) {
    const code = err.data?.error?.code || err.response?.data?.error?.code
    if (code === 'token_expired' || code === 'token_invalid') {
      errorCode.value = code
    } else {
      message.error(err.data?.error?.message || 'Password reset failed')
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-wrapper">
    <div class="auth-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
        <h2>Set new password</h2>
      </div>

      <!-- Error state -->
      <template v-if="errorCode">
        <NAlert
          :type="errorCode === 'token_expired' ? 'warning' : 'error'"
          :title="errorCode === 'token_expired' ? 'Link expired' : 'Invalid link'"
          style="margin-bottom: 16px;"
        >
          <template v-if="errorCode === 'token_expired'">
            This reset link has expired. Please request a new one.
          </template>
          <template v-else>
            This reset link is invalid. It may have already been used.
          </template>
        </NAlert>
        <NButton type="primary" block @click="router.push('/forgot-password')">
          Request New Reset Link
        </NButton>
      </template>

      <!-- Success state -->
      <template v-else-if="success">
        <NAlert type="success" title="Password reset successful!" style="margin-bottom: 16px;">
          Your password has been updated. Redirecting to dashboard...
        </NAlert>
      </template>

      <!-- Form state -->
      <template v-else>
        <NForm ref="formRef" :model="form" :rules="rules">
          <NFormItem path="password" label="New Password">
            <NInput
              v-model:value="form.password"
              type="password"
              show-password-on="click"
              placeholder="At least 8 characters"
            />
          </NFormItem>
          <NFormItem path="confirmPassword" label="Confirm Password">
            <NInput
              v-model:value="form.confirmPassword"
              type="password"
              show-password-on="click"
              placeholder="Re-enter your password"
              @keyup.enter="handleReset"
            />
          </NFormItem>
          <NButton
            type="primary"
            block
            :loading="loading"
            @click.prevent="handleReset"
          >
            Reset Password
          </NButton>
        </NForm>
      </template>

      <div class="auth-footer">
        <router-link to="/login">Back to Login</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
}
.auth-card {
  width: 420px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
.brand-header {
  text-align: center;
  margin-bottom: 24px;
}
.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 16px;
}
.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #fff;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 800;
}
.logo-text {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}
.brand-header h2 {
  font-size: 16px;
  font-weight: 500;
  color: #64748b;
  margin: 0;
}
.auth-footer {
  text-align: center;
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
  font-size: 14px;
}
.auth-footer a {
  color: #2563eb;
  font-weight: 600;
  text-decoration: none;
}
</style>
```

- [ ] **Step 5: Verify frontend compiles**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npx vue-tsc --noEmit 2>&1 | head -20`
Expected: No new errors from these files (pre-existing errors in other views may exist).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/router/index.ts frontend/src/views/ForgotPasswordView.vue frontend/src/views/ResetPasswordView.vue
git commit -m "feat(frontend): add forgot-password + reset-password pages

- ForgotPasswordView: email form + success state
- ResetPasswordView: password form + auto-login + error states
- API client methods + routes added"
```

---

## Task 6: Frontend — LoginView Updates (Forgot Password Link + Resend Verification + Jurisdiction Persistence)

**Files:**
- Modify: `frontend/src/views/LoginView.vue`

- [ ] **Step 1: Add resend verification state + jurisdiction persistence**

In `LoginView.vue`, add after the existing refs (line 17-18):

```ts
const showResendVerification = ref(false)
const resendEmail = ref('')
const resendLoading = ref(false)
const resendSent = ref(false)
```

Add imports: `NAlert` to the NaiveUI import line.
Add import: `import { authAPI } from '../api/client'`

- [ ] **Step 2: Add jurisdiction persistence on mount**

Add `import { ref, computed, onMounted } from 'vue'` (add `onMounted`).

Add after the refs:
```ts
onMounted(() => {
  const saved = localStorage.getItem('halaos_jurisdiction')
  if (saved) selectedJurisdiction.value = saved
})
```

- [ ] **Step 3: Update jurisdiction click handler**

Change line 88 from:
```html
@click="selectedJurisdiction = j.code"
```
To:
```html
@click="selectJurisdiction(j.code)"
```

Add the function in script:
```ts
function selectJurisdiction(code: string) {
  selectedJurisdiction.value = code
  localStorage.setItem('halaos_jurisdiction', code)
}
```

- [ ] **Step 4: Update error handling in handleLogin**

Replace the catch block (lines 51-57) in `handleLogin()`:

```ts
} catch (e: unknown) {
  const err = e as { response?: { data?: { error?: { code?: string; message?: string } } }; data?: { error?: { code?: string; message?: string } } }
  const errorCode = err.data?.error?.code || err.response?.data?.error?.code
  if (errorCode === 'email_not_verified') {
    showResendVerification.value = true
    resendEmail.value = form.value.email
  } else {
    const msg = err.response?.data?.error?.message || err.data?.error?.message || t('auth.loginFailed')
    message.error(msg)
  }
}
```

- [ ] **Step 5: Add resend verification function**

```ts
async function handleResendVerification() {
  resendLoading.value = true
  try {
    await authAPI.resendVerification(resendEmail.value)
    resendSent.value = true
  } catch (err: any) {
    message.error('Failed to resend verification email')
  } finally {
    resendLoading.value = false
  }
}
```

- [ ] **Step 6: Add forgot password link + resend verification UI to template**

After the `</NForm>` tag (line 126), before the `auth-footer` div, add:

```html
<!-- Forgot password link -->
<div style="text-align: right; margin-top: 8px; margin-bottom: 8px;">
  <router-link to="/forgot-password" class="forgot-link">Forgot password?</router-link>
</div>

<!-- Resend verification banner -->
<NAlert
  v-if="showResendVerification && !resendSent"
  type="warning"
  title="Email not verified"
  style="margin-top: 16px;"
>
  <p style="margin: 0 0 12px;">Your email hasn't been verified yet. Please check your inbox or resend the verification email.</p>
  <NButton
    size="small"
    type="warning"
    :loading="resendLoading"
    @click="handleResendVerification"
  >
    Resend Verification Email
  </NButton>
</NAlert>
<NAlert
  v-if="resendSent"
  type="success"
  title="Verification email sent!"
  style="margin-top: 16px;"
>
  Check your inbox for the verification link.
</NAlert>
```

Add to the `<style scoped>` section:
```css
.forgot-link {
  font-size: 13px;
  color: #64748b;
  text-decoration: none;
}
.forgot-link:hover {
  color: #2563eb;
}
```

- [ ] **Step 7: Verify it compiles**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npx vue-tsc --noEmit 2>&1 | grep -i LoginView`
Expected: No errors for LoginView.vue.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/views/LoginView.vue
git commit -m "feat(frontend): login page improvements

- Add 'Forgot password?' link
- Show resend verification UI when email_not_verified
- Persist jurisdiction selection in localStorage"
```

---

## Task 7: Frontend — VerifyEmailView + RegisterView Updates

**Files:**
- Modify: `frontend/src/views/VerifyEmailView.vue`
- Modify: `frontend/src/views/RegisterView.vue`

- [ ] **Step 1: Update VerifyEmailView to handle semantic error codes**

Rewrite `frontend/src/views/VerifyEmailView.vue` to handle `token_expired`, `token_invalid`, and `already_verified`:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NButton, NAlert, NInput, NFormItem, useMessage } from 'naive-ui'
import { authAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()

const loading = ref(true)
const status = ref<'verifying' | 'success' | 'already_verified' | 'token_expired' | 'token_invalid'>('verifying')
const resendEmail = ref('')
const resendLoading = ref(false)
const resendSent = ref(false)

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    status.value = 'token_invalid'
    loading.value = false
    return
  }

  try {
    const res = await authAPI.verifyEmail(token)
    const data = res.data || res

    // Check for "already_verified" success response
    if (data.status === 'already_verified') {
      status.value = 'already_verified'
      loading.value = false
      return
    }

    // Normal verification success — auto-login
    if (data.token) {
      localStorage.setItem('access_token', data.token)
      localStorage.setItem('refresh_token', data.refresh_token)
      auth.setUser(data.user)
    }
    status.value = 'success'
    loading.value = false
    setTimeout(() => router.push('/setup'), 1500)
  } catch (err: any) {
    const code = err.data?.error?.code || err.response?.data?.error?.code
    if (code === 'token_expired') {
      status.value = 'token_expired'
    } else {
      status.value = 'token_invalid'
    }
    loading.value = false
  }
})

async function handleResend() {
  if (!resendEmail.value) {
    message.warning('Please enter your email address')
    return
  }
  resendLoading.value = true
  try {
    await authAPI.resendVerification(resendEmail.value)
    resendSent.value = true
    message.success('Verification email sent!')
  } catch {
    message.error('Failed to resend verification email')
  } finally {
    resendLoading.value = false
  }
}
</script>

<template>
  <div class="auth-wrapper">
    <div class="auth-card">
      <div class="brand-header">
        <router-link to="/" class="brand-logo">
          <span class="logo-icon">H</span>
          <span class="logo-text">HalaOS</span>
        </router-link>
      </div>

      <!-- Loading -->
      <div v-if="loading" style="text-align: center; padding: 40px 0;">
        <p style="color: #64748b;">Verifying your email...</p>
      </div>

      <!-- Success -->
      <template v-else-if="status === 'success'">
        <NAlert type="success" title="Email verified!">
          Your account is now active. Redirecting to setup...
        </NAlert>
      </template>

      <!-- Already verified -->
      <template v-else-if="status === 'already_verified'">
        <NAlert type="info" title="Email already verified">
          Your email is already verified. You can log in to your account.
        </NAlert>
        <NButton type="primary" block style="margin-top: 16px;" @click="router.push('/login')">
          Go to Login
        </NButton>
      </template>

      <!-- Token expired -->
      <template v-else-if="status === 'token_expired'">
        <NAlert type="warning" title="Verification link expired">
          This link has expired (valid for 24 hours). Enter your email below to receive a new one.
        </NAlert>
        <template v-if="!resendSent">
          <NFormItem label="Email" style="margin-top: 16px;">
            <NInput
              v-model:value="resendEmail"
              placeholder="email@company.com"
              :input-props="{ type: 'email' }"
              @keyup.enter="handleResend"
            />
          </NFormItem>
          <NButton type="primary" block :loading="resendLoading" @click="handleResend">
            Resend Verification Email
          </NButton>
        </template>
        <NAlert v-else type="success" title="Email sent!" style="margin-top: 16px;">
          Check your inbox for the new verification link.
        </NAlert>
      </template>

      <!-- Token invalid -->
      <template v-else>
        <NAlert type="error" title="Invalid verification link">
          This link is invalid. It may have already been used or was copied incorrectly.
        </NAlert>
        <NButton type="primary" block style="margin-top: 16px;" @click="router.push('/register')">
          Go to Register
        </NButton>
      </template>

      <div class="auth-footer">
        <router-link to="/login">Back to Login</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 50%, #f8fafc 100%);
}
.auth-card {
  width: 420px;
  max-width: 90vw;
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}
.brand-header {
  text-align: center;
  margin-bottom: 24px;
}
.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  margin-bottom: 16px;
}
.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #fff;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 800;
}
.logo-text {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}
.auth-footer {
  text-align: center;
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #f1f5f9;
  font-size: 14px;
}
.auth-footer a {
  color: #2563eb;
  font-weight: 600;
  text-decoration: none;
}
</style>
```

- [ ] **Step 2: Update RegisterView — jurisdiction persistence**

In `frontend/src/views/RegisterView.vue`:

Add `onMounted` to the vue import: `import { ref, onMounted } from 'vue'`

Find the jurisdiction ref (it may be `selectedJurisdiction` or similar). Add:
```ts
onMounted(() => {
  const saved = localStorage.getItem('halaos_jurisdiction')
  if (saved) selectedJurisdiction.value = saved
})
```

Update the jurisdiction click handler to persist:
```ts
function selectJurisdiction(code: string) {
  selectedJurisdiction.value = code
  localStorage.setItem('halaos_jurisdiction', code)
}
```

Replace direct `@click="selectedJurisdiction = j.code"` in template with `@click="selectJurisdiction(j.code)"`.

- [ ] **Step 3: Verify frontend compiles**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npx vue-tsc --noEmit 2>&1 | grep -E "VerifyEmail|Register" | head -10`
Expected: No new errors for these files.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/VerifyEmailView.vue frontend/src/views/RegisterView.vue
git commit -m "feat(frontend): improved verify email UX + register jurisdiction persistence

- VerifyEmailView handles token_expired/token_invalid/already_verified
- Resend verification from expired token page
- RegisterView persists jurisdiction in localStorage"
```

---

## Task 8: Frontend — SetupWizardView Mobile Layout

**Files:**
- Modify: `frontend/src/views/SetupWizardView.vue`

- [ ] **Step 1: Read the current SetupWizardView**

Read `frontend/src/views/SetupWizardView.vue` and identify:
1. All `<n-grid>` usages with fixed `cols` values
2. Any modals that need `max-width` mobile fixes
3. The `<n-steps>` component for step indicator

- [ ] **Step 2: Make NGrid responsive**

Find all `<n-grid :cols="3">` or `<n-grid :cols="4">` and change to responsive breakpoints:
```html
<!-- Before -->
<n-grid :cols="3" :x-gap="12" :y-gap="12">

<!-- After -->
<n-grid cols="1 s:2 m:3" :x-gap="12" :y-gap="12">
```

For grids with 4 columns:
```html
<n-grid cols="1 s:2 m:3 l:4" :x-gap="12" :y-gap="12">
```

Note: When using responsive string format, use `cols` (no `:` binding) as a plain string attribute, not `:cols` with a number.

- [ ] **Step 3: Add mobile step indicator**

If the wizard uses `<n-steps>`, add a simpler text indicator for mobile:

```html
<!-- Desktop steps -->
<n-steps v-if="!isMobile" :current="currentStep" class="wizard-steps">
  <!-- existing steps -->
</n-steps>
<!-- Mobile step indicator -->
<div v-else class="mobile-step-indicator">
  Step {{ currentStep }} of 4
</div>
```

Add to script:
```ts
import { ref, computed, onMounted, onUnmounted } from 'vue'

const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 640)

function handleResize() {
  windowWidth.value = window.innerWidth
}

onMounted(() => window.addEventListener('resize', handleResize))
onUnmounted(() => window.removeEventListener('resize', handleResize))
```

Add to style:
```css
.mobile-step-indicator {
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  color: #4f46e5;
  padding: 12px 0;
  margin-bottom: 16px;
}
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npx vue-tsc --noEmit 2>&1 | grep SetupWizard | head -5`
Expected: No new errors from our changes (pre-existing TS errors may remain).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/SetupWizardView.vue
git commit -m "feat(frontend): responsive setup wizard for mobile

- NGrid uses responsive cols (1 s:2 m:3)
- Mobile step indicator (< 640px)"
```

---

## Task 9: Build Verification + Final Tests

- [ ] **Step 1: Run full Go test suite**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/auth/ -v -count=1 2>&1 | tail -40`
Expected: All tests pass (existing + 10 new).

- [ ] **Step 2: Run Go build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: No errors.

- [ ] **Step 3: Run frontend type check**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npx vue-tsc --noEmit 2>&1 | tail -20`
Expected: No new errors from our files. Pre-existing errors in other files are acceptable.

- [ ] **Step 4: Run go mod tidy**

Run: `cd /Users/anna/Documents/aigonhr && go mod tidy`
Expected: No changes needed (or minimal updates).

- [ ] **Step 5: Final commit if needed**

If `go mod tidy` changed files:
```bash
git add go.mod go.sum
git commit -m "chore: go mod tidy"
```

- [ ] **Step 6: Push**

```bash
git push
```
