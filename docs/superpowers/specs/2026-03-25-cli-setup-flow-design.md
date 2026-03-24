# CLI Setup Flow: One-Step Registration & API Key Provisioning

## Problem

New HalaOS users must: register in browser → verify email → login → navigate to Settings → create API Key → copy key → configure MCP. This 7-step flow blocks MCP Server adoption since users need `HALAOS_API_KEY` before any tool works.

## Solution

Add a `setup_account` MCP tool that handles register/login + API key creation in a single CLI interaction. Backed by two new backend endpoints (`cli-register`, `cli-login`) that bundle authentication with automatic API key provisioning.

## User Flow

```
User installs halaos-hr skill
  │
  ▼
First use: HALAOS_API_KEY not set
  │
  ▼
Skill detects missing key → suggests setup_account tool
  │
  ▼
Claude asks: "Do you have a HalaOS account?"
  ├─ No  → setup_account(action=register, email, password)
  └─ Yes → setup_account(action=login, email, password)
  │
  ▼
MCP Server calls cli-register or cli-login endpoint
  │
  ▼
Backend returns: JWT + API Key (full key, shown once)
  │
  ▼
Tool output includes key + config instructions
  │
  ▼
User sets HALAOS_API_KEY → all tools now work
```

## Backend Changes

### New Endpoint: `POST /v1/auth/cli-register`

Public endpoint. Rate limited: shares the existing login rate limiter (default 10 req/5min per IP, configurable via `RATE_LIMIT_LOGIN_RATE` / `RATE_LIMIT_LOGIN_WINDOW`).

**Request:**
```json
{
  "email": "user@company.com",
  "password": "SecurePass123",
  "company_name": "My Corp",
  "country": "PHL"
}
```

- `email` — required, valid email format
- `password` — required, min 8 chars. **Implementation note:** define a new `cliRegisterRequest` struct with `binding:"required,min=8"` on the password field (the existing `registerRequest` allows empty passwords for magic-link flow)
- `company_name` — optional, defaults to email domain
- `country` — optional, defaults to "PHL"
- `referral_code` — optional, processed identically to existing register

**Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "token": "eyJ...",
    "refresh_token": "eyJ...",
    "api_key": "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
    "api_key_prefix": "halaos_a1b2c3",
    "user": {
      "id": 1,
      "email": "user@company.com",
      "first_name": "User",
      "last_name": "",
      "role": "super_admin",
      "company_id": 1
    }
  }
}
```

**Error responses:**
- `409 Conflict` — email already registered (`{"error": {"code": "email_exists", "message": "..."}}`). Response includes hint to use `cli-login` instead.
- `400 Bad Request` — invalid input (missing email, password too short)
- `429 Too Many Requests` — rate limited

**Behavior vs existing `/auth/register`:**
- Auto-sets `email_verified = true` (skips verification)
- Auto-creates API key with name `"cli-default"`
- Returns API key in response body (existing register does not)
- Does NOT send verification email
- All other company setup logic (leave types, holidays, token balance) runs identically

### New Endpoint: `POST /v1/auth/cli-login`

Public endpoint. Rate limited: shares the existing login rate limiter.

**Request:**
```json
{
  "email": "user@company.com",
  "password": "SecurePass123"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "token": "eyJ...",
    "refresh_token": "eyJ...",
    "api_key": "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
    "api_key_prefix": "halaos_a1b2c3",
    "user": {
      "id": 1,
      "email": "user@company.com",
      "first_name": "John",
      "last_name": "Doe",
      "role": "super_admin",
      "company_id": 1
    }
  }
}
```

**Behavior:**
- Uses `GetUserByEmailAny` (not `GetUserByEmail`) to find user regardless of status, then checks status explicitly — matching the existing `Login` handler pattern
- Standard login validation (email, password, account active)
- If user has `email_verified=false`, auto-verify it (CLI users have proven identity via password)
- Revokes any existing `cli-default` API key for this user, then creates a fresh one. This avoids key accumulation from repeated CLI setups while giving each session a clean key.
- Returns the new key in response

**Error responses:**
- `401 Unauthorized` — invalid credentials (generic message for both "user not found" and "wrong password" to prevent email enumeration)
- `403 Forbidden` — account disabled/suspended

**Note on email enumeration:** Unlike the `cli-register` 409 (which necessarily reveals existence), `cli-login` returns a generic 401 for all auth failures. The MCP tool handles the UX: if login fails, it suggests the user try registering instead.

### Implementation Location

Both handlers go in `internal/auth/handler.go` (alongside existing Register/Login). They share most logic with the existing handlers but add the auto-verify + auto-key-create steps.

Define new request structs:
```go
type cliRegisterRequest struct {
    Email       string `json:"email" binding:"required,email"`
    Password    string `json:"password" binding:"required,min=8"`
    CompanyName string `json:"company_name"`
    Country     string `json:"country"`
    ReferralCode string `json:"referral_code"`
}

type cliLoginRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required"`
}
```

### Routes

Add to `internal/auth/routes.go` in the `auth` group (which is under `public`):
```go
auth.POST("/cli-register", loginLimiter, h.CLIRegister)
auth.POST("/cli-login", loginLimiter, h.CLILogin)
```

## MCP Server Changes

### Startup Restructuring (`main.go`)

The current `main.go` hard-exits when `HALAOS_API_KEY` is empty. This must change:

1. **Remove the hard exit** — `HALAOS_API_KEY` becomes optional
2. **Make the API client optional** — `nil` when no key provided
3. **Guard tool registration** — all existing tools registered only when client is valid
4. **Always register `setup_account`** — this tool works without authentication

```go
// Pseudocode for new startup flow
apiKey := os.Getenv("HALAOS_API_KEY")
baseURL := os.Getenv("HALAOS_BASE_URL")

