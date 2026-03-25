# Virtual Office: 2D Pixel-Art Office Visualization

## Problem

Remote and hybrid teams lack visibility into who's working, who's in a meeting, who's on leave, and what everyone is doing. HalaOS already tracks attendance, leave, and employee status — but this data is buried in tables and dashboards. Managers need to open multiple pages to understand team availability.

## Solution

A 2D pixel-art virtual office that visualizes every employee's real-time status on an interactive office map. Employees appear at their desks when clocked in, disappear when offline, show up in meeting rooms when in meetings, and display leave badges when on leave. Users can set custom statuses, choose pixel avatars (people or animals), and click colleagues to see details.

**Phase 1 (this spec):** Observation + light interaction — status visualization, custom status/avatar, click-to-view. Polling-based updates (30s).

**Future phases:** Keyboard movement, proximity chat, WebSocket real-time, video/voice integration (Gather.town direction).

## User Flow

```
Admin first-time setup:
  Settings → Virtual Office → Choose template (small/medium/large)
  → Assign employees to seats (manual or auto-by-department)
  → Office is live

Employee daily experience:
  Sidebar → Virtual Office → See the office map
  → Own avatar appears at desk (auto from clock-in)
  → Set custom status: "写Q1报表" 📊
  → Click colleague → see their custom status, role, clock-in time
  → Set "in meeting" → avatar moves to meeting room
  → Clock out → avatar disappears from desk
```

## Data Model

### New Tables

#### `virtual_office_config`

Company-level office configuration. One row per company.

| Column | Type | Description |
|--------|------|-------------|
| company_id | BIGINT PK | FK to companies |
| template | TEXT NOT NULL DEFAULT 'small' | Template ID: small/medium/large |
| created_at | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| updated_at | TIMESTAMPTZ NOT NULL DEFAULT now() | |

#### `virtual_office_seats`

Employee seat assignment and personalization. One row per assigned employee.

| Column | Type | Description |
|--------|------|-------------|
| id | BIGSERIAL PK | |
| company_id | BIGINT NOT NULL | FK to companies |
| employee_id | BIGINT NOT NULL UNIQUE | FK to employees, one seat per person |
| floor | INT DEFAULT 1 | Floor number |
| zone | TEXT DEFAULT 'desk' | Zone ID from template (desk-a, meeting-a, cafe) |
| seat_x | INT NOT NULL | Grid X coordinate |
| seat_y | INT NOT NULL | Grid Y coordinate |
| avatar_type | TEXT DEFAULT 'person_1' | Sprite ID: person_1-6, cat, dog, rabbit, bear, penguin, shiba |
| avatar_color | TEXT DEFAULT '#4A90D9' | Clothing/accent color hex |
| custom_status | TEXT | User-set status text, e.g. "写Q1报表" or "Sprint planning" (max 50 chars) |
| custom_emoji | TEXT | User-set emoji, e.g. "📊" |
| manual_status | TEXT | Manual override: in_meeting, on_break, focused, away (NULL = auto-derive) |
| meeting_room_zone | TEXT | Which meeting room zone ID (when manual_status = in_meeting) |
| created_at | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| updated_at | TIMESTAMPTZ NOT NULL DEFAULT now() | |

**Indexes:**
- UNIQUE (company_id, employee_id)
- UNIQUE (company_id, floor, seat_x, seat_y) — no two employees on same seat
- (company_id) — snapshot query join

**Constraints:**
- `meeting_room_zone` must be validated against the template's meeting room zone IDs when `manual_status = 'in_meeting'`. Invalid zone values are rejected with 400.
- Only employees with `status IN ('active', 'probationary')` can be assigned seats. Terminated/separated employees are excluded from seat assignment and snapshot results.

**No real-time status table needed.** Online/offline/on_leave status is derived from existing `attendance_logs` and `leave_requests` at query time.

### Status Derivation Logic

