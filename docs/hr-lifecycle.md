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
| "Everything that needs you" task list | `GET /inbox` + **Inbox** page | Approvals, onboarding, compliance alerts, expiring docs, milestones on separate pages |

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

## Possible next steps (further Warp parity)

- Employee self-onboarding portal (new hire fills personal/tax/bank info and e-signs documents before day 1)
- Offer letters with e-signature as step 0 of hire
- Device management events (lock/wipe) as provisioning actions
- Auto-compute final pay when the separation becomes effective
- Off-cycle / final payroll run triggered from terminate
