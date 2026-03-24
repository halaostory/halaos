# HalaOS Integration Test Suite Design

## Goal

Go integration test suite that calls the live REST API (3.1.66.212:8080) to create 50+ employees with full associated data, then verifies every major module works correctly.

## Approach

API-based integration tests under `tests/integration/`. Each test file covers one module. Tests run sequentially in dependency order — employee creation first, then modules that depend on employees. Uses the existing admin account (admin@demo.com) on company_id=1.

Go sorts test files alphabetically — the `01_`, `02_`, ... prefix guarantees cross-file ordering. Within each file, tests run top-to-bottom. No `t.Parallel()` is used.

## Environment

- **Target**: `http://3.1.66.212:8080/api/v1`
- **Auth**: Login via `POST /api/v1/auth/login` (public endpoint, no auth header). Extract JWT from `response.data.token`.
- **Run**: `go test ./tests/integration/ -v -count=1 -timeout 10m`
- **Config**: `HALAOS_BASE_URL` env var (defaults to `http://3.1.66.212:8080`)

## File Structure

```
tests/integration/
  main_test.go          — TestMain: login, get JWT, shared state
  helpers_test.go       — HTTP client, assertions, data generators
  01_employees_test.go  — Departments, positions, 50+ employees, user accounts, salaries
  02_attendance_test.go — Clock in/out, records, summary
  03_leave_test.go      — Balances, requests, approval flow
  04_payroll_test.go    — Cycles, runs, payslips
  05_compliance_test.go — SSS, PhilHealth, PagIBIG, BIR tax tables
  06_dashboard_test.go  — All dashboard endpoints
  07_directory_test.go  — Directory listing, org chart
  08_ai_test.go         — AI chat endpoint
```

## Shared State

`main_test.go` holds test-wide state passed via package-level vars:

```go
var (
    baseURL       string
    authToken     string              // JWT from admin login
    empUserTokens map[int64]string    // employee_id → JWT token (for employee-context calls)
    createdEmps   []int64             // employee IDs created in 01_
    deptIDs       map[string]int64    // department code → ID
    posIDs        map[string]int64    // position code → ID
)
```

## Test Details

### main_test.go — Setup

- `POST /api/v1/auth/login` with `{"email":"admin@demo.com","password":"Admin123abc"}`
- Extract JWT from `response.data.token`, store in `authToken`
- Skip all tests if login fails (server unreachable)

### helpers_test.go — Shared Utilities

- `apiGet(path, query) → (json.RawMessage, int, error)` — GET with `Authorization: Bearer {authToken}`
- `apiPost(path, body) → (json.RawMessage, int, error)` — POST with auth header
- `apiPut(path, body) → (json.RawMessage, int, error)` — PUT with auth header
- `apiPostAs(token, path, body) → (json.RawMessage, int, error)` — POST with specific token (for employee-context calls)
- `apiGetAs(token, path, query) → (json.RawMessage, int, error)` — GET with specific token
- `requireSuccess(t, resp, status)` — Assert HTTP 200 and `success: true`
- `randomEmployee(seq int) → map[string]any` — Generate employee with PH name/data
- `extractID(resp) → int64` — Extract `data.id` from response
- `extractList(resp) → []any` — Extract `data` array from response
- `extractField(resp, field) → any` — Extract `data.{field}` from response
- Rate-limit retry: if HTTP 429, sleep `Retry-After` seconds and retry once

### 01_employees_test.go

**TestCreateDepartments** — `POST /api/v1/company/departments` (AdminOnly). Create 7 departments with codes (ENG, HR, FIN, MKT, OPS, ADM, SAL). On 409 conflict, look up existing ID from `GET /api/v1/company/departments`. Store in `deptIDs`.

**TestCreatePositions** — `POST /api/v1/company/positions` (AdminOnly). Create 14 positions with codes (ENG-CTO, ENG-SR, ENG-JR, etc.) mapped to department IDs. On 409 conflict, look up existing. Store in `posIDs`.