```
Priority (highest to lowest):
1. Approved leave today (leave_requests WHERE status='approved' AND today BETWEEN start_date AND end_date)
   → status: "on_leave", leave_type from leave_types.name
2. Manual status set by user (virtual_office_seats.manual_status IS NOT NULL)
   → status: manual_status value (in_meeting/on_break/focused/away)
3. Clocked in today and not clocked out (attendance_logs WHERE clock_in_at today AND clock_out_at IS NULL)
   → status: "working"
   → If current_time > shift.end_time: "overtime"
   → If late_minutes > 0: add "late" flag
4. Clocked out today OR no attendance record
   → status: "offline"
```

### Snapshot SQL Query

The snapshot endpoint uses a single query with CTEs to derive status for all seated employees:

```sql
-- name: GetVirtualOfficeSnapshot :many
WITH today_leaves AS (
    SELECT DISTINCT ON (lr.employee_id)
           lr.employee_id, lt.name AS leave_type
    FROM leave_requests lr
    JOIN leave_types lt ON lt.id = lr.leave_type_id
    WHERE lr.company_id = $1
      AND lr.status = 'approved'
      AND CURRENT_DATE BETWEEN lr.start_date AND lr.end_date
    ORDER BY lr.employee_id, lr.created_at DESC
),
today_attendance AS (
    SELECT DISTINCT ON (employee_id)
           employee_id,
           clock_in_at,
           clock_out_at,
           late_minutes
    FROM attendance_logs
    WHERE company_id = $1
      AND clock_in_at >= CURRENT_DATE
      AND clock_in_at < CURRENT_DATE + INTERVAL '1 day'
    ORDER BY employee_id, clock_in_at DESC
)
SELECT
    s.id AS seat_id,
    s.employee_id,
    e.first_name || ' ' || e.last_name AS name,
    COALESCE(p.title, '') AS position,
    COALESCE(d.name, '') AS department,
    s.floor,
    s.zone,
    s.seat_x,
    s.seat_y,
    s.avatar_type,
    s.avatar_color,
    s.custom_status,
    s.custom_emoji,
    s.manual_status,
    s.meeting_room_zone,
    tl.leave_type,
    ta.clock_in_at,
    ta.clock_out_at,
    COALESCE(ta.late_minutes, 0) AS late_minutes
FROM virtual_office_seats s
JOIN employees e ON e.id = s.employee_id AND e.company_id = s.company_id
LEFT JOIN positions p ON p.id = e.position_id
LEFT JOIN departments d ON d.id = e.department_id
LEFT JOIN today_leaves tl ON tl.employee_id = s.employee_id
LEFT JOIN today_attendance ta ON ta.employee_id = s.employee_id
WHERE s.company_id = $1
  AND e.status IN ('active', 'probationary')
ORDER BY s.floor, s.zone, s.seat_y, s.seat_x;
```

**Required indexes** (beyond the table-level indexes above):
- `attendance_logs(company_id, clock_in_at)` — existing index works since query uses range condition `>= CURRENT_DATE AND < CURRENT_DATE + 1 day`
- `leave_requests(company_id, status, start_date, end_date)` — add if missing

Status derivation is done in Go code after the query, using the priority rules defined above.

### Auto-Assign Algorithm

`POST /seats/auto` assigns unassigned active employees to empty seats:

1. Load the company's template definition from `templates.go`
2. Query all active/probationary employees not yet assigned a seat
3. Query all occupied seat positions for the company
4. Group unassigned employees by `department_id`
5. Iterate through template desk zones in order; within each zone, iterate through seats in row-major order (y then x)
6. For each empty seat, assign the next unassigned employee from the current department group. When a department group is exhausted, move to the next.
7. Return count of assigned vs skipped (skipped = no more empty seats)

## API Design

### New Endpoints

All under `/api/v1/virtual-office/`.

#### Admin Endpoints (AdminOnly or ManagerOrAbove)

**`GET /config`** — Get company's virtual office config.
- Response: `{ template }` or 404 if not configured.

**`PUT /config`** — Create or update office config.
- Request: `{ "template": "medium" }`
- Validates template is one of: small, medium, large.

