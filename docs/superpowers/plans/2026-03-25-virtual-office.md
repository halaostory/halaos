# Virtual Office Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a 2D pixel-art virtual office that visualizes every employee's real-time status on an interactive map with Pixi.js, polling every 30 seconds.

**Architecture:** Two new PostgreSQL tables (`virtual_office_config`, `virtual_office_seats`) with a Go handler package (`internal/virtualoffice/`) that derives status from existing attendance/leave data via a CTE query cached in Redis. Frontend uses Pixi.js v8 for canvas rendering with NaiveUI overlays, added as a new route `/virtual-office` in the existing SPA.

**Tech Stack:** Go 1.25 (Gin, sqlc, pgx/v5, go-redis), PostgreSQL 16, Redis 7, Vue 3 + TypeScript + NaiveUI, Pixi.js v8

**Spec:** `docs/superpowers/specs/2026-03-25-virtual-office-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `db/migrations/00084_virtual_office.sql` | Migration: create 2 tables + indexes |
| `db/query/virtual_office.sql` | sqlc queries: config CRUD, seat CRUD, snapshot CTE |
| `internal/virtualoffice/handler.go` | Handler struct, NewHandler, RegisterRoutes |
| `internal/virtualoffice/templates.go` | Server-side template definitions for validation |
| `internal/virtualoffice/handler_config.go` | Config + seat admin endpoints |
| `internal/virtualoffice/handler_snapshot.go` | GET /snapshot with status derivation + Redis cache |
| `internal/virtualoffice/handler_employee.go` | PUT /my-status, PUT /my-avatar |
| `internal/virtualoffice/handler_test.go` | Unit tests for all handlers |
| `frontend/src/views/VirtualOfficeView.vue` | Main page: polling, layout, state |
| `frontend/src/components/virtual-office/OfficeCanvas.vue` | Pixi.js Application mount/unmount |
| `frontend/src/components/virtual-office/OfficeRenderer.ts` | Tilemap + furniture rendering |
| `frontend/src/components/virtual-office/SpriteManager.ts` | Character sprite CRUD + diff |
| `frontend/src/components/virtual-office/SeatInfoCard.vue` | Click popup: employee details |
| `frontend/src/components/virtual-office/StatusBar.vue` | Bottom bar: set status/emoji |
| `frontend/src/components/virtual-office/OfficeSetup.vue` | Admin: template + seat assignment |
| `frontend/src/components/virtual-office/OfficeStats.vue` | Top bar: online/leave/meeting counts |
| `frontend/src/components/virtual-office/MiniMap.vue` | Thumbnail for larger maps |
| `frontend/src/assets/virtual-office/templates/small.json` | Small office template (10 seats) |
| `frontend/src/assets/virtual-office/templates/medium.json` | Medium office template (30 seats) |
| `frontend/src/assets/virtual-office/templates/large.json` | Large office template (100 seats) |

### Modified Files

| File | Change |
|------|--------|
| `internal/app/bootstrap.go` | Create `virtualoffice.NewHandler`, call `RegisterRoutes` |
| `frontend/src/api/client.ts` | Add `virtualOfficeAPI` export |
| `frontend/src/router/index.ts` | Add `/virtual-office` route |
| `frontend/src/i18n/en.ts` | Add `nav.virtualOffice`, `virtualOffice.*` keys |
| `frontend/src/i18n/zh.ts` | Add Chinese translations |
| `frontend/src/components/DashboardLayout.vue` | Add sidebar item in Engagement group |
| `frontend/package.json` | Add `pixi.js` dependency |

---

### Task 1: Database Migration

**Files:**
- Create: `db/migrations/00084_virtual_office.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
CREATE TABLE virtual_office_config (
    company_id   BIGINT PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    template     TEXT NOT NULL DEFAULT 'small',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE virtual_office_seats (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    employee_id       BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    floor             INT NOT NULL DEFAULT 1,
    zone              TEXT NOT NULL DEFAULT 'desk-a',
    seat_x            INT NOT NULL,
    seat_y            INT NOT NULL,
    avatar_type       TEXT NOT NULL DEFAULT 'person_1',
    avatar_color      TEXT NOT NULL DEFAULT '#4A90D9',
    custom_status     TEXT,
    custom_emoji      TEXT,
    manual_status     TEXT,
    meeting_room_zone TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_vo_seats_company_employee ON virtual_office_seats(company_id, employee_id);
CREATE UNIQUE INDEX idx_vo_seats_company_position ON virtual_office_seats(company_id, floor, seat_x, seat_y);
CREATE INDEX idx_vo_seats_company ON virtual_office_seats(company_id);

-- +goose Down
DROP TABLE IF EXISTS virtual_office_seats;
DROP TABLE IF EXISTS virtual_office_config;
```

- [ ] **Step 2: Verify migration syntax**

Run: `cat db/migrations/00084_virtual_office.sql`
Expected: File exists with valid SQL.

- [ ] **Step 3: Commit**

```bash
git add db/migrations/00084_virtual_office.sql
git commit -m "feat(virtual-office): add migration for config and seats tables"
```

---

### Task 2: sqlc Queries

**Files:**
- Create: `db/query/virtual_office.sql`

- [ ] **Step 1: Write the query file**

```sql
-- name: UpsertVirtualOfficeConfig :one
INSERT INTO virtual_office_config (company_id, template)
VALUES ($1, $2)
ON CONFLICT (company_id)
DO UPDATE SET template = $2, updated_at = NOW()
RETURNING *;

-- name: GetVirtualOfficeConfig :one
SELECT * FROM virtual_office_config WHERE company_id = $1;

-- name: AssignSeat :one
INSERT INTO virtual_office_seats (company_id, employee_id, floor, zone, seat_x, seat_y)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: RemoveSeat :exec
DELETE FROM virtual_office_seats WHERE company_id = $1 AND employee_id = $2;

-- name: ListSeats :many
SELECT s.*, e.first_name, e.last_name,
       COALESCE(d.name, '') AS department_name
FROM virtual_office_seats s
JOIN employees e ON e.id = s.employee_id
LEFT JOIN departments d ON d.id = e.department_id
WHERE s.company_id = $1
ORDER BY s.floor, s.zone, s.seat_y, s.seat_x;

-- name: GetSeatByEmployee :one
SELECT * FROM virtual_office_seats WHERE company_id = $1 AND employee_id = $2;

-- name: UpdateSeatStatus :exec
UPDATE virtual_office_seats SET
    custom_status = $3,
    custom_emoji = $4,
    manual_status = $5,
    meeting_room_zone = $6,
    updated_at = NOW()
WHERE company_id = $1 AND employee_id = $2;

-- name: UpdateSeatAvatar :exec
UPDATE virtual_office_seats SET
    avatar_type = $3,
    avatar_color = $4,
    updated_at = NOW()
WHERE company_id = $1 AND employee_id = $2;

-- name: GetSnapshotSeats :many
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

-- name: ListUnassignedActiveEmployees :many
SELECT e.id, e.first_name, e.last_name, e.department_id
FROM employees e
WHERE e.company_id = $1
  AND e.status IN ('active', 'probationary')
  AND NOT EXISTS (
    SELECT 1 FROM virtual_office_seats s
    WHERE s.company_id = e.company_id AND s.employee_id = e.id
  )
ORDER BY e.department_id, e.last_name, e.first_name;

-- name: ListOccupiedPositions :many
SELECT floor, seat_x, seat_y FROM virtual_office_seats WHERE company_id = $1;
```

- [ ] **Step 2: Generate sqlc code**

Run: `~/go/bin/sqlc generate`
Expected: No errors. New files appear in `internal/store/` for virtual office queries.

- [ ] **Step 3: Verify generated code compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./internal/store/...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add db/query/virtual_office.sql internal/store/
git commit -m "feat(virtual-office): add sqlc queries for config, seats, and snapshot"
```

---

### Task 3: Server-Side Templates

**Files:**
- Create: `internal/virtualoffice/templates.go`

- [ ] **Step 1: Write the template definitions**

```go
package virtualoffice

// TemplateZone defines a zone within an office template.
type TemplateZone struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"` // desk_area, meeting_room, cafe, lounge, phone_booth
	Label    string      `json:"label"`
	X, Y     int         `json:"x"`
	W, H     int         `json:"w"`
	Capacity int         `json:"capacity,omitempty"`
	Seats    []SeatPos   `json:"seats,omitempty"`
}

// SeatPos is a grid coordinate for a seat.
type SeatPos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// TemplateInfo defines an office layout template.
type TemplateInfo struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	MaxSeats int            `json:"max_seats"`
	Zones    []TemplateZone `json:"zones"`
}

