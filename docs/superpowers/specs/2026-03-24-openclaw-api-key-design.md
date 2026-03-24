# HalaOS OpenClaw API Key Authentication — Design Spec

## Goal

Add API Key authentication to HalaOS so all 150+ existing endpoints are accessible via `Authorization: Bearer halaos_...` tokens. This enables the published ClawHub skill (`halaos-hr@1.0.0`) to interact with HalaOS programmatically through OpenClaw.

## Context

HalaOS currently uses JWT-only authentication. The `halaos-hr` skill was published to ClawHub referencing `HALAOS_API_KEY`, but no API Key system exists yet. Management Brain (`management-brain@2.3.0`) has a working implementation of this pattern that we replicate here.

## Non-Goals

- New endpoints or features beyond API Key auth
- Scoped permissions (future enhancement)
- Rate limiting per API Key (existing API rate limiter covers this)
- US jurisdiction support (separate spec)
- Frontend UI for API Key management (API-only for now)

---

## Database

### Migration: `db/migrations/00082_api_keys.sql`

```sql
CREATE TABLE api_keys (
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

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash) WHERE is_active = true;
CREATE INDEX idx_api_keys_user ON api_keys(user_id) WHERE is_active = true;
```

Design decisions:
- `BIGSERIAL` PK (consistent with all other HalaOS tables, not UUID)
- Bound to `user_id` + `company_id` (inherits user's role in that company)
- Only SHA-256 hash stored; raw key returned once at creation
- Soft delete via `is_active` flag
- `last_used_at` updated asynchronously (best-effort)

### Key Format

`halaos_` + 40 hex characters (20 random bytes) = 47 characters total.

Example: `halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0`

Prefix for display: first 12 characters, e.g. `halaos_a1b2c`

---

## API Key Middleware

### File: `internal/auth/apikey.go`

**Functions:**

```go
func generateAPIKey() string
// Returns "halaos_" + 40 hex chars (20 random bytes)

func apiKeyPrefix(key string) string
// Returns first 12 chars for display

func hashAPIKey(key string) string
// Returns SHA-256 hex digest (64 chars)

func APIKeyMiddleware(queries *store.Queries) gin.HandlerFunc
// Middleware: checks Bearer token for "halaos_" prefix
```

**Middleware flow:**

1. Extract `Authorization: Bearer <token>` header
2. If token does not start with `halaos_` → call `c.Next()` (fall through to JWT)
3. Hash token with SHA-256
4. Query `GetAPIKeyByHash(ctx, hash)` — joins `users` table to get role + email
5. If not found or inactive → 401 Unauthorized
6. Set context: `user_id`, `email`, `role`, `company_id` (same keys as JWT middleware)
7. Async update `last_used_at` via goroutine (best-effort)
8. Call `c.Next()`

### Middleware chain change in `internal/app/bootstrap.go`

```go
protected := v1.Group("/")
protected.Use(auth.APIKeyMiddleware(queries))  // NEW: check API Key first
protected.Use(auth.JWTMiddleware(jwtSvc))      // EXISTING: fallback to JWT
```

### JWTMiddleware early return in `internal/auth/middleware.go`

Add at the top of JWTMiddleware:
```go
if _, exists := c.Get("user_id"); exists {
    c.Next()
    return
}
```

This allows API Key auth to set context and skip JWT validation entirely.

---

## API Key CRUD Endpoints

### Routes (added to `internal/auth/routes.go`)

```go
protected.POST("/api-keys", h.CreateAPIKey)
protected.GET("/api-keys", h.ListAPIKeys)
protected.DELETE("/api-keys/:id", h.RevokeAPIKey)
```

These are in the protected group (require auth — either JWT or API Key).

### POST /api/v1/api-keys — Create API Key

**Request:**
```json
{
    "name": "My OpenClaw Key"
}
```

**Response (201):**
```json
{
    "success": true,
    "data": {
        "id": 1,
        "prefix": "halaos_a1b2c",
        "name": "My OpenClaw Key",
        "key": "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
        "created_at": "2026-03-24T10:30:45Z"
    }
}
```

**Logic:**
1. Generate random key with `generateAPIKey()`
2. Compute prefix with `apiKeyPrefix(key)`
3. Compute hash with `hashAPIKey(key)`
4. Insert into `api_keys` with current user's `user_id` and `company_id`
5. Return the raw key (this is the only time it's shown)

### GET /api/v1/api-keys — List API Keys

**Response (200):**
```json
{
    "success": true,
    "data": [
        {
            "id": 1,
            "prefix": "halaos_a1b2c",
            "name": "My OpenClaw Key",
            "created_at": "2026-03-24T10:30:45Z",
            "last_used_at": "2026-03-24T15:22:10Z"
        }
    ]
}
```

Filtered by `WHERE user_id = $current_user AND is_active = true`.

### DELETE /api/v1/api-keys/:id — Revoke API Key

**Response (200):**
```json
{
    "success": true,
    "data": {"revoked": true}
}
```

Soft delete: `SET is_active = false WHERE id = $1 AND user_id = $2`.

---

## sqlc Queries

### File: `db/query/api_keys.sql`

```sql
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

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `db/migrations/00082_api_keys.sql` | New | Table + indexes |
| `db/query/api_keys.sql` | New | 5 sqlc queries |
| `internal/store/` | Regenerate | `sqlc generate` |
| `internal/auth/apikey.go` | New | Key gen, hash, middleware, 3 CRUD handlers |
| `internal/auth/middleware.go` | Modify | Add early return if user_id already set |
| `internal/auth/routes.go` | Modify | Register 3 API Key routes |
| `internal/app/bootstrap.go` | Modify | Insert APIKeyMiddleware before JWTMiddleware |

**Zero changes to:** all existing handlers (employee, payroll, leave, attendance, compliance, etc.)

---

## Testing

- Unit tests for `generateAPIKey()`, `apiKeyPrefix()`, `hashAPIKey()`
- Unit test for APIKeyMiddleware: valid key, invalid key, JWT fallthrough
- Unit tests for CRUD handlers using `testutil.MockDBTX` pattern
- Integration: verify existing JWT auth still works after middleware chain change

## Security Considerations

- Raw key never stored, only SHA-256 hash
- Raw key returned only once at creation
- Keys bound to user + company (no privilege escalation)
- Soft delete preserves audit trail
- Existing rate limiter applies to API Key requests
- `last_used_at` enables stale key detection