**`POST /seats/assign`** — Assign an employee to a seat.
- Request: `{ "employee_id": 42, "floor": 1, "zone": "desk-a", "seat_x": 3, "seat_y": 2 }`
- Validates:
  - Employee exists and has `status IN ('active', 'probationary')` — rejects terminated/separated.
  - Seat not already occupied (UNIQUE constraint on company_id, floor, seat_x, seat_y).
  - Employee not already assigned (UNIQUE constraint on company_id, employee_id).
  - Coordinates within template bounds (checked against `templates.go` definitions).
  - Zone is a valid zone ID for the company's selected template.

**`POST /seats/auto`** — Auto-assign all unassigned active/probationary employees by department.
- See "Auto-Assign Algorithm" section above for the full algorithm.
- Only assigns employees with `status IN ('active', 'probationary')`.
- Skips already-assigned employees and occupied seats.
- Response: `{ "assigned": 25, "skipped": 5, "no_seats": 0 }` — `no_seats` is employees that couldn't be assigned due to template capacity.

**`DELETE /seats/:employee_id`** — Remove an employee's seat assignment.

**`GET /seats`** — List all seat assignments for the company.
- Response: array of seat records with employee name/department for the admin seat management UI.
- Used by OfficeSetup.vue to show current seat assignments and allow drag-to-reassign.

#### Employee Endpoints (Authenticated)

**`GET /snapshot`** — Core polling endpoint. Returns full office state.
- Uses the snapshot SQL query (see "Snapshot SQL Query" section) to fetch all seat data.
- Status derivation is done in Go code after the query, applying the priority rules.
- `meeting_rooms` array is computed by grouping employees with `manual_status = 'in_meeting'` by `meeting_room_zone`, with labels from the template definition.
- Cached in Redis for 30 seconds (key: `vo:snapshot:{company_id}`). If Redis is unavailable, falls back to direct query (no error).
- Invalidated on clock-in/clock-out/leave-approval events (best-effort, not blocking).
- Response:

```json
{
  "template": "medium",
  "stats": {
    "total_assigned": 30,
    "online": 12,
    "on_leave": 3,
    "in_meeting": 5,
    "offline": 10
  },
  "seats": [
    {
      "seat_id": 1,
      "employee_id": 42,
      "name": "Maria Santos",
      "position": "Accountant",
      "department": "Finance",
      "floor": 1,
      "zone": "desk-a",
      "seat_x": 3,
      "seat_y": 2,
      "avatar_type": "cat",
      "avatar_color": "#4A90D9",
      "status": "working",
      "is_late": false,
      "custom_status": "写Q1报表",
      "custom_emoji": "📊",
      "clock_in_at": "2026-03-25T09:02:00Z",
      "leave_type": null,
      "meeting_room_zone": null
    }
  ],
  "meeting_rooms": [
    {
      "zone_id": "meeting-a",
      "label": "Meeting Room",
      "occupant_ids": [45, 48, 51]
    }
  ]
}
```

**`PUT /my-status`** — Set own manual status, custom status text, and emoji.
- Request: `{ "manual_status": "in_meeting", "meeting_room_zone": "meeting-a", "custom_status": "Sprint planning", "custom_emoji": "📊" }`
- All fields are optional; only provided fields are updated (PATCH semantics).
- Setting `manual_status` to `null` or `""` clears manual override (reverts to auto-derive).
- When `manual_status = "in_meeting"`, `meeting_room_zone` is required and validated against the company's template meeting room zones. When clearing `in_meeting`, `meeting_room_zone` is also cleared.
- Auth: uses `auth.GetUserID(c)` → `GetEmployeeByUserID` to resolve the calling user's seat.
- Invalidates Redis snapshot cache.

**`PUT /my-avatar`** — Change own avatar appearance only (no status fields).
- Request: `{ "avatar_type": "cat", "avatar_color": "#E74C3C" }`
- Validates `avatar_type` is one of: person_1-6, cat, dog, rabbit, bear, penguin, shiba.
- Validates `avatar_color` is a valid hex color (regex `^#[0-9A-Fa-f]{6}$`).
- Auth: uses `auth.GetUserID(c)` → `GetEmployeeByUserID` to resolve the calling user's seat.
- Invalidates Redis snapshot cache.

### Implementation Location

