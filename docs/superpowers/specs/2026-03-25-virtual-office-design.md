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
  → Set status: "写Q1报表" 📊
  → Click colleague → see their status, role, clock-in time
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
| floor_count | INT NOT NULL DEFAULT 1 | Number of floors (basic: always 1) |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

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
| custom_status | TEXT | User-set status text (max 50 chars) |
| custom_emoji | TEXT | User-set emoji |
| current_task | TEXT | What the user is working on (max 50 chars) |
| manual_status | TEXT | Manual override: in_meeting, on_break, focused, away (NULL = auto-derive) |
| meeting_room_zone | TEXT | Which meeting room zone ID (when manual_status = in_meeting) |
| updated_at | TIMESTAMPTZ | |

**Indexes:**
- UNIQUE (company_id, employee_id)
- UNIQUE (company_id, floor, seat_x, seat_y) — no two employees on same seat
- (company_id, floor) — snapshot query

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

## API Design

### New Endpoints

All under `/api/v1/virtual-office/`.

#### Admin Endpoints (AdminOnly or ManagerOrAbove)

**`GET /config`** — Get company's virtual office config.
- Response: `{ template, floor_count }` or 404 if not configured.

**`PUT /config`** — Create or update office config.
- Request: `{ "template": "medium" }`
- Validates template is one of: small, medium, large.

**`POST /seats/assign`** — Assign an employee to a seat.
- Request: `{ "employee_id": 42, "floor": 1, "zone": "desk-a", "seat_x": 3, "seat_y": 2 }`
- Validates: seat not occupied, employee not already assigned, coordinates within template bounds.

**`POST /seats/auto`** — Auto-assign all unassigned employees by department.
- Groups employees by department, assigns sequentially to desk zones.
- Skips already-assigned employees.
- Response: `{ "assigned": 25, "skipped": 5 }`

**`DELETE /seats/:employee_id`** — Remove an employee's seat assignment.

#### Employee Endpoints (Authenticated)

**`GET /snapshot`** — Core polling endpoint. Returns full office state.
- Cached in Redis for 30 seconds (key: `vo:snapshot:{company_id}`).
- Invalidated on clock-in/clock-out/leave-approval events (best-effort, not blocking).
- Response:

```json
{
  "template": "medium",
  "floor_count": 1,
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
      "current_task": "Q1 Financial Report",
      "clock_in_at": "2026-03-25T09:02:00Z",
      "leave_type": null,
      "meeting_room_zone": null
    }
  ],
  "meeting_rooms": [
    {
      "zone_id": "meeting-a",
      "label": "会议室 A",
      "occupant_ids": [45, 48, 51]
    }
  ]
}
```

**`PUT /my-status`** — Set own status, emoji, current task.
- Request: `{ "manual_status": "in_meeting", "meeting_room_zone": "meeting-a", "custom_emoji": "📊", "current_task": "Sprint planning" }`
- Setting `manual_status` to `null` clears manual override (reverts to auto-derive).
- Invalidates Redis snapshot cache.

**`PUT /my-avatar`** — Change own avatar.
- Request: `{ "avatar_type": "cat", "avatar_color": "#E74C3C" }`
- Validates avatar_type is one of the allowed set.
- Invalidates Redis snapshot cache.

### Implementation Location

```
internal/virtualoffice/
  handler.go         — Handler struct{queries, pool, logger, redis}, RegisterRoutes
  handler_config.go  — GET/PUT config, POST seats/assign, POST seats/auto, DELETE seats
  handler_status.go  — GET snapshot (with derivation logic), PUT my-status
  handler_avatar.go  — PUT my-avatar
```

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
| working | Computer on, chair occupied | Normal idle animation | custom_emoji + current_task |
| overtime | Desk lamp on | Normal + 🔥 overhead | custom text |
| focused | Computer on | Normal + 🎧 overhead | custom text |
| in_meeting | Computer on (empty chair) | Appears in meeting room | none at desk |
| on_break | Computer on (empty chair) | Appears in cafe zone | ☕ |
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

- **Snapshot caching:** Redis cache with 30s TTL. Single query joins attendance_logs + leave_requests + virtual_office_seats. For 100 employees, this is a lightweight query with proper indexes.
- **Cache invalidation:** Best-effort invalidation on clock-in/clock-out/status-change. If cache is stale, worst case is 30s delay — acceptable for observation mode.
- **Pixi.js rendering:** 100 sprites at 32×32 is trivial for Pixi.js. No performance concern.
- **Polling:** Single GET /snapshot every 30s per connected client. For 100 concurrent viewers, that's ~3.3 req/s — negligible server load with Redis cache.

## Security

- All endpoints require JWT authentication.
- Admin endpoints (config, seat assignment) require AdminOnly or ManagerOrAbove middleware.
- Users can only modify their own status/avatar (enforced via `auth.GetEmployeeID(c)`).
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
| `db/migrations/000XX_virtual_office.sql` | Create virtual_office_config + virtual_office_seats tables |
| `db/query/virtual_office.sql` | sqlc queries for all CRUD + snapshot |
| `internal/virtualoffice/handler.go` | Handler struct, RegisterRoutes |
| `internal/virtualoffice/handler_config.go` | Config + seat assignment endpoints |
| `internal/virtualoffice/handler_status.go` | Snapshot + my-status endpoints |
| `internal/virtualoffice/handler_avatar.go` | Avatar change endpoint |
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
