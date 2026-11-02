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

## Employee Lifecycle (Warp-style)
- **Package**: `internal/lifecycle` — `Service.Hire` / `Service.Terminate` run all downstream steps in one tx; use these instead of chaining employee/salary/onboarding/clearance calls
- **Routes**: `POST /lifecycle/hire`, `GET /lifecycle/employees/:id`, `POST|DELETE /lifecycle/employees/:id/terminate`, `GET /inbox`, `/custom-fields`, `/employees/:id/custom-fields`
- **Scheduled separations**: `employee_separations` (status scheduled → completed by worker on last working day)
- **Docs**: `docs/hr-lifecycle.md`

## Government filing agent / e-sign / devices
- `internal/govfiling`: catalog (forms × channels), dataset+file builders, pre-flight rules + AI review, service (prepare/approve/execute), browser engine `browser/` (chromedp playbooks, human_input hand-off, dry runs), autopilot. Never submit without approval; never auto-solve CAPTCHA.
- `internal/selfonboard`: onboarding invites + e-signature (public routes under `/api/v1/public`, token hashed).
- `internal/devices`, `internal/finalpay/compute.go`; AI tools in `internal/ai/tools_lifecycle.go` (write tools must be listed in `internal/ai/draft/service.go`).
- Private files (`GOV_FILES_DIR`, `DOC_FILES_DIR`) must never live under the public `/uploads`.
- Docs: `docs/hr-lifecycle.md`. Local demo portal: `go run ./cmd/mockgov`.

## Editions (HalaOS / HalaPeople)
- `internal/edition` + `frontend/src/edition.ts` define modules per product; `APP_EDITION=people` builds HalaPeople.
- New handlers must be registered via `a.mod(edition.ModX, protected)` in `internal/app/bootstrap.go`; new AI tool groups need a module in `toolGroups` (`internal/ai/tools.go`); new pages need a module in `routeModule` (`frontend/src/edition.ts`).
- Docs: `docs/halapeople.md`; deploy: `docker-compose.people.yml`.
