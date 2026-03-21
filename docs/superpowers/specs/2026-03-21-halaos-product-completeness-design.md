# HalaOS Product Completeness — Design Spec

## Goal

Complete the unfinished backend infrastructure from the UI Unification project, deploy the Finance Agent Phase 1 to production, and extend Finance i18n to all internal pages. After this work, SSO flows bidirectionally between HR and Finance, company switching works on HR, agent tool execution is live on Finance, and Finance supports full English/Chinese localization.

## Context

The UI Unification (2026-03-21) shipped frontend-only changes across both products. Several features have frontend code that calls backend endpoints that don't exist yet:

- HR auth store calls `POST /auth/sso`, `POST /auth/switch-company`, `POST /auth/logout`, `GET /companies` — none implemented
- Finance DashboardLayout calls `GET /integrations/hr/sso-token` — not implemented
- Finance Agent Phase 1 code is committed but not deployed to production
- Finance i18n only covers the shell (login, register, dashboard layout); ~35 internal views are English-only

## Non-Goals

- New features beyond what the frontend already expects
- HR i18n (HR already has full i18n coverage)
- Finance Agent Phase 2/3 (multi-step plans, workflow templates)
- Indonesia jurisdiction tax rules
- Shared npm component library
- Unified auth microservice

---

## Workstream 1: HR Backend Auth Endpoints

### 1.1 POST /v1/auth/sso — Incoming SSO from Finance

**Purpose:** When a Finance user clicks "HR" in the sidebar, Finance generates an SSO token and opens `https://hr.halaos.com/sso?token=xxx`. The HR frontend sends this token to `POST /v1/auth/sso`.

**Request:**
```json
{"sso_token": "<JWT signed with INTEGRATION_JWT_SECRET>"}
```

**SSO Token Claims (Finance→HR direction):**

Define a new `FinanceToHRClaims` struct in `internal/integration/sso.go`, separate from the existing `CrossAppClaims` (which is HR→Finance direction). Use `iss` field for direction discrimination:
- HR→Finance tokens: `iss="aigonhr"` (existing `CrossAppClaims`)
- Finance→HR tokens: `iss="aistarlight"` (new `FinanceToHRClaims`)

```go
// Finance→HR SSO claims (defined in aigonhr's sso.go for validation)
type FinanceToHRClaims struct {
    jwt.RegisteredClaims           // iss="aistarlight", exp=+5min, jti=unique
    Email            string        `json:"email"`
    FinanceCompanyID string        `json:"finance_company_id"` // UUID string
    FinanceUserID    string        `json:"finance_user_id"`    // UUID string
}
```

Add a new `ValidateFinanceToken(tokenStr string) (*FinanceToHRClaims, error)` method to `SSOService` that:
- Parses with `FinanceToHRClaims` struct
- Validates HS256 signature with `INTEGRATION_JWT_SECRET`
- Checks `iss == "aistarlight"`
- Returns parsed claims

**Flow:**
1. Validate JWT signature using `ssoSvc.ValidateFinanceToken()`
2. Check `exp` not expired (handled by jwt-go library)
3. Look up HR user by `email` in `users` table
4. Check `user.Status == "active"` → 403 if not
5. Check `user.EmailVerified == true` → 403 if not
6. If user not found → 403 `{"error": "no HR account for this email"}`
7. Issue HR `token` + `refresh_token` using existing `jwtSvc.GenerateToken()` / `GenerateRefreshToken()`
8. Return `{token, refresh_token, user: {id, email, first_name, last_name, role, company_id, company_country, company_currency, company_timezone}}`

**Note on response field names:** HR's existing `authResponse` uses `token` (not `access_token`). The HR frontend auth store maps this to `accessToken` internally. Keep using `token` for consistency with existing HR login response. The frontend already handles this.

**Location:** `internal/auth/handler.go` — add `SSOLogin` method to existing `AuthHandler`. Requires injecting `SSOService` into `AuthHandler` (or calling it via the integration package).

