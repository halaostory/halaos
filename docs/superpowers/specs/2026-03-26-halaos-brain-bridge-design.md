# HalaOS → AI Management Brain Data Bridge — Design Spec

> **Date:** 2026-03-26
> **Status:** Approved
> **Scope:** Sub-project 1 of Communication Intelligence Layer
> **Codebases:** AIGoNHR (HalaOS) + AI Management Brain (boss-ai-agent)

---

## Goal

Push real HR analytics data (risk scores, burnout, team health, blind spots, attendance anomalies, org snapshots) from HalaOS to AI Management Brain via signed webhooks, so the Boss AI Agent can give management advice based on actual employee data instead of only chat-derived signals.

## Architecture

```
HalaOS (AIGoNHR)                         AI Management Brain
┌─────────────────┐                     ┌──────────────────────┐
│ Score Crons      │                     │ POST /webhooks/halaos│
│  flight_risk ────┤                     │  ├─ HMAC verify      │
│  burnout ────────┤   brain_outbox      │  ├─ Idempotency check│
│  team_health ────┼──► dispatcher ──►  │  ├─ Employee mapping │
│  blind_spots ────┤   (5s poll,HMAC)    │  ├─ → exec_signals   │
│  attendance ─────┤                     │  ├─ → comm_events    │
│  org_snapshot ───┘                     │  └─ → context_service│                     │                      │
│                  │                     │ halaos_links         │
│ brain_links      │                     │ halaos_events        │
│ brain_outbox     │                     └──────────────────────┘
└─────────────────┘
```

**Integration pattern:** Push via event outbox (reusing the accounting_outbox pattern from AIStarlight integration). HMAC-SHA256 signed webhooks, idempotent delivery, exponential backoff retry.

---

## HalaOS Side Changes

### New Tables

#### `brain_links`

Per-company link to a Brain tenant. Mirrors `accounting_links` pattern.

| Column | Type | Constraints |
|--------|------|-------------|
| id | BIGSERIAL | PRIMARY KEY |
| company_id | BIGINT | NOT NULL, UNIQUE, FK companies(id) |
| brain_tenant_id | UUID | NOT NULL |
| api_endpoint | TEXT | NOT NULL (e.g. `https://manageaibrain.com`) |
| api_key_enc | VARCHAR(500) | NOT NULL (encrypted, matches accounting_links pattern) |
| webhook_secret | VARCHAR(500) | NOT NULL |
| is_active | BOOLEAN | NOT NULL DEFAULT true |
| last_synced_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |

#### `brain_outbox`

Event queue — identical structure to `accounting_outbox`.

| Column | Type | Constraints |
|--------|------|-------------|
| id | BIGSERIAL | PRIMARY KEY |
| company_id | BIGINT | NOT NULL |
| event_type | VARCHAR(100) | NOT NULL |
| aggregate_type | VARCHAR(50) | NOT NULL |
| aggregate_id | BIGINT | NOT NULL |
| payload | JSONB | NOT NULL |
| idempotency_key | VARCHAR(200) | NOT NULL, UNIQUE |
| status | VARCHAR(20) | NOT NULL DEFAULT 'pending' |
| retry_count | INT | NOT NULL DEFAULT 0 |
| max_retries | INT | NOT NULL DEFAULT 5 |
| next_retry_at | TIMESTAMPTZ | |
| sent_at | TIMESTAMPTZ | |
| error_message | TEXT | |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |

**Indexes:**
- `idx_brain_outbox_pending` ON (status, next_retry_at) WHERE status IN ('pending', 'failed')
- `idx_brain_outbox_company` ON (company_id, created_at DESC)

### Event Types

| Event Type | Trigger | Aggregate Type | Payload Fields |
|-----------|---------|----------------|----------------|
| `hr.risk.updated` | `flightrisk.Scorer.UpsertScores()` | employee | `{employee_id, employee_no, name, department, risk_score, factors[], prev_score}` |
| `hr.burnout.updated` | `burnout.Scorer.UpsertScores()` | employee | `{employee_id, employee_no, name, department, burnout_score, factors[], prev_score}` |
| `hr.team_health.updated` | `teamhealth.Scorer.UpsertScores()` | department | `{department_id, department_name, health_score, factors[], prev_score}` |
| `hr.blindspot.detected` | `blindspot.Scorer.UpsertSpots()` | manager | `{manager_id, manager_name, spot_type, severity, title, description, employees[{id, employee_no, name, detail}]}` |
| `hr.attendance.anomaly` | Daily attendance cron | company | `{date, anomalies: [{employee_id, employee_no, name, type, detail}]}` |
| `hr.org_snapshot.weekly` | Weekly analytics cron (Monday — runs with other scorers) | company | `{avg_flight_risk, avg_burnout, avg_team_health, high_risk_count, high_burnout_count, headcount, low_health_dept_count}` |

