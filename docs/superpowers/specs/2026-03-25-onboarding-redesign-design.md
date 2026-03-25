# Onboarding Redesign — Design Spec

## Goal

Redesign the new user onboarding experience for both employee (H5 mobile) and HR admin (Desktop) personas, making the product intuitive for newcomers through guided checklists, contextual navigation, and action-oriented empty states.

## Context

AIGoNHR has two frontends:
- **Desktop** (`/`) — NaiveUI, for HR administrators (full-featured management)
- **H5 Mobile** (`/m/`) — Vant, for employees (clock-in, leave, payslips, AI chat)

Existing onboarding features:
- Setup Wizard (4-step admin company config)
- Interactive tour (driver.js, 8 admin steps / 6 employee steps)
- Onboarding task management (HR workflow templates, in `internal/onboarding/`)
- Dashboard "Get Started" card (basic, only for 0-employee state)

Problems:
1. After the initial tour ends, users don't know what to do next
2. Empty pages show a bare "No Data" message with no guidance
3. 40+ sidebar items overwhelm new admins (though role filtering exists)

## Architecture

Three independent components, each usable standalone:

1. **H5 Employee Onboarding Checklist** — `frontend-mobile/`
2. **Desktop Admin Getting Started Checklist** — `frontend/`
3. **Reusable Empty State Components** — both frontends

Backend tracks checklist progress via a new `onboarding_progress` table. New handler lives in `internal/onboarding_checklist/` (separate from the existing `internal/onboarding/` HR workflow package). Frontend reads state on mount and auto-detects step completion.

---

## Component 1: H5 Employee Onboarding Checklist

### What it does

When a new employee logs into the H5 app for the first time, a gradient welcome card with a 5-step checklist appears at the top of HomeView. Each step links to the relevant feature. Steps auto-complete when the user performs the action.

### Steps

| # | Step | Auto-complete trigger | Links to (named route) |
|---|------|----------------------|----------|
| 1 | Complete your profile | `users.avatar_url` is non-null OR `employee_profiles.emergency_name` is non-null | `{ name: 'profile' }` |
| 2 | Clock in for the first time | First `attendance_logs` record exists for this employee | `{ name: 'attendance' }` |
| 3 | Learn how to request leave | User visits the leave page (frontend-triggered) | `{ name: 'leave' }` |
| 4 | Check your payslip | User visits the payslips page (frontend-triggered) | `{ name: 'payslips' }` |
| 5 | Meet the AI Assistant | User sends first AI chat message | `{ name: 'ai-chat' }` |

Note: H5 mobile router uses `createWebHistory("/m/")` as base. All navigation uses named routes (e.g., `{ name: 'profile' }`), not hardcoded `/m/...` paths.

### UI Design

- **Welcome card**: Gradient purple header with user's first name, progress bar (X/5), "Skip ×" button
- **Checklist items**: White card below header. Done = green check + strikethrough. Current = blue highlight + arrow. Pending = grey circle + number.
- **Completion**: When all 5 done, show a brief celebration (confetti or checkmark animation), then card auto-hides after 2 seconds
- **Dismissal**: "Skip" sets `dismissed=true` in backend. Can re-access from Profile page "View Tutorial" link.
- **Position**: Top of HomeView, above the attendance and other content cards

### Data Model

Migration file: `db/migrations/00085_onboarding_checklist.sql`

```sql
CREATE TABLE onboarding_progress (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    persona VARCHAR(20) NOT NULL DEFAULT 'employee',  -- 'employee' or 'admin'
    steps JSONB NOT NULL DEFAULT '{}',
    dismissed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, user_id, persona)
);
```

The `steps` JSONB stores completion state:
```json
{
  "profile": {"done": true, "done_at": "2026-03-25T10:00:00Z"},
  "first_clock": {"done": true, "done_at": "2026-03-25T10:05:00Z"},
  "view_leave": {"done": false},
  "view_payslip": {"done": false},
  "ai_chat": {"done": false}
}
```

### API Endpoints

New handler package: `internal/onboarding_checklist/`