**Route registration:** Add to `internal/auth/routes.go` in the public group (no auth middleware, since the user doesn't have an HR token yet):
```go
auth.POST("/sso", loginLimiter, h.SSOLogin)
```

**Dependencies:** `INTEGRATION_JWT_SECRET` env var (already exists on HR server for the HR→Finance direction).

### 1.2 GET /v1/companies — List User's Companies

**Current state:** HR users have a single `company_id` FK in the `users` table. There is no multi-company join table.

**Approach:** Add a `user_companies` join table to support multi-company access. Seed it from existing `users.company_id`. The `users.company_id` field remains as the user's "active" company.

**Migration** (next available number after checking `db/migrations/`):
```sql
CREATE TABLE user_companies (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    role       VARCHAR(20) NOT NULL DEFAULT 'employee',
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, company_id)
);

-- Seed from existing user→company relationships
INSERT INTO user_companies (user_id, company_id, role)
SELECT id, company_id, role FROM users WHERE company_id IS NOT NULL
ON CONFLICT DO NOTHING;
```

**sqlc queries** (`db/query/user_companies.sql`):
```sql
-- name: GetUserCompanies :many
SELECT c.id, c.name, c.country, c.timezone, c.currency, c.logo_url
FROM user_companies uc
JOIN companies c ON c.id = uc.company_id
WHERE uc.user_id = $1
ORDER BY c.name;

-- name: GetUserCompanyMembership :one
SELECT user_id, company_id, role
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

**Endpoint:**
- Auth required
- Query `GetUserCompanies(auth.GetUserID(c))`
- Return `[{id, name, country, timezone, currency, logo_url}]`

**Location:** `internal/company/handler.go` — add `ListUserCompanies` method to existing company handler, register route in `internal/company/routes.go`:
```go
protected.GET("/companies", h.ListUserCompanies)
```

### 1.3 POST /v1/auth/switch-company

**Request:**
```json
{"company_id": 123}
```

**Flow (within a database transaction):**
1. Begin transaction using `h.pool.Begin()`
2. Query `GetUserCompanyMembership(userID, companyID)` using the tx
3. If not found → rollback, 403 `{"error": "access denied"}`
4. Call `UpdateUserActiveCompany(userID, companyID)` using the tx
5. Fetch user's role for the new company from the membership record
6. Commit transaction
7. Issue new `token` + `refresh_token` with updated `company_id` and `role`
8. Return `{token, refresh_token, user: {...}}`

If token generation fails after commit, the DB change is already applied but the user still has their old token — the next login or refresh will pick up the new company. This is acceptable since the alternative (generating tokens inside the tx) would hold the tx open longer.

**Location:** `internal/auth/handler.go`, route in `internal/auth/routes.go`:
```go
protected.POST("/switch-company", h.SwitchCompany)
```

### 1.4 POST /v1/auth/logout — Server-Side Token Revocation

**Request:**
```json
{"refresh_token": "<refresh_token_string>"}
```

**Flow:**
1. Extract `refresh_token` from request body
2. Parse the refresh token to get its `exp` claim (without full validation — it may already be the current user's token)
3. Compute `sha256(refresh_token)` as the blacklist key
4. Store `blacklist:refresh:{hash} → "1"` in Redis with TTL = remaining time until `exp`
5. Return 204 No Content

**Blacklist check:** Modify the `tryRefreshToken` flow in the frontend's API client — on the backend side, the refresh endpoint (`POST /auth/refresh`) must check Redis before issuing new tokens:
```go
// In the Refresh handler, before issuing new tokens:
hash := sha256Hex(req.RefreshToken)
if rdb.Exists(ctx, "blacklist:refresh:"+hash).Val() > 0 {
    return 401 "token revoked"
}
```

**Middleware change:** The access token middleware does NOT need a Redis blacklist check. Only the refresh endpoint checks it. This avoids the breaking change of adding `*redis.Client` to `JWTMiddleware`. When the user's access token expires naturally, the refresh call will fail, forcing re-login.

**Location:** `internal/auth/handler.go`, route in `internal/auth/routes.go`:
```go
protected.POST("/logout", h.Logout)
```

**Redis dependency:** The `AuthHandler` struct needs `*redis.Client` added. Wire it in `internal/app/bootstrap.go` where `AuthHandler` is constructed (line ~340). The `App.Redis` field already exists.

### 1.5 Jurisdiction Mapping

HR's `companies.country` uses ISO 3166-1 alpha-3 codes (`PHL`, `LKA`, `SGP`, `IDN`). The frontend jurisdiction badge and selector use 2-letter codes (`PH`, `SG`, `LK`).

**No DB change needed.** The mapping is already handled:
- HR backend returns `company_country` in the user profile response
- HR frontend's auth store already has a `jurisdiction` computed that maps the 3-letter code or falls back to localStorage
- The only change: ensure `GET /v1/auth/me` returns `company_country` consistently (it already does via the users query joining companies)

---

## Workstream 2: Finance→HR SSO Token Endpoint

### GET /api/v1/integrations/hr/sso-token (Finance Go Backend)

**Purpose:** Finance frontend's `openHR()` calls this to get an SSO token before navigating to HR.

**Pre-check:** Before issuing a token, verify that an active integration source exists linking this Finance company to an HR instance. Query `integration_sources` for `source_system="aigonhr"` with the current company ID. If no active source found, return 404 `{"error": "no HR integration configured"}`. This mirrors the HR-side check in `accounting_handler.go` that verifies `accounting_links` before issuing HR→Finance tokens.

**Flow:**
1. Auth required (user must be logged in to Finance)
2. Check active integration source exists for `source_system="aigonhr"` and current company
3. Build `FinanceToHRClaims`:
   - `iss = "aistarlight"`
   - `exp = now + 5 minutes`
   - `jti = "sso-{userID}-{unixMilli}"`
   - `Email = current user's email`
   - `FinanceCompanyID = current company UUID`
   - `FinanceUserID = current user UUID`
4. Sign JWT with `INTEGRATION_JWT_SECRET` (HS256)
5. Derive `target_url` from the integration source's `api_endpoint` field (e.g., `https://hr.halaos.com`), falling back to env var `HR_BASE_URL` if not set
6. Return `{sso_token: "...", target_url: "<derived>"}`

**Location:** `internal/handler/integration_handler.go` — add `GetHRSSOToken` method. Register route in `router.go`:
```go
integrationGroup.GET("/hr/sso-token", rt.Integration.GetHRSSOToken)
```

**Claims struct:** Define `FinanceToHRClaims` in `internal/handler/integration_handler.go` or a shared `internal/integration/sso_claims.go`. Must use `iss="aistarlight"` to match what HR's `ValidateFinanceToken()` expects.

**Note:** HR→Finance SSO token generation already exists in `aigonhr/internal/integration/accounting_handler.go` (`getAccountingSSOToken`). This is the mirror direction.

---

## Workstream 3: Deploy Finance Agent Phase 1

### Pre-Deployment Checklist

All code is committed and binaries rebuilt. Steps:

1. **Pull latest code** on Finance server (`34.124.185.43`)
2. **Run migration** `000042_action_plans.up.sql` — creates `action_plans` table
3. **Rebuild containers** — api + worker (they include the new tool executor, action plan manager, and agent endpoints)
4. **Restart nginx** (stale container IPs after rebuild)
5. **Verify migration** — check table exists and has correct structure:
   ```sql
   SELECT column_name, data_type FROM information_schema.columns
   WHERE table_name = 'action_plans' ORDER BY ordinal_position;
   ```

### Post-Deployment Verification

Agent routes use nested paths: `/api/v1/agents/:agentId/threads/:threadId/...` and `/api/v1/agents/:agentId/actions/:planId/...`.

1. **Low-risk tool test:** Send a chat message to an agent that triggers a low-risk tool (e.g., lookup_transaction). Verify:
   - SSE stream includes tool call + result in real-time
   - No ActionPlan created (low-risk tools auto-execute)

2. **High-risk tool test:** Send a chat message that triggers a high-risk tool (e.g., create_journal_entry). Verify:
   - SSE stream pauses with `action_plan_created` event
   - `GET /api/v1/agents/:agentId/threads/:threadId/pending-actions` returns the plan
   - `POST /api/v1/agents/:agentId/actions/:planId/confirm` resumes execution
   - Tool result appears in the continued SSE stream

3. **Cancel test:** Trigger high-risk tool, then `POST /api/v1/agents/:agentId/actions/:planId/cancel`. Verify the agent acknowledges cancellation.

---

## Workstream 4: Finance i18n Phase 2

### Scope

Extend `t()` calls to all ~35 internal page views. Shell (login, register, dashboard layout) is already done.

### Languages

English (`en.ts`) + Simplified Chinese (`zh.ts`). No new locales.

### Approach

For each view:
1. Find all hardcoded user-facing strings (labels, placeholders, messages, tooltips, table headers, button text, empty states, error messages)
2. Replace with `t('module.key')` calls
3. Add translations to both `en.ts` and `zh.ts`

### Locale Key Convention

Follow existing pattern in `en.ts`:
```typescript
{
  moduleName: {
    title: 'Page Title',
    description: 'Page description',
    table: {
      columnName: 'Column Name',
    },
    form: {
      fieldName: 'Field Name',
    },
    action: {
      create: 'Create Item',
      delete: 'Delete',
    },
    empty: 'No items found',
    error: {
      loadFailed: 'Failed to load data',
    },
  },
}
```

### Batches (by menu group)

| Batch | Views | Est. Keys |
|-------|-------|-----------|
| Data & Transactions | Upload, ReceiptUpload, Transactions, TransactionClassification, Vendors, Tags, Approvals, Mapping | ~120 |
| Accounting | ChartOfAccounts, JournalEntries, GeneralLedger, FinancialStatements | ~80 |
| Tax & Filing | TaxPrep, Reports, ReportEdit, FormRouter, FilingCalendar, PenaltyCalculator, Withholding, CASCompliance, TaxBridge | ~150 |
| Reconciliation | Reconciliation, BankReconciliation | ~50 |
| More | Chat, Knowledge, LearningInsights, VendorPolicies, Spending, PeriodComparison, Invoices | ~100 |
| System | Settings, Memory, Guide, OrgDashboard, OrgManage, Integration, GLMapping, Dashboard | ~80 |

**Total estimate:** ~580 translation keys across 35 views.

### Verification

After each batch:
- Switch locale to Chinese, verify all strings render correctly
- Switch back to English, verify no broken keys (`t('...')` showing raw key)
- Check console for missing translation warnings
- Run a grep for untranslated strings: `grep -rn ">[A-Z][a-z]" src/views/` to catch remaining hardcoded English text

### Key Parity Enforcement

After all batches complete, verify en/zh key parity:
- Extract all top-level and nested keys from both `en.ts` and `zh.ts`
- Diff the key sets to find any keys present in one but missing in the other
- Fix any mismatches before final commit

---

## Execution Order

1. **Workstream 3** (Agent deployment) — independent, fast (30 min ops work)
2. **Workstream 1** (HR auth endpoints) — depends on nothing, enables Workstream 2
3. **Workstream 2** (Finance SSO token) — depends on Workstream 1 for end-to-end testing
4. **Workstream 4** (i18n) — fully independent, can run in parallel with 1-2

After all workstreams:
- End-to-end SSO test: HR → Finance → HR round-trip
- Company switching test on HR
- Agent tool execution test on Finance
- Full locale switching test on Finance

---

## Files Impact Summary

### AIGoNHR (HR)

| File | Action |
|------|--------|
| `db/migrations/000XX_user_companies.sql` | Create — new join table + seed |
| `db/query/user_companies.sql` | Create — sqlc queries (GetUserCompanies, GetUserCompanyMembership, UpdateUserActiveCompany, InsertUserCompany) |
| `internal/integration/sso.go` | Modify — add `FinanceToHRClaims` struct + `ValidateFinanceToken()` method |
| `internal/auth/handler.go` | Modify — add SSOLogin, SwitchCompany, Logout methods; add `*redis.Client` to handler struct |
| `internal/auth/routes.go` | Modify — register `/sso` (public), `/switch-company` (protected), `/logout` (protected) routes |
| `internal/company/handler.go` | Modify — add `ListUserCompanies` method |
| `internal/company/routes.go` | Modify — register `GET /companies` route |
| `internal/app/bootstrap.go` | Modify — wire Redis client into AuthHandler constructor |

### AIStarlight-Go (Finance Backend)

| File | Action |
|------|--------|
| `internal/handler/router.go` | Modify — add `GET /integrations/hr/sso-token` route |
| `internal/handler/integration_handler.go` | Modify — add `GetHRSSOToken` method |

### AIStarlight (Finance Frontend)

| File | Action |
|------|--------|
| `frontend/src/locales/en.ts` | Modify — add ~580 keys |
| `frontend/src/locales/zh.ts` | Modify — add ~580 keys |
| `frontend/src/views/*.vue` (35 files) | Modify — replace hardcoded strings with t() calls |
