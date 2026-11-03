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
| Routines written in plain language | Plain-language extra steps per routine. Each becomes a checklist item in every run | 🟡 | The agent does not yet execute arbitrary free-text steps |
| Run log: trigger, what it did, what it left for a person | Run log with trigger, actions (done / for you / recommended / waiting / failed), rationale and links | ✅ | Routines → run log; Home → Agent activity |
| Same permission model as people; hard limits on payroll/payments | Admin-only routines. Submissions, payroll approval and final pay always need a person | ✅ | |
| Rollback per agent action | — | ⬜ | Actions are drafts or reversible records; no one-click undo yet |
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
| Money movement (ACH/wire funding, direct deposit) | Bank files only; no money movement | ⬜ |
| Payroll history import on migration | CSV import of employees; no payroll-history import | 🟡 |
| GL journal entries to QuickBooks / Xero / NetSuite | Balanced journal per payroll run (wages, employer contributions, statutory payables, withholding, other deductions, net pay) mapped to your chart of accounts; QuickBooks / Xero CSV import files; webhook sync for AIStarlight. No direct API push yet | 🟡 |

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
| Documents with e-signature, timestamped and audit-ready | Templates, e-signature, signed PDF with audit trail | ✅ |
| Self-service onboarding checklist (offer, NDA, tax forms, bank, ID upload, benefits) | Self-onboarding portal (offer letter e-sign, IDs, bank, documents) | ✅ |
| Offboarding: one action → final pay, Google/Okta suspended, device locked | One action: clearance, final pay, device lock/recall, access revocation (Google/Slack/GitHub) | ✅ |
| PTO policies, accruals, approvals, payroll sync | Leave types, accrual, approvals, encashment, payroll sync | ✅ |
| Policy assignment by location / tenure / department | Leave types per company. No rule-based assignment | 🟡 |
| Time tracking add-on (clock in/out, geofence, shifts, timesheets → payroll) | Suite edition: attendance, geofences, schedules, DTR, overtime → payroll | ✅ |
| Roles & permissions, MFA, SSO | Roles (super admin / admin / manager / employee), SSO via JWT. No MFA | 🟡 |
| Mobile app for employees | `frontend-mobile` (H5); not yet adapted to editions | 🟡 |

## Contractors

| Warp | HalaPeople | Status |
|---|---|:-:|
| US 1099 contractors (W-9, payments, 1099-NEC) | PH freelancers: EWT, bank file, BIR 2307, 0619-E / 1601-EQ + QAP | ✅ |
| International contractors in 150+ countries, FX payouts | — | ⬜ |

## Benefits

| Warp | HalaPeople | Status |
|---|---|:-:|
| Licensed brokerage, carriers, EDI, deduction sync | Benefit plans, enrollment, claims, payroll deductions. No brokerage or HMO integrations | 🟡 |
| 401(k) partners | — (SSS/Pag-IBIG MP2 are statutory/voluntary in-payroll) | — |

## IT

| Warp | HalaPeople | Status |
|---|---|:-:|
| Google Workspace / Slack account on hire, suspended at exit | App provisioning (Google, Slack, GitHub) on hire/exit | ✅ |
| Okta provisioning | — | ⬜ |
| In-house MDM (enroll, lock, wipe, patch, compliance) | Device inventory + lock/wipe via an MDM gateway; no own agent | 🟡 |
| Audit log of provisioning and device actions | Audit trail + device action log | ✅ |

## Integrations and platform

| Warp | HalaPeople | Status |
|---|---|:-:|
| Slack payroll/tax alerts | Slack connector for provisioning; alerts go to in-app notifications and e-mail | 🟡 |
| Public REST API | REST API (the web app's API) + MCP server | ✅ |
| Merge / Unified connectors | — | ⬜ |

## Next up

1. Push journals to QuickBooks / Xero through their APIs (CSV import ships today).
2. Rule-based policy assignment (leave and time policies by location, tenure, department).
3. Slack alerts for to-dos, filings due and notices.
4. MFA.
5. Money movement through a licensed payout partner (PH: InstaPay/PESONet via a partner).
6. Singapore (CPF, IRAS AIS) and Malaysia (EPF, SOCSO, EIS, PCB) payroll and filings.