Routes registered under `/api/v1/onboarding-checklist/` (distinct from existing `/api/v1/onboarding/` workflow routes):

```
GET  /api/v1/onboarding-checklist/my-progress    → { steps, dismissed, completed_at }
POST /api/v1/onboarding-checklist/complete-step   → { step: "first_clock" }
POST /api/v1/onboarding-checklist/dismiss         → sets dismissed=true
```

### Auto-completion Strategy

Two mechanisms:
1. **Frontend-triggered**: When user navigates to leave/payslip page, frontend calls `POST /complete-step` with the step key. Simple page-visit steps.
2. **Backend-triggered**: When attendance is logged or profile is updated, the relevant handler checks and updates the onboarding step. No extra API call needed.

### Edge Cases

- `GET /my-progress` for a user with no record: auto-create a new `onboarding_progress` row with all steps `{"done": false}` and return it.
- `POST /complete-step` with already-completed step: idempotent, return success without error.
- `POST /complete-step` with invalid step key: return 400 Bad Request with `"invalid step key"`.
- Rate limiting: frontend-triggered completions are debounced client-side (only call once per page visit per session, not on every mount).

---

## Component 2: Desktop Admin Getting Started Checklist

### What it does

After the Setup Wizard completes, the DashboardView shows a Getting Started card (replacing the current basic "Get Started" card). A 7-step checklist in a 2-column grid tracks setup progress.

### Steps