```
internal/virtualoffice/
  handler.go         — Handler struct, NewHandler, RegisterRoutes
  handler_config.go  — GET/PUT config, GET seats, POST seats/assign, POST seats/auto, DELETE seats
  handler_status.go  — GET snapshot (with derivation logic), PUT my-status
  handler_avatar.go  — PUT my-avatar
  templates.go       — Embedded template definitions (small/medium/large) for server-side validation
```

**Handler struct:**
```go
type Handler struct {
    queries *store.Queries
    pool    *pgxpool.Pool
    logger  *slog.Logger
    rdb     *redis.Client  // for snapshot cache
}
```

Constructor: `NewHandler(queries, pool, logger, rdb)`. Wired in `bootstrap.go` as:
```go
virtualOfficeHandler := virtualoffice.NewHandler(a.Queries, a.Pool, a.Logger, a.Redis)
```

**Server-side templates:** `templates.go` contains a `var Templates map[string]TemplateInfo` with each template's max grid dimensions and valid zone IDs. Used for:
- Validating seat coordinates are within template bounds on `POST /seats/assign`
- Validating `meeting_room_zone` is a valid meeting room zone on `PUT /my-status`
- Auto-assign algorithm uses template seat positions

New sqlc queries in `db/query/virtual_office.sql`. Migration file for the two new tables.

## Frontend Architecture

### Rendering Stack

**Pixi.js v8** for the office canvas:
- Tilemap rendering (floor, walls, furniture) from template JSON
- Character sprites with idle animation (3-frame breathing)
- Status bubbles (emoji + text) as Pixi.js sprites above characters
- Zone highlights (meeting rooms glow when occupied)

**NaiveUI** for all UI overlays:
- SeatInfoCard: NPopover on character click
- StatusBar: Bottom bar for setting own status
- OfficeSetup: Admin panel for template selection and seat assignment
- MiniMap: Canvas thumbnail for medium/large offices

### New Files

```
frontend/src/
  views/
    VirtualOfficeView.vue         — Main page, polling logic, layout
  components/
    virtual-office/
      OfficeCanvas.vue            — Pixi.js Application container (mount/unmount)
      OfficeRenderer.ts           — Load template, render tilemap, zones, furniture
      SpriteManager.ts            — Create/update/remove character sprites, diff logic
      SeatInfoCard.vue            — Click popup: name, role, status, clock-in time
      StatusBar.vue               — Bottom: set status, emoji, current task
      OfficeSetup.vue             — Admin: template picker, seat assignment grid
      OfficeStats.vue             — Top bar: online/leave/meeting counts
      MiniMap.vue                 — Thumbnail overview for larger maps
  assets/
    virtual-office/
      templates/
        small.json                — 10-person layout
        medium.json               — 30-person layout
        large.json                — 100-person layout
      sprites/
        characters.png            — 12 character spritesheets (6 people + 6 animals)
        furniture.png             — Desk, chair, plant, whiteboard, lamp tileset
        tiles.png                 — Floor, wall, carpet tileset
        status-icons.png          — Meeting, break, focus, away, leave icons
```

### Data Flow

```
VirtualOfficeView.vue
  │
  ├── onMounted: fetch template JSON + first snapshot
  ├── setInterval(30s): poll GET /snapshot
  │
  ├── OfficeStats.vue ← snapshot.stats
  │
  ├── OfficeCanvas.vue
  │     ├── OfficeRenderer.ts ← template JSON (static, loaded once)
  │     └── SpriteManager.ts ← snapshot.seats (diffed on each poll)
  │           └── on sprite click → emit('select', employee)
  │
  ├── SeatInfoCard.vue ← selected employee data
  │
  ├── StatusBar.vue → PUT /my-status, PUT /my-avatar
  │
  ├── MiniMap.vue ← OfficeCanvas viewport
  │
  └── (admin) OfficeSetup.vue → PUT /config, POST /seats/assign
```

### Sprite Diff Update

On each snapshot poll, SpriteManager compares previous and current state:

- **New employee online:** Create sprite at seat position with fade-in animation
- **Employee went offline:** Fade-out sprite, show empty desk
- **Status changed:** Update bubble icon/text, play brief highlight animation
- **Moved to meeting room:** Slide sprite from desk to meeting room zone
- **Returned from meeting:** Slide sprite back to desk

