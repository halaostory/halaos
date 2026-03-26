# HalaOS → Brain Data Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Push HR analytics data (risk scores, burnout, team health, blind spots, attendance anomalies, org snapshots) from HalaOS to AI Management Brain via signed webhooks.

**Architecture:** Event outbox pattern (mirrors existing accounting_outbox) on HalaOS side. Webhook receiver on Brain side with per-tenant HMAC verification, automatic employee mapping, and signal/event storage. Two codebases: AIGoNHR (`/Users/anna/Documents/aigonhr`) and AI Management Brain (`/Users/anna/Documents/ai-management-brain`).

**Tech Stack:** Go 1.25, Gin, sqlc/pgx, PostgreSQL, HMAC-SHA256

**Spec:** `docs/superpowers/specs/2026-03-26-halaos-brain-bridge-design.md`

---

## File Structure

### HalaOS (AIGoNHR) — New/Modified Files

| File | Responsibility |
|------|---------------|
| `db/migrations/00087_brain_integration.sql` | brain_links + brain_outbox tables (goose) |
| `db/query/brain_integration.sql` | sqlc queries for brain_links + brain_outbox |
| `internal/integration/brain_events.go` | 6 event payload structs + constants |
| `internal/integration/brain_outbox.go` | Enqueue with date-based idempotency key |
| `internal/integration/brain_emitter.go` | 6 Emit methods + guard check for active brain_link |
| `internal/integration/brain_dispatcher.go` | Poll loop, HMAC signing (`sha256=` prefix), batch delivery |
| `internal/integration/brain_emitter_test.go` | Unit tests for emitter |
| `internal/integration/brain_dispatcher_test.go` | Unit tests for dispatcher |
| `cmd/worker/flightrisk.go` | Modified: emit after UpsertScores |
| `cmd/worker/burnout.go` | Modified: emit after UpsertScores |
| `cmd/worker/teamhealth.go` | Modified: emit after UpsertScores |
| `cmd/worker/blindspot.go` | Modified: emit after UpsertSpots |
| `cmd/worker/org_snapshot.go` | Modified: emit after snapshot insert |
| `internal/app/bootstrap.go` | Modified: wire brain dispatcher |

### AI Management Brain — New/Modified Files

| File | Responsibility |
|------|---------------|
| `sql/migrations/000016_halaos_bridge.up.sql` | employees columns + halaos_links + halaos_events |
| `sql/migrations/000016_halaos_bridge.down.sql` | Rollback migration |
| `sql/queries/halaos.sql` | sqlc queries for halaos_links, halaos_events, employee mapping |
| `sql/queries/execution_signals.sql` | Modified: add new signal types to GetTopRisks |
| `internal/api/halaos_webhook.go` | Webhook handler + HMAC verification + event dispatch |
| `internal/brain/halaos_mapper.go` | Employee mapping + event→signal/event conversion |
| `internal/brain/context_service.go` | Modified: add HRInsightsContext |
| `internal/api/router.go` | Modified: register webhook route |

---

## Task Dependencies

```
HalaOS Side:
  Task 1 (migration) → Task 2 (sqlc queries) → Task 3 (events) → Task 4 (outbox) → Task 5 (emitter) → Task 6 (dispatcher)
  Task 6 → Task 7 (hook into scorers)
  Task 6 → Task 8 (bootstrap wiring)
  Task 8 → Task 9 (HalaOS tests)

Brain Side (parallel with HalaOS after Task 1-2 pattern is clear):
  Task 10 (migration) → Task 11 (sqlc queries) → Task 12 (update GetTopRisks)
  Task 12 → Task 13 (webhook handler) → Task 14 (mapper)
  Task 14 → Task 15 (context service)
  Task 15 → Task 16 (Brain tests)

Final:
  Task 9 + Task 16 → Task 17 (integration verification)
```

---

### Task 1: HalaOS Migration — brain_links + brain_outbox

**Files:**
- Create: `db/migrations/00087_brain_integration.sql`

**Context:** Mirrors `db/migrations/00076_accounting_integration.sql`. Uses goose format with `-- +goose Up` / `-- +goose Down` markers. The `brain_links` table follows `accounting_links` pattern (with `api_key_enc` for encrypted key storage). The `brain_outbox` table is identical to `accounting_outbox`.

- [ ] **Step 1: Write migration file**

