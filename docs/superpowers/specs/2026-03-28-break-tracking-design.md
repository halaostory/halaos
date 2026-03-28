# Break Tracking & Monthly Report Design

## Overview

Add mid-work break clock-in/out functionality to HalaOS (AIGoNHR). Employees can track 4 types of breaks (meal, bathroom, rest, leave-post) via Web UI, Telegram Bot, or CLI. Break data aggregates into a monthly Excel report matching the existing company report format.

## Scope

- **Backend**: New database tables, API endpoints, break logic
- **Telegram Bot**: Inline keyboard break start/end flow
- **Frontend**: Break clock UI in attendance page, admin break policy config
- **CLI**: New `halaos hr break` commands
- **Report**: Monthly Excel export matching the 23-column format

## Data Model

### `break_policies` table

Company-level configuration for overtime thresholds per break type.

```sql
CREATE TABLE break_policies (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    break_type      VARCHAR(20) NOT NULL,  -- meal, bathroom, rest, leave_post
    max_minutes     INT NOT NULL,          -- per-break allowed time; exceeding = overtime
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, break_type)
);
```

Default seeds per company:
| break_type | max_minutes | Description |
|------------|-------------|-------------|
| meal | 30 | Lunch/dinner |
| bathroom | 5 | Restroom |
| rest | 0 | 0 = no limit (no overtime tracking) |
| leave_post | 0 | 0 = no limit |

### `break_logs` table

One row per break session. `end_at IS NULL` means break is active.

```sql
CREATE TABLE break_logs (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id),
    employee_id       BIGINT NOT NULL REFERENCES employees(id),
    attendance_log_id BIGINT NOT NULL REFERENCES attendance_logs(id),
    break_type        VARCHAR(20) NOT NULL CHECK (break_type IN ('meal','bathroom','rest','leave_post')),
    start_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_at            TIMESTAMPTZ,
    duration_minutes  INT,
    overtime_minutes  INT DEFAULT 0,
    note              TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_break_logs_attendance ON break_logs(attendance_log_id);
CREATE INDEX idx_break_logs_employee_date ON break_logs(employee_id, start_at DESC);
CREATE INDEX idx_break_logs_company_period ON break_logs(company_id, start_at);
```

### Break type mapping (report columns)

| break_type | Chinese label | Report columns |
|------------|--------------|----------------|
| meal | 吃饭 | J (count), K (total time), L (overtime) |
| bathroom | 上厕所 | M (count), N (total time), O (overtime) |
| rest | 休息 | P (count), Q (total time) |
| leave_post | 中途离岗 | R (count), S (total time) |

## API Endpoints

### Employee endpoints (Protected)

**POST `/api/v1/attendance/breaks/start`**

Start a break. Requires active attendance (clocked in, not clocked out). Only one active break at a time.

Request:
```json
{
  "break_type": "bathroom",
  "note": "optional"
}
```

Validation:
1. Employee must have open attendance (clocked in, `clock_out_at IS NULL`)
2. No other active break (`end_at IS NULL`) for this employee
3. `break_type` must be one of: meal, bathroom, rest, leave_post

Response: 201 Created with break_log record.

**POST `/api/v1/attendance/breaks/end`**

End the current active break. No request body needed.

Logic:
1. Find open break for employee (`end_at IS NULL`)
2. Set `end_at = NOW()`
3. Calculate `duration_minutes = EXTRACT(EPOCH FROM end_at - start_at) / 60`
4. Lookup `break_policies` for this company + break_type
5. If `max_minutes > 0` and `duration_minutes > max_minutes`: `overtime_minutes = duration_minutes - max_minutes`
6. Return updated break_log

Response: 200 OK with break_log record.

**GET `/api/v1/attendance/breaks`**

List today's breaks for current employee. Query params: `date` (optional, defaults to today).

**GET `/api/v1/attendance/breaks/active`**

Get current active break (if any). Returns 200 with break_log or 200 with `null` data.

### Admin endpoints (Manager+)

**GET `/api/v1/attendance/break-policies`**

List break policies for company.

**PUT `/api/v1/attendance/break-policies`**

Batch upsert break policies.