### Routing & Navigation

Add to existing router:
```typescript
{ path: '/virtual-office', component: VirtualOfficeView, meta: { requiresAuth: true } }
```

Add to sidebar navigation with 🏢 icon, label: "Virtual Office" / "虚拟办公室".

## Office Templates

### Template Format

Static JSON files in `frontend/src/assets/virtual-office/templates/`.

```json
{
  "id": "small",
  "name": "Small Office",
  "description": "Cozy office for teams up to 10",
  "width": 20,
  "height": 16,
  "tileSize": 32,
  "zones": [
    {
      "id": "desk-a",
      "type": "desk_area",
      "label": "Workstations",
      "x": 1, "y": 1, "w": 12, "h": 6,
      "seats": [
        { "x": 2, "y": 2 },
        { "x": 5, "y": 2 },
        { "x": 8, "y": 2 },
        { "x": 11, "y": 2 },
        { "x": 2, "y": 5 },
        { "x": 5, "y": 5 },
        { "x": 8, "y": 5 },
        { "x": 11, "y": 5 }
      ]
    },
    {
      "id": "meeting-a",
      "type": "meeting_room",
      "label": "Meeting Room",
      "x": 1, "y": 9, "w": 6, "h": 5,
      "capacity": 6,
      "meeting_seats": [
        { "x": 2, "y": 10 },
        { "x": 4, "y": 10 },
        { "x": 2, "y": 12 },
        { "x": 4, "y": 12 }
      ]
    },
    {
      "id": "cafe",
      "type": "cafe",
      "label": "Tea Room",
      "x": 9, "y": 9, "w": 5, "h": 5
    }
  ],
  "furniture": [
    { "type": "desk", "x": 2, "y": 2 },
    { "type": "computer", "x": 2, "y": 1 },
    { "type": "plant", "x": 0, "y": 0 },
    { "type": "plant", "x": 19, "y": 0 },
    { "type": "whiteboard", "x": 2, "y": 9 },
    { "type": "coffee_machine", "x": 10, "y": 10 }
  ]
}
```

### Template Specifications

| Template | Grid Size | Max Seats | Desk Areas | Meeting Rooms | Other Zones |
|----------|-----------|-----------|------------|---------------|-------------|
| small | 20×16 | 10 | 1 | 1 | cafe |
| medium | 32×24 | 30 | 2 | 2 | cafe, lounge |
| large | 48×36 | 100 | 4 | 3 | cafe, lounge, phone booths |

## Visual Status Reference

| Status | Desk Appearance | Character | Bubble |
|--------|----------------|-----------|--------|
| working | Computer on, chair occupied | Normal idle animation | custom_emoji + custom_status |
| overtime | Desk lamp on | Normal + 🔥 overhead | custom_status |
| focused | Computer on | Normal + 🎧 overhead | custom_status |
| in_meeting | Computer on (empty chair) | Appears in meeting room | none at desk |
| on_break | Computer on (empty chair) | Remains at desk with ☕ icon (cafe zone movement is future-phase) | ☕ |
| away | Computer on, chair occupied | Semi-transparent | 💤 |
| on_leave | Computer off, leave badge on desk | Not visible | Badge shows leave type icon |
| offline | Computer off, empty | Not visible | none |

### Leave Type Icons

| Leave Type | Desk Badge |
|------------|------------|
| Sick Leave | 🏥 |
| Vacation Leave | 🏖️ |
| Maternity Leave | 👶 |
| Personal Leave | 📋 |
| Other | 📝 |

## Pixel Art Resources

### Character Sprites (32×32 pixels each)

**12 avatars total:**
- 6 people: person_1 through person_6 (3 male, 3 female, different hairstyles)
- 6 animals: cat, dog, rabbit, bear, penguin, shiba

Each avatar spritesheet: 4 directions × 3 frames (idle breathing animation) = 12 frames.
Clothing/accent color applied via Pixi.js tint using `avatar_color`.

**Source:** Open-source pixel art (Kenney.nl style) or AI-generated. Exact assets to be created during implementation.

