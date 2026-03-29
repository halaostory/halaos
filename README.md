# HalaOS

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

AI-powered HR Operating System for Southeast Asia.

HalaOS is a full-featured HR platform with payroll processing, attendance tracking, leave management, tax compliance, and 9 specialized AI agents — built for Philippine, Singaporean, and Sri Lankan labor law.

## Features

- **Employee Management** — Profiles, 201 files, org chart, directory
- **Attendance & Time** — Clock in/out, GPS geofencing, shift scheduling, DTR reports
- **Leave Management** — Balances, requests, approvals, calendar, carryover, encashment
- **Payroll** — Multi-frequency cycles, automated computation, payslips, 13th month pay, final pay
- **Tax Compliance** — PH (SSS, PhilHealth, Pag-IBIG, BIR), SG (CPF, IRAS), LK (EPF/ETF)
- **Government Forms** — BIR 2316/2550M/2550Q/1601C/1701/1702/0619E/SAWT
- **Benefits & Loans** — Plan enrollment, claims, amortization, salary deduction
- **Training & Performance** — Programs, certifications, KPIs, review cycles
- **Onboarding/Offboarding** — Checklists, clearance workflows, final pay
- **9 AI Agents** — HR assistant, payroll specialist, compliance officer, leave advisor, and more
- **Org Intelligence** — Flight risk detection, burnout risk, team health, blind spot analysis
- **Multi-tenant** — Company isolation with role-based access control

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, Gin, sqlc, pgx/v5 |
| Frontend | Vue 3, TypeScript, NaiveUI |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| AI | Claude API (Anthropic) |
| Infra | Docker Compose |

## Quick Start

```bash
# Clone the repository
git clone https://github.com/halaostory/halaos.git
cd halaos

# Start PostgreSQL and Redis
docker compose up -d

# Configure environment
cp .env.example .env
# Edit .env — set JWT_SECRET and POSTGRES_PASSWORD (required)

# Run migrations
go run ./cmd/migrate -cmd up

# Start API server
go run ./cmd/api

# Start worker (separate terminal)
go run ./cmd/worker

# Start frontend (separate terminal)
cd frontend && npm install && npm run dev
```

The app will be available at http://localhost:5173 (frontend) and http://localhost:8080 (API).

## Production Deployment

```bash
# Build Go binaries
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api ./cmd/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/worker ./cmd/worker
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/migrate ./cmd/migrate

# Build frontend
cd frontend && npm ci && npm run build

# Deploy with Docker Compose
docker compose -f docker-compose.deploy.yml up -d --build
```

## Project Structure

```
halaos/
├── cmd/                    # Application entrypoints
│   ├── api/                # HTTP API server
│   ├── worker/             # Background worker
│   ├── migrate/            # Database migration tool
│   └── mcp/                # MCP server for AI integrations
├── internal/               # Private application code
│   ├── auth/               # JWT authentication
│   ├── config/             # Configuration loading
│   ├── handler/            # HTTP handlers (per domain)
│   ├── integration/        # Cross-app integrations
│   └── ...
├── pkg/                    # Shared packages
│   └── response/           # API response helpers
├── db/
│   ├── migrations/         # SQL migrations (goose)
│   └── query/              # sqlc query files
├── frontend/               # Vue 3 desktop frontend
├── frontend-mobile/        # Vue 3 mobile (H5) frontend
├── e2e/                    # End-to-end tests (Playwright)
└── openclaw-skill/         # OpenClaw skill definition
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

[MIT](LICENSE)
