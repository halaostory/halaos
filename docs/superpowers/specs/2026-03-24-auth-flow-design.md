# HalaOS Auth Flow Improvements — Design Spec

## Goal

Improve the registration/login experience with 6 changes: forgot password flow, resend verification from login, magic link expiry UX, jurisdiction persistence, setup wizard mobile layout, and semantic error codes.

## Context

Current issues:
- No password recovery mechanism — users locked out if they forget password
- Unverified email users get a generic 403 on login with no way to resend verification
- Magic link expiry shows a generic "Invalid or expired" error with no guidance
- Jurisdiction selector resets every visit (not persisted)
- Setup wizard multi-column grids may break on mobile
- Error responses use generic codes (`forbidden`, `bad_request`) instead of semantic ones

## Non-Goals

- Session management UI (active sessions, remote logout)
- API Key management UI
- OAuth / social login
- Multi-factor authentication
- Frontend i18n translations (use English; i18n keys exist but are secondary)

---

## 1. Forgot Password Flow

### Database

**Migration: `db/migrations/00083_password_reset.sql`**

```sql
ALTER TABLE users ADD COLUMN reset_token VARCHAR(100);
ALTER TABLE users ADD COLUMN reset_token_expires_at TIMESTAMPTZ;

CREATE INDEX idx_users_reset_token ON users(reset_token) WHERE reset_token IS NOT NULL;
```

Reuses the same pattern as `verification_token` / `verification_token_expires_at` from migration 00077.

### sqlc Queries

**File: `db/query/auth.sql` (append)**

```sql
-- name: SetResetToken :exec
UPDATE users SET reset_token = $2, reset_token_expires_at = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetUserByResetToken :one
SELECT * FROM users WHERE reset_token = $1 AND reset_token_expires_at > NOW();

-- name: ClearResetToken :exec
UPDATE users SET reset_token = NULL, reset_token_expires_at = NULL, updated_at = NOW()
WHERE id = $1;
```

### Backend Endpoints

**`POST /auth/forgot-password`** (public, rate-limited)

Request:
```json
{"email": "user@company.com"}
```

Response (always 200, regardless of whether email exists):
```json
{"success": true, "data": {"message": "If an account exists, a reset link has been sent"}}
```

Logic:
1. Look up user by email — if not found, return success anyway (prevent enumeration)
2. Generate 64-hex token via `generateVerificationToken()` (reuse existing function)
3. Set `reset_token` + `reset_token_expires_at` (NOW + 1 hour) via `SetResetToken`
4. Send password reset email via `email.SendPasswordResetEmail(email, firstName, resetURL)`
5. Return success

**`POST /auth/reset-password`** (public, rate-limited)

Request:
```json
{"token": "abc123...", "password": "newPassword123"}
```

Response (200):
```json
{
  "success": true,
  "data": {
    "token": "jwt...",
    "refresh_token": "jwt...",
    "user": {"id": 1, "email": "user@company.com", ...}
  }
}
```

Logic:
1. Validate password length >= 8
2. Look up user by `GetUserByResetToken(token)` — if not found, return `token_expired` or `token_invalid` error
3. Hash new password with bcrypt
4. Update password via `AdminResetPassword` (reuse existing query)
5. Clear reset token via `ClearResetToken`
6. Auto-login: generate JWT + refresh tokens
7. Return tokens + user data (same as Login response)

### Email Template

**`email.SendPasswordResetEmail(toEmail, firstName, resetURL string)`**

Reuse the verification email template style:
- Header: HalaOS branding
- Greeting: "Hi {firstName},"
- Body: "We received a request to reset your password."
- CTA: Blue button "Reset Password" → `{baseURL}/reset-password?token={token}`
- Fallback: Copy-paste link
- Expiry: "This link expires in 1 hour"
- Footer: "If you didn't request this, you can safely ignore this email."

### Frontend Pages

**`/forgot-password` — ForgotPasswordView.vue**

Two states:
1. **Form**: Email input + "Send Reset Link" button
2. **Success**: "Check your email" message (same style as register success)

Link from login page: "Forgot password?" link below the password field.

**`/reset-password?token=xxx` — ResetPasswordView.vue**