Request:
```json
{
  "policies": [
    { "break_type": "meal", "max_minutes": 30 },
    { "break_type": "bathroom", "max_minutes": 5 },
    { "break_type": "rest", "max_minutes": 0 },
    { "break_type": "leave_post", "max_minutes": 0 }
  ]
}
```

### Report endpoint (Manager+)

**GET `/api/v1/attendance/report/monthly?year=2026&month=2`**

Returns Excel (.xlsx) file download. Format matches the 23-column report:

| Col | Header | Source |
|-----|--------|--------|
| A | 用户昵称 | Telegram display name OR employee first_name |
| B | 用户标识 | bot_user_links.platform_user_id (Telegram ID) |
| C | 工作天数 | COUNT(DISTINCT attendance_logs.clock_in_at::date) |
| D | 工作时间总计 | SUM(attendance_logs.work_hours) formatted as "X.X 小时" |
| E | 纯工作时间总计 | D minus U (total work - all break time) |
| F | 迟到天数 | COUNT where late_minutes > 0 |
| G | 迟到总时长 | SUM(late_minutes) formatted as "X 分钟" |
| H | 早退天数 | COUNT where undertime_minutes > 0 |
| I | 早退总时长 | SUM(undertime_minutes) formatted as "X 分钟" |
| J | 吃饭总次数 | COUNT(break_logs where type=meal) |
| K | 吃饭总用时 | SUM(duration_minutes where type=meal) formatted as "X.X 小时" |
| L | 吃饭总超时 | SUM(overtime_minutes where type=meal) formatted as "X 分钟（共 N 次）" |
| M | 上厕所总次数 | COUNT(break_logs where type=bathroom) |
| N | 上厕所总用时 | SUM(duration_minutes where type=bathroom) as "X.X 小时" |
| O | 上厕所总超时 | SUM(overtime_minutes where type=bathroom) |
| P | 休息总次数 | COUNT(break_logs where type=rest) |
| Q | 休息总用时 | SUM(duration_minutes where type=rest) as "X.X 小时" |
| R | 中途离岗总次数 | COUNT(break_logs where type=leave_post) |
| S | 中途离岗总用时 | SUM(duration_minutes where type=leave_post) as "X.X 小时" |
| T | 所有次数总计 | J + M + P + R |
| U | 所有用时总计 | K + N + Q + S |
| V | 所有超时总计 | L + O (sum of all overtime) |
| W | 手动惩罚 | "——" (not implemented) |

Formatting rules:
- Empty/zero values display as "——"
- Hours: "X.X 小时" (1 decimal)
- Minutes: "X 分钟"
- Overtime with count: "X 分钟（共 N 次）"

Uses `excelize` Go library for .xlsx generation.

## Telegram Bot

### New commands

| Trigger | Action |
|---------|--------|
| `/break` | Show inline keyboard with 4 break types |
| `/break_end` | End current break |
| `/break_status` | Show current break status |

### Inline keyboard flow

**Start break**: User sends `/break` →

```
选择休息类型:
[🍽 吃饭] [🚻 上厕所]
[😌 休息] [🚪 中途离岗]
```

User taps button → callback `break:meal` (etc.) → Bot creates break_log via DB queries → responds:

```
✅ 开始休息: 吃饭
⏰ 开始时间: 14:32
[结束休息]
```

**End break**: User taps "结束休息" button (callback `break:end`) →

```
✅ 休息结束: 吃饭
⏱ 时长: 28 分钟
```

Or if overtime:
```
⚠️ 休息结束: 吃饭
⏱ 时长: 35 分钟（超时 5 分钟）
```

### Implementation

Add to `dispatcher.go`:
- New case in `dispatchCommand()` for "break", "break_end", "break_status"
- New callback handlers in `HandleCallback()` for "break:" prefix
- Direct DB access via `store.Queries` (same pattern as existing commands)

New file: `internal/bot/handler_break.go` containing:
- `handleBreakStart(msg)` - sends type selection keyboard
- `handleBreakTypeCallback(msg, breakType)` - creates break_log
- `handleBreakEnd(msg)` - ends active break
- `handleBreakStatus(msg)` - shows current state

## Frontend

### Attendance page changes

Add "Break Clock" section to the existing attendance dashboard:

