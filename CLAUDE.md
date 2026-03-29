# HalaOS

AI-powered HR Operating System for Southeast Asia.

## Stack
- Go 1.25, Gin, sqlc (pgx/v5), PostgreSQL, Redis, Docker Compose
- Frontend: Vue3 + TS + NaiveUI (in `frontend/`)
- Mobile: Vue3 + TS + NaiveUI (in `frontend-mobile/`)

## Development

```bash
# Start database and Redis
docker compose up -d

# Run API
go run ./cmd/api

# Run worker
go run ./cmd/worker

# Run tests
go test ./...

# Generate sqlc
sqlc generate

# Frontend
cd frontend && npm install && npm run dev
```

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
- **JWT**: Shared signing secret `INTEGRATION_JWT_SECRET` for cross-app SSO