### New Files

| File | Purpose |
|------|---------|
| `db/migrations/00087_brain_integration.sql` | brain_links + brain_outbox tables (goose format: `-- +goose Up` / `-- +goose Down` markers) |
| `db/query/brain_integration.sql` | sqlc queries (CRUD for brain_links, outbox operations) |
| `internal/integration/brain_events.go` | Event payload struct definitions |
| `internal/integration/brain_emitter.go` | 6 Emit methods (one per event type) |
| `internal/integration/brain_outbox.go` | Enqueue logic (idempotent insertion). **Idempotency key format**: `{event_type}:{aggregate_type}:{aggregate_id}:{date}` e.g. `hr.risk.updated:employee:123:2026-03-26`. Date component ensures recurring daily/weekly score recalculations create new outbox entries. |
| `internal/integration/brain_dispatcher.go` | Poll loop (5s interval, batch 20, HMAC-SHA256 signed POST) |

### Hook Injection Points

| Scorer | File | After Method | Emit Call |
|--------|------|-------------|-----------|
| Flight Risk | `internal/analytics/flightrisk/scorer.go` | `UpsertScores()` | `EmitRiskUpdated()` per employee with score change |
| Burnout | `internal/analytics/burnout/scorer.go` | `UpsertScores()` | `EmitBurnoutUpdated()` per employee with score change |
| Team Health | `internal/analytics/teamhealth/scorer.go` | `UpsertScores()` | `EmitTeamHealthUpdated()` per department |
| Blind Spots | `internal/analytics/blindspot/scorer.go` | `UpsertSpots()` | `EmitBlindspotDetected()` per new spot. **Note:** Existing `AffectedEmployee` struct has `{ID, Name, Detail}` but no `EmployeeNo`. The emitter must JOIN employee_no from the employees table when building the payload. |
| Attendance | Daily cron (TBD — may be in cmd/api or scheduler) | After processing | `EmitAttendanceAnomaly()` batch |
| Org Snapshot | Weekly cron (Sunday) | After `org_score_snapshots` insert | `EmitOrgSnapshot()` |

### Dispatcher Details