```sql
-- +goose Up

CREATE TABLE brain_links (
    id            BIGSERIAL    PRIMARY KEY,
    company_id    BIGINT       NOT NULL REFERENCES companies(id),
    brain_tenant_id UUID       NOT NULL,
    api_endpoint  TEXT         NOT NULL,
    api_key_enc   VARCHAR(500) NOT NULL,
    webhook_secret VARCHAR(500) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    last_synced_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_brain_links_company ON brain_links(company_id);

CREATE TABLE brain_outbox (
    id              BIGSERIAL    PRIMARY KEY,
    company_id      BIGINT       NOT NULL,
    event_type      VARCHAR(100) NOT NULL,
    aggregate_type  VARCHAR(50)  NOT NULL,
    aggregate_id    BIGINT       NOT NULL,
    payload         JSONB        NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    retry_count     INT          NOT NULL DEFAULT 0,
    max_retries     INT          NOT NULL DEFAULT 5,
    next_retry_at   TIMESTAMPTZ,
    sent_at         TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_brain_outbox_pending
    ON brain_outbox(status, next_retry_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_brain_outbox_company
    ON brain_outbox(company_id, created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS brain_outbox;
DROP TABLE IF EXISTS brain_links;
```

- [ ] **Step 2: Verify migration syntax**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add db/migrations/00087_brain_integration.sql
git commit -m "feat: add brain_links and brain_outbox migration (00087)"
```

---

### Task 2: HalaOS sqlc Queries — brain_integration.sql

**Files:**
- Create: `db/query/brain_integration.sql`
- Reference: `db/query/accounting_integration.sql` for pattern

**Context:** Same query patterns as accounting_integration.sql. Need CRUD for brain_links, outbox insert/list/mark operations. The `SKIP LOCKED` pattern is critical for concurrent-safe batch processing.

- [ ] **Step 1: Write sqlc queries**

```sql
-- name: CreateBrainLink :one
INSERT INTO brain_links (
    company_id, brain_tenant_id, api_endpoint, api_key_enc, webhook_secret
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetActiveBrainLink :one
SELECT * FROM brain_links
WHERE company_id = $1 AND is_active = true
LIMIT 1;

-- name: GetBrainLinkByID :one
SELECT * FROM brain_links
WHERE id = $1 AND company_id = $2;

-- name: UpdateBrainLinkSyncedAt :exec
UPDATE brain_links
SET last_synced_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: UpdateBrainLinkStatus :exec
UPDATE brain_links
SET is_active = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteBrainLink :exec
DELETE FROM brain_links WHERE id = $1 AND company_id = $2;

-- name: ListBrainLinks :many
SELECT * FROM brain_links
WHERE company_id = $1
ORDER BY created_at DESC;

-- name: InsertBrainOutbox :one
INSERT INTO brain_outbox (
    company_id, event_type, aggregate_type, aggregate_id,
    payload, idempotency_key
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPendingBrainOutbox :many
SELECT * FROM brain_outbox
WHERE status IN ('pending', 'failed')
  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkBrainOutboxSent :exec
UPDATE brain_outbox
SET status = 'sent', sent_at = NOW(), error_message = NULL
WHERE id = $1;

-- name: MarkBrainOutboxFailed :exec
UPDATE brain_outbox
SET status = CASE WHEN retry_count + 1 >= max_retries THEN 'dead' ELSE 'failed' END,
    retry_count = retry_count + 1,
    next_retry_at = NOW() + (INTERVAL '1 minute' * POWER(2, retry_count)),
    error_message = $2
WHERE id = $1;
```

- [ ] **Step 2: Generate sqlc**

Run: `cd /Users/anna/Documents/aigonhr && ~/go/bin/sqlc generate`
Expected: No errors, new functions in `internal/store/brain_integration.sql.go`

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 4: Commit**

```bash
git add db/query/brain_integration.sql internal/store/
git commit -m "feat: add sqlc queries for brain integration"
```

---

### Task 3: HalaOS Event Structs — brain_events.go

**Files:**
- Create: `internal/integration/brain_events.go`
- Reference: `internal/integration/accounting_events.go` for pattern

**Context:** 6 event types. Each event has standard metadata (EventID, EventType, EventVersion, OccurredAt, HRCompanyID). Payload fields match the spec. The `WebhookPayload` envelope from accounting_events.go is reused.

- [ ] **Step 1: Write event structs**

Define constants:
```go
const (
    BrainEventRiskUpdated      = "hr.risk.updated"
    BrainEventBurnoutUpdated   = "hr.burnout.updated"
    BrainEventTeamHealthUpdated = "hr.team_health.updated"
    BrainEventBlindspotDetected = "hr.blindspot.detected"
    BrainEventAttendanceAnomaly = "hr.attendance.anomaly"
    BrainEventOrgSnapshot      = "hr.org_snapshot.weekly"
)
```

Define 6 event structs:
- `RiskUpdatedEvent` — employee_id, employee_no, name, department, risk_score (int), factors ([]EventFactor where EventFactor has Factor string, Points int, Detail string — matches `flightrisk.RiskFactor` struct), prev_score (int)
- `BurnoutUpdatedEvent` — same structure as risk (factors match `burnout.BurnoutFactor` struct)
- `TeamHealthUpdatedEvent` — department_id, department_name, health_score (int), factors ([]EventFactor — same struct, matches `teamhealth.Factor`), prev_score (int)
- `BlindspotDetectedEvent` — manager_id, manager_name, spot_type, severity, title, description, employees ([]BlindspotEmployee where each has ID, EmployeeNo, Name, Detail)
- `AttendanceAnomalyEvent` — date (string), anomalies ([]AttendanceAnomaly where each has EmployeeID, EmployeeNo, Name, Type, Detail)
- `OrgSnapshotEvent` — avg_flight_risk, avg_burnout, avg_team_health (float64), high_risk_count, high_burnout_count, headcount, low_health_dept_count (int). **Note:** `store.OrgScoreSnapshot` uses `pgtype.Numeric` for averages — need a `numericToFloat64()` helper (export from `internal/integration/` or create new one since `cmd/worker/main.go`'s is unexported).

All events include standard metadata: EventID (string, uuid), EventType (string), EventVersion (int, =1), OccurredAt (time.Time), HRCompanyID (int64).

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/integration/brain_events.go
git commit -m "feat: add brain event payload structs"
```

---

### Task 4: HalaOS Outbox — brain_outbox.go

**Files:**
- Create: `internal/integration/brain_outbox.go`
- Reference: `internal/integration/accounting_outbox.go`

**Context:** Same pattern as AccountingOutbox but with date in idempotency key: `{eventType}:{aggregateType}:{aggregateID}:{YYYY-MM-DD}`. This ensures recurring weekly scores create new outbox entries each run.

- [ ] **Step 1: Write outbox**

```go
type BrainOutbox struct {
    queries *store.Queries
    logger  *slog.Logger
}

func NewBrainOutbox(queries *store.Queries, logger *slog.Logger) *BrainOutbox

func (o *BrainOutbox) Enqueue(ctx context.Context, companyID int64, eventType, aggregateType string, aggregateID int64, payload any) error
```

Key difference from accounting: idempotency key includes date:
```go
idempotencyKey := fmt.Sprintf("%s:%s:%d:%s", eventType, aggregateType, aggregateID, time.Now().Format("2006-01-02"))
```

Uses `InsertBrainOutbox` sqlc query.

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/integration/brain_outbox.go
git commit -m "feat: add brain outbox with date-based idempotency"
```

---

### Task 5: HalaOS Emitter — brain_emitter.go

**Files:**
- Create: `internal/integration/brain_emitter.go`
- Reference: `internal/integration/accounting_emitter.go`

**Context:** 6 Emit methods. Each first checks `GetActiveBrainLink` — silently returns nil if no link configured. Builds event struct, calls `outbox.Enqueue()`.

**Important for BlindspotDetected:** The existing `AffectedEmployee` struct has `{ID, Name, Detail}` but no `EmployeeNo`. The emitter must do a separate query (`GetEmployeeByID`) to fetch employee_no for each affected employee.

- [ ] **Step 1: Write emitter**

```go
type BrainEmitter struct {
    outbox  *BrainOutbox
    queries *store.Queries
    logger  *slog.Logger
}

func NewBrainEmitter(queries *store.Queries, logger *slog.Logger) *BrainEmitter

// Guard: check active brain link, skip silently if not configured
func (e *BrainEmitter) hasActiveLink(ctx context.Context, companyID int64) bool

func (e *BrainEmitter) EmitRiskUpdated(ctx context.Context, companyID int64, emp flightrisk.EmployeeRisk, prevScore int) error
func (e *BrainEmitter) EmitBurnoutUpdated(ctx context.Context, companyID int64, emp burnout.EmployeeBurnout, prevScore int) error
func (e *BrainEmitter) EmitTeamHealthUpdated(ctx context.Context, companyID int64, dept teamhealth.DepartmentHealth, prevScore int) error
func (e *BrainEmitter) EmitBlindspotDetected(ctx context.Context, companyID int64, spot blindspot.BlindSpot) error
func (e *BrainEmitter) EmitAttendanceAnomaly(ctx context.Context, companyID int64, date string, anomalies []AttendanceAnomalyItem) error
// EmitOrgSnapshot takes individual field values (not the store struct) because
// UpsertOrgScoreSnapshot returns only error, not the struct. Worker builds these
// values from the scorer results before calling upsert.
func (e *BrainEmitter) EmitOrgSnapshot(ctx context.Context, companyID int64, avgFlightRisk, avgBurnout, avgTeamHealth float64, highRiskCount, highBurnoutCount, headcount, lowHealthDeptCount int) error
```

Each method: guard check → build event → `outbox.Enqueue()` → log.

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/integration/brain_emitter.go
git commit -m "feat: add brain emitter with 6 event types"
```

---

### Task 6: HalaOS Dispatcher — brain_dispatcher.go

**Files:**
- Create: `internal/integration/brain_dispatcher.go`
- Reference: `internal/integration/accounting_dispatcher.go`

**Context:** Same poll-deliver pattern. **Critical difference:** HMAC signature must use `sha256=` prefix (Brain convention) instead of raw hex (accounting convention). Header is `X-Signature-256` not `X-Webhook-Signature`.

- [ ] **Step 1: Write dispatcher**

```go
type BrainDispatcher struct {
    queries    *store.Queries
    httpClient *http.Client
    logger     *slog.Logger
    batchSize  int32
    interval   time.Duration
}

func NewBrainDispatcher(queries *store.Queries, logger *slog.Logger) *BrainDispatcher
func (d *BrainDispatcher) Run(ctx context.Context)     // blocking poll loop
func (d *BrainDispatcher) processBatch(ctx context.Context)
func (d *BrainDispatcher) deliver(ctx context.Context, link store.BrainLink, event store.BrainOutbox) error
```

In `deliver()`:
- Wrap event in `WebhookPayload{ID, Timestamp, EventType, Data}`
- Compute HMAC: `"sha256=" + hex.EncodeToString(mac.Sum(nil))` ← Brain format
- POST to `{link.ApiEndpoint}/webhooks/halaos` (**Note:** Brain's webhook group is at `/webhooks`, NOT `/api/v1/webhooks`. The existing Stripe webhook is at `/webhooks/stripe`.)
- Headers: `X-Signature-256`, `X-Event-Type`, `X-Event-ID`, `Authorization: Bearer {decrypted api_key_enc}`
- On success: `MarkBrainOutboxSent` + `UpdateBrainLinkSyncedAt`
- On failure: `MarkBrainOutboxFailed` (exponential backoff built into SQL)

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/integration/brain_dispatcher.go
git commit -m "feat: add brain dispatcher with sha256= signature format"
```

---

### Task 7: HalaOS — Hook Emitter into Worker Scorers

**Files:**
- Modify: `cmd/worker/flightrisk.go`
- Modify: `cmd/worker/burnout.go`
- Modify: `cmd/worker/teamhealth.go`
- Modify: `cmd/worker/blindspot.go` (find actual file name — may be in `cmd/worker/`)
- Modify: `cmd/worker/org_snapshot.go` or wherever `snapshotScoreHistory` is
- Modify: `cmd/worker/main.go` (pass BrainEmitter to worker functions)

**Context:** Each scorer function (`calculateFlightRisk`, `calculateBurnoutScores`, etc.) currently runs in `cmd/worker/` as a periodic job. After `scorer.UpsertScores()` succeeds, we emit brain events.

**Pattern for flightrisk.go:**
After `scorer.UpsertScores(ctx, company.ID, risks)` succeeds, add:
```go
// Emit to Brain (prevScore=0 for now — getting actual prev would require pre-query)
for _, risk := range risks {
    _ = brainEmitter.EmitRiskUpdated(ctx, company.ID, risk, 0)
}
```

Similar pattern for burnout, teamhealth, blindspot.

For org_snapshot (in `cmd/worker/scorehistory.go` `snapshotCompany` function): The `snapshotCompany` function computes avg scores from the scorer results before calling `UpsertOrgScoreSnapshot`. Capture these computed values and pass them to `EmitOrgSnapshot` as individual fields.

**Note on prev_score:** Passing `0` for all prev_score fields is acceptable for v1. Getting the actual previous score would require a pre-query before upsert (e.g., `GetEmployeeRiskScore`), adding complexity. The Brain side doesn't currently use prev_score for logic — it's informational only.

**Wiring `brainEmitter` through the worker:**
1. Create `BrainEmitter` in `cmd/worker/main.go` (same place where `queries` and `pool` are created)
2. Add `brainEmitter *integration.BrainEmitter` parameter to `runPeriodicJobs()` function signature
3. Thread it through to each scorer function: `calculateFlightRisk(ctx, queries, pool, logger, brainEmitter)`
4. Add `brainEmitter *integration.BrainEmitter` parameter to each scorer function signature

**Note on attendance anomaly:** The spec mentions `hr.attendance.anomaly` but there's no dedicated anomaly detection cron in the current worker. The burnout scorer already detects clock-in time irregularity (STDDEV). For v1, skip the attendance anomaly event — the burnout score already captures this signal. Can add as a follow-up.

- [ ] **Step 1: Modify cmd/worker/main.go**

1. Import `integration` package
2. Create: `brainEmitter := integration.NewBrainEmitter(queries, logger)`
3. Add `brainEmitter` param to `runPeriodicJobs()` call
4. Update `runPeriodicJobs` signature to accept `brainEmitter`
5. Pass `brainEmitter` to each scorer function call

- [ ] **Step 2: Modify each scorer file**

Add emit calls after UpsertScores/UpsertSpots. Log errors but don't fail the scorer.

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 4: Commit**

```bash
git add cmd/worker/
git commit -m "feat: hook brain emitter into analytics scorers"
```

---

### Task 8: HalaOS — Bootstrap Wiring

**Files:**
- Modify: `internal/app/bootstrap.go`

**Context:** At line ~249 in bootstrap.go, the accounting dispatcher is wired up. Add brain dispatcher in similar fashion.

- [ ] **Step 1: Add brain dispatcher startup**

After the accounting dispatcher setup:
```go
// Wire brain integration (outbox + dispatcher)
brainDispatcher := integration.NewBrainDispatcher(a.Queries, a.Logger)
go brainDispatcher.Run(context.Background())
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/aigonhr && go vet ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/app/bootstrap.go
git commit -m "feat: wire brain dispatcher into app bootstrap"
```

---

### Task 9: HalaOS — Unit Tests

**Files:**
- Create: `internal/integration/brain_emitter_test.go`
- Create: `internal/integration/brain_dispatcher_test.go`

**Context:** Use `testutil.MockDBTX` pattern. Test:
- Emitter: guard check skips when no link, enqueue called with correct event_type + payload
- Dispatcher: HMAC signature uses `sha256=` prefix, correct headers, retry logic

- [ ] **Step 1: Write emitter tests**

Tests:
- `TestEmitRiskUpdated_NoLink` — returns nil silently
- `TestEmitRiskUpdated_Success` — enqueues with correct key format `hr.risk.updated:employee:123:2026-03-26`
- `TestEmitBurnoutUpdated_Success`
- `TestEmitTeamHealthUpdated_Success`
- `TestEmitBlindspotDetected_Success`
- `TestEmitOrgSnapshot_Success`

- [ ] **Step 2: Write dispatcher tests**

Tests:
- `TestBrainDispatcher_SignatureFormat` — verify `sha256=<hex>` format
- `TestBrainDispatcher_Headers` — verify `X-Signature-256`, `X-Event-Type`, `X-Event-ID`
- `TestBrainDispatcher_NoPendingEvents` — processBatch is a no-op

- [ ] **Step 3: Run tests**

Run: `cd /Users/anna/Documents/aigonhr && go test ./internal/integration/... -v -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/integration/brain_*_test.go
git commit -m "test: add brain emitter and dispatcher unit tests"
```

---

### Task 10: Brain Migration — 000016_halaos_bridge

**Files:**
- Create: `sql/migrations/000016_halaos_bridge.up.sql`
- Create: `sql/migrations/000016_halaos_bridge.down.sql`

**Context:** Brain uses golang-migrate (NOT goose). File format: `NNNNNN_description.{up,down}.sql`. No goose markers. Adds employee linking columns + halaos_links + halaos_events tables.

- [ ] **Step 1: Write up migration**

```sql
-- Employee linking fields
ALTER TABLE employees ADD COLUMN halaos_employee_id BIGINT;
ALTER TABLE employees ADD COLUMN halaos_employee_no TEXT;
CREATE UNIQUE INDEX idx_employees_halaos_id
  ON employees(tenant_id, halaos_employee_id)
  WHERE halaos_employee_id IS NOT NULL;

-- HalaOS webhook configuration per tenant
CREATE TABLE halaos_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
  webhook_secret TEXT NOT NULL,
  halaos_company_id BIGINT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- HalaOS event audit log + idempotency
CREATE TABLE halaos_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  payload JSONB NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_halaos_events_idempotency
  ON halaos_events(tenant_id, idempotency_key);
CREATE INDEX idx_halaos_events_tenant_time
  ON halaos_events(tenant_id, processed_at DESC);
```

- [ ] **Step 2: Write down migration**

```sql
DROP INDEX IF EXISTS idx_halaos_events_tenant_time;
DROP INDEX IF EXISTS idx_halaos_events_idempotency;
DROP TABLE IF EXISTS halaos_events;
DROP TABLE IF EXISTS halaos_links;
DROP INDEX IF EXISTS idx_employees_halaos_id;
ALTER TABLE employees DROP COLUMN IF EXISTS halaos_employee_no;
ALTER TABLE employees DROP COLUMN IF EXISTS halaos_employee_id;
```

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add sql/migrations/000016_halaos_bridge.*
git commit -m "feat: add halaos bridge migration (000016)"
```

---

### Task 11: Brain sqlc Queries — halaos.sql

**Files:**
- Create: `sql/queries/halaos.sql`

**Context:** Brain uses sqlc with `pgx/v5`. Queries go in `sql/queries/`. Generated code in `internal/db/sqlc/`. Run `sqlc generate` from project root.

- [ ] **Step 1: Write sqlc queries**

```sql
-- name: GetHalaOSLinkByCompanyID :one
SELECT * FROM halaos_links
WHERE halaos_company_id = $1 AND is_active = true
LIMIT 1;

-- name: GetHalaOSLinkByTenant :one
SELECT * FROM halaos_links
WHERE tenant_id = $1 AND is_active = true
LIMIT 1;

-- name: CreateHalaOSLink :one
INSERT INTO halaos_links (tenant_id, webhook_secret, halaos_company_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteHalaOSLink :exec
DELETE FROM halaos_links WHERE id = $1 AND tenant_id = $2;

-- name: GetHalaOSEventByKey :one
SELECT id FROM halaos_events
WHERE tenant_id = $1 AND idempotency_key = $2
LIMIT 1;

-- name: CreateHalaOSEvent :one
INSERT INTO halaos_events (tenant_id, event_type, idempotency_key, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetEmployeeByHalaOSID :one
SELECT * FROM employees
WHERE tenant_id = $1 AND halaos_employee_id = $2
LIMIT 1;

-- name: GetEmployeeByHalaOSNo :one
SELECT * FROM employees
WHERE tenant_id = $1 AND halaos_employee_no = $2
LIMIT 1;

-- name: CreateEmployeeFromHalaOS :one
INSERT INTO employees (tenant_id, name, role, halaos_employee_id, halaos_employee_no)
VALUES ($1, $2, 'member', $3, $4)
RETURNING *;

-- name: UpdateEmployeeHalaOSLink :exec
UPDATE employees
SET halaos_employee_id = $2, halaos_employee_no = $3
WHERE id = $1;

-- name: CountHRSignalsByType :many
SELECT signal_type, COUNT(*) as count
FROM execution_signals
WHERE tenant_id = $1
  AND signal_type IN ('flight_risk', 'burnout_risk', 'team_health', 'org_health')
  AND generated_at >= $2
GROUP BY signal_type;

-- name: CountHighRiskSignals :one
SELECT COUNT(*) FROM execution_signals
WHERE tenant_id = $1
  AND signal_type = $2
  AND score >= @min_score::numeric
  AND generated_at >= $3;

-- name: GetLatestOrgHealthSignal :one
SELECT * FROM execution_signals
WHERE tenant_id = $1
  AND signal_type = 'org_health'
  AND subject_type = 'organization'
ORDER BY generated_at DESC
LIMIT 1;

-- name: CountRecentCommunicationEvents :one
SELECT COUNT(*) FROM communication_events
WHERE tenant_id = $1
  AND source_type = 'halaos'
  AND event_type = $2
  AND occurred_at >= $3;
```

- [ ] **Step 2: Generate sqlc**

Run: `cd /Users/anna/Documents/ai-management-brain && sqlc generate`

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/ai-management-brain && go vet ./...`

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add sql/queries/halaos.sql internal/db/sqlc/
git commit -m "feat: add sqlc queries for halaos bridge"
```

---

### Task 12: Brain — Update GetTopRisks Query

**Files:**
- Modify: `sql/queries/execution_signals.sql`

**Context:** The existing `GetTopRisks` query has a hardcoded `signal_type IN ('slow_response', 'missed_deadline', 'overloaded', 'blocker_risk', 'declining')`. Must add the 4 new HalaOS signal types.

- [ ] **Step 1: Update GetTopRisks query**

Find the `GetTopRisks` query and add `'flight_risk', 'burnout_risk', 'team_health', 'org_health'` to the IN clause.

- [ ] **Step 2: Regenerate sqlc**

Run: `cd /Users/anna/Documents/ai-management-brain && sqlc generate`

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/ai-management-brain && go vet ./...`

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add sql/queries/execution_signals.sql internal/db/sqlc/
git commit -m "feat: add halaos signal types to GetTopRisks whitelist"
```

---

### Task 13: Brain — Webhook Handler

**Files:**
- Create: `internal/api/halaos_webhook.go`
- Modify: `internal/api/router.go`

**Context:** Custom HMAC verification (NOT using WebhookVerifier middleware — that's provider-global, not per-tenant). Flow: read raw body → lookup halaos_link by company_id from payload → verify HMAC with tenant-specific secret → check idempotency → dispatch to mapper.

Brain uses `pgtype.UUID` for UUIDs. Use `parseUUID()` helper from `internal/api/handlers.go`.

- [ ] **Step 1: Write webhook handler**

```go
// HalaOSWebhookHandler handles incoming webhooks from HalaOS.
type HalaOSWebhookHandler struct {
    queries *sqlc.Queries
    mapper  *brain.HalaOSMapper
    logger  *slog.Logger
}

func NewHalaOSWebhookHandler(q *sqlc.Queries, mapper *brain.HalaOSMapper, logger *slog.Logger) *HalaOSWebhookHandler

// HandleWebhook is the main POST /api/v1/webhooks/halaos handler.
// 1. Read raw body
// 2. Parse envelope to get company_id
// 3. Lookup halaos_links by company_id → get tenant_id + webhook_secret
// 4. Verify HMAC-SHA256 (X-Signature-256 header, sha256=<hex> format)
// 5. Check idempotency (X-Event-ID as idempotency_key)
// 6. Dispatch to mapper by event_type
// 7. Log event to halaos_events
// 8. Return 200 OK
func (h *HalaOSWebhookHandler) HandleWebhook(c *gin.Context)
```

Webhook envelope from HalaOS (matches `WebhookPayload` struct):
```json
{
  "id": "uuid",
  "timestamp": "2026-03-26T00:00:00Z",
  "event_type": "hr.risk.updated",
  "data": { ... event-specific payload ... }
}
```

The `data` contains `hr_company_id` field for tenant lookup.

- [ ] **Step 2: Wire into router.go**

1. Add `HalaOSMapper *brain.HalaOSMapper` field to `RouterConfig` struct
2. In `NewRouter()`, construct the handler:
```go
if cfg.HalaOSMapper != nil {
    halaosHandler := NewHalaOSWebhookHandler(cfg.Queries, cfg.HalaOSMapper, cfg.Logger)
    webhooks.POST("/halaos", halaosHandler.HandleWebhook)
}
```
3. The webhook route is in the existing `webhooks` group (`r.Group("/webhooks")`) — no auth middleware (uses HMAC instead).
4. In the Brain's server setup (wherever `RouterConfig` is constructed), create the mapper and set it:
```go
halaosMapper := brain.NewHalaOSMapper(queries, logger)
cfg.HalaOSMapper = halaosMapper
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/ai-management-brain && go vet ./...`

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add internal/api/halaos_webhook.go internal/api/router.go
git commit -m "feat: add HalaOS webhook endpoint with HMAC verification"
```

---

### Task 14: Brain — HalaOS Mapper

**Files:**
- Create: `internal/brain/halaos_mapper.go`

**Context:** Maps HalaOS events to Brain's execution_signals and communication_events. Handles 3-tier employee matching (halaos_employee_id → halaos_employee_no → auto-create). Uses `pgtype.UUID` for all IDs.

For department subject_id in team_health: use UUID v5 `uuid.NewSHA1(uuid.NameSpaceDNS, []byte(tenantUUID + ":dept:" + deptName))`. Import `github.com/google/uuid` (already indirect dep).

- [ ] **Step 1: Write mapper**

```go
type HalaOSMapper struct {
    queries *sqlc.Queries
    logger  *slog.Logger
}

func NewHalaOSMapper(q *sqlc.Queries, logger *slog.Logger) *HalaOSMapper

// ResolveEmployee does 3-tier matching:
// 1. By halaos_employee_id
// 2. By halaos_employee_no
// 3. Auto-create
// Returns the Brain employee UUID.
func (m *HalaOSMapper) ResolveEmployee(ctx context.Context, tenantID pgtype.UUID, halaosEmpID int64, halaosEmpNo, name string) (pgtype.UUID, error)

// MapRiskUpdated converts hr.risk.updated → execution_signal
func (m *HalaOSMapper) MapRiskUpdated(ctx context.Context, tenantID pgtype.UUID, data json.RawMessage) error

// MapBurnoutUpdated converts hr.burnout.updated → execution_signal
func (m *HalaOSMapper) MapBurnoutUpdated(ctx context.Context, tenantID pgtype.UUID, data json.RawMessage) error

// MapTeamHealthUpdated converts hr.team_health.updated → execution_signal
// Uses deterministic UUID v5 for department subject_id
func (m *HalaOSMapper) MapTeamHealthUpdated(ctx context.Context, tenantID pgtype.UUID, data json.RawMessage) error

// MapBlindspotDetected converts hr.blindspot.detected → communication_event
func (m *HalaOSMapper) MapBlindspotDetected(ctx context.Context, tenantID pgtype.UUID, data json.RawMessage) error

// MapAttendanceAnomaly converts hr.attendance.anomaly → communication_events (one per anomaly)
func (m *HalaOSMapper) MapAttendanceAnomaly(ctx context.Context, tenantID pgtype.UUID, data json.RawMessage) error

// MapOrgSnapshot converts hr.org_snapshot.weekly → execution_signal
func (m *HalaOSMapper) MapOrgSnapshot(ctx context.Context, tenantID pgtype.UUID, data json.RawMessage) error
```

Each Map method: parse data JSON → resolve employee(s) → call `CreateExecutionSignal` or `CreateCommunicationEvent`.

- [ ] **Step 2: Verify build**

Run: `cd /Users/anna/Documents/ai-management-brain && go vet ./...`

- [ ] **Step 3: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add internal/brain/halaos_mapper.go
git commit -m "feat: add HalaOS mapper for event→signal/event conversion"
```

---

### Task 15: Brain — Context Service Enhancement

**Files:**
- Modify: `internal/brain/context_service.go`

**Context:** Add `HRInsightsContext` to `CompanyContext` struct. Query execution_signals for HalaOS-sourced data. Add to `FormatContextForPrompt()`.

- [ ] **Step 1: Add HRInsightsContext struct**

```go
type HRInsightsContext struct {
    HighRiskEmployees int     `json:"high_risk_employees,omitempty"` // flight_risk score > 70
    HighBurnoutCount  int     `json:"high_burnout_count,omitempty"`  // burnout score > 70
    AvgTeamHealth     float64 `json:"avg_team_health,omitempty"`
    ActiveBlindSpots  int     `json:"active_blindspots,omitempty"`   // last 30 days
    RecentAnomalies   int     `json:"recent_anomalies,omitempty"`    // last 7 days
}
```

Add `HRInsights *HRInsightsContext` to `CompanyContext` struct.

- [ ] **Step 2: Populate HRInsights in GetCompanyContext()**

After the existing data aggregation, add:
```go
// HalaOS HR Insights (only if signals exist)
hrInsights := &HRInsightsContext{}
thirtyDaysAgo := pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -30), Valid: true}
sevenDaysAgo := pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true}

// Note: CountHighRiskSignals uses @min_score::numeric param
highRisk, _ := q.CountHighRiskSignals(ctx, sqlc.CountHighRiskSignalsParams{
    TenantID: tenantID, SignalType: "flight_risk", MinScore: "70", GeneratedAt: thirtyDaysAgo,
})
hrInsights.HighRiskEmployees = int(highRisk)
// ... similar for burnout (min_score=70), anomalies (CountRecentCommunicationEvents), blindspots
// Only set cc.HRInsights if at least one field > 0
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/anna/Documents/ai-management-brain && go vet ./...`

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add internal/brain/context_service.go
git commit -m "feat: add HRInsights to CompanyContext from HalaOS signals"
```

---

### Task 16: Brain — Unit Tests

**Files:**
- Create: `internal/api/halaos_webhook_test.go`
- Create: `internal/brain/halaos_mapper_test.go`

**Context:** Brain test pattern uses `mockDBTX` from `internal/api/handlers_test.go`. For the mapper, mock the sqlc queries.

- [ ] **Step 1: Write webhook handler tests**

Tests:
- `TestHalaOSWebhook_MissingSignature` → 401
- `TestHalaOSWebhook_InvalidSignature` → 401
- `TestHalaOSWebhook_UnknownCompany` → 404
- `TestHalaOSWebhook_DuplicateEvent` → 200 (idempotent skip)
- `TestHalaOSWebhook_ValidRiskEvent` → 200

- [ ] **Step 2: Write mapper tests**

Tests:
- `TestResolveEmployee_ExactMatch` — finds by halaos_employee_id
- `TestResolveEmployee_FallbackToNo` — finds by halaos_employee_no
- `TestResolveEmployee_AutoCreate` — creates new employee
- `TestMapRiskUpdated_CreatesSignal` — verify signal_type, score, reasons
- `TestMapBlindspotDetected_CreatesEvent` — verify event_type, source_type

- [ ] **Step 3: Run tests**

Run: `cd /Users/anna/Documents/ai-management-brain && go test ./internal/... -v -count=1`

- [ ] **Step 4: Commit**

```bash
cd /Users/anna/Documents/ai-management-brain
git add internal/api/halaos_webhook_test.go internal/brain/halaos_mapper_test.go
git commit -m "test: add halaos webhook and mapper unit tests"
```

---

### Task 17: Integration Verification

**Files:** None (verification only)

- [ ] **Step 1: HalaOS build + tests**

Run:
```bash
cd /Users/anna/Documents/aigonhr
go vet ./...
go test ./... -count=1
```
Expected: All pass

- [ ] **Step 2: Brain build + tests**

Run:
```bash
cd /Users/anna/Documents/ai-management-brain
go vet ./...
go test ./... -count=1
```
Expected: All pass

- [ ] **Step 3: Verify no regressions**

Check that existing tests in both codebases still pass, especially:
- HalaOS: `internal/integration/...` (accounting tests still pass)
- Brain: `internal/api/...` (existing handler tests still pass)

- [ ] **Step 4: Push both repos**

```bash
cd /Users/anna/Documents/aigonhr && git push
cd /Users/anna/Documents/ai-management-brain && git push
```
