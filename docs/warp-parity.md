# HalaPeople vs Warp: feature parity

HalaPeople aims to be an open-source [Warp](https://www.warp.co) for Southeast Asia. This page
maps Warp's product to what HalaPeople ships today.

The Warp side is based on public material as of late 2026: warp.co product pages, help-center
articles, the Warp Agent / "Warp 2.0" launch, G2 reviews and press. Where Warp's US-specific
concept has a Southeast Asian equivalent (state tax registration → SSS/PhilHealth/Pag-IBIG/BIR
registration), the row maps them.

Legend: ✅ shipped · 🟡 partial · ⬜ not yet · — not applicable

## Agent and automation

| Warp | HalaPeople | Status | Where |
|---|---|:-:|---|
| Warp Agent chat in the sidebar; answers policy/employee questions and acts | AI assistant with HR tools. Write tools create confirm-before-act drafts | ✅ | `internal/ai`, Agent page |
| Routines: event/schedule triggered jobs the agent owns | New hire, Offboarding, Pre-payroll check, Filing autopilot | ✅ | `internal/routines`, Routines page |
| Per-routine autonomy (recommend / act after approval / autonomous) | Recommend / Ask first / Autonomous. Filings and payroll are capped at "ask first" | ✅ | Routines page |
| Routines written in plain language | Plain-language steps per routine. At Autonomous the agent reads each one (AI, or rules without a model) and does it — adds a checklist task or notifies the manager, employee or HR — with undo; at lower levels they are left for a person | 🟡 | Steps that need other tools (Slack messages, purchasing) become tracked tasks rather than being executed |
| Run log: trigger, what it did, what it left for a person | Run log with trigger, actions (done / for you / recommended / waiting / failed), rationale and links | ✅ | Routines → run log; Home → Agent activity |
| Same permission model as people; hard limits on payroll/payments | Admin-only routines. Submissions, payroll approval and final pay always need a person | ✅ | |
| Rollback per agent action | One-click Undo in the run log for reversible actions (prepared government returns are cancelled, manager notifications retracted); who undid it is recorded | ✅ | Routines → run log |
| Pre-payroll variance / anomaly check | Rule-based anomaly detection on every calculated run (gross/net variance, zero contributions/tax, negative net, OT, duplicates), logged as a routine run | ✅ | `internal/payroll/anomaly.go` |
| MCP access to the agent | MCP server | ✅ | `cmd/mcp` |

## Home and navigation

| Warp | HalaPeople | Status |
|---|---|:-:|
| Personalised greeting ("Good afternoon, Albert") | Same, with today's to-do count | ✅ |
| "State Tax Registrations — 28 / 34" with Active / Needs info / Awaiting state / Filing started pills | "Government registrations — n / 4 active" per agency (BIR, SSS, PhilHealth, Pag-IBIG; US states) with Active / Needs info / Needs approval / Filing started / Awaiting agency, plus employees-with-ID coverage | ✅ |
| "Onboard a new employee — send an invite" widget | Same; opens the hire flow with the invite pre-filled | ✅ |
| Agent activity ("Running 'New Hire' routine") | Agent activity list linking to each run log | ✅ |
| To-dos / approvals | HR inbox (approvals, tasks, filings, notices, documents, devices) on Home and its own page | ✅ |
| Next payroll | Days to payday, cycle status, last payroll total | ✅ |
| Sidebar: Dashboard, Payroll, Warp Agent, … | Home, Inbox, Agent, Routines at the top; People, Time off, Payroll, Compliance, Settings | ✅ |

## Payroll

| Warp | HalaPeople | Status |
|---|---|:-:|
| Full-service payroll, gross-to-net, review and approve | PH (and LK/US engines), review and approve, payslips | ✅ |
| Multiple pay schedules, proration | Monthly, semi-monthly, bi-weekly and weekly schedules per company, prorated | ✅ |
| Auto-Pay (runs on schedule) | Auto-run, plus optional auto-approve under a limit | ✅ |
| Unlimited off-cycle runs; termination payroll created automatically | Off-cycle runs; final pay computed on termination and released as an off-cycle run after approval | ✅ |
| Money movement (ACH/wire funding, direct deposit) | Pay employees: approved net pay sent to each employee's bank or e-wallet through a licensed payout partner (Xendit Batch Disbursements), with a line-by-line review, explicit confirmation of the total, once per run, and per-transfer status from the partner's callback; bank upload files as the alternative. Needs the company's own Xendit account; HalaPeople never holds funds | ✅ |
| Payroll history import on migration | Payroll → Import history: CSV of earlier payslips (validated, all-or-nothing, no double counting) becomes locked imported pay runs that year-end forms (BIR 2316, 1604-C alphalist) and YTD reports include; plus employee CSV import | ✅ |
| GL journal entries to QuickBooks / Xero / NetSuite | Balanced journal per payroll run mapped to your chart of accounts; one-click send to Xero (draft manual journal) or QuickBooks Online (journal entry) over OAuth, once per run, tokens refreshed automatically; CSV import files; webhook sync. Needs your own Xero / Intuit developer app. No NetSuite | ✅ |

## Tax and compliance

| Warp | HalaPeople | Status |
|---|---|:-:|
| Automatic registration when you hire in a new state | New-hire SSS R-1A, PhilHealth ER2, Pag-IBIG and BIR 1902 prepared on hire (New hire routine). US: state registration task per new work state | ✅ |
| Quarterly / annual filings and payments done automatically | Filing calendar + autopilot prepares and pre-flights every return. Submission after approval via API / e-mail / supervised browser agent | ✅ |
| Notice handling: AI triages, researches and drafts/files a response; ~80% auto-resolved | Notices: AI or rules read the notice and match it to your filings. Proof-of-filing reply is drafted, missing return prepared in one click, audits escalated. The share the agent can resolve is shown | ✅ |
| "Tax & Compliance" dashboard with a log of what was automated | Government filings page + Home registrations panel + routine run logs | ✅ |
| Compliance team that takes responsibility for errors | — (open-source software; no service team) | — |

## HRIS and lifecycle

| Warp | HalaPeople | Status |
|---|---|:-:|
| Employee profiles, custom fields, reporting lines, job history, compensation | Same + 201 file | ✅ |
| Directory and org chart | Directory with org chart | ✅ |
| Departments, workplaces | Departments, positions and workplaces (offices / remote) with headcount; workplace set at hire or on the profile | ✅ |
| Documents with e-signature, timestamped and audit-ready | Templates, e-signature, signed PDF with audit trail | ✅ |
| Self-service onboarding checklist (offer, NDA, tax forms, bank, ID upload, benefits) | Self-onboarding portal (offer letter e-sign, IDs, bank, documents) | ✅ |
| Offboarding: one action → final pay, Google/Okta suspended, device locked | One action: clearance, final pay, device lock/recall, access revocation (Google/Slack/GitHub) | ✅ |
| PTO policies, accruals, approvals, payroll sync | Leave types, accrual, approvals, encashment, payroll sync | ✅ |
| Policy assignment by location / tenure / department | Time off → Policies: entitlement rules by department, employment type, gender and tenure band with priorities; applied at hire, re-applied daily (tenure upgrades), preview before apply; rules can also target workplaces | ✅ |
| Time tracking add-on (clock in/out, geofence, shifts, timesheets → payroll) | Suite edition: attendance, geofences, schedules, DTR, overtime → payroll | ✅ |
| Roles & permissions, MFA, SSO | Roles (super admin / admin / manager / employee); two-factor sign-in (TOTP + recovery codes); single sign-on with any OpenID Connect provider (Google Workspace, Okta, Entra ID) per company e-mail domain. No SAML | ✅ |
| Mobile app for employees | `frontend-mobile` (mobile web): edition-aware tabs (Home, Clock in suite, Leave, Pay, Expenses, Me), expense claims, leave balances, payslips, two-factor sign-in; installable as an app (PWA: home-screen icon, standalone window, opens offline). Not in the app stores | 🟡 |

## Contractors

| Warp | HalaPeople | Status |
|---|---|:-:|
| US 1099 contractors (W-9, payments, 1099-NEC) | PH freelancers: EWT, bank file, BIR 2307, 0619-E / 1601-EQ + QAP | ✅ |
| International contractors in 150+ countries, FX payouts | Pay via Wise: contractors linked to a recipient in the company's Wise Business account are paid in their currency from the Wise balance — live quote (rate, fee, amount received), explicit confirmation, idempotent transfer, status sync, payment marked paid with the Wise reference. Needs the company's own Wise account; W-8BEN-style forms not built | 🟡 |

## Benefits

| Warp | HalaPeople | Status |
|---|---|:-:|
| Licensed brokerage, carriers, EDI, deduction sync | Benefit plans, enrollment, claims, payroll deductions. No brokerage or HMO integrations | 🟡 |
| 401(k) partners | — (SSS/Pag-IBIG MP2 are statutory/voluntary in-payroll) | — |

## IT

| Warp | HalaPeople | Status |
|---|---|:-:|
| Google Workspace / Slack account on hire, suspended at exit | App provisioning (Google, Slack, GitHub) on hire/exit | ✅ |
| Okta provisioning | Okta connector (Integrations → Okta): account created and activated at hire, deactivated (or deleted) at exit, re-activated on rehire, status sync | ✅ |
| In-house MDM (enroll, lock, wipe, patch, compliance) | Device inventory + lock/wipe via an MDM gateway; no own agent | 🟡 |
| Audit log of provisioning and device actions | Audit trail + device action log | ✅ |

## Integrations and platform

| Warp | HalaPeople | Status |
|---|---|:-:|
| Slack payroll/tax alerts | Slack alerts via incoming webhook: agent hand-offs, new notices, daily filings-due digest, payroll ready (Routines → Slack alerts) | ✅ |
| Public REST API with scoped keys | REST API (the web app's API) with scoped API keys (full, read-only, or per-resource read/write) + MCP server | ✅ |
| Merge / Unified connectors | — | ⬜ |

## Next up

1. Singapore (CPF, IRAS AIS) and Malaysia (EPF, SOCSO, EIS, PCB) payroll and filings.