Mirrors `accounting_dispatcher.go`:
- Poll interval: 5 seconds
- Batch size: 20 events per cycle
- Delivery: POST to `{brain_links.api_endpoint}/webhooks/halaos` (Brain's webhook group is at `/webhooks`, no `/api/v1` prefix)
- Headers: `X-Signature-256` (value: `sha256=<hex_digest>`, matching Brain's `WebhookVerifier` convention), `X-Event-Type`, `X-Event-ID`, `Content-Type: application/json`
- Auth: `Authorization: Bearer {brain_links.api_key_enc}` (decrypted before use)
- **Note:** This differs from `accounting_dispatcher.go` which uses `X-Webhook-Signature` with raw hex. The Brain dispatcher must use Brain's `sha256=` prefix convention.
- Retry: Exponential backoff (1min, 5min, 15min, 1hr, 4hr), max 5 retries
- On success: status → 'sent', set sent_at
- On failure: increment retry_count, set next_retry_at, log error_message

---

## Brain Side Changes

### Database Migration

File: `sql/migrations/000016_halaos_bridge.up.sql`

#### Employee linking fields

```sql
ALTER TABLE employees ADD COLUMN halaos_employee_id BIGINT;
ALTER TABLE employees ADD COLUMN halaos_employee_no TEXT;
CREATE UNIQUE INDEX idx_employees_halaos_id
  ON employees(tenant_id, halaos_employee_id)
  WHERE halaos_employee_id IS NOT NULL;
```

#### `halaos_links` table

Per-tenant webhook configuration.

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PRIMARY KEY DEFAULT gen_random_uuid() |
| tenant_id | UUID | NOT NULL, UNIQUE, FK tenants(id) ON DELETE CASCADE |
| webhook_secret | TEXT | NOT NULL |
| halaos_company_id | BIGINT | NOT NULL |
| is_active | BOOLEAN | NOT NULL DEFAULT true |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() |

#### `halaos_events` table

Audit log + idempotency.

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PRIMARY KEY DEFAULT gen_random_uuid() |
| tenant_id | UUID | NOT NULL, FK tenants(id) ON DELETE CASCADE |
| event_type | TEXT | NOT NULL |
| idempotency_key | TEXT | NOT NULL |
| payload | JSONB | NOT NULL |
| processed_at | TIMESTAMPTZ | NOT NULL DEFAULT now() |

**Indexes:**
- UNIQUE ON (tenant_id, idempotency_key)
- ON (tenant_id, processed_at DESC)

### Webhook Endpoint

```
POST /api/v1/webhooks/halaos
```

**Processing flow:**

1. **Lookup tenant**: Extract `company_id` from payload → find `halaos_links` record → get `tenant_id`
2. **HMAC verify**: Custom verification (NOT reusing `WebhookVerifier` middleware, since that uses a global provider→secret map). Instead: read raw body, compute HMAC-SHA256 with `halaos_links.webhook_secret`, compare with `X-Signature-256` header (`sha256=<hex>` format)
3. **Idempotency check**: Check `halaos_events` for duplicate `idempotency_key`. If exists → 200 OK (skip)
4. **Employee mapping** (for employee-scoped events):
   - Query `employees WHERE tenant_id = ? AND halaos_employee_id = ?`
   - If not found: query `employees WHERE tenant_id = ? AND halaos_employee_no = ?`
   - If still not found: INSERT new employee with `name` from payload, `role='member'`, set `halaos_employee_id` + `halaos_employee_no`
5. **Event dispatch**: Route by `event_type` to handler
6. **Log event**: INSERT into `halaos_events`
7. **Return 200 OK**

### Event → Signal/Event Mapping

| HalaOS Event | Brain Target Table | Mapping |
|-------------|-------------------|---------|
| `hr.risk.updated` | `execution_signals` | `signal_type='flight_risk'`, `score=risk_score`, `subject_type='employee'`, `subject_id=brain_employee_uuid`, `reasons=factors[]` |
| `hr.burnout.updated` | `execution_signals` | `signal_type='burnout_risk'`, `score=burnout_score`, `subject_type='employee'`, `subject_id=brain_employee_uuid`, `reasons=factors[]` |
| `hr.team_health.updated` | `execution_signals` | `signal_type='team_health'`, `score=health_score`, `subject_type='department'`, `subject_id=uuid.NewSHA1(uuid.NameSpaceDNS, []byte(tenantID+":dept:"+deptName))` (deterministic UUID v5), `reasons=factors[]` |
| `hr.blindspot.detected` | `communication_events` | `event_type='blindspot_detected'`, `source_type='halaos'`, `actor_id=manager_brain_uuid`, `payload={spot_type, severity, title, employees[]}`, `confidence=1.0` |
| `hr.attendance.anomaly` | `communication_events` | One event per anomaly employee: `event_type='attendance_anomaly'`, `source_type='halaos'`, `actor_id=employee_brain_uuid`, `payload={type, detail}`, `confidence=1.0` |
| `hr.org_snapshot.weekly` | `execution_signals` | `signal_type='org_health'`, `subject_type='organization'`, `subject_id=tenant_id` (cast to UUID), `score=avg_team_health`, `reasons=[summary stats including high_risk_count, headcount, etc.]` |

### Context Service Enhancement

Modify `internal/brain/context_service.go`:

```go
type CompanyContext struct {
    Organization *OrgContext        // existing
    Goals        []GoalContext      // existing
    Metrics      []MetricContext    // existing
    TopRisks     []RiskContext      // existing
    TeamSize     int                // existing
    HRInsights   *HRInsightsContext // NEW
}

type HRInsightsContext struct {
    HighRiskEmployees int     // flight_risk score > 70
    HighBurnoutCount  int     // burnout score > 70
    AvgTeamHealth     float64 // company-wide average
    ActiveBlindSpots  int     // unresolved blind spots (last 30 days)
    RecentAnomalies   int     // attendance anomalies in last 7 days
}
```

`GetCompanyContext()` queries `execution_signals` for recent HalaOS-sourced signals and aggregates into `HRInsightsContext`. This data is automatically included in every LLM prompt via `FormatContextForPrompt()`.

### New Files (Brain side)

| File | Purpose |
|------|---------|
| `sql/migrations/000016_halaos_bridge.up.sql` | employees columns + halaos_links + halaos_events |
| `sql/queries/halaos.sql` | sqlc queries for halaos_links, halaos_events, employee mapping |
| `internal/api/halaos_webhook.go` | Webhook handler + event dispatch |
| `internal/brain/halaos_mapper.go` | Employee mapping + event→signal/event conversion |

### Required sqlc Query Changes

**New queries** (in `sql/queries/halaos.sql`):
- `GetHalaOSLinkByCompanyID` — lookup tenant by HalaOS company_id
- `CreateHalaOSEvent` — insert into halaos_events (idempotency log)
- `GetHalaOSEventByKey` — check idempotency_key exists
- `GetEmployeeByHalaOSID` — match by `tenant_id + halaos_employee_id`
- `GetEmployeeByHalaOSNo` — match by `tenant_id + halaos_employee_no`
- `CreateEmployeeFromHalaOS` — INSERT with `halaos_employee_id`, `halaos_employee_no`, `name`, `tenant_id`, `role`
- `UpdateEmployeeHalaOSLink` — SET `halaos_employee_id`, `halaos_employee_no` on existing employee
- `CountHRSignalsByType` — aggregate counts for HRInsightsContext (flight_risk>70, burnout>70, etc.)

**Modified queries** (in `sql/queries/execution_signals.sql`):
- `GetTopRisks` — **MUST** add `'flight_risk', 'burnout_risk', 'team_health', 'org_health'` to the hardcoded `signal_type IN (...)` whitelist, otherwise HalaOS signals will be silently excluded

### MCP Tool Impact

- `getTopRisks()` — after query fix, now includes `flight_risk` and `burnout_risk` signals from HalaOS
- `getExecutionSignals()` — already uses `ListExecutionSignals` (no type filter), so HalaOS signals appear automatically
- `getEmployeeProfile()` — employee's risk/burnout signals appear in their signal history
- `getCompanyState()` — `HRInsights` section added via context_service enhancement
- No MCP TypeScript code changes needed — all enrichment happens at the SQL/Go level

---

## Employee Matching Strategy

**Approach: Hybrid explicit ID + employee_no fallback**

HalaOS webhook payloads always include `employee_id` (BIGINT), `employee_no` (TEXT), and `name` (TEXT).

Brain matching priority:
1. **Exact match** by `halaos_employee_id` — fastest, most reliable
2. **Fallback** by `halaos_employee_no` within tenant — reliable if employee_no is stable
3. **Auto-create** — if no match found, create new Brain employee with name from payload, role='member', set both `halaos_employee_id` and `halaos_employee_no`

No email dependency. No Telegram ID dependency. No fuzzy name matching.

---

## Security

- **HMAC-SHA256**: Every webhook signed with shared secret. Brain verifies before processing.
- **Idempotency**: `idempotency_key` in both `brain_outbox` (HalaOS) and `halaos_events` (Brain) prevents duplicate processing.
- **No sensitive data in payloads**: No SSN, salary amounts, personal addresses, or bank details. Only scores, factors, department names, employee names.
- **Rate limiting**: Brain webhook endpoint: 100 requests/minute per tenant.
- **API key auth**: `Authorization: Bearer` header on every webhook request.

---

## Admin Configuration Flow

### HalaOS side (Company Settings → "AI Brain Integration")

1. Enter Brain tenant ID + API endpoint (e.g. `https://manageaibrain.com`)
2. System generates webhook_secret (32-byte random hex)
3. Display webhook_secret for admin to copy
4. Toggle is_active to enable/disable push

### Brain side (Tenant Settings → "HalaOS Connection")

1. Enter HalaOS company_id
2. Paste webhook_secret from HalaOS
3. System creates `halaos_links` record
4. "Test Connection" button sends a ping event to verify

---

## Testing Strategy

| Layer | Scope | Method |
|-------|-------|--------|
| HalaOS unit tests | `brain_emitter.go` | MockDBTX: verify Enqueue called with correct event_type + payload |
| HalaOS unit tests | `brain_dispatcher.go` | Mock HTTP: verify HMAC signature, headers, retry logic |
| Brain unit tests | webhook handler | Mock store: verify HMAC verification, idempotency, event routing |
| Brain unit tests | employee mapping | Mock store: verify 3-tier matching (id → employee_no → auto-create) |
| Brain unit tests | event→signal mapping | Mock store: verify correct signal_type, score, subject mapping |
| Integration test | End-to-end | curl with signed payload → verify Brain DB records created |

---

## Out of Scope

- H5 mobile UI changes (not needed)
- MCP TypeScript code changes (all enrichment at SQL/Go level)
- WebSocket real-time push (future iteration)
- Reverse sync Brain → HalaOS (future iteration)
- Admin UI for configuration (can be done via API/DB initially, UI in future)