// Templates is the registry of all available templates.
var Templates = map[string]TemplateInfo{
	"small": {
		ID: "small", Name: "Small Office", Width: 20, Height: 16, MaxSeats: 10,
		Zones: []TemplateZone{
			{
				ID: "desk-a", Type: "desk_area", Label: "Workstations",
				X: 1, Y: 1, W: 12, H: 6,
				Seats: []SeatPos{
					{2, 2}, {5, 2}, {8, 2}, {11, 2},
					{2, 5}, {5, 5}, {8, 5}, {11, 5},
					{2, 3}, {5, 3},
				},
			},
			{
				ID: "meeting-a", Type: "meeting_room", Label: "Meeting Room",
				X: 1, Y: 9, W: 6, H: 5, Capacity: 6,
				Seats: []SeatPos{{2, 10}, {4, 10}, {2, 12}, {4, 12}},
			},
			{
				ID: "cafe", Type: "cafe", Label: "Tea Room",
				X: 9, Y: 9, W: 5, H: 5,
			},
		},
	},
	"medium": {
		ID: "medium", Name: "Medium Office", Width: 32, Height: 24, MaxSeats: 30,
		Zones: []TemplateZone{
			{
				ID: "desk-a", Type: "desk_area", Label: "Workstations A",
				X: 1, Y: 1, W: 14, H: 10,
				Seats: func() []SeatPos {
					var seats []SeatPos
					for y := 2; y <= 8; y += 3 {
						for x := 2; x <= 12; x += 3 {
							seats = append(seats, SeatPos{x, y})
						}
					}
					return seats // 15 seats
				}(),
			},
			{
				ID: "desk-b", Type: "desk_area", Label: "Workstations B",
				X: 17, Y: 1, W: 14, H: 10,
				Seats: func() []SeatPos {
					var seats []SeatPos
					for y := 2; y <= 8; y += 3 {
						for x := 18; x <= 28; x += 3 {
							seats = append(seats, SeatPos{x, y})
						}
					}
					return seats // 15 seats
				}(),
			},
			{
				ID: "meeting-a", Type: "meeting_room", Label: "Meeting Room A",
				X: 1, Y: 13, W: 8, H: 6, Capacity: 8,
				Seats: []SeatPos{{2, 14}, {4, 14}, {6, 14}, {2, 16}, {4, 16}, {6, 16}},
			},
			{
				ID: "meeting-b", Type: "meeting_room", Label: "Meeting Room B",
				X: 11, Y: 13, W: 8, H: 6, Capacity: 8,
				Seats: []SeatPos{{12, 14}, {14, 14}, {16, 14}, {12, 16}, {14, 16}, {16, 16}},
			},
			{
				ID: "cafe", Type: "cafe", Label: "Tea Room",
				X: 21, Y: 13, W: 10, H: 6,
			},
			{
				ID: "lounge", Type: "lounge", Label: "Lounge",
				X: 1, Y: 20, W: 30, H: 3,
			},
		},
	},
	"large": {
		ID: "large", Name: "Large Office", Width: 48, Height: 36, MaxSeats: 100,
		Zones: func() []TemplateZone {
			zones := []TemplateZone{}
			// 4 desk areas with 25 seats each
			deskConfigs := []struct{ id, label string; x, y int }{
				{"desk-a", "Workstations A", 1, 1},
				{"desk-b", "Workstations B", 25, 1},
				{"desk-c", "Workstations C", 1, 13},
				{"desk-d", "Workstations D", 25, 13},
			}
			for _, dc := range deskConfigs {
				var seats []SeatPos
				for dy := 0; dy < 5; dy++ {
					for dx := 0; dx < 5; dx++ {
						seats = append(seats, SeatPos{dc.x + 1 + dx*4, dc.y + 1 + dy*2})
					}
				}
				zones = append(zones, TemplateZone{
					ID: dc.id, Type: "desk_area", Label: dc.label,
					X: dc.x, Y: dc.y, W: 22, H: 10,
					Seats: seats,
				})
			}
			// 3 meeting rooms
			zones = append(zones, TemplateZone{
				ID: "meeting-a", Type: "meeting_room", Label: "Meeting Room A",
				X: 1, Y: 25, W: 10, H: 6, Capacity: 10,
				Seats: []SeatPos{{2, 26}, {4, 26}, {6, 26}, {8, 26}, {2, 28}, {4, 28}, {6, 28}, {8, 28}},
			})
			zones = append(zones, TemplateZone{
				ID: "meeting-b", Type: "meeting_room", Label: "Meeting Room B",
				X: 13, Y: 25, W: 10, H: 6, Capacity: 10,
				Seats: []SeatPos{{14, 26}, {16, 26}, {18, 26}, {20, 26}, {14, 28}, {16, 28}, {18, 28}, {20, 28}},
			})
			zones = append(zones, TemplateZone{
				ID: "meeting-c", Type: "meeting_room", Label: "Meeting Room C",
				X: 25, Y: 25, W: 10, H: 6, Capacity: 10,
				Seats: []SeatPos{{26, 26}, {28, 26}, {30, 26}, {32, 26}, {26, 28}, {28, 28}, {30, 28}, {32, 28}},
			})
			// Other zones
			zones = append(zones, TemplateZone{
				ID: "cafe", Type: "cafe", Label: "Cafeteria",
				X: 37, Y: 25, W: 10, H: 6,
			})
			zones = append(zones, TemplateZone{
				ID: "lounge", Type: "lounge", Label: "Lounge",
				X: 1, Y: 33, W: 46, H: 3,
			})
			zones = append(zones, TemplateZone{
				ID: "phone-a", Type: "phone_booth", Label: "Phone Booth A",
				X: 37, Y: 33, W: 3, H: 3,
			})
			zones = append(zones, TemplateZone{
				ID: "phone-b", Type: "phone_booth", Label: "Phone Booth B",
				X: 42, Y: 33, W: 3, H: 3,
			})
			return zones
		}(),
	},
}

// ValidTemplate returns true if the template ID is valid.
func ValidTemplate(id string) bool {
	_, ok := Templates[id]
	return ok
}

// ValidTemplateNames returns the list of valid template IDs.
func ValidTemplateNames() []string {
	return []string{"small", "medium", "large"}
}

// GetMeetingRoomZones returns all meeting room zone IDs for a template.
func GetMeetingRoomZones(templateID string) []string {
	t, ok := Templates[templateID]
	if !ok {
		return nil
	}
	var zones []string
	for _, z := range t.Zones {
		if z.Type == "meeting_room" {
			zones = append(zones, z.ID)
		}
	}
	return zones
}

// IsValidMeetingRoom checks if a zone ID is a meeting room in the given template.
func IsValidMeetingRoom(templateID, zoneID string) bool {
	for _, id := range GetMeetingRoomZones(templateID) {
		if id == zoneID {
			return true
		}
	}
	return false
}

// InBounds checks if coordinates are within the template grid.
func InBounds(templateID string, x, y int) bool {
	t, ok := Templates[templateID]
	if !ok {
		return false
	}
	return x >= 0 && x < t.Width && y >= 0 && y < t.Height
}

// GetDeskSeats returns all desk seat positions for a template (for auto-assign).
func GetDeskSeats(templateID string) []struct{ Zone string; Seats []SeatPos } {
	t, ok := Templates[templateID]
	if !ok {
		return nil
	}
	var result []struct{ Zone string; Seats []SeatPos }
	for _, z := range t.Zones {
		if z.Type == "desk_area" && len(z.Seats) > 0 {
			result = append(result, struct{ Zone string; Seats []SeatPos }{z.ID, z.Seats})
		}
	}
	return result
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./internal/virtualoffice/...`
Expected: May fail because handler.go doesn't exist yet. That's fine — just check for syntax errors.
Run: `cd /Users/anna/Documents/aigonhr && go vet ./internal/virtualoffice/templates.go` (if single-file check possible)
Alternatively, just visually verify no syntax errors.

- [ ] **Step 3: Commit**

```bash
git add internal/virtualoffice/templates.go
git commit -m "feat(virtual-office): add server-side template definitions"
```

---

### Task 4: Handler Scaffold + Routes

**Files:**
- Create: `internal/virtualoffice/handler.go`
- Modify: `internal/app/bootstrap.go`

- [ ] **Step 1: Write the handler scaffold**

```go
package virtualoffice

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler handles virtual office endpoints.
type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	rdb     *redis.Client
}

// NewHandler creates a new virtual office handler.
func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, rdb *redis.Client) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, rdb: rdb}
}

// RegisterRoutes registers virtual office routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	vo := protected.Group("/virtual-office")

	// Admin endpoints
	vo.GET("/config", auth.ManagerOrAbove(), h.GetConfig)
	vo.PUT("/config", auth.AdminOnly(), h.UpdateConfig)
	vo.GET("/seats", auth.ManagerOrAbove(), h.ListSeats)
	vo.POST("/seats/assign", auth.AdminOnly(), h.AssignSeat)
	vo.POST("/seats/auto", auth.AdminOnly(), h.AutoAssign)
	vo.DELETE("/seats/:employee_id", auth.AdminOnly(), h.RemoveSeat)

	// Employee endpoints
	vo.GET("/snapshot", h.GetSnapshot)
	vo.PUT("/my-status", h.UpdateMyStatus)
	vo.PUT("/my-avatar", h.UpdateMyAvatar)
}
```

- [ ] **Step 2: Wire handler in bootstrap.go**

In `internal/app/bootstrap.go`, add after the last handler creation (around line 279, after `workflowHandler`):

Import: `"github.com/tonypk/aigonhr/internal/virtualoffice"`

Handler creation:
```go
virtualOfficeHandler := virtualoffice.NewHandler(a.Queries, a.Pool, a.Logger, a.Redis)
```

Route registration (around line 385, after `hrrequestHandler.RegisterRoutes`):
```go
virtualOfficeHandler.RegisterRoutes(protected)
```

- [ ] **Step 3: Create stub handler files so the project compiles**

Create `internal/virtualoffice/handler_config.go`:
```go
package virtualoffice

