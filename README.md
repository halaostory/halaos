# HalaPeople

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Open-source HR on autopilot for Southeast Asia: an open alternative to [Warp](https://www.warp.co).**

HalaPeople runs payroll, government compliance and the whole employee lifecycle with an AI
agent that does the work and asks before anything consequential. It is built for Philippine
labor and tax law first, with Singapore, Malaysia and Indonesia next. Self-host it, read the
code, change it.

## What it does

**The agent and its routines**
- **Home**: a greeting, your to-dos, the next payroll, government registration status for each
  agency (Active / Needs info / Needs approval / Filing started / Awaiting agency), upcoming and
  overdue filings, who is starting and leaving, and what the agent did.
- **Routines**: jobs the agent owns end to end.
  - *New hire*, *Offboarding*, *Pre-payroll check* and *Filing autopilot* are built in.
  - Each runs at the autonomy you pick: recommend, ask first, or autonomous.
  - You can add your own steps in plain language.
  - Every run is logged with what the agent did, why, and what it left for you.
  - Hard limits always apply: government submissions, payroll approval and final pay wait for a
    person.
- **Government notices**: paste or upload a letter from BIR, SSS, PhilHealth or Pag-IBIG.
  - The agent reads it and checks it against your filings.
  - Already filed: it drafts a reply citing your filing reference.
  - Not filed: it offers to prepare the missing return.
  - Audit: it escalates to your tax adviser.
  - Nothing is sent automatically.
- **AI assistant**: answers HR questions over your data. Write actions go through
  confirm-before-act drafts.

**Payroll and compliance**
- **Payroll**:
  - several pay schedules per company (monthly, semi-monthly, bi-weekly, weekly) with prorated
    pay, contributions and tax
  - 13th month pay
  - final pay and off-cycle runs
  - loans and payslips
  - auto-run
- **Statutory filings**: BIR 1601-C, 1604-C/alphalist, 2316, 0619-E, 1601-EQ + QAP; SSS R-3;
  PhilHealth RF-1; Pag-IBIG MCRF. These are generated from payroll and reviewed by rules + AI.
  After approval they are submitted via API, e-mail or a supervised browser agent (CAPTCHA/OTP
  are handed to a human).
- **New-hire registrations**: SSS R-1A, PhilHealth ER2, Pag-IBIG and BIR 1902 are prepared
  automatically.
- **Contractors**: freelancer payments with expanded withholding (EWT), bank file, BIR 2307,
  0619-E / 1601-EQ.

**People and lifecycle**
- **One-step hire and terminate**:
  - Hire creates the employee, compensation, leave balances, onboarding checklist and app
    accounts.
  - Terminate handles clearance, final pay, device recall and access revocation.
- **Self-onboarding portal and e-signature**: offer letters, IDs, bank details, and a signed
  PDF with an audit trail.
- **HR inbox**: every approval, task, expiring document, filing and notice in one list.
- **Other modules**:
  - employee records, 201 file, custom fields, documents
  - time off and overtime
  - benefits and expenses
  - devices/MDM and app provisioning

The optional **suite** edition adds attendance and time tracking (geofenced clock-in, schedules,
DTR), performance and training, engagement (pulse surveys, recognition) and people analytics.

See [docs/warp-parity.md](docs/warp-parity.md) for a feature-by-feature comparison with Warp,
and [docs/halapeople.md](docs/halapeople.md) for editions, deployment and the market roadmap.

## Tech stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, Gin, sqlc, pgx/v5 |
| Frontend | Vue 3, TypeScript, NaiveUI |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| AI | Claude API (Anthropic), optional; everything works without it |
| Browser automation | chromedp (government portals) |
| Infra | Docker Compose |

## Quick start

```bash
git clone https://github.com/halaostory/halaos.git
cd halaos

docker compose up -d                 # PostgreSQL + Redis
cp .env.example .env                 # set JWT_SECRET and POSTGRES_PASSWORD

go run ./cmd/migrate -cmd up
go run ./cmd/api                     # API on :8080
go run ./cmd/worker                  # routines, filing autopilot, scheduled separations
cd frontend && npm install && npm run dev   # web app on :5173
```

Useful settings (see `.env.example`):

| Variable | Default | Purpose |
|---|---|---|
| `APP_EDITION` | `people` | `people` or `suite` (adds attendance, talent, engagement, analytics) |
| `ANTHROPIC_API_KEY` | — | Turns on the AI assistant, AI notice triage and AI filing review |
| `BILLING_ENABLED` | `false` | Meter AI usage with prepaid tokens (hosted deployments) |
| `GOV_FILES_DIR`, `DOC_FILES_DIR` | `./data/...` | Private storage for returns, notices and signed documents |
| `INTEGRATION_ENCRYPTION_KEY` | — | Encrypts portal credentials and integration secrets |

A demo government portal for trying the browser agent: `go run ./cmd/mockgov`.

## Production

```bash
docker compose -f docker-compose.people.yml --env-file .env.people up -d --build
```

## Project structure

```
cmd/            api, worker, migrate, mcp (MCP server), mockgov (demo agency portal)
internal/
  lifecycle/    one-step hire/terminate, HR inbox, custom fields
  routines/     agent routines (autonomy, run log)
  home/         home dashboard
  govfiling/    filing catalog, datasets, pre-flight, notices, browser agent, autopilot
  payroll/      payroll engines (PH, LK, US), pay schedules, anomaly checks
  contractors/  freelancer payments, EWT, BIR 2307
  selfonboard/  onboarding portal + e-signature
  devices/      device inventory + MDM actions
  edition/      editions and modules
  ai/           assistant, agents, tools, drafts
db/             migrations (goose) and queries (sqlc)
frontend/       Vue 3 web app
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). New markets need a payroll calculator, form datasets and
catalog entries; the filing agent, routines and notices are country-agnostic.

## License

[MIT](LICENSE)