1. **Break action area** (visible when clocked in):
   - 4 break type buttons (meal, bathroom, rest, leave_post)
   - When break is active: show timer + "End Break" button
   - Break type label shown during active break

2. **Today's break log** (table below actions):
   - Columns: Type, Start, End, Duration, Overtime
   - Shows all breaks for today

### Admin break policy page

New settings section under Attendance admin:
- Table of 4 break types with editable max_minutes
- Save button → PUT /api/v1/attendance/break-policies

### Report download

Add "Monthly Report" button to attendance report page:
- Year/month picker
- Download button → triggers Excel generation and download

## CLI Commands

### Break commands

```bash
# Start a break
halaos hr break start --type meal [--note "lunch"]

# End current break
halaos hr break end

# List today's breaks
halaos hr break list [--date 2026-03-28]

# Check active break
halaos hr break active
```

### Break policy commands

```bash
# List policies
halaos hr break-policy list

# Set policy
halaos hr break-policy set --type meal --max-minutes 30
```

### Report command

```bash
# Generate monthly report
halaos hr report monthly --year 2026 --month 2 [--out report.xlsx]
```

## SQL Queries (sqlc)

### break_logs queries

```sql
-- CreateBreakLog
INSERT INTO break_logs (company_id, employee_id, attendance_log_id, break_type, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- EndBreakLog
UPDATE break_logs SET end_at = NOW(),
  duration_minutes = EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60,
  overtime_minutes = CASE
    WHEN $2 > 0 AND EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60 > $2
    THEN EXTRACT(EPOCH FROM (NOW() - start_at))::int / 60 - $2
    ELSE 0
  END
WHERE id = $1 AND end_at IS NULL
RETURNING *;

-- GetActiveBreak
SELECT * FROM break_logs
WHERE employee_id = $1 AND company_id = $2 AND end_at IS NULL
ORDER BY start_at DESC LIMIT 1;

-- ListBreaksByAttendance
SELECT * FROM break_logs
WHERE attendance_log_id = $1
ORDER BY start_at;

-- ListBreaksByDate
SELECT * FROM break_logs
WHERE employee_id = $1 AND company_id = $2
  AND start_at >= $3 AND start_at < $4
ORDER BY start_at;

-- GetMonthlyBreakSummary
SELECT
  bl.employee_id,
  bl.break_type,
  COUNT(*) as total_count,
  SUM(bl.duration_minutes) as total_minutes,
  SUM(bl.overtime_minutes) as total_overtime_minutes,
  COUNT(CASE WHEN bl.overtime_minutes > 0 THEN 1 END) as overtime_count
FROM break_logs bl
WHERE bl.company_id = $1
  AND bl.start_at >= $2 AND bl.start_at < $3
  AND bl.end_at IS NOT NULL
GROUP BY bl.employee_id, bl.break_type;
```

### break_policies queries

```sql
-- ListBreakPolicies
SELECT * FROM break_policies
WHERE company_id = $1 AND is_active = true;

-- UpsertBreakPolicy
INSERT INTO break_policies (company_id, break_type, max_minutes)
VALUES ($1, $2, $3)
ON CONFLICT (company_id, break_type)
DO UPDATE SET max_minutes = $3, updated_at = NOW()
RETURNING *;

-- GetBreakPolicy
SELECT * FROM break_policies
WHERE company_id = $1 AND break_type = $2 AND is_active = true;
```

## Error Handling

| Scenario | Response |
|----------|----------|
| Not clocked in | 400: "Must clock in before starting a break" |
| Already on break | 400: "Already on break (type: meal). End it first." |
| No active break to end | 400: "No active break to end" |
| Invalid break type | 400: "Invalid break type. Must be: meal, bathroom, rest, leave_post" |

## Testing

- Unit tests for break handler (start/end/list/active)
- Unit tests for overtime calculation logic
- Unit tests for monthly report aggregation SQL
- Unit tests for Telegram bot break command/callback handlers
- Unit tests for CLI break commands (mock HTTP server pattern from existing tests)
- Integration test: full break cycle (clock in → start break → end break → verify report)

## Dependencies

- `github.com/xuri/excelize/v2` - Excel file generation (new dependency)
- No other new dependencies needed

## Migration number

Next migration: `00088_break_tracking.sql` (after existing `00087_brain_integration.sql`)