**TestCreateEmployees** — `POST /api/v1/employees` (AdminOnly). Create 50 employees:
- Unique employee_no: `INT-{unix_ms}-{seq}`
- Randomized PH names from predefined lists
- Distributed across departments and positions
- Mix: regular 70%, probationary 20%, contractual 10%
- Hire dates spread across 2024-2026
- First 5 employees created as managers, rest assigned to them
- Required fields: `employee_no`, `first_name`, `last_name`, `hire_date`
- Optional fields: `email`, `phone`, `birth_date`, `gender`, `civil_status`, `nationality`, `department_id`, `position_id`, `manager_id`, `employment_type`
- Store all created IDs in `createdEmps`

**TestCreateEmployeeUserAccounts** — `POST /api/v1/users/employee-account` (AdminOnly). Create user accounts for 3 test employees so they can clock in/out and submit leave requests. Login as each to get their JWT tokens, store in `empUserTokens`.

**TestAssignSalaries** — `POST /api/v1/employees/:id/salary` (AdminOnly). Assign basic salary to first 10 employees (range 15,000-80,000 PHP). Required for payroll tests to produce meaningful results.

**TestListEmployees** — `GET /api/v1/employees`, verify pagination, verify total count includes created employees.

**TestGetEmployee** — `GET /api/v1/employees/:id` for first created employee, verify all fields returned.

### 02_attendance_test.go

Uses employee JWT tokens from `empUserTokens` (not admin token).

**TestClockIn** — `POST /api/v1/attendance/clock-in` as each of 3 employee users. Send source, lat/lng, note.

**TestClockOut** — `POST /api/v1/attendance/clock-out` as same 3 employees.

**TestGetAttendanceRecords** — `GET /api/v1/attendance/records` (admin token) with today's date range, verify the 3 records exist.

**TestGetAttendanceSummary** — `GET /api/v1/attendance/summary` (employee token), verify response structure.

### 03_leave_test.go

**TestGetLeaveBalances** — `GET /api/v1/leaves/balances` as employee user. Verify leave types and balances returned (seed data should have initialized them).

**TestCreateLeaveRequest** — `POST /api/v1/leaves/requests` as employee user. Request body: `leave_type_id`, `start_date`, `end_date`, `days`, `reason`. Use a future date to avoid conflicts.

**TestListLeaveRequests** — `GET /api/v1/leaves/requests` (admin token), verify created request appears with status "pending".

### 04_payroll_test.go

**TestListPayrollCycles** — `GET /api/v1/payroll/cycles` (admin), verify existing seed cycles returned.

**TestCreatePayrollCycle** — `POST /api/v1/payroll/cycles` (admin). Create a test cycle for a past period (e.g., Feb 2026 first half).

**TestRunPayroll** — `POST /api/v1/payroll/runs` (admin) with the created cycle_id. Verify run starts (status: processing/completed).

**TestGetPayslips** — `GET /api/v1/payroll/payslips` (employee token). May return empty if payroll hasn't completed; accept either data or empty list.

### 05_compliance_test.go

All read-only, admin token.

**TestGetSSSTable** — `GET /api/v1/compliance/sss-table`, verify brackets returned (expect 28+ rows).

**TestGetPhilHealthTable** — `GET /api/v1/compliance/philhealth-table`, verify rate structure.

**TestGetPagIBIGTable** — `GET /api/v1/compliance/pagibig-table`, verify contribution table exists.

**TestGetBIRTaxTable** — `GET /api/v1/compliance/bir-tax-table`, verify tax brackets. Test both `frequency=semi_monthly` and `frequency=monthly`.

### 06_dashboard_test.go

All with admin token. Every endpoint is a simple GET that should return `success: true`.

**TestGetDashboardStats** — `GET /api/v1/dashboard/stats`, verify total_employees > 0.

**TestGetDashboardAttendance** — `GET /api/v1/dashboard/attendance`.

**TestGetDepartmentDistribution** — `GET /api/v1/dashboard/department-distribution`.

**TestGetPayrollTrend** — `GET /api/v1/dashboard/payroll-trend` (AdminOnly).

