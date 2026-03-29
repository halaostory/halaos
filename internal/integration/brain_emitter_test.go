package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

// brainLinkScanValues returns the ordered scan values for a BrainLink row.
// Order must match the SELECT column order in brain_integration.sql.go:
// id, company_id, brain_tenant_id, api_endpoint, api_key_enc, webhook_secret,
// is_active, last_synced_at, created_at, updated_at
func brainLinkScanValues(companyID int64, apiEndpoint, apiKeyEnc, webhookSecret string) []interface{} {
	return []interface{}{
		int64(1),                    // id
		companyID,                   // company_id
		uuid.MustParse("00000000-0000-0000-0000-000000000001"), // brain_tenant_id
		apiEndpoint,                 // api_endpoint
		apiKeyEnc,                   // api_key_enc
		webhookSecret,               // webhook_secret
		true,                        // is_active
		pgtype.Timestamptz{},        // last_synced_at
		time.Now(),                  // created_at
		time.Now(),                  // updated_at
	}
}

// brainOutboxScanValues returns the ordered scan values for a BrainOutbox row.
// Order must match the RETURNING column order in brain_integration.sql.go:
// id, company_id, event_type, aggregate_type, aggregate_id, payload,
// idempotency_key, status, retry_count, max_retries, next_retry_at,
// sent_at, error_message, created_at
func brainOutboxScanValues() []interface{} {
	return []interface{}{
		int64(42),                   // id
		int64(1),                    // company_id
		"hr.risk.updated",           // event_type
		"employee",                  // aggregate_type
		int64(10),                   // aggregate_id
		[]byte(`{"event_id":"x"}`),  // payload (json.RawMessage underlying type)
		"hr.risk.updated:employee:10:2026-03-26", // idempotency_key
		"pending",                   // status
		int32(0),                    // retry_count
		int32(3),                    // max_retries
		pgtype.Timestamptz{},        // next_retry_at
		pgtype.Timestamptz{},        // sent_at
		(*string)(nil),              // error_message
		time.Now(),                  // created_at
	}
}

func newTestEmitter(mockDB *testutil.MockDBTX) *BrainEmitter {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewBrainEmitter(queries, logger)
}

// TestEmitRiskUpdated_NoLink verifies that when GetActiveBrainLink returns an
// error (no active link configured), EmitRiskUpdated returns nil silently.
func TestEmitRiskUpdated_NoLink(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	emitter := newTestEmitter(mockDB)

	// GetActiveBrainLink will return an error — no link exists.
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	err := emitter.EmitRiskUpdated(
		context.Background(),
		1, 10,
		"EMP-001", "Alice", "Engineering",
		75,
		[]EventFactor{{Factor: "absence", Points: 20, Detail: "3 days"}},
		50,
	)

	require.NoError(t, err)
}

// TestEmitRiskUpdated_Success verifies that when GetActiveBrainLink succeeds
// and InsertBrainOutbox succeeds, EmitRiskUpdated returns nil.
func TestEmitRiskUpdated_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	emitter := newTestEmitter(mockDB)

	// First QueryRow: GetActiveBrainLink succeeds.
	mockDB.OnQueryRow(testutil.NewRow(brainLinkScanValues(1, "http://brain.test", "key123", "secret-abc")...))
	// Second QueryRow: InsertBrainOutbox succeeds.
	mockDB.OnQueryRow(testutil.NewRow(brainOutboxScanValues()...))

	err := emitter.EmitRiskUpdated(
		context.Background(),
		1, 10,
		"EMP-001", "Alice", "Engineering",
		75,
		[]EventFactor{{Factor: "absence", Points: 20, Detail: "3 days"}},
		50,
	)

	assert.NoError(t, err)
}

// TestEmitBurnoutUpdated_Success verifies that when GetActiveBrainLink and
// InsertBrainOutbox both succeed, EmitBurnoutUpdated returns nil.
func TestEmitBurnoutUpdated_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	emitter := newTestEmitter(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(brainLinkScanValues(1, "http://brain.test", "key123", "secret-abc")...))
	mockDB.OnQueryRow(testutil.NewRow(brainOutboxScanValues()...))

	err := emitter.EmitBurnoutUpdated(
		context.Background(),
		1, 10,
		"EMP-001", "Alice", "Engineering",
		80,
		[]EventFactor{{Factor: "overtime", Points: 30, Detail: "10h/week avg"}},
		60,
	)

	assert.NoError(t, err)
}

// TestEmitTeamHealthUpdated_Success verifies that EmitTeamHealthUpdated enqueues
// an event when an active brain link exists.
func TestEmitTeamHealthUpdated_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	emitter := newTestEmitter(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(brainLinkScanValues(1, "http://brain.test", "key123", "secret-abc")...))
	mockDB.OnQueryRow(testutil.NewRow(brainOutboxScanValues()...))

	err := emitter.EmitTeamHealthUpdated(
		context.Background(),
		1, 5,
		"Engineering",
		72,
		[]EventFactor{{Factor: "turnover", Points: 15, Detail: "2 exits"}},
		85,
	)

	assert.NoError(t, err)
}

// TestEmitBlindspotDetected_Success verifies that EmitBlindspotDetected enqueues
// an event when an active brain link exists.
func TestEmitBlindspotDetected_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	emitter := newTestEmitter(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(brainLinkScanValues(1, "http://brain.test", "key123", "secret-abc")...))
	mockDB.OnQueryRow(testutil.NewRow(brainOutboxScanValues()...))

	err := emitter.EmitBlindspotDetected(
		context.Background(),
		1, 20,
		"Bob Manager", "engagement", "high",
		"Low engagement in Engineering",
		"3 employees show below-average engagement scores",
		[]BlindspotEmployee{
			{ID: 10, EmployeeNo: "EMP-001", Name: "Alice", Detail: "score 30"},
		},
	)

	assert.NoError(t, err)
}

// TestEmitOrgSnapshot_Success verifies that EmitOrgSnapshot enqueues an event
// when an active brain link exists.
func TestEmitOrgSnapshot_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	emitter := newTestEmitter(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(brainLinkScanValues(1, "http://brain.test", "key123", "secret-abc")...))
	mockDB.OnQueryRow(testutil.NewRow(brainOutboxScanValues()...))

	err := emitter.EmitOrgSnapshot(
		context.Background(),
		1,
		42.5, 38.1, 76.3,
		5, 3, 120, 2,
	)

	assert.NoError(t, err)
}