### Furniture Tileset

Standard office furniture at 32×32: desk, chair, computer, plant, whiteboard, coffee_machine, lamp, bookshelf, sofa, table.

### Floor Tileset

Floor types: wood, carpet, tile. Wall segments. Zone boundary indicators.

## Performance Considerations

- **Snapshot caching:** Redis cache with 30s TTL. Single CTE query joins attendance_logs + leave_requests + virtual_office_seats + employees + departments + positions. For 100 employees, this is a lightweight query with proper indexes. Redis failure gracefully falls back to direct query.
- **Cache invalidation:** Best-effort invalidation on clock-in/clock-out/status-change. If cache is stale, worst case is 30s delay — acceptable for observation mode.
- **Pixi.js rendering:** 100 sprites at 32×32 is trivial for Pixi.js. No performance concern.
- **Polling:** Single GET /snapshot every 30s per connected client. For 100 concurrent viewers, that's ~3.3 req/s — negligible server load with Redis cache.

## Security

- All endpoints require JWT authentication.
- Admin endpoints (config, seat assignment) require AdminOnly or ManagerOrAbove middleware.
- Users can only modify their own status/avatar (enforced via `auth.GetUserID(c)` → `GetEmployeeByUserID` lookup).
- Snapshot is visible to all authenticated users in the same company (company_id from JWT).
- No sensitive data in snapshot (no salary, no personal details — just name, position, department, status).

## Testing

- Unit tests for status derivation logic (all priority combinations)
- Unit tests for seat assignment validation (no duplicates, bounds checking)
- Unit tests for auto-assign by department
- Handler tests for all CRUD endpoints following existing MockDBTX pattern
- Frontend: manual testing of Pixi.js rendering (visual)

## Out of Scope (Future Phases)

| Feature | Phase |
|---------|-------|
| Keyboard character movement | Phase 2 |
| WebSocket real-time push | Phase 2 |
| Proximity-based video/voice | Phase 3 |
| In-app chat | Phase 2-3 |
| Custom map editor (drag-and-drop) | Phase 2 |
| Multi-floor switching | Phase 2 |
| Meeting room calendar/booking | Phase 2 |
| Activity feed / notifications | Phase 2 |
| Mobile-optimized view | Phase 2 |

## Files to Create/Modify

### New Files

| File | Purpose |
|------|---------|
| `db/migrations/00084_virtual_office.sql` | Create virtual_office_config + virtual_office_seats tables |
| `db/query/virtual_office.sql` | sqlc queries for all CRUD + snapshot |
| `internal/virtualoffice/handler.go` | Handler struct, NewHandler, RegisterRoutes |
| `internal/virtualoffice/handler_config.go` | Config + seat CRUD endpoints (GET/PUT config, GET/POST/DELETE seats) |
| `internal/virtualoffice/handler_status.go` | Snapshot + my-status endpoints |
| `internal/virtualoffice/handler_avatar.go` | Avatar change endpoint |
| `internal/virtualoffice/templates.go` | Server-side template definitions for validation |
| `internal/virtualoffice/handler_test.go` | Unit tests |
| `frontend/src/views/VirtualOfficeView.vue` | Main page |
| `frontend/src/components/virtual-office/*.vue` | 6 Vue components |
| `frontend/src/components/virtual-office/*.ts` | OfficeRenderer + SpriteManager |
| `frontend/src/assets/virtual-office/templates/*.json` | 3 template files |
| `frontend/src/assets/virtual-office/sprites/*` | Pixel art spritesheets |

### Modified Files

| File | Change |
|------|--------|
| `frontend/src/api/client.ts` | Add `virtualOfficeAPI` export |
| `frontend/src/router/index.ts` | Add /virtual-office route |
| `frontend/src/i18n/en.ts` | Add virtualOffice.* translations |
| `frontend/src/i18n/zh.ts` | Add virtualOffice.* translations |
| `frontend/src/components/DashboardLayout.vue` | Add sidebar menu item |
| `cmd/api/main.go` (or routes registration) | Register virtualoffice handler |
| `frontend/package.json` | Add pixi.js dependency |