import "github.com/gin-gonic/gin"

func (h *Handler) GetConfig(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) UpdateConfig(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) ListSeats(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) AssignSeat(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) AutoAssign(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) RemoveSeat(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
```

Create `internal/virtualoffice/handler_snapshot.go`:
```go
package virtualoffice

import "github.com/gin-gonic/gin"

func (h *Handler) GetSnapshot(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
```

Create `internal/virtualoffice/handler_employee.go`:
```go
package virtualoffice

import "github.com/gin-gonic/gin"

func (h *Handler) UpdateMyStatus(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) UpdateMyAvatar(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
```

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/virtualoffice/handler.go internal/virtualoffice/handler_config.go internal/virtualoffice/handler_snapshot.go internal/virtualoffice/handler_employee.go internal/app/bootstrap.go
git commit -m "feat(virtual-office): add handler scaffold with routes and bootstrap wiring"
```

---

### Task 5: Config + Seat Admin Endpoints

**Files:**
- Modify: `internal/virtualoffice/handler_config.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/virtualoffice/handler_test.go`:

```go
package virtualoffice

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

var empAuth = testutil.AuthContext{
	UserID: 10, Email: "emp@test.com", Role: auth.RoleEmployee, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger, nil)
}

// --- Config Tests ---

func TestUpdateConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// UpsertVirtualOfficeConfig returns one row
	mockDB.OnQueryRow(testutil.NewMockRow(int64(1), "medium", nil, nil))
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{"template": "medium"}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfig_InvalidTemplate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{"template": "huge"}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfig_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/config", nil, adminAuth)
	h.GetConfig(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v -run "TestUpdateConfig|TestGetConfig"`
Expected: FAIL (stubs return 501)

- [ ] **Step 3: Implement config endpoints**

Replace `internal/virtualoffice/handler_config.go`:

```go
package virtualoffice

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// GetConfig returns the company's virtual office configuration.
func (h *Handler) GetConfig(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	cfg, err := h.queries.GetVirtualOfficeConfig(c.Request.Context(), companyID)
	if err != nil {
		response.NotFound(c, "Virtual office not configured")
		return
	}
	response.OK(c, cfg)
}

// UpdateConfig creates or updates the virtual office configuration.
func (h *Handler) UpdateConfig(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var req struct {
		Template string `json:"template" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if !ValidTemplate(req.Template) {
		response.BadRequest(c, fmt.Sprintf("Invalid template. Must be one of: %v", ValidTemplateNames()))
		return
	}

	cfg, err := h.queries.UpsertVirtualOfficeConfig(c.Request.Context(), store.UpsertVirtualOfficeConfigParams{
		CompanyID: companyID,
		Template:  req.Template,
	})
	if err != nil {
		h.logger.Error("failed to upsert virtual office config", "error", err)
		response.InternalError(c, "Failed to save configuration")
		return
	}
	response.OK(c, cfg)
}

// ListSeats returns all seat assignments for the company.
func (h *Handler) ListSeats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	seats, err := h.queries.ListSeats(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("failed to list seats", "error", err)
		response.InternalError(c, "Failed to list seats")
		return
	}
	response.OK(c, seats)
}

// AssignSeat assigns an employee to a specific seat.
func (h *Handler) AssignSeat(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var req struct {
		EmployeeID int64  `json:"employee_id" binding:"required"`
		Floor      int32  `json:"floor"`
		Zone       string `json:"zone" binding:"required"`
		SeatX      int32  `json:"seat_x" binding:"required"`
		SeatY      int32  `json:"seat_y" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Floor == 0 {
		req.Floor = 1
	}

	ctx := c.Request.Context()

	// Validate employee is active
	emp, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
		ID: req.EmployeeID, CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}
	if emp.Status != "active" && emp.Status != "probationary" {
		response.BadRequest(c, "Only active or probationary employees can be assigned seats")
		return
	}

	// Validate coordinates within template bounds
	cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
	if err != nil {
		response.BadRequest(c, "Virtual office not configured. Set up a template first.")
		return
	}
	if !InBounds(cfg.Template, int(req.SeatX), int(req.SeatY)) {
		response.BadRequest(c, "Seat coordinates out of template bounds")
		return
	}

	seat, err := h.queries.AssignSeat(ctx, store.AssignSeatParams{
		CompanyID:  companyID,
		EmployeeID: req.EmployeeID,
		Floor:      req.Floor,
		Zone:       req.Zone,
		SeatX:      req.SeatX,
		SeatY:      req.SeatY,
	})
	if err != nil {
		// Check for unique constraint violations
		h.logger.Error("failed to assign seat", "error", err)
		response.BadRequest(c, "Seat already occupied or employee already assigned")
		return
	}

	h.invalidateSnapshot(c.Request.Context(), companyID)
	response.Created(c, seat)
}

// RemoveSeat removes an employee's seat assignment.
func (h *Handler) RemoveSeat(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	employeeID, err := strconv.ParseInt(c.Param("employee_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	if err := h.queries.RemoveSeat(c.Request.Context(), store.RemoveSeatParams{
		CompanyID: companyID, EmployeeID: employeeID,
	}); err != nil {
		h.logger.Error("failed to remove seat", "error", err)
		response.InternalError(c, "Failed to remove seat")
		return
	}

	h.invalidateSnapshot(c.Request.Context(), companyID)
	response.OK(c, gin.H{"message": "Seat removed"})
}

// AutoAssign assigns all unassigned active employees to empty desk seats.
func (h *Handler) AutoAssign(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
	if err != nil {
		response.BadRequest(c, "Virtual office not configured")
		return
	}

	// Get unassigned employees
	unassigned, err := h.queries.ListUnassignedActiveEmployees(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to list unassigned employees", "error", err)
		response.InternalError(c, "Failed to list employees")
		return
	}
	if len(unassigned) == 0 {
		response.OK(c, gin.H{"assigned": 0, "skipped": 0, "no_seats": 0})
		return
	}

	// Get occupied positions
	occupied, err := h.queries.ListOccupiedPositions(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to list occupied positions", "error", err)
		response.InternalError(c, "Failed to list seats")
		return
	}

	// Build set of occupied coordinates
	type pos struct{ x, y int }
	occupiedSet := make(map[pos]bool)
	for _, o := range occupied {
		occupiedSet[pos{int(o.SeatX), int(o.SeatY)}] = true
	}

	// Collect all available desk seats from template
	deskGroups := GetDeskSeats(cfg.Template)
	var availableSeats []struct {
		Zone string
		X, Y int
	}
	for _, group := range deskGroups {
		for _, s := range group.Seats {
			if !occupiedSet[pos{s.X, s.Y}] {
				availableSeats = append(availableSeats, struct {
					Zone string
					X, Y int
				}{group.Zone, s.X, s.Y})
			}
		}
	}

	assigned := 0
	noSeats := 0
	for i, emp := range unassigned {
		if i >= len(availableSeats) {
			noSeats = len(unassigned) - i
			break
		}
		seat := availableSeats[i]
		_, err := h.queries.AssignSeat(ctx, store.AssignSeatParams{
			CompanyID:  companyID,
			EmployeeID: emp.ID,
			Floor:      1,
			Zone:       seat.Zone,
			SeatX:      int32(seat.X),
			SeatY:      int32(seat.Y),
		})
		if err != nil {
			h.logger.Error("auto-assign failed for employee", "employee_id", emp.ID, "error", err)
			continue
		}
		assigned++
	}

	h.invalidateSnapshot(ctx, companyID)
	response.OK(c, gin.H{
		"assigned": assigned,
		"skipped":  0,
		"no_seats": noSeats,
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v -run "TestUpdateConfig|TestGetConfig"`
Expected: PASS

Note: Uses `pgx.ErrNoRows` from `"github.com/jackc/pgx/v5"` for not-found simulation.

- [ ] **Step 5: Commit**

```bash
git add internal/virtualoffice/handler_config.go internal/virtualoffice/handler_test.go
git commit -m "feat(virtual-office): implement config and seat admin endpoints"
```

---

### Task 6: Snapshot Endpoint with Status Derivation

**Files:**
- Modify: `internal/virtualoffice/handler_snapshot.go`

- [ ] **Step 1: Write failing test for snapshot**

Add to `internal/virtualoffice/handler_test.go`:

```go
func TestGetSnapshot_NotConfigured(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetVirtualOfficeConfig returns not found
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/snapshot", nil, empAuth)
	h.GetSnapshot(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v -run TestGetSnapshot`
Expected: FAIL (returns 501)

- [ ] **Step 3: Implement snapshot endpoint**

Replace `internal/virtualoffice/handler_snapshot.go`:

```go
package virtualoffice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// snapshotCacheKey returns the Redis key for a company's snapshot cache.
func snapshotCacheKey(companyID int64) string {
	return fmt.Sprintf("vo:snapshot:%d", companyID)
}

// invalidateSnapshot deletes the cached snapshot. Best-effort, never errors.
func (h *Handler) invalidateSnapshot(ctx context.Context, companyID int64) {
	if h.rdb != nil {
		_ = h.rdb.Del(ctx, snapshotCacheKey(companyID)).Err()
	}
}

// SeatStatus represents a derived employee status.
type SeatStatus struct {
	SeatID          int64   `json:"seat_id"`
	EmployeeID      int64   `json:"employee_id"`
	Name            string  `json:"name"`
	Position        string  `json:"position"`
	Department      string  `json:"department"`
	Floor           int32   `json:"floor"`
	Zone            string  `json:"zone"`
	SeatX           int32   `json:"seat_x"`
	SeatY           int32   `json:"seat_y"`
	AvatarType      string  `json:"avatar_type"`
	AvatarColor     string  `json:"avatar_color"`
	Status          string  `json:"status"`
	IsLate          bool    `json:"is_late"`
	CustomStatus    *string `json:"custom_status"`
	CustomEmoji     *string `json:"custom_emoji"`
	ClockInAt       *string `json:"clock_in_at"`
	LeaveType       *string `json:"leave_type"`
	MeetingRoomZone *string `json:"meeting_room_zone"`
}

// MeetingRoom represents an occupied meeting room.
type MeetingRoom struct {
	ZoneID      string  `json:"zone_id"`
	Label       string  `json:"label"`
	OccupantIDs []int64 `json:"occupant_ids"`
}

// SnapshotResponse is the full office state.
type SnapshotResponse struct {
	Template     string            `json:"template"`
	Stats        map[string]int    `json:"stats"`
	Seats        []SeatStatus      `json:"seats"`
	MeetingRooms []MeetingRoom     `json:"meeting_rooms"`
}

// GetSnapshot returns the full office state with derived statuses.
func (h *Handler) GetSnapshot(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	// Try Redis cache first
	if h.rdb != nil {
		cached, err := h.rdb.Get(ctx, snapshotCacheKey(companyID)).Bytes()
		if err == nil {
			var snap SnapshotResponse
			if json.Unmarshal(cached, &snap) == nil {
				response.OK(c, snap)
				return
			}
		}
	}

	// Get config
	cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
	if err != nil {
		response.NotFound(c, "Virtual office not configured")
		return
	}

	// Get raw seat data with attendance/leave info
	rows, err := h.queries.GetSnapshotSeats(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to get snapshot", "error", err)
		response.InternalError(c, "Failed to load office data")
		return
	}

	// Derive status for each seat
	seats := make([]SeatStatus, 0, len(rows))
	stats := map[string]int{
		"total_assigned": len(rows),
		"online":         0,
		"on_leave":       0,
		"in_meeting":     0,
		"offline":        0,
	}

	// Track meeting room occupants
	meetingOccupants := map[string][]int64{}

	for _, row := range rows {
		ss := SeatStatus{
			SeatID:      row.SeatID,
			EmployeeID:  row.EmployeeID,
			Name:        row.Name,
			Position:    row.Position,
			Department:  row.Department,
			Floor:       row.Floor,
			Zone:        row.Zone,
			SeatX:       row.SeatX,
			SeatY:       row.SeatY,
			AvatarType:  row.AvatarType,
			AvatarColor: row.AvatarColor,
			CustomStatus: row.CustomStatus,
			CustomEmoji: row.CustomEmoji,
		}

		// Status derivation priority
		switch {
		case row.LeaveType != nil:
			ss.Status = "on_leave"
			ss.LeaveType = row.LeaveType
			stats["on_leave"]++

		case row.ManualStatus != nil && *row.ManualStatus != "":
			ss.Status = *row.ManualStatus
			if *row.ManualStatus == "in_meeting" {
				ss.MeetingRoomZone = row.MeetingRoomZone
				stats["in_meeting"]++
				if row.MeetingRoomZone != nil {
					meetingOccupants[*row.MeetingRoomZone] = append(
						meetingOccupants[*row.MeetingRoomZone], row.EmployeeID)
				}
			} else {
				stats["online"]++
			}

		case row.ClockInAt != nil && row.ClockOutAt == nil:
			ss.Status = "working"
			ss.IsLate = row.LateMinutes > 0
			clockIn := row.ClockInAt.Format(time.RFC3339)
			ss.ClockInAt = &clockIn
			stats["online"]++

		default:
			ss.Status = "offline"
			stats["offline"]++
		}

		seats = append(seats, ss)
	}

	// Build meeting rooms from template + occupants
	meetingRooms := make([]MeetingRoom, 0)
	tmpl, ok := Templates[cfg.Template]
	if ok {
		for _, zone := range tmpl.Zones {
			if zone.Type == "meeting_room" {
				ids := meetingOccupants[zone.ID]
				if ids == nil {
					ids = []int64{}
				}
				meetingRooms = append(meetingRooms, MeetingRoom{
					ZoneID:      zone.ID,
					Label:       zone.Label,
					OccupantIDs: ids,
				})
			}
		}
	}

	snap := SnapshotResponse{
		Template:     cfg.Template,
		Stats:        stats,
		Seats:        seats,
		MeetingRooms: meetingRooms,
	}

	// Cache in Redis for 30 seconds
	if h.rdb != nil {
		data, err := json.Marshal(snap)
		if err == nil {
			_ = h.rdb.Set(ctx, snapshotCacheKey(companyID), data, 30*time.Second).Err()
		}
	}

	response.OK(c, snap)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/virtualoffice/handler_snapshot.go internal/virtualoffice/handler_test.go
git commit -m "feat(virtual-office): implement snapshot endpoint with status derivation and Redis cache"
```

---

### Task 7: Employee Status + Avatar Endpoints

**Files:**
- Modify: `internal/virtualoffice/handler_employee.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/virtualoffice/handler_test.go`:

```go
func TestUpdateMyStatus_NoSeat(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetEmployeeByUserID returns employee
	mockDB.OnQueryRow(testutil.NewMockRow(int64(42), int64(1), "EMP001", "John", "Doe", nil, nil, nil, "john@test.com", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "active", nil, nil))
	// GetSeatByEmployee returns not found
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-status",
		gin.H{"custom_status": "Working on Q1"}, empAuth)
	h.UpdateMyStatus(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyAvatar_InvalidType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-avatar",
		gin.H{"avatar_type": "unicorn", "avatar_color": "#FF0000"}, empAuth)
	h.UpdateMyAvatar(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v -run "TestUpdateMy"`
Expected: FAIL

- [ ] **Step 3: Implement employee endpoints**

Replace `internal/virtualoffice/handler_employee.go`:

```go
package virtualoffice

import (
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

var validAvatarTypes = map[string]bool{
	"person_1": true, "person_2": true, "person_3": true,
	"person_4": true, "person_5": true, "person_6": true,
	"cat": true, "dog": true, "rabbit": true,
	"bear": true, "penguin": true, "shiba": true,
}

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// UpdateMyStatus updates the calling user's status, emoji, and manual status.
func (h *Handler) UpdateMyStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	ctx := c.Request.Context()

	var req struct {
		ManualStatus    *string `json:"manual_status"`
		MeetingRoomZone *string `json:"meeting_room_zone"`
		CustomStatus    *string `json:"custom_status"`
		CustomEmoji     *string `json:"custom_emoji"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Resolve employee
	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID: &userID, CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}

	// Verify seat exists
	seat, err := h.queries.GetSeatByEmployee(ctx, store.GetSeatByEmployeeParams{
		CompanyID: companyID, EmployeeID: emp.ID,
	})
	if err != nil {
		response.NotFound(c, "No seat assigned. Ask your admin to assign you a seat.")
		return
	}

	// Validate meeting room zone if setting in_meeting
	manualStatus := seat.ManualStatus
	meetingZone := seat.MeetingRoomZone
	if req.ManualStatus != nil {
		if *req.ManualStatus == "" {
			manualStatus = nil
			meetingZone = nil
		} else {
			manualStatus = req.ManualStatus
			if *req.ManualStatus == "in_meeting" {
				if req.MeetingRoomZone == nil || *req.MeetingRoomZone == "" {
					response.BadRequest(c, "meeting_room_zone is required when status is in_meeting")
					return
				}
				// Validate zone against template
				cfg, cfgErr := h.queries.GetVirtualOfficeConfig(ctx, companyID)
				if cfgErr == nil && !IsValidMeetingRoom(cfg.Template, *req.MeetingRoomZone) {
					response.BadRequest(c, "Invalid meeting room zone")
					return
				}
				meetingZone = req.MeetingRoomZone
			} else {
				meetingZone = nil
			}
		}
	}

	customStatus := seat.CustomStatus
	if req.CustomStatus != nil {
		if *req.CustomStatus == "" {
			customStatus = nil
		} else {
			s := *req.CustomStatus
			if len(s) > 50 {
				s = s[:50]
			}
			customStatus = &s
		}
	}

	customEmoji := seat.CustomEmoji
	if req.CustomEmoji != nil {
		if *req.CustomEmoji == "" {
			customEmoji = nil
		} else {
			customEmoji = req.CustomEmoji
		}
	}

	if err := h.queries.UpdateSeatStatus(ctx, store.UpdateSeatStatusParams{
		CompanyID:       companyID,
		EmployeeID:      emp.ID,
		CustomStatus:    customStatus,
		CustomEmoji:     customEmoji,
		ManualStatus:    manualStatus,
		MeetingRoomZone: meetingZone,
	}); err != nil {
		h.logger.Error("failed to update status", "error", err)
		response.InternalError(c, "Failed to update status")
		return
	}

	h.invalidateSnapshot(ctx, companyID)
	response.OK(c, gin.H{"message": "Status updated"})
}

// UpdateMyAvatar updates the calling user's avatar type and color.
func (h *Handler) UpdateMyAvatar(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	ctx := c.Request.Context()

	var req struct {
		AvatarType  string `json:"avatar_type" binding:"required"`
		AvatarColor string `json:"avatar_color" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if !validAvatarTypes[req.AvatarType] {
		response.BadRequest(c, "Invalid avatar type")
		return
	}
	if !hexColorRe.MatchString(req.AvatarColor) {
		response.BadRequest(c, "Invalid color. Must be hex format like #4A90D9")
		return
	}

	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID: &userID, CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}

	if err := h.queries.UpdateSeatAvatar(ctx, store.UpdateSeatAvatarParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		AvatarType: req.AvatarType,
		AvatarColor: req.AvatarColor,
	}); err != nil {
		h.logger.Error("failed to update avatar", "error", err)
		response.InternalError(c, "Failed to update avatar")
		return
	}

	h.invalidateSnapshot(ctx, companyID)
	response.OK(c, gin.H{"message": "Avatar updated"})
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v`
Expected: All tests PASS

- [ ] **Step 5: Verify full project compiles**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/virtualoffice/handler_employee.go internal/virtualoffice/handler_test.go
git commit -m "feat(virtual-office): implement employee status and avatar endpoints"
```

---

### Task 8: Frontend API Client + Router + Sidebar + i18n

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/DashboardLayout.vue`
- Modify: `frontend/src/i18n/en.ts`
- Modify: `frontend/src/i18n/zh.ts`

- [ ] **Step 1: Add API client**

Add to `frontend/src/api/client.ts` after the `npsAPI` export:

```typescript
// Virtual Office
export const virtualOfficeAPI = {
  getConfig: () => get("/v1/virtual-office/config"),
  updateConfig: (data: { template: string }) => put("/v1/virtual-office/config", data),
  listSeats: () => get("/v1/virtual-office/seats"),
  assignSeat: (data: { employee_id: number; floor: number; zone: string; seat_x: number; seat_y: number }) =>
    post("/v1/virtual-office/seats/assign", data),
  autoAssign: () => post("/v1/virtual-office/seats/auto", {}),
  removeSeat: (employeeId: number) => del(`/v1/virtual-office/seats/${employeeId}`),
  getSnapshot: () => get("/v1/virtual-office/snapshot"),
  updateMyStatus: (data: { manual_status?: string | null; meeting_room_zone?: string | null; custom_status?: string | null; custom_emoji?: string | null }) =>
    put("/v1/virtual-office/my-status", data),
  updateMyAvatar: (data: { avatar_type: string; avatar_color: string }) =>
    put("/v1/virtual-office/my-avatar", data),
}
```

- [ ] **Step 2: Add router entry**

In `frontend/src/router/index.ts`, add inside the DashboardLayout children (after the pulse-surveys/respond route, before notifications):

```typescript
        // Virtual Office
        {
          path: "virtual-office",
          name: "virtual-office",
          component: () => import("../views/VirtualOfficeView.vue"),
        },
```

- [ ] **Step 3: Add sidebar menu item**

In `frontend/src/components/DashboardLayout.vue`:

1. Add `'virtual-office': true` to the `features` record.
2. In the "Engagement" group (section 6), add after the last `pushIf`:
```typescript
  pushIf(engagement, 'virtual-office', 'nav.virtualOffice', BusinessOutline)
```

- [ ] **Step 4: Add i18n keys**

In `frontend/src/i18n/en.ts`, add to `nav:` section:
```typescript
    virtualOffice: "Virtual Office",
```

Add a new `virtualOffice:` section after the `nav` section's closing brace (or at the end of the file before the final closing):
```typescript
  virtualOffice: {
    title: "Virtual Office",
    setup: "Set Up Virtual Office",
    chooseTemplate: "Choose a template",
    small: "Small Office (up to 10)",
    medium: "Medium Office (up to 30)",
    large: "Large Office (up to 100)",
    saveConfig: "Save & Continue",
    assignSeats: "Assign Seats",
    autoAssign: "Auto-Assign by Department",
    autoAssignSuccess: "{count} employees assigned",
    removeSeat: "Remove",
    seatAssigned: "Seat assigned",
    seatRemoved: "Seat removed",
    online: "Online",
    onLeave: "On Leave",
    inMeeting: "In Meeting",
    offline: "Offline",
    setStatus: "Set Status",
    statusPlaceholder: "What are you working on?",
    chooseAvatar: "Choose Avatar",
    chooseColor: "Accent Color",
    avatarSaved: "Avatar updated",
    statusSaved: "Status updated",
    clickToView: "Click to view details",
    noOffice: "No virtual office configured yet.",
    setupFirst: "Set up your virtual office to visualize your team.",
    meetingRoom: "Meeting Room",
    people: "People",
    animals: "Animals",
    working: "Working",
    overtime: "Overtime",
    focused: "Focused",
    inMeetingStatus: "In Meeting",
    onBreak: "On Break",
    away: "Away",
    clearStatus: "Clear Status",
    clockedIn: "Clocked in",
    leaveType: "Leave type",
    noSeat: "No seat assigned",
  },
```

In `frontend/src/i18n/zh.ts`, add matching Chinese translations:
```typescript
    virtualOffice: "虚拟办公室",
```
And the `virtualOffice` section:
```typescript
  virtualOffice: {
    title: "虚拟办公室",
    setup: "设置虚拟办公室",
    chooseTemplate: "选择模板",
    small: "小型办公室（最多10人）",
    medium: "中型办公室（最多30人）",
    large: "大型办公室（最多100人）",
    saveConfig: "保存并继续",
    assignSeats: "分配座位",
    autoAssign: "按部门自动分配",
    autoAssignSuccess: "已分配 {count} 位员工",
    removeSeat: "移除",
    seatAssigned: "座位已分配",
    seatRemoved: "座位已移除",
    online: "在线",
    onLeave: "请假中",
    inMeeting: "会议中",
    offline: "离线",
    setStatus: "设置状态",
    statusPlaceholder: "你正在做什么？",
    chooseAvatar: "选择头像",
    chooseColor: "主题色",
    avatarSaved: "头像已更新",
    statusSaved: "状态已更新",
    clickToView: "点击查看详情",
    noOffice: "尚未配置虚拟办公室。",
    setupFirst: "设置你的虚拟办公室来可视化你的团队。",
    meetingRoom: "会议室",
    people: "人物",
    animals: "动物",
    working: "工作中",
    overtime: "加班中",
    focused: "专注中",
    inMeetingStatus: "会议中",
    onBreak: "休息中",
    away: "离开",
    clearStatus: "清除状态",
    clockedIn: "签到时间",
    leaveType: "假期类型",
    noSeat: "未分配座位",
  },
```

- [ ] **Step 5: Install Pixi.js**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm install pixi.js@^8`

- [ ] **Step 6: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: Build succeeds (VirtualOfficeView.vue doesn't exist yet, but lazy-loaded routes won't fail the build)

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/router/index.ts frontend/src/components/DashboardLayout.vue frontend/src/i18n/en.ts frontend/src/i18n/zh.ts frontend/package.json frontend/package-lock.json
git commit -m "feat(virtual-office): add frontend API client, router, sidebar, and i18n"
```

---

### Task 9: Office Templates (JSON)

**Files:**
- Create: `frontend/src/assets/virtual-office/templates/small.json`
- Create: `frontend/src/assets/virtual-office/templates/medium.json`
- Create: `frontend/src/assets/virtual-office/templates/large.json`

- [ ] **Step 1: Create small template**

`frontend/src/assets/virtual-office/templates/small.json`:
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
        { "x": 2, "y": 2 }, { "x": 5, "y": 2 }, { "x": 8, "y": 2 }, { "x": 11, "y": 2 },
        { "x": 2, "y": 5 }, { "x": 5, "y": 5 }, { "x": 8, "y": 5 }, { "x": 11, "y": 5 },
        { "x": 2, "y": 3 }, { "x": 5, "y": 3 }
      ]
    },
    {
      "id": "meeting-a",
      "type": "meeting_room",
      "label": "Meeting Room",
      "x": 1, "y": 9, "w": 6, "h": 5,
      "capacity": 6,
      "meeting_seats": [
        { "x": 2, "y": 10 }, { "x": 4, "y": 10 },
        { "x": 2, "y": 12 }, { "x": 4, "y": 12 }
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
    { "type": "desk", "x": 2, "y": 2 }, { "type": "desk", "x": 5, "y": 2 },
    { "type": "desk", "x": 8, "y": 2 }, { "type": "desk", "x": 11, "y": 2 },
    { "type": "desk", "x": 2, "y": 5 }, { "type": "desk", "x": 5, "y": 5 },
    { "type": "desk", "x": 8, "y": 5 }, { "type": "desk", "x": 11, "y": 5 },
    { "type": "plant", "x": 0, "y": 0 }, { "type": "plant", "x": 19, "y": 0 },
    { "type": "whiteboard", "x": 2, "y": 9 },
    { "type": "coffee_machine", "x": 10, "y": 10 }
  ]
}
```

- [ ] **Step 2: Create medium and large templates**

Create similarly structured JSON for `medium.json` (32×24, 30 seats, 2 desk areas + 2 meeting rooms + cafe + lounge) and `large.json` (48×36, 100 seats, 4 desk areas + 3 meeting rooms + cafe + lounge + phone booths). Follow the same zone/seat structure.

**CRITICAL:** The seat coordinates in the JSON files MUST exactly match the Go template definitions in `internal/virtualoffice/templates.go`. Derive the JSON zone/seat positions from the Go code — do not create independently.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/assets/virtual-office/templates/
git commit -m "feat(virtual-office): add office template JSON files"
```

---

### Task 10: Pixi.js Rendering Engine (OfficeCanvas + OfficeRenderer + SpriteManager)

**Files:**
- Create: `frontend/src/components/virtual-office/OfficeCanvas.vue`
- Create: `frontend/src/components/virtual-office/OfficeRenderer.ts`
- Create: `frontend/src/components/virtual-office/SpriteManager.ts`

- [ ] **Step 1: Create OfficeRenderer.ts**

This handles loading the template JSON and rendering the tilemap (floor, walls, zones, furniture) using Pixi.js Graphics primitives (no sprite assets needed for Phase 1 — use colored rectangles and simple shapes).

```typescript
import { Application, Container, Graphics, Text, TextStyle } from 'pixi.js'

export interface TemplateZone {
  id: string
  type: string
  label: string
  x: number; y: number; w: number; h: number
  capacity?: number
  seats?: { x: number; y: number }[]
  meeting_seats?: { x: number; y: number }[]
}

export interface OfficeTemplate {
  id: string
  name: string
  width: number
  height: number
  tileSize: number
  zones: TemplateZone[]
  furniture: { type: string; x: number; y: number }[]
}

const ZONE_COLORS: Record<string, number> = {
  desk_area: 0xF5F5DC,
  meeting_room: 0xE8F5E9,
  cafe: 0xFFF3E0,
  lounge: 0xE3F2FD,
  phone_booth: 0xFCE4EC,
}

const FURNITURE_COLORS: Record<string, number> = {
  desk: 0x8D6E63,
  plant: 0x4CAF50,
  whiteboard: 0xEEEEEE,
  coffee_machine: 0x795548,
  computer: 0x37474F,
}

export class OfficeRenderer {
  private container: Container
  private tileSize: number = 32
  private template: OfficeTemplate | null = null

  constructor(private app: Application) {
    this.container = new Container()
    app.stage.addChild(this.container)
  }

  loadTemplate(template: OfficeTemplate) {
    this.template = template
    this.tileSize = template.tileSize
    this.container.removeChildren()
    this.drawFloor()
    this.drawZones()
    this.drawFurniture()
    this.drawGrid()
  }

  getTemplate(): OfficeTemplate | null {
    return this.template
  }

  private drawFloor() {
    if (!this.template) return
    const floor = new Graphics()
    floor.rect(0, 0, this.template.width * this.tileSize, this.template.height * this.tileSize)
    floor.fill(0xFAFAFA)
    floor.stroke({ color: 0xE0E0E0, width: 1 })
    this.container.addChild(floor)
  }

  private drawZones() {
    if (!this.template) return
    for (const zone of this.template.zones) {
      const g = new Graphics()
      const color = ZONE_COLORS[zone.type] ?? 0xF5F5F5
      g.rect(
        zone.x * this.tileSize,
        zone.y * this.tileSize,
        zone.w * this.tileSize,
        zone.h * this.tileSize,
      )
      g.fill({ color, alpha: 0.5 })
      g.stroke({ color: 0xBDBDBD, width: 1 })
      this.container.addChild(g)

      // Zone label
      const label = new Text({
        text: zone.label,
        style: new TextStyle({ fontSize: 10, fill: 0x757575 }),
      })
      label.x = zone.x * this.tileSize + 4
      label.y = zone.y * this.tileSize + 2
      this.container.addChild(label)

      // Draw empty seat markers
      const seats = zone.seats ?? zone.meeting_seats ?? []
      for (const s of seats) {
        const marker = new Graphics()
        marker.circle(s.x * this.tileSize + this.tileSize / 2, s.y * this.tileSize + this.tileSize / 2, 6)
        marker.fill({ color: 0xE0E0E0, alpha: 0.5 })
        this.container.addChild(marker)
      }
    }
  }

  private drawFurniture() {
    if (!this.template) return
    for (const f of this.template.furniture) {
      const g = new Graphics()
      const color = FURNITURE_COLORS[f.type] ?? 0x9E9E9E
      g.rect(
        f.x * this.tileSize + 4,
        f.y * this.tileSize + 4,
        this.tileSize - 8,
        this.tileSize - 8,
      )
      g.fill(color)
      this.container.addChild(g)
    }
  }

  private drawGrid() {
    if (!this.template) return
    const grid = new Graphics()
    for (let x = 0; x <= this.template.width; x++) {
      grid.moveTo(x * this.tileSize, 0)
      grid.lineTo(x * this.tileSize, this.template.height * this.tileSize)
    }
    for (let y = 0; y <= this.template.height; y++) {
      grid.moveTo(0, y * this.tileSize)
      grid.lineTo(this.template.width * this.tileSize, y * this.tileSize)
    }
    grid.stroke({ color: 0xF0F0F0, width: 0.5 })
    this.container.addChild(grid)
  }

  destroy() {
    this.container.destroy({ children: true })
  }
}
```

- [ ] **Step 2: Create SpriteManager.ts**

Handles creating/updating/removing character sprites based on snapshot diff. Phase 1 uses simple colored circles with text labels (no actual pixel art sprites yet).

```typescript
import { Container, Graphics, Text, TextStyle } from 'pixi.js'

export interface SeatData {
  seat_id: number
  employee_id: number
  name: string
  position: string
  department: string
  floor: number
  zone: string
  seat_x: number
  seat_y: number
  avatar_type: string
  avatar_color: string
  status: string
  is_late: boolean
  custom_status: string | null
  custom_emoji: string | null
  clock_in_at: string | null
  leave_type: string | null
  meeting_room_zone: string | null
}

interface SpriteEntry {
  container: Container
  data: SeatData
}

const STATUS_EMOJI: Record<string, string> = {
  working: '💻',
  overtime: '🔥',
  focused: '🎧',
  in_meeting: '🤝',
  on_break: '☕',
  away: '💤',
  on_leave: '🏥',
  offline: '',
}

export class SpriteManager {
  private sprites = new Map<number, SpriteEntry>()
  private container: Container
  private tileSize: number

  onSeatClick: ((seat: SeatData) => void) | null = null

  constructor(parentContainer: Container, tileSize: number) {
    this.tileSize = tileSize
    this.container = new Container()
    parentContainer.addChild(this.container)
  }

  update(seats: SeatData[]) {
    const currentIds = new Set(seats.map(s => s.employee_id))

    // Remove sprites for employees no longer present
    for (const [id, entry] of this.sprites) {
      if (!currentIds.has(id)) {
        entry.container.destroy({ children: true })
        this.sprites.delete(id)
      }
    }

    // Add or update sprites
    for (const seat of seats) {
      if (seat.status === 'offline' && !seat.custom_status) {
        // Remove offline sprites
        const existing = this.sprites.get(seat.employee_id)
        if (existing) {
          existing.container.destroy({ children: true })
          this.sprites.delete(seat.employee_id)
        }
        continue
      }

      const existing = this.sprites.get(seat.employee_id)
      if (existing) {
        this.updateSprite(existing, seat)
      } else {
        this.createSprite(seat)
      }
    }
  }

  private createSprite(seat: SeatData) {
    const c = new Container()
    c.x = seat.seat_x * this.tileSize
    c.y = seat.seat_y * this.tileSize
    c.eventMode = 'static'
    c.cursor = 'pointer'
    c.on('pointertap', () => this.onSeatClick?.(seat))

    // Avatar circle
    const avatar = new Graphics()
    const color = parseInt(seat.avatar_color.replace('#', ''), 16)
    avatar.circle(this.tileSize / 2, this.tileSize / 2, 12)
    avatar.fill(seat.status === 'on_leave' ? 0xBDBDBD : color)
    if (seat.status === 'away') avatar.alpha = 0.5
    c.addChild(avatar)

    // Name label
    const name = new Text({
      text: seat.name.split(' ')[0],
      style: new TextStyle({ fontSize: 8, fill: 0x333333, align: 'center' }),
    })
    name.x = this.tileSize / 2 - name.width / 2
    name.y = this.tileSize - 2
    c.addChild(name)

    // Status bubble
    const emoji = seat.custom_emoji ?? STATUS_EMOJI[seat.status] ?? ''
    if (emoji) {
      const bubble = new Text({
        text: emoji,
        style: new TextStyle({ fontSize: 12 }),
      })
      bubble.x = this.tileSize - 8
      bubble.y = -4
      c.addChild(bubble)
    }

    // Custom status text
    if (seat.custom_status) {
      const statusText = new Text({
        text: seat.custom_status.length > 12 ? seat.custom_status.slice(0, 12) + '…' : seat.custom_status,
        style: new TextStyle({ fontSize: 7, fill: 0x666666 }),
      })
      statusText.x = this.tileSize / 2 - statusText.width / 2
      statusText.y = -10
      c.addChild(statusText)
    }

    // Late indicator
    if (seat.is_late) {
      const late = new Text({
        text: '⚠️',
        style: new TextStyle({ fontSize: 8 }),
      })
      late.x = -4
      late.y = -4
      c.addChild(late)
    }

    this.container.addChild(c)
    this.sprites.set(seat.employee_id, { container: c, data: seat })
  }

  private updateSprite(entry: SpriteEntry, seat: SeatData) {
    // Simple approach: destroy and recreate if data changed
    if (JSON.stringify(entry.data) !== JSON.stringify(seat)) {
      entry.container.destroy({ children: true })
      this.sprites.delete(seat.employee_id)
      this.createSprite(seat)
    }
  }

  destroy() {
    this.container.destroy({ children: true })
    this.sprites.clear()
  }
}
```

- [ ] **Step 3: Create OfficeCanvas.vue**

```vue
<template>
  <div ref="canvasContainer" class="office-canvas" />
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { Application } from 'pixi.js'
import { OfficeRenderer, type OfficeTemplate } from './OfficeRenderer'
import { SpriteManager, type SeatData } from './SpriteManager'

const props = defineProps<{
  template: OfficeTemplate | null
  seats: SeatData[]
}>()

const emit = defineEmits<{
  (e: 'select', seat: SeatData): void
}>()

const canvasContainer = ref<HTMLElement>()
let app: Application | null = null
let renderer: OfficeRenderer | null = null
let spriteManager: SpriteManager | null = null

onMounted(async () => {
  if (!canvasContainer.value) return

  app = new Application()
  await app.init({
    background: '#FAFAFA',
    resizeTo: canvasContainer.value,
    antialias: true,
  })
  canvasContainer.value.appendChild(app.canvas as HTMLCanvasElement)

  renderer = new OfficeRenderer(app)
  spriteManager = new SpriteManager(app.stage, 32)
  spriteManager.onSeatClick = (seat) => emit('select', seat)

  if (props.template) {
    renderer.loadTemplate(props.template)
  }
  if (props.seats.length) {
    spriteManager.update(props.seats)
  }
})

watch(() => props.template, (tmpl) => {
  if (tmpl && renderer) renderer.loadTemplate(tmpl)
})

watch(() => props.seats, (seats) => {
  if (spriteManager) spriteManager.update(seats)
}, { deep: true })

onBeforeUnmount(() => {
  spriteManager?.destroy()
  renderer?.destroy()
  app?.destroy(true)
})
</script>

<style scoped>
.office-canvas {
  width: 100%;
  height: 100%;
  min-height: 400px;
  border-radius: 8px;
  overflow: hidden;
}
</style>
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: BUILD SUCCESS (components aren't used yet, but should compile)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/virtual-office/OfficeCanvas.vue frontend/src/components/virtual-office/OfficeRenderer.ts frontend/src/components/virtual-office/SpriteManager.ts
git commit -m "feat(virtual-office): add Pixi.js rendering engine with OfficeCanvas, OfficeRenderer, and SpriteManager"
```

---

### Task 11: UI Components (SeatInfoCard, StatusBar, OfficeStats, OfficeSetup, MiniMap)

**Files:**
- Create: `frontend/src/components/virtual-office/SeatInfoCard.vue`
- Create: `frontend/src/components/virtual-office/StatusBar.vue`
- Create: `frontend/src/components/virtual-office/OfficeStats.vue`
- Create: `frontend/src/components/virtual-office/OfficeSetup.vue`
- Create: `frontend/src/components/virtual-office/MiniMap.vue`

- [ ] **Step 1: Create SeatInfoCard.vue**

NPopover that shows employee details when clicking a character.

```vue
<template>
  <n-card v-if="seat" size="small" :bordered="true" style="width: 260px">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 8px">
      <div :style="{ width: '40px', height: '40px', borderRadius: '50%', backgroundColor: seat.avatar_color, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontWeight: 'bold', fontSize: '16px' }">
        {{ seat.name.charAt(0) }}
      </div>
      <div>
        <div style="font-weight: 600">{{ seat.name }}</div>
        <div style="font-size: 12px; color: #999">{{ seat.position }}</div>
      </div>
    </div>
    <n-descriptions :column="1" label-placement="left" size="small">
      <n-descriptions-item :label="t('common.department')">{{ seat.department }}</n-descriptions-item>
      <n-descriptions-item :label="t('common.status')">
        <n-tag :type="statusType" size="small">{{ statusLabel }}</n-tag>
      </n-descriptions-item>
      <n-descriptions-item v-if="seat.custom_status" :label="t('virtualOffice.setStatus')">
        {{ seat.custom_emoji ?? '' }} {{ seat.custom_status }}
      </n-descriptions-item>
      <n-descriptions-item v-if="seat.clock_in_at" :label="t('virtualOffice.clockedIn')">
        {{ new Date(seat.clock_in_at).toLocaleTimeString() }}
      </n-descriptions-item>
      <n-descriptions-item v-if="seat.leave_type" :label="t('virtualOffice.leaveType')">
        {{ seat.leave_type }}
      </n-descriptions-item>
    </n-descriptions>
  </n-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SeatData } from './SpriteManager'

const props = defineProps<{ seat: SeatData | null }>()
const { t } = useI18n()

const statusType = computed(() => {
  switch (props.seat?.status) {
    case 'working': return 'success'
    case 'overtime': return 'warning'
    case 'focused': return 'info'
    case 'in_meeting': return 'info'
    case 'on_leave': return 'error'
    case 'offline': return 'default'
    default: return 'default'
  }
})

const statusLabel = computed(() => {
  const key = props.seat?.status ?? 'offline'
  const map: Record<string, string> = {
    working: t('virtualOffice.working'),
    overtime: t('virtualOffice.overtime'),
    focused: t('virtualOffice.focused'),
    in_meeting: t('virtualOffice.inMeetingStatus'),
    on_break: t('virtualOffice.onBreak'),
    away: t('virtualOffice.away'),
    on_leave: t('virtualOffice.onLeave'),
    offline: t('virtualOffice.offline'),
  }
  return map[key] ?? key
})
</script>
```

- [ ] **Step 2: Create StatusBar.vue**

Bottom bar for setting own status, emoji, and manual status.

```vue
<template>
  <div style="display: flex; gap: 8px; align-items: center; padding: 8px 0">
    <n-input
      v-model:value="customStatus"
      :placeholder="t('virtualOffice.statusPlaceholder')"
      size="small"
      style="flex: 1; max-width: 300px"
      @keyup.enter="saveStatus"
    />
    <n-select
      v-model:value="manualStatus"
      size="small"
      :options="statusOptions"
      style="width: 140px"
      clearable
      :placeholder="t('virtualOffice.setStatus')"
    />
    <n-select
      v-if="manualStatus === 'in_meeting'"
      v-model:value="meetingRoomZone"
      size="small"
      :options="meetingRoomOptions"
      style="width: 160px"
      :placeholder="t('virtualOffice.meetingRoom')"
    />
    <n-button size="small" type="primary" @click="saveStatus" :loading="saving">
      {{ t('virtualOffice.setStatus') }}
    </n-button>
    <n-button v-if="manualStatus" size="small" quaternary @click="clearStatus">
      {{ t('virtualOffice.clearStatus') }}
    </n-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'

const props = defineProps<{ meetingRooms?: { zone_id: string; label: string }[] }>()
const { t } = useI18n()
const message = useMessage()

const customStatus = ref('')
const manualStatus = ref<string | null>(null)
const meetingRoomZone = ref<string | null>(null)
const saving = ref(false)

const statusOptions = computed(() => [
  { label: t('virtualOffice.focused'), value: 'focused' },
  { label: t('virtualOffice.inMeetingStatus'), value: 'in_meeting' },
  { label: t('virtualOffice.onBreak'), value: 'on_break' },
  { label: t('virtualOffice.away'), value: 'away' },
])

const meetingRoomOptions = computed(() =>
  (props.meetingRooms ?? []).map(r => ({ label: r.label, value: r.zone_id }))
)

const emit = defineEmits<{ (e: 'updated'): void }>()

async function saveStatus() {
  if (manualStatus.value === 'in_meeting' && !meetingRoomZone.value) {
    message.warning(t('virtualOffice.meetingRoom') + ' is required')
    return
  }
  saving.value = true
  try {
    await virtualOfficeAPI.updateMyStatus({
      custom_status: customStatus.value || null,
      manual_status: manualStatus.value,
      meeting_room_zone: manualStatus.value === 'in_meeting' ? meetingRoomZone.value : null,
    })
    message.success(t('virtualOffice.statusSaved'))
    emit('updated')
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function clearStatus() {
  saving.value = true
  try {
    await virtualOfficeAPI.updateMyStatus({ manual_status: null, meeting_room_zone: null })
    manualStatus.value = null
    meetingRoomZone.value = null
    message.success(t('virtualOffice.statusSaved'))
    emit('updated')
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}
</script>
```

- [ ] **Step 3: Create OfficeStats.vue**

```vue
<template>
  <div style="display: flex; gap: 16px; padding: 8px 0">
    <n-statistic :label="t('virtualOffice.online')" :value="stats.online">
      <template #prefix><span style="color: #18a058">●</span></template>
    </n-statistic>
    <n-statistic :label="t('virtualOffice.inMeeting')" :value="stats.in_meeting">
      <template #prefix><span style="color: #2080f0">●</span></template>
    </n-statistic>
    <n-statistic :label="t('virtualOffice.onLeave')" :value="stats.on_leave">
      <template #prefix><span style="color: #d03050">●</span></template>
    </n-statistic>
    <n-statistic :label="t('virtualOffice.offline')" :value="stats.offline">
      <template #prefix><span style="color: #999">●</span></template>
    </n-statistic>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ stats: Record<string, number> }>()
const { t } = useI18n()
</script>
```

- [ ] **Step 4: Create OfficeSetup.vue**

Admin component for template selection and auto-assign.

```vue
<template>
  <n-card :title="t('virtualOffice.setup')">
    <n-space vertical>
      <n-radio-group v-model:value="selectedTemplate">
        <n-space>
          <n-radio value="small">{{ t('virtualOffice.small') }}</n-radio>
          <n-radio value="medium">{{ t('virtualOffice.medium') }}</n-radio>
          <n-radio value="large">{{ t('virtualOffice.large') }}</n-radio>
        </n-space>
      </n-radio-group>
      <n-space>
        <n-button type="primary" @click="saveConfig" :loading="saving">
          {{ t('virtualOffice.saveConfig') }}
        </n-button>
        <n-button @click="autoAssign" :loading="assigning">
          {{ t('virtualOffice.autoAssign') }}
        </n-button>
      </n-space>
    </n-space>
  </n-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { virtualOfficeAPI } from '../../api/client'

const props = defineProps<{ currentTemplate?: string }>()
const emit = defineEmits<{ (e: 'saved'): void }>()
const { t } = useI18n()
const message = useMessage()

const selectedTemplate = ref(props.currentTemplate ?? 'small')
const saving = ref(false)
const assigning = ref(false)

async function saveConfig() {
  saving.value = true
  try {
    await virtualOfficeAPI.updateConfig({ template: selectedTemplate.value })
    message.success(t('common.saved'))
    emit('saved')
  } catch {
    message.error(t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function autoAssign() {
  assigning.value = true
  try {
    const res = await virtualOfficeAPI.autoAssign()
    const data = res as { assigned: number }
    message.success(t('virtualOffice.autoAssignSuccess', { count: data.assigned }))
    emit('saved')
  } catch {
    message.error(t('common.failed'))
  } finally {
    assigning.value = false
  }
}
</script>
```

- [ ] **Step 5: Create MiniMap.vue**

```vue
<template>
  <div class="minimap" style="width: 150px; height: 100px; border: 1px solid #eee; border-radius: 4px; overflow: hidden; background: #fafafa">
    <canvas ref="miniCanvas" width="150" height="100" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import type { SeatData } from './SpriteManager'

const props = defineProps<{
  templateWidth: number
  templateHeight: number
  tileSize: number
  seats: SeatData[]
}>()

const miniCanvas = ref<HTMLCanvasElement>()

function draw() {
  const canvas = miniCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const scaleX = 150 / (props.templateWidth * props.tileSize)
  const scaleY = 100 / (props.templateHeight * props.tileSize)
  const scale = Math.min(scaleX, scaleY)

  ctx.clearRect(0, 0, 150, 100)
  ctx.fillStyle = '#fafafa'
  ctx.fillRect(0, 0, 150, 100)

  for (const seat of props.seats) {
    if (seat.status === 'offline') continue
    const x = seat.seat_x * props.tileSize * scale
    const y = seat.seat_y * props.tileSize * scale
    ctx.fillStyle = seat.status === 'on_leave' ? '#d03050' : seat.avatar_color
    ctx.beginPath()
    ctx.arc(x + 3, y + 3, 3, 0, Math.PI * 2)
    ctx.fill()
  }
}

onMounted(draw)
watch(() => props.seats, draw, { deep: true })
</script>
```

- [ ] **Step 6: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/virtual-office/
git commit -m "feat(virtual-office): add UI components (SeatInfoCard, StatusBar, OfficeStats, OfficeSetup, MiniMap)"
```

---

### Task 12: Main View (VirtualOfficeView.vue)

**Files:**
- Create: `frontend/src/views/VirtualOfficeView.vue`

- [ ] **Step 1: Create the main view**

```vue
<template>
  <n-space vertical :size="12">
    <n-page-header :title="t('virtualOffice.title')" />

    <!-- No config yet: show setup -->
    <template v-if="!config && !loading">
      <n-result status="info" :title="t('virtualOffice.noOffice')" :description="t('virtualOffice.setupFirst')">
        <template #footer>
          <OfficeSetup @saved="loadData" />
        </template>
      </n-result>
    </template>

    <!-- Loading -->
    <n-spin v-else-if="loading" size="large" style="display: flex; justify-content: center; padding: 80px 0" />

    <!-- Office view -->
    <template v-else>
      <!-- Stats bar -->
      <OfficeStats :stats="snapshot?.stats ?? {}" />

      <!-- Admin setup toggle -->
      <n-collapse v-if="isAdmin" style="margin-bottom: 8px">
        <n-collapse-item :title="t('virtualOffice.setup')">
          <OfficeSetup :current-template="config?.template" @saved="loadData" />
        </n-collapse-item>
      </n-collapse>

      <!-- Canvas + sidebar -->
      <div style="display: flex; gap: 12px">
        <div style="flex: 1; position: relative">
          <OfficeCanvas
            :template="template"
            :seats="snapshot?.seats ?? []"
            @select="selectedSeat = $event"
          />
        </div>
        <div style="width: 170px">
          <MiniMap
            v-if="template"
            :template-width="template.width"
            :template-height="template.height"
            :tile-size="template.tileSize"
            :seats="snapshot?.seats ?? []"
          />
          <SeatInfoCard v-if="selectedSeat" :seat="selectedSeat" style="margin-top: 12px" />
        </div>
      </div>

      <!-- Status bar -->
      <StatusBar :meeting-rooms="snapshot?.meeting_rooms as any" @updated="fetchSnapshot" />
    </template>
  </n-space>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { virtualOfficeAPI } from '../api/client'
import OfficeCanvas from '../components/virtual-office/OfficeCanvas.vue'
import OfficeStats from '../components/virtual-office/OfficeStats.vue'
import OfficeSetup from '../components/virtual-office/OfficeSetup.vue'
import StatusBar from '../components/virtual-office/StatusBar.vue'
import SeatInfoCard from '../components/virtual-office/SeatInfoCard.vue'
import MiniMap from '../components/virtual-office/MiniMap.vue'
import type { SeatData } from '../components/virtual-office/SpriteManager'
import type { OfficeTemplate } from '../components/virtual-office/OfficeRenderer'

const { t } = useI18n()
const authStore = useAuthStore()

const loading = ref(true)
const config = ref<{ template: string } | null>(null)
const snapshot = ref<{ template: string; stats: Record<string, number>; seats: SeatData[]; meeting_rooms: unknown[] } | null>(null)
const template = ref<OfficeTemplate | null>(null)
const selectedSeat = ref<SeatData | null>(null)

const isAdmin = computed(() => authStore.isAdmin)

let pollTimer: ReturnType<typeof setInterval> | null = null

async function loadData() {
  loading.value = true
  try {
    const cfgRes = await virtualOfficeAPI.getConfig()
    config.value = cfgRes as { template: string }

    // Load template JSON
    const tmplModule = await import(`../assets/virtual-office/templates/${config.value.template}.json`)
    template.value = tmplModule.default as OfficeTemplate

    await fetchSnapshot()
  } catch {
    config.value = null
  } finally {
    loading.value = false
  }
}

async function fetchSnapshot() {
  try {
    const res = await virtualOfficeAPI.getSnapshot()
    snapshot.value = res as typeof snapshot.value
  } catch {
    // Silent fail — will retry on next poll
  }
}

onMounted(async () => {
  await loadData()
  pollTimer = setInterval(fetchSnapshot, 30000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: BUILD SUCCESS with no errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/VirtualOfficeView.vue
git commit -m "feat(virtual-office): add main VirtualOfficeView with polling and all components wired"
```

---

### Task 13: Build + Verify Full Stack

- [ ] **Step 1: Run Go tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/virtualoffice/ -v`
Expected: All tests PASS

- [ ] **Step 2: Run full Go build**

Run: `cd /Users/anna/Documents/aigonhr && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Run sqlc generate to verify queries are clean**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors

- [ ] **Step 4: Run frontend build**

Run: `cd /Users/anna/Documents/aigonhr/frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 5: Run all existing tests to verify no regressions**

Run: `cd /Users/anna/Documents/aigonhr && go test ./... -count=1 -timeout=120s 2>&1 | tail -30`
Expected: All existing tests still PASS

- [ ] **Step 6: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix(virtual-office): address build and test issues from integration"
```