**TestGetLeaveSummary** — `GET /api/v1/dashboard/leave-summary`.

**TestGetActionItems** — `GET /api/v1/dashboard/action-items` (ManagerOrAbove).

**TestGetCelebrations** — `GET /api/v1/dashboard/celebrations`.

**TestGetSuggestions** — `GET /api/v1/dashboard/suggestions` (ManagerOrAbove).

**TestGetFlightRisk** — `GET /api/v1/dashboard/flight-risk` (ManagerOrAbove).

**TestGetTeamHealth** — `GET /api/v1/dashboard/team-health` (ManagerOrAbove).

**TestGetBurnoutRisk** — `GET /api/v1/dashboard/burnout-risk` (ManagerOrAbove).

**TestGetComplianceAlerts** — `GET /api/v1/dashboard/compliance-alerts` (ManagerOrAbove).

### 07_directory_test.go

**TestGetDirectory** — `GET /api/v1/directory`, verify employees appear in directory.

**TestGetOrgChart** — `GET /api/v1/directory/org-chart`, verify hierarchical structure.

### 08_ai_test.go

**TestAIChat** — `POST /api/v1/ai/chat` with `{"message":"How many employees are there?"}`. Skip if server returns 503 (AI not configured). Verify response contains a non-empty answer.

## Test Data: PH Names

```go
var phFirstNames = []string{
    "Juan", "Maria", "Carlo", "Ana", "Jose", "Rosa",
    "Miguel", "Sofia", "Paolo", "Isabella", "Marco",
    "Gabriela", "Rafael", "Camille", "Antonio",
    "Patricia", "Gabriel", "Beatriz", "Francisco", "Elena",
}

var phLastNames = []string{
    "Santos", "Reyes", "Cruz", "Bautista", "Gonzales",
    "Garcia", "Mendoza", "Torres", "Villanueva", "Ramos",
    "Aquino", "Dela Cruz", "Fernandez", "Castillo", "Rivera",
    "Lopez", "Morales", "Navarro", "Flores", "Perez",
}
```

## Key Design Decisions

1. **Uses existing admin account** — No need to create a new company. Tests against company_id=1 with existing seed data.
2. **Employee user accounts** — 3 employees get user accounts via `POST /users/employee-account` so attendance clock-in/out and leave requests work (these endpoints resolve employee from JWT user_id, not request body).
3. **Salary assignment** — First 10 employees get salary assignments so payroll tests produce meaningful results.
4. **Sequential execution** — Go sorts test files alphabetically. `01_`..`08_` prefix ensures correct dependency order.
5. **Idempotent where possible** — Employee creation uses unique timestamped IDs. Department/position creation handles 409 conflict gracefully.
6. **No cleanup** — Test data stays in DB. Acceptable for dev server.
7. **Graceful skip** — If server unreachable or feature unavailable, tests skip rather than fail.
8. **Rate-limit aware** — HTTP 429 responses trigger retry after delay.
9. **10-minute timeout** — Sufficient for 50+ employee creation + all module tests over network.

## Dependency Chain

```
01_employees  → departments, positions, 50 employees, 3 user accounts, 10 salary assignments
02_attendance → requires employee user accounts (from 01)
03_leave      → requires employee user accounts (from 01)
04_payroll    → requires salary assignments (from 01)
05_compliance → read-only reference tables (no deps)
06_dashboard  → benefits from employee data (from 01)
07_directory  → benefits from employee data (from 01)
08_ai         → standalone
```

## Verification Checklist

- [ ] 50+ employees created successfully
- [ ] 3 employee user accounts created and can login
- [ ] Salaries assigned to 10 employees
- [ ] Employee list/detail/directory/org-chart all return data
- [ ] Attendance clock-in/out works and records appear
- [ ] Leave balances returned, leave request created with status "pending"
- [ ] Payroll cycle created, payroll run executed
- [ ] All 4 compliance tables (SSS, PhilHealth, PagIBIG, BIR) return valid data
- [ ] All 12 dashboard endpoints return success responses
- [ ] AI chat returns a response (if configured)
