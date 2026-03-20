# AIGoNHR

PH/LK/SG HR system — employees, attendance, leave, payroll, compliance, benefits, training.

## Stack
- Go 1.25, Gin, sqlc (pgx/v5), PostgreSQL, Redis, Docker Compose
- Frontend: Vue3 + TS + NaiveUI (in `frontend/`)
- Tests: 258 unit tests, `testutil.MockDBTX` pattern

## Build & Deploy
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api ./cmd/api`
- Server: 3.1.66.212 (`ssh aigonhr`), compose: `docker-compose.deploy.yml`
- sqlc: `~/go/bin/sqlc generate`

## Key Patterns
- Handler: `struct{queries, pool, logger}` → `RegisterRoutes(protected)`
- Response: `pkg/response` wraps `{success, data, error, meta}`
- Auth: `auth.GetCompanyID(c)`, `auth.GetUserID(c)`
- SQL: Optional filters MUST use `$N = '' OR` / `$N = 0 OR` alongside `$N IS NULL OR`
- Migrations: `db/migrations/` (goose), queries: `db/query/`

## Integration with AIStarlight (Accounting)
- **Pattern**: Event outbox → signed webhook → AIStarlight inbox
- **Tables**: `accounting_links` (company link), `accounting_outbox` (event queue)
- **Event types**: `payroll.run.completed`, `payroll.run.reversed`, `employee.upserted`, `employee.terminated`
- **Event structs**: `internal/integration/accounting_events.go`
- **Queries**: `db/query/accounting_integration.sql`
- **AIStarlight endpoint**: POST `/api/v1/webhooks/aigonhr` (HMAC-signed)
- **Export APIs** (backfill): `/api/v1/integrations/accounting/export/employees`, `/export/payroll-runs`
- **JWT**: Shared signing secret `INTEGRATION_JWT_SECRET` for cross-app SSO

## gstack

Use the `/browse` skill from gstack for all web browsing. **Never use `mcp__claude-in-chrome__*` tools.**

### Available Skills
`/office-hours`, `/plan-ceo-review`, `/plan-eng-review`, `/plan-design-review`, `/design-consultation`, `/review`, `/ship`, `/browse`, `/qa`, `/qa-only`, `/design-review`, `/setup-browser-cookies`, `/retro`, `/investigate`, `/document-release`, `/codex`, `/careful`, `/freeze`, `/guard`, `/unfreeze`, `/gstack-upgrade`