| # | Step | Auto-complete trigger | Links to |
|---|------|----------------------|----------|
| 1 | Company Information | `companies.legal_name` is non-null AND `companies.tin` is non-null | `/settings` |
| 2 | Departments & Positions | At least 1 `departments` row AND 1 `positions` row exist for this company | `/departments` |
| 3 | Import Employees | `employees` count > 1 for this company (excluding the admin's own record) | `/employees` |
| 4 | Configure Leave Policies | At least 1 `leave_types` row with `is_statutory = false` exists for this company | `/settings` (leave tab) |
| 5 | Set Up Work Schedules | At least 1 `schedule_templates` row exists for this company | `/schedules` |
| 6 | Configure Payroll | At least 1 `salary_structures` row exists for this company | `/salary` |
| 7 | Run First Payroll | At least 1 `payroll_runs` row with `status = 'completed'` exists for this company | `/payroll` |

### UI Design

- **Card**: White rounded card at top of DashboardView. Header with title, "2/7 completed" counter, and "Dismiss ×"
- **Progress bar**: Gradient blue-purple, width = completedSteps / totalSteps
- **Grid**: 2 columns on desktop, 1 column on narrow screens. Same visual pattern as H5 (green check / blue current / grey pending) but in card grid layout
- **Current step**: Blue background, includes "Go to [Feature] →" link
- **Completion**: All 7 done → celebration animation → card auto-hides
- **Dismissal**: Backend-stored (not localStorage), can re-enable from Settings

### Data Model

Reuses the same `onboarding_progress` table with `persona='admin'`.

Admin steps JSONB:
```json
{
  "company_info": {"done": true, "done_at": "..."},
  "departments": {"done": true, "done_at": "..."},
  "import_employees": {"done": false},
  "leave_policies": {"done": false},
  "schedules": {"done": false},
  "payroll_config": {"done": false},
  "first_payroll": {"done": false}
}
```

### Auto-completion Strategy

All admin steps use **backend auto-detection**. The `GET /my-progress` endpoint checks actual DB state each time (not relying on localStorage flags):
- Step 1: Query `companies` for `legal_name IS NOT NULL AND tin IS NOT NULL`
- Step 2: Count `departments` > 0 AND count `positions` > 0
- Step 3: Count `employees` > 1
- Step 4: Count `leave_types WHERE is_statutory = false` > 0
- Step 5: Count `schedule_templates` > 0
- Step 6: Count `salary_structures` > 0
- Step 7: Count `payroll_runs WHERE status = 'completed'` > 0

When a step is newly detected as done, the backend updates the JSONB `done_at` timestamp. This approach is resilient to browser changes and doesn't depend on localStorage.

Additionally, the Setup Wizard handler should call `POST /complete-step` for steps 1 and 2 upon wizard completion, for immediate feedback.

---

## Component 3: Action-Oriented Empty States

### What it does

Replace bare `NEmpty` / `van-empty` instances with a reusable `EmptyState` component that shows an icon, title, description, and action buttons.

### Reusable Component

**Desktop** (`frontend/src/components/EmptyState.vue`):
```
Props:
  icon: string (emoji)
  title: string
  description: string
  primaryAction?: { label: string, handler: () => void }
  secondaryAction?: { label: string, handler: () => void }
```

Wraps NaiveUI `NEmpty` with custom slot content. Centered layout with gradient icon circle, text, and NButton actions.

**H5** (`frontend-mobile/src/components/EmptyState.vue`):
Same props, wraps Vant `van-empty` with custom slot content. Mobile-optimized sizing.

### Pages to Update

Priority pages (highest traffic, most likely to be empty for new users):

| Page | Frontend | Icon | Title | Description | CTA |
|------|----------|------|-------|-------------|-----|
| Leaves | Both | 🏖️ | No leave requests yet | You have X vacation days available. File a request and your manager will be notified. | Request Leave |
| Attendance | Both | ⏰ | No attendance records | Clock in to start tracking your work hours. | Clock In Now |
| Payslips | Both | 💰 | No payslips available | Payslips are generated after each payroll run. Check back after the next pay period. | — |
| Notifications | Both | 🔔 | No notifications | You'll be notified about approvals, announcements, and important updates. | — |
| Employees | Desktop only | 👥 | No employees yet | Add your team to start managing attendance, leave, and payroll. | Add Employee / Import CSV |
| Approvals | Desktop only | ✅ | All caught up! | No pending approvals. When employees request leave or overtime, they'll appear here. | — |
| Loans | Desktop only | 🏦 | No active loans | Need a salary advance or company loan? Apply here. | Apply for Loan |
| Expenses | Desktop only | 🧾 | No expense claims | Submit receipts for reimbursement. | File Expense |
| Training | Desktop only | 📚 | No training courses | Check back soon for assigned courses and certifications. | — |
| Overtime | Desktop only | ⏱️ | No overtime records | When you work extra hours, file an overtime request here. | File OT Request |

### Tone Guidelines

- **Positive framing**: "All caught up!" not "No pending items"
- **Explain the feature**: Brief sentence about what the page does
- **Actionable when possible**: Include CTA button if user can take action
- **Role-aware**: Admin sees "Add Employee →", employee does not
- **No jargon**: Plain language, avoid HR terminology in descriptions

---

## Non-Goals

- No changes to the existing Setup Wizard
- No changes to the existing driver.js tour (it stays as-is for now)
- No progressive feature unlock / sidebar changes
- No video tutorials or media content
- No changes to the existing onboarding task management system (`internal/onboarding/` HR workflow templates)

## Technical Notes

- **Backend**: New `onboarding_progress` table (migration `00085`) + sqlc queries in `db/query/onboarding_checklist.sql` + handler in `internal/onboarding_checklist/`. Reuses existing `auth.GetUserID/GetCompanyID` pattern.
- **Route namespace**: New routes under `/api/v1/onboarding-checklist/` (distinct from existing `/api/v1/onboarding/` workflow routes).
- **Frontend auto-detection**: Some steps complete via page visit (frontend calls API), others via backend hooks in existing handlers.
- **i18n**: All text through vue-i18n. Add keys to both en.ts and zh.ts for Desktop, and en.ts/zh.ts for H5.
- **Existing code changes**: DashboardView.vue replaces current "Get Started" card. H5 HomeView.vue adds checklist above existing content. ~10 view files across both frontends get EmptyState replacement.

## Testing

- Unit tests for onboarding checklist handler (get progress, complete step, dismiss)
- Unit tests for auto-detection queries (each admin step trigger)
- Test edge cases: idempotent completion, invalid step key, first-access auto-create
- Frontend: verify checklist renders, steps link correctly, dismiss works
- Verify empty states render on pages with no data
