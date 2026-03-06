# AIGoNHR

Go + Vue 3 HR management system for Philippine companies. Covers payroll, attendance, leave, compliance (SSS/PhilHealth/Pag-IBIG/BIR), and 30+ HR modules.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, Gin, sqlc, pgx v5 |
| Frontend | Vue 3, TypeScript, Naive UI, Vite |
| Database | PostgreSQL 16, 31 migrations |
| Cache | Redis 7 |
| Auth | JWT (access + refresh tokens) |
| AI | Claude API (optional) |
| Deploy | Docker Compose |
| CI | GitHub Actions |

## Quick Start

```bash
# 1. Start databases
docker compose up -d

# 2. Copy and edit env
cp .env.example .env

# 3. Run migrations
make migrate-up

# 4. Start API server
make run

# 5. Start frontend (separate terminal)
cd frontend && npm install && npm run dev
```

API: `http://localhost:8080/api/v1`
Frontend: `http://localhost:5173`

## Modules

| Module | Description |
|--------|------------|
| Auth | Login, JWT, roles (admin/hr/manager/employee) |
| Employee | CRUD, departments, positions, employment history |
| Attendance | Clock in/out, shifts, schedules, geofencing |
| Leave | Types, balances, requests, approvals, carryover |
| Overtime | Requests, approvals, rate calculation |
| Payroll | Cycles, computation, payslips, 13th month |
| Compliance | SSS, PhilHealth, Pag-IBIG, BIR tax tables |
| Benefits | Enrollment, plans, dependents |
| Loans | Applications, amortization, deductions |
| Performance | Reviews, KPIs, goals |
| Onboarding | Checklists, task tracking |
| Training | Programs, enrollment, completion |
| Disciplinary | Cases, hearings, sanctions |
| Grievance | Filing, investigation, resolution |
| Expense | Claims, receipts, reimbursement |
| Documents | Employee files, expiry tracking |
| Policies | Company policies, acknowledgment |
| Knowledge | Knowledge base, search |
| Clearance | Separation clearance workflow |
| Final Pay | Computation, release |
| Reports | HR analytics, exports |
| Dashboard | Stats, charts, summaries |
| Notifications | In-app notifications |
| Announcements | Company-wide announcements |
| Holidays | Holiday calendar management |
| AI | AI-powered HR assistant (optional) |

## Project Structure

```
aigonhr/
├── cmd/
│   ├── api/          # API server entry point
│   ├── worker/       # Background worker
│   └── migrate/      # Migration CLI
├── internal/
│   ├── app/          # Bootstrap, routing
│   ├── auth/         # Auth handlers + middleware
│   ├── store/        # sqlc generated code
│   ├── testutil/     # Test infrastructure
│   └── .../          # 30+ handler packages
├── db/
│   ├── migrations/   # 31 SQL migrations
│   └── query/        # sqlc query files
├── frontend/         # Vue 3 SPA
├── pkg/              # Shared utilities
└── docker-compose.yml
```

## Development

```bash
make build          # Build all binaries
make test           # Run tests (151 tests)
make test-cover     # Generate coverage report
make vet            # Run go vet
make lint           # Run golangci-lint
make sqlc           # Regenerate sqlc code
make tidy           # go mod tidy
```

## Production Deployment

```bash
# Build for Linux
make build-linux

# Deploy with Docker Compose
docker compose -f docker-compose.prod.yml up -d
```

Services: PostgreSQL, Redis, migrate (init), API, worker, frontend (Nginx).

## Environment Variables

See [`.env.example`](.env.example) for all configuration options.

Key variables:
- `POSTGRES_*` — Database connection
- `REDIS_*` — Cache connection
- `JWT_SECRET` — Auth token signing key
- `ANTHROPIC_API_KEY` — AI features (optional)

## License

Private