var client *Client  // nil if no API key
if apiKey != "" {
    client = NewClient(baseURL, apiKey)
}

// Always register setup tool (uses baseURL directly, no API key)
registerSetupTool(server, baseURL)

// Only register operational tools when authenticated
if client != nil {
    registerEmployeeTools(server, client)
    registerLeaveTools(server, client)
    // ... all other tools
}
```

### New Tool: `setup_account`

**Location:** `cmd/mcp/tools_setup.go` (new file)

**HTTP calls:** This tool makes direct HTTP calls using `net/http` (not the existing `Client` struct, which requires an API key). It only calls two public endpoints so a standalone function is sufficient:

```go
func callCLIEndpoint(baseURL, path string, body interface{}) (map[string]interface{}, error) {
    // Direct POST to baseURL + "/api/v1/auth/" + path
    // No Authorization header
    // Returns parsed JSON response
}
```

**Tool Definition:**
```
Name: setup_account
Description: Register or login to HalaOS to get an API key for MCP Server access.
             Use this when HALAOS_API_KEY is not configured.

Input Schema:
  action:        enum("register", "login") — required
  email:         string — required
  password:      string — required (min 8 chars)
  company_name:  string — optional (register only)
  country:       string — optional (register only), default "PHL"
  referral_code: string — optional (register only)
```

**Output (success):**
```json
{
  "api_key": "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
  "message": "Account created successfully. Your API key has been generated.",
  "config_hint": "Set HALAOS_API_KEY=halaos_a1b2... in your MCP server configuration."
}
```

**Output (error):**
```json
{
  "error": "Email already registered. Try action='login' instead.",
  "suggestion": "login"
}
```

**Key behaviors:**
- Does NOT require `HALAOS_API_KEY` to work (calls public endpoints)
- Uses `HALAOS_BASE_URL` to know which server to call
- This tool is always registered, even without a valid API key
- All other tools still require a valid API key
- Tool output MUST NOT echo the password back

## Skill Changes

### SKILL.md Update

Add a setup section to the skill instructions:

```markdown
## First-Time Setup

If `HALAOS_API_KEY` is not set, the only available tool is `setup_account`.
Use it to register or login:

1. Ask the user if they have an existing account
2. Call setup_account with action="register" or action="login"
3. Show the returned API key and config instructions
4. Tell the user to add HALAOS_API_KEY to their MCP config and restart
```

## Security Considerations

- **Rate limiting**: Both CLI endpoints share the existing login rate limiter (default 10 req/5min per IP). This prevents brute-force registration or credential stuffing.
- **Password in MCP tool arguments**: Passwords passed as tool arguments are visible in the conversation transcript and may be logged by the MCP host. This is an accepted trade-off for CLI convenience — the same exposure exists when users type passwords in any CLI tool. The tool output MUST NOT echo the password back. Users concerned about this can register via browser instead.
- **Password required**: Unlike browser register (which can auto-generate), CLI register requires an explicit password with min 8 chars. Users choosing their password in a CLI context should be intentional.
- **Email enumeration**: `cli-register` returns 409 for existing emails (unavoidable — user needs to know to switch to login). `cli-login` returns generic 401 for all auth failures (no enumeration). The rate limiter bounds probe attempts.
- **API key shown once**: The full key is returned only in the cli-register/cli-login response. It's never stored in plaintext on the server.
- **Auto-verify on cli-login**: If a user registered via browser but never verified, cli-login auto-verifies them. This is acceptable: password knowledge proves identity, and the alternative (blocking CLI users who forgot to verify) creates worse UX. The email address was already provided by the user at registration time.
- **Key rotation on cli-login**: Each cli-login revokes the previous `cli-default` key and creates a fresh one. This limits stale key accumulation while ensuring each CLI setup is clean. Users who need multiple keys should create them via the Settings UI.

## Files to Modify

| File | Change |
|------|--------|
| `internal/auth/handler.go` | Add `CLIRegister()` and `CLILogin()` handlers with new request structs |
| `internal/auth/routes.go` | Add `/cli-register` and `/cli-login` to auth group with login limiter |
| `cmd/mcp/tools_setup.go` | New file: `setup_account` tool with standalone HTTP client |
| `cmd/mcp/main.go` | Remove hard exit on missing key; make client optional; conditional tool registration |
| `openclaw-skill/SKILL.md` | Add first-time setup instructions |

## Testing

- Unit test: `CLIRegister` creates user + company + API key, returns key in response
- Unit test: `CLIRegister` with existing email returns 409 with login hint
- Unit test: `CLIRegister` password validation (rejects < 8 chars)
- Unit test: `CLILogin` with valid creds returns new API key
- Unit test: `CLILogin` with unverified email auto-verifies and returns key
- Unit test: `CLILogin` with invalid creds returns generic 401 (no email leak)
- Unit test: `CLILogin` revokes previous `cli-default` key before creating new one
- Unit test: `CLILogin` called twice — only one active `cli-default` key exists
- MCP tool test: `setup_account` register flow returns valid key
- MCP tool test: `setup_account` login flow returns valid key
- MCP tool test: `setup_account` is available without HALAOS_API_KEY
- MCP tool test: other tools are NOT available without HALAOS_API_KEY
- Integration: Full flow — register via tool → set key → use other tools

## Out of Scope

- Auto-writing the key to MCP config files (user does this manually)
- OAuth/SSO flow from CLI
- Token refresh in MCP server (API keys don't expire)
- Browser-based setup wizard