Three states:
1. **Form**: New password + confirm password inputs, "Reset Password" button
2. **Success**: "Password reset successful" + auto-redirect to dashboard (auto-login from response)
3. **Error**: "Link expired or invalid" + link to forgot-password page

### Routes

Add to `internal/auth/routes.go` public group:
```go
auth.POST("/forgot-password", loginLimiter, h.ForgotPassword)
auth.POST("/reset-password", loginLimiter, h.ResetPassword)
```

Add to `frontend/src/router/index.ts`:
```ts
{ path: '/forgot-password', component: ForgotPasswordView }
{ path: '/reset-password', component: ResetPasswordView }
```

---

## 2. Login Page — Resend Verification for Unverified Users

### Backend Change

In `handler.go` Login handler (line 327-329), change the error response from:

```go
response.Forbidden(c, "Please verify your email address before logging in")
```

To:

```go
response.Error(c, http.StatusForbidden, "email_not_verified", "Please verify your email address before logging in")
```

This uses `response.Error()` directly to set a semantic error code.

### Frontend Change

In `LoginView.vue`, update the error handling in `handleLogin()`:

```ts
catch (e) {
  const errorCode = err.data?.error?.code || err.response?.data?.error?.code
  if (errorCode === 'email_not_verified') {
    // Show inline "resend verification" UI
    showResendVerification.value = true
    resendEmail.value = form.value.email
  } else {
    message.error(msg)
  }
}
```

Add a new section below the login form (conditionally shown):
- Yellow info banner: "Your email hasn't been verified yet"
- "Resend Verification Email" button → calls `authAPI.resendVerification(email)`
- Success state: "Verification email sent! Check your inbox."

---

## 3. Magic Link Expiry UX

### Backend Changes

In `handler.go` VerifyEmail handler, differentiate error responses:

Currently (line 653-657):
```go
user, err := h.queries.GetUserByVerificationToken(c.Request.Context(), &token)
if err != nil {
    response.BadRequest(c, "Invalid or expired verification token")
    return
}
```

Change to: Try looking up by token without expiry check first. If found but expired → `token_expired`. If not found at all → `token_invalid`.

Add a new sqlc query:
```sql
-- name: GetUserByVerificationTokenAny :one
SELECT * FROM users WHERE verification_token = $1;
```

New logic:
```go
// Try with expiry check first
user, err := h.queries.GetUserByVerificationToken(ctx, &token)
if err != nil {
    // Check if token exists but is expired
    expiredUser, err2 := h.queries.GetUserByVerificationTokenAny(ctx, &token)
    if err2 != nil {
        response.Error(c, 400, "token_invalid", "This verification link is invalid")
        return
    }
    if expiredUser.EmailVerified {
        response.Error(c, 400, "already_verified", "This email is already verified")
        return
    }
    response.Error(c, 400, "token_expired", "This verification link has expired")
    return
}
```

Also change the "already verified" response (line 659-661):
```go
if user.EmailVerified {
    response.Error(c, http.StatusOK, "already_verified", "Email already verified")
    return
}
```

### Frontend Changes

In `VerifyEmailView.vue`, handle different error codes:

- `token_expired` → Show "Link has expired (valid for 24 hours)" + email input to resend verification
- `token_invalid` → Show "Invalid link" + "Go to Register" button
- `already_verified` → Show success "Email already verified" + "Go to Login" button

---

## 4. Jurisdiction Persistence

### Frontend-Only Changes

**LoginView.vue + RegisterView.vue:**

On mount:
```ts
const saved = localStorage.getItem('halaos_jurisdiction')
if (saved) selectedJurisdiction.value = saved
```

On jurisdiction selection:
```ts
function selectJurisdiction(code: string) {
  selectedJurisdiction.value = code
  localStorage.setItem('halaos_jurisdiction', code)
}
```

**Zero backend changes.**

---

## 5. Setup Wizard Mobile Layout

### Frontend-Only Changes

In `SetupWizardView.vue`:

- Change NGrid `cols` from fixed `3` or `4` to responsive: `cols="1 s:2 m:3 l:4"`
- Ensure modals use `max-width: min(600px, 95vw)` on mobile
- Step indicator: keep current NSteps on desktop, show "Step 1 of 4" text on mobile (< 640px)

Use CSS media query `@media (max-width: 640px)` for mobile overrides.

**Zero backend changes.**

---

## 6. Semantic Error Codes

### Backend Changes

Update Login handler error responses in `handler.go`:

| Current | New Code | HTTP | Message |
|---------|----------|------|---------|
| `response.Unauthorized(c, "Invalid email or password")` (line 313) | `response.Error(c, 401, "invalid_credentials", "Invalid email or password")` | 401 | Same |
| `response.Unauthorized(c, "Invalid email or password")` (line 323) | `response.Error(c, 401, "invalid_credentials", "Invalid email or password")` | 401 | Same |
| `response.Forbidden(c, "Account is not active")` (line 318) | `response.Error(c, 403, "account_disabled", "Account has been disabled")` | 403 | Updated |
| `response.Forbidden(c, "Please verify...")` (line 328) | `response.Error(c, 403, "email_not_verified", "...")` | 403 | Same (see section 2) |

Update VerifyEmail handler error responses (see section 3).

These codes enable the frontend to show contextual UI (resend verification, contact admin, etc.) instead of generic error toasts.

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `db/migrations/00083_password_reset.sql` | Create | reset_token columns + index |
| `db/query/auth.sql` | Modify | Add 4 queries (SetResetToken, GetUserByResetToken, ClearResetToken, GetUserByVerificationTokenAny) |
| `internal/store/` | Regenerate | sqlc generate |
| `internal/auth/handler.go` | Modify | Add ForgotPassword + ResetPassword handlers, update Login + VerifyEmail error codes |
| `internal/auth/routes.go` | Modify | Add 2 public routes |
| `internal/email/resend.go` | Modify | Add SendPasswordResetEmail method |
| `internal/auth/handler_test.go` | Modify | Add tests for new handlers + error code changes |
| `frontend/src/views/ForgotPasswordView.vue` | Create | Forgot password form |
| `frontend/src/views/ResetPasswordView.vue` | Create | Reset password form |
| `frontend/src/views/LoginView.vue` | Modify | Add forgot password link + resend verification UI |
| `frontend/src/views/VerifyEmailView.vue` | Modify | Handle token_expired/token_invalid/already_verified |
| `frontend/src/views/RegisterView.vue` | Modify | Fix TS error (jurisdiction type) |
| `frontend/src/views/SetupWizardView.vue` | Modify | Responsive grid + fix TS errors |
| `frontend/src/router/index.ts` | Modify | Add 2 new routes |
| `frontend/src/api/client.ts` | Modify | Add forgotPassword + resetPassword API calls |

**Zero changes to:** all existing handlers (employee, payroll, leave, attendance, etc.)

---

## Testing

### Backend Unit Tests

For each new handler, test with MockDBTX pattern:
- `TestForgotPassword_Success` — email exists, sends reset email
- `TestForgotPassword_EmailNotFound` — returns 200 anyway (anti-enumeration)
- `TestResetPassword_Success` — valid token, resets password, auto-login
- `TestResetPassword_ExpiredToken` — returns token_expired error
- `TestResetPassword_InvalidToken` — returns token_invalid error
- `TestResetPassword_WeakPassword` — returns bad_request for short password
- `TestLogin_EmailNotVerified` — returns email_not_verified code
- `TestLogin_AccountDisabled` — returns account_disabled code
- `TestVerifyEmail_TokenExpired` — returns token_expired code
- `TestVerifyEmail_AlreadyVerified` — returns already_verified code

### Frontend

Fix TS compilation errors in RegisterView, SetupWizardView, VerifyEmailView so `vue-tsc` passes.

## Security Considerations

- Reset token is 64 hex chars (32 random bytes) — same entropy as verification token
- Reset token expires in 1 hour (shorter than verification's 24 hours)
- Reset token cleared after use (one-time)
- `POST /auth/forgot-password` always returns 200 (prevents email enumeration)
- Rate limiting applied to forgot-password and reset-password endpoints
- Password minimum length enforced (8 chars)
