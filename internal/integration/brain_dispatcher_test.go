package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

func newTestDispatcher(mockDB *testutil.MockDBTX) *BrainDispatcher {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewBrainDispatcher(queries, logger)
}

// brainOutboxEventScanValues returns ordered scan values for a BrainOutbox row
// as returned by ListPendingBrainOutbox.
// Order: id, company_id, event_type, aggregate_type, aggregate_id, payload,
//
//	idempotency_key, status, retry_count, max_retries, next_retry_at,
//	sent_at, error_message, created_at
func brainOutboxEventScanValues(id, companyID int64, eventType string, payload []byte) []interface{} {
	return []interface{}{
		id,                          // id
		companyID,                   // company_id
		eventType,                   // event_type
		"employee",                  // aggregate_type
		int64(10),                   // aggregate_id
		payload,                     // payload (json.RawMessage)
		eventType + ":employee:10",  // idempotency_key
		"pending",                   // status
		int32(0),                    // retry_count
		int32(3),                    // max_retries
		pgtype.Timestamptz{},        // next_retry_at
		pgtype.Timestamptz{},        // sent_at
		(*string)(nil),              // error_message
		time.Now(),                  // created_at
	}
}

// brainDispatcherLinkScanValues returns ordered scan values for a BrainLink row.
func brainDispatcherLinkScanValues(companyID int64, apiEndpoint, apiKeyEnc, webhookSecret string) []interface{} {
	return []interface{}{
		int64(1),                    // id
		companyID,                   // company_id
		uuid.MustParse("00000000-0000-0000-0000-000000000002"), // brain_tenant_id
		apiEndpoint,                 // api_endpoint
		apiKeyEnc,                   // api_key_enc
		webhookSecret,               // webhook_secret
		true,                        // is_active
		pgtype.Timestamptz{},        // last_synced_at
		time.Now(),                  // created_at
		time.Now(),                  // updated_at
	}
}

// TestBrainDispatcher_SignatureFormat verifies that the dispatcher sends the
// HMAC-SHA256 signature in the format "sha256=<hex>" via the X-Signature-256 header.
func TestBrainDispatcher_SignatureFormat(t *testing.T) {
	const webhookSecret = "test-webhook-secret"
	const apiKey = "test-api-key"

	var receivedSig string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := []byte(`{"event_id":"test"}`)
	link := store.BrainLink{
		ID:            1,
		CompanyID:     1,
		ApiEndpoint:   srv.URL,
		ApiKeyEnc:     apiKey,
		WebhookSecret: webhookSecret,
		IsActive:      true,
	}
	event := store.BrainOutbox{
		ID:        99,
		CompanyID: 1,
		EventType: "hr.risk.updated",
		Payload:   json.RawMessage(payload),
		CreatedAt: time.Now(),
	}

	mockDB := testutil.NewMockDBTX()
	d := newTestDispatcher(mockDB)

	err := d.deliver(context.Background(), link, event)
	require.NoError(t, err)

	// Verify the signature header starts with "sha256="
	assert.True(t, strings.HasPrefix(receivedSig, "sha256="), "expected signature to start with 'sha256=', got: %s", receivedSig)

	// Verify the HMAC value is correct
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(receivedBody)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expectedSig, receivedSig)
}

// TestBrainDispatcher_Headers verifies that the dispatcher sets all required
// HTTP headers: X-Signature-256, X-Event-Type, X-Event-ID, Content-Type, Authorization.
func TestBrainDispatcher_Headers(t *testing.T) {
	const webhookSecret = "header-test-secret"
	const apiKey = "header-api-key"
	const eventType = "hr.burnout.updated"

	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	link := store.BrainLink{
		ID:            2,
		CompanyID:     1,
		ApiEndpoint:   srv.URL,
		ApiKeyEnc:     apiKey,
		WebhookSecret: webhookSecret,
		IsActive:      true,
	}
	event := store.BrainOutbox{
		ID:        101,
		CompanyID: 1,
		EventType: eventType,
		Payload:   json.RawMessage(`{"event_id":"burnout-test"}`),
		CreatedAt: time.Now(),
	}

	mockDB := testutil.NewMockDBTX()
	d := newTestDispatcher(mockDB)

	err := d.deliver(context.Background(), link, event)
	require.NoError(t, err)

	// Content-Type
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))

	// X-Event-Type
	assert.Equal(t, eventType, capturedHeaders.Get("X-Event-Type"))

	// X-Event-ID must be the event ID as a string
	assert.Equal(t, "101", capturedHeaders.Get("X-Event-ID"))

	// X-Signature-256 must be present and start with "sha256="
	sig := capturedHeaders.Get("X-Signature-256")
	assert.NotEmpty(t, sig)
	assert.True(t, strings.HasPrefix(sig, "sha256="), "expected 'sha256=' prefix, got: %s", sig)

	// Authorization must use Bearer scheme
	assert.Equal(t, "Bearer "+apiKey, capturedHeaders.Get("Authorization"))
}

// TestBrainDispatcher_NoPendingEvents verifies that processBatch is a no-op
// when ListPendingBrainOutbox returns an empty set of rows.
func TestBrainDispatcher_NoPendingEvents(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	d := newTestDispatcher(mockDB)

	// ListPendingBrainOutbox returns empty rows — no events to process.
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	// processBatch should complete without panicking or querying anything else.
	d.processBatch(context.Background())

	calls := mockDB.Calls()
	// Only one DB call should have been made (the ListPendingBrainOutbox Query).
	require.Len(t, calls, 1)
	assert.Equal(t, "Query", calls[0].Method)
}
