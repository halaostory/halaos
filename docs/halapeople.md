# HalaPeople — payroll, compliance & HR for Southeast Asia

HalaPeople is an open-source, Warp-style "HR on autopilot" platform for small and growing
companies in Southeast Asia. This repository is HR-only (the former HalaOS extras such as the
virtual office, referral programme and NPS were removed). It builds two **editions** of the
same product:

- `people` (default): the Warp-style core.
- `suite`: the core plus attendance, talent, engagement and people analytics. `halaos` is
  accepted as a legacy alias.

> `PRODUCT_NAME` / `PRODUCT_TAGLINE` override the brand (e.g. white label); the UI and API pick
> them up at runtime. Feature-by-feature comparison with Warp: [warp-parity.md](warp-parity.md).

## Positioning

| | Warp (US) | HalaPeople (SEA) |
|---|---|---|
| Core promise | Payroll + compliance on autopilot for startups | Same, for Southeast Asian employers |
| Statutory filings | State tax registrations & filings | BIR, SSS, PhilHealth, Pag-IBIG (PH live); IRAS/CPF, LHDN/KWSP, DJP/BPJS next |
| Government interaction | APIs / e-file | Agent: official API → e-mail submission → supervised browser on the agency portal (CAPTCHA/OTP handed to a human) → guided manual |
| HR | Onboarding, PTO, documents, e-sign | Same + 201 file, 13th month, final pay (Labor Code), clearance |
| Registrations | State tax registrations for new hires/states | New-hire SSS R-1A, PhilHealth ER2, Pag-IBIG and BIR 1902 prepared automatically per hire month |
| Contractors | Contractor payments, 1099s | Freelancer invoices with expanded withholding (EWT), bank file, BIR 2307, 0619-E / 1601-EQ + QAP |
| Pay schedules | Multiple pay schedules | Several schedules per company (monthly, semi-monthly, bi-weekly, weekly) with prorated pay, contributions and tax |
| IT | Device management, app provisioning | Same (MDM gateway, Google/Slack/GitHub provisioning) |
| Agent | Warp Agent, Routines with autonomy levels, run logs | Assistant with confirm-before-act tools; routines (New hire, Offboarding, Pre-payroll check, Filing autopilot) with recommend / ask first / autonomous and run logs |
| Notices | AI triage + response, ~80% auto-resolved | Notices matched to filings: proof-of-filing replies drafted, missing returns prepared, audits escalated |

## What ships in each edition

Modules are defined in `internal/edition` (backend) and `frontend/src/edition.ts` (frontend).

| Module | people | suite |
|---|:-:|:-:|
| core — home, company, users, approvals, holidays, announcements, self-service, HR requests, policies, billing (off unless `BILLING_ENABLED`) | ✓ | ✓ |
| people — employees, directory, 201 file, custom fields | ✓ | ✓ |
| lifecycle — one-step hire/terminate, checklists, clearance, milestones, HR inbox | ✓ | ✓ |
| payroll — payroll, 13th month, final pay & off-cycle release, loans, reports, accounting sync | ✓ | ✓ |
| compliance — government forms, tax filings, filing agent & autopilot, notices | ✓ | ✓ |
| time_off — leave, encashment, overtime requests | ✓ | ✓ |
| benefits, expenses | ✓ | ✓ |
| contractors — freelancers, EWT, BIR 2307, 0619-E/1601-EQ | ✓ | ✓ |
| esign — self-onboarding portal, templates, e-signature | ✓ | ✓ |
| it — devices/MDM, app provisioning | ✓ | ✓ |
| ai — assistant, agents, routines, messaging bot (tools filtered to the edition) | ✓ | ✓ |
| attendance — clock in/out, breaks, geofences, schedules, DTR | — | ✓ |
| talent — performance, training, disciplinary, grievance | — | ✓ |
| engagement — pulse, recognition | — | ✓ |
| intelligence — analytics, org intelligence, knowledge base, workflow automation | — | ✓ |

How the split is enforced:

- **API**: routes of modules outside the edition are registered on a detached router → 404.
- **Worker**: periodic jobs of excluded modules (attendance, scorers, pulse) do not run.
- **AI**: tool definitions of excluded modules are not offered and are refused if called by name.
- **Web app**: `GET /api/v1/edition` drives brand, menu, reachable routes, home page and the
  list of markets on sign-up; UI copy is re-branded at load. Admins and managers land on Home,
  employees on self-service.
- **Sign-up**: only `live` countries can create a company; others get a "coming soon" message.

## Deploy

```bash
cp .env.example .env.people   # set passwords, JWT_SECRET, INTEGRATION_ENCRYPTION_KEY, SMTP, PEOPLE_BASE_URL
docker compose -f docker-compose.people.yml --env-file .env.people up -d --build
```

`docker-compose.people.yml` is its own compose project (`halapeople`) with Postgres, Redis,
volumes and network, `APP_EDITION=people`, web on `PEOPLE_HTTP_PORT` (default 8081) behind
your TLS proxy. Locally: `go run ./cmd/api` (people is the default; `APP_EDITION=suite` for
the suite).

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

## Still missing vs Warp

See [warp-parity.md](warp-parity.md) for the full matrix. The largest gaps:

- **Money movement**: payroll and contractor payments produce bank files; HalaPeople does not
  move money itself yet. Needs a licensed payout partner (e.g. PayMongo / Xendit disbursements).
- **Integrations**: Okta / Microsoft 365 provisioning, Xero / QuickBooks / NetSuite.
- **MDM agent**: device actions go through an MDM gateway; no own enrollment agent.
- **Insurance marketplace**: benefits are administered, not brokered (no HMO integrations).
- **Mobile**: `frontend-mobile` is not yet adapted to the HalaPeople edition.

## Open decisions

- Domain and hosted pricing (`halapeople.com` in the compose file is a placeholder).
- Which SEA market follows the Philippines.
