# HalaPeople — payroll, compliance & HR for Southeast Asia

HalaPeople is the HR product carved out of HalaOS: a Warp-style "HR on autopilot" service for
small and growing companies in Southeast Asia. It is built from this repository as a separate
**edition** — own domain, database, brand and deployment — while sharing the code (payroll
engines, filing agent, auth) with HalaOS, so fixes land in both products at once.

> Working name. Set `PRODUCT_NAME` to rebrand; the UI and API pick it up at runtime.

## Positioning

| | Warp (US) | HalaPeople (SEA) |
|---|---|---|
| Core promise | Payroll + compliance on autopilot for startups | Same, for Southeast Asian employers |
| Statutory filings | State tax registrations & filings | BIR, SSS, PhilHealth, Pag-IBIG (PH live); IRAS/CPF, LHDN/KWSP, DJP/BPJS next |
| Government interaction | APIs / e-file | Agent: official API → e-mail submission → supervised browser on the agency portal (CAPTCHA/OTP handed to a human) → guided manual |
| HR | Onboarding, PTO, documents, e-sign | Same + 201 file, 13th month, final pay (Labor Code), clearance |
| IT | Device management, app provisioning | Same (MDM gateway, Google/Slack/GitHub provisioning) |
| AI | — | Assistant with confirm-before-act tools, inbox brief, pre-flight review of filings |

## What ships in each product

Modules are defined in `internal/edition` (backend) and `frontend/src/edition.ts` (frontend).

| Module | HalaPeople | HalaOS |
|---|:-:|:-:|
| core — company, users, approvals, holidays, announcements, self-service, HR requests, policies, billing | ✓ | ✓ |
| people — employees, directory, 201 file, custom fields | ✓ | ✓ |
| lifecycle — one-step hire/terminate, checklists, clearance, milestones, HR inbox | ✓ | ✓ |
| payroll — payroll, 13th month, final pay & off-cycle release, loans, reports, accounting sync | ✓ | ✓ |
| compliance — government forms, tax filings, filing agent & autopilot | ✓ | ✓ |
| time_off — leave, encashment, overtime requests | ✓ | ✓ |
| benefits, expenses | ✓ | ✓ |
| esign — self-onboarding portal, templates, e-signature | ✓ | ✓ |
| it — devices/MDM, app provisioning | ✓ | ✓ |
| ai — assistant, agents, messaging bot (tools filtered to the edition) | ✓ | ✓ |
| attendance — clock in/out, breaks, geofences, schedules, DTR | — | ✓ |
| talent — performance, training, disciplinary, grievance | — | ✓ |
| engagement — pulse, recognition, referrals | — | ✓ |
| intelligence — analytics, org intelligence, knowledge base, workflow automation | — | ✓ |
| workspace — virtual office | — | ✓ |

How the split is enforced:

- **API**: routes of modules outside the edition are registered on a detached router → 404.
- **Worker**: periodic jobs of excluded modules (attendance, scorers, pulse, virtual office) do not run.
- **AI**: tool definitions of excluded modules are not offered and are refused if called by name.
- **Web app**: `GET /api/v1/edition` drives brand, menu, reachable routes, home page and the
  list of markets on sign-up; UI copy is re-branded at load. HalaPeople admins land on the HR
  inbox, employees on self-service.
- **Sign-up**: only `live` countries can create a company; others get a "coming soon" message.

## Deploy

```bash
cp .env.example .env.people   # set passwords, JWT_SECRET, INTEGRATION_ENCRYPTION_KEY, SMTP, PEOPLE_BASE_URL
docker compose -f docker-compose.people.yml --env-file .env.people up -d --build
```

`docker-compose.people.yml` is its own compose project (`halapeople`): separate Postgres,
Redis, volumes and network from HalaOS, `APP_EDITION=people`, web on `PEOPLE_HTTP_PORT`
(default 8081) behind your TLS proxy. Locally: `APP_EDITION=people go run ./cmd/api`.

## Market roadmap

A country goes `live` only when payroll, statutory contributions and filings work end to end.

| Country | Status | Needed |
|---|---|---|
| Philippines | **Live** | Validate SSS R-3 / RF-1 / Pag-IBIG / alphalist files against agency validators; record & verify portal playbooks per customer |
| Singapore | Coming soon | Payroll engine: CPF (OW/AW ceilings, age bands), SDL, SHG funds; IR8A data; IRAS AIS API (CorpPass auth) adapter; CPF EZPay file |
| Malaysia | Coming soon | EPF/KWSP, SOCSO/PERKESO, EIS, PCB/MTD, HRDF; LHDN e-Filing (EA / CP8D), e-Caruman submissions |
| Indonesia | Coming soon | BPJS Kesehatan & Ketenagakerjaan, PPh 21 (TER method), THR; DJP e-Bupot / Coretax |
| Vietnam / Thailand | Coming soon | Social insurance + PIT engines; scoping after SG/MY/ID |

The filing agent, autopilot calendar and pre-flight framework are country-agnostic: a new
market needs a payroll calculator, form datasets/file renderers and catalog entries
(`internal/govfiling/catalog.go`).

## Working with HalaOS (later phase)

Companies using both products should not double-enter people. Planned, reusing the existing
AIStarlight integration pattern (signed JWT SSO + event outbox → signed webhook):

1. SSO between HalaPeople and HalaOS (`INTEGRATION_JWT_SECRET`).
2. HalaPeople is the system of record for employees, org structure and pay; it emits
   `employee.upserted`, `employee.terminated`, `payroll.run.completed`.
3. HalaOS sends attendance/overtime summaries back as payroll inputs.

## Open decisions

- Final product name, domain and pricing (current `/pricing` page is HalaOS's).
- Whether to move HalaPeople into its own repository later (the edition split keeps that
  possible: copy the shipped modules + `internal/edition`).
- Which SEA market follows the Philippines.
