# Employee Lifecycle, HR Inbox & Custom Fields

HalaOS already had most HR building blocks (employee records, onboarding
templates, clearance, final pay, Integration Hub provisioning, accounting
outbox), but an admin had to visit 5–7 screens to hire or offboard someone.
This module borrows the core product idea of [Warp](https://www.warp.co):
**one action per lifecycle event; the system fans out everything else, and an
inbox shows only what still needs a human.**

## Warp → HalaOS mapping

| Warp HR capability | HalaOS (this change) | Before |
|---|---|---|
| "Hire in 30 seconds": one flow creates the employee and sets up pay, accounts, devices | `POST /lifecycle/hire` + **Hire** wizard (`/hire`) | `POST /employees`, then salary, onboarding initiate, etc. separately |
| One-click termination: suspend Google/Okta, lock device, close final payroll | `POST /lifecycle/employees/:id/terminate` + **Terminate** modal on employee page | Status change + manual clearance + no deprovision event from status change |
| Scheduled offboarding on last day | Worker applies due separations daily (`ProcessDueSeparations`) | — |
| Employee profiles with custom fields | `employee_custom_fields` + **Custom Fields** admin page + card on employee page | — |
| Lifecycle status per person | `GET /lifecycle/employees/:id` (stage: pre_start / onboarding / active / offboarding / separated) | — |
| "Everything that needs you" task list | `GET /inbox` (+ `GET /inbox/brief` AI brief) + **Inbox** page | Approvals, onboarding, compliance alerts, expiring docs, milestones on separate pages |
| Self-service onboarding portal (new hire done in ~10 min) | Public `/onboard/:token` portal: personal info, gov IDs (validated), bank, emergency contact, documents | HR typed everything |
| Offer letters & documents with e-signature | Templates with merge fields + AI drafting, `/sign/:token`, signed PDF with audit trail filed to the 201 file | — |
| Device management (lock / wipe on termination) | Device inventory, MDM gateway (signed webhook), auto-lock + return request on termination; wipe needs approval | — |
| Closes out final payroll on termination | Final pay auto-computed when the separation becomes effective (PH Labor Code / PD 851 / RA 7641) | Hand-entered totals |
| Compliance autopilot (filings & registrations handled for you) | Government filing agent: prepare → rules + AI pre-flight → approve → submit via API / e-mail / browser agent / guided manual; hourly autopilot; US state-registration tasks | Forms as JSON only |

## Hire (`POST /api/v1/lifecycle/hire`, admin)

One transaction:

1. Create employee (employee number auto-generated as `EMP-00001…` when omitted)
2. Employment history `hire`
3. Salary (if `basic_salary` given)
4. Leave balances for every active leave type
5. Custom field values (validated; required fields enforced)
6. Onboarding checklist from `onboarding_templates` (unless `skip_onboarding`)
7. `employee.hired` hr_event → worker → Integration Hub provisioning (Google Workspace, Slack, GitHub…)

After commit: accounting `employee.upserted`. The response lists every step
with `done / skipped / scheduled` so the UI can show what happened.

## Terminate (`POST /api/v1/lifecycle/employees/:id/terminate`, admin)

```json
{ "separation_type": "resignation", "reason": "…", "notice_date": "2026-11-02",
  "last_working_day": "2026-11-30", "rehire_eligible": true, "revoke_access": true }
```

Immediately: clearance request + items (company templates or built-in
defaults), offboarding checklist, `employee_separations` row (`scheduled`).

On the last working day (immediately if it is today or earlier; otherwise the
worker's periodic job): employee → `separated`, history `separated`, open
onboarding tasks closed, separation → `completed`, `employee.terminated`
hr_event (drives deprovisioning when `revoke_access`), accounting
`employee.terminated`.

`DELETE /api/v1/lifecycle/employees/:id/terminate` cancels a scheduled
separation (cancels its clearance and closes its offboarding tasks).

## HR Inbox (`GET /api/v1/inbox`, admin)

One query unions pending leave / overtime / expense approvals, attendance
corrections, open onboarding & offboarding tasks, documents expiring within 30
days, unresolved compliance alerts, pending contract milestones, and
provisioning jobs that need approval or failed. Overdue / high-severity items
are flagged `priority: "high"` and sorted first. Response includes per-kind
counts; `?kind=` filters items.

## Custom fields

- `GET/POST /api/v1/custom-fields`, `PUT /api/v1/custom-fields/:id`
- `GET/PUT /api/v1/employees/:id/custom-fields` (`{"values": {"key": "value"}}`; empty value clears)
- Types: `text`, `number`, `date` (YYYY-MM-DD), `select` (options), `boolean`
- `is_sensitive` values are hidden from non-admins

## Code

- Migration: `db/migrations/00089_employee_lifecycle.sql`
- Queries: `db/query/lifecycle.sql`
- Package: `internal/lifecycle` (service = orchestration, handler = HTTP, validate = pure rules)
- Worker hook: `cmd/worker/main.go` → `runPeriodicJobs`
- Frontend: `views/HireWizardView.vue`, `views/InboxView.vue`, `views/CustomFieldsView.vue`,
  `components/lifecycle/EmployeeLifecycleCard.vue`

## Government filing agent (`internal/govfiling`)

Pipeline per return: **prepare** (file built from payroll) → **pre-flight** (deterministic
rules + optional AI review that only sees aggregates) → **human approval** → **submit** through
the agency's channel → reference number stored, `tax_filings` row marked filed.

| Channel | How | Used for |
|---|---|---|
| `api` | Signed (HMAC-SHA256) webhook to a licensed e-file partner / gateway; must return a reference number | IRS 941, US state registrations, IRAS via partner |
| `email` | MIME e-mail with the file attached (`GOV_EMAIL_OVERRIDE_TO` for staging) | BIR 1604-C alphalist via eSubmission |
| `browser` | Headless Chromium (chromedp) running a declarative **playbook** | eFPS, My.SSS, PhilHealth EPRS, Pag-IBIG eSRS… |
| `manual` | File + step-by-step instructions; HR records the reference | Anything else (eBIRForms, CPF EZPay, EPF/ETF) |

Safety rules built into the agent:

- Nothing is sent without an explicit approval; blocking pre-flight issues prevent approval.
- CAPTCHA, OTP and similar steps are `human_input`: the run pauses, a screenshot is shown in
  HalaOS (inbox + filing drawer) and resumes with the human's answer. The agent never solves
  CAPTCHAs. Answers are encrypted and wiped after use.
- Real-portal playbooks are **per company and must be verified**: a dry run executes every step
  except the one marked `final` and stores screenshots; only then can an admin mark the playbook
  verified and approve real runs. HalaOS ships no guessed selectors for real portals.
- Portal credentials are AES-256-GCM encrypted (`INTEGRATION_ENCRYPTION_KEY`), never returned by
  the API, and passwords / OTPs are masked in run logs.
- Cancelling stops a running session before its next step; the prepared file is checksummed and
  re-verified before upload; prepared files and screenshots live in a private directory
  (`GOV_FILES_DIR`), not the public `/uploads`.
- The **autopilot** (worker, hourly) keeps the PH filing calendar, prepares returns due within
  10 days (and up to 45 days overdue) so HR only approves, and opens US state-registration tasks.

Try it locally: `go run ./cmd/mockgov`, set `GOV_MOCK_PORTAL_URL=http://127.0.0.1:8091`, add a
portal account for agency `MOCK` / portal `mock` (employer01 / s3cret!), prepare "Demo return",
approve, then answer CAPTCHA `K7Q2P` and OTP `246810` in the UI.

## Self-onboarding & e-signature (`internal/selfonboard`)

- `POST /lifecycle/employees/:id/onboarding-invite` → single-use link (SHA-256 hashed token, 14 days).
- Public `GET/POST /public/onboarding/:token/...` and `/public/sign/:token` (rate limited).
- Templates use merge fields (`{{employee.full_name}}`, `{{employee.basic_salary}}`, …); unknown or
  empty fields block sending. `POST /documents/templates/ai-draft` drafts a template with AI.
- Signing requires consent + typed name containing the signer's last name; optional drawn signature.
  A PDF with an audit page (document SHA-256, sent/viewed/signed times, IP, user agent) is stored
  privately (`DOC_FILES_DIR`) and filed to the employee's documents.
- Hire can send the offer letter and the portal link in the same step.

## Final pay & devices

- `internal/finalpay/compute.go`: unpaid salary since the last completed payroll (or since the
  start of the month when HalaOS has no payroll history), PD 851 prorated 13th month, convertible
  leave, Art. 298 separation pay for layoffs (½-year rule), RA 7641 retirement pay, loan
  deductions. Stored as `calculated` with the basis and notes for HR approval.
- `internal/devices`: inventory, assignment, MDM actions (lock / unlock / wipe / return) via a
  signed webhook; on termination devices are locked automatically (configurable) and a return is
  requested; wipe always needs a second approval; without an MDM gateway actions become IT tasks.

## AI tools (agents)

`get_hr_inbox`, `get_employee_lifecycle`, `hire_employee`, `terminate_employee`,
`send_onboarding_invite`, `send_document_for_signature`, `list_gov_filings`,
`prepare_gov_filing`, `lock_device`. Write tools go through action drafts (user confirmation).
Approving a government submission is intentionally not an AI tool.

## Known limits

- Upload layouts for SSS R-3, PhilHealth RF-1, Pag-IBIG and the BIR alphalist follow published
  column orders but must pass each agency's own validator; the pre-flight reminds the reviewer.
- IRAS AIS direct API (CorpPass flow) and SG/LK data generators are not built yet — use a partner
  or the manual channel.
- Off-cycle payroll runs for a single leaver are not implemented; final pay is a reviewed record.
