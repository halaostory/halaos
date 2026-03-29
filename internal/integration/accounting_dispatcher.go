package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/halaostory/halaos/internal/store"
)

// AccountingDispatcher polls the outbox and delivers webhooks to AIStarlight.
type AccountingDispatcher struct {
	queries    *store.Queries
	httpClient *http.Client
	logger     *slog.Logger
	batchSize  int32
	interval   time.Duration
}

// NewAccountingDispatcher creates a new dispatcher.
func NewAccountingDispatcher(queries *store.Queries, logger *slog.Logger) *AccountingDispatcher {
	return &AccountingDispatcher{
		queries: queries,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:    logger,
		batchSize: 20,
		interval:  5 * time.Second,
	}
}

// Run starts the dispatcher loop. Blocks until ctx is cancelled.
func (d *AccountingDispatcher) Run(ctx context.Context) {
	d.logger.Info("accounting dispatcher started")
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("accounting dispatcher stopped")
			return
		case <-ticker.C:
			d.processBatch(ctx)
		}
	}
}

func (d *AccountingDispatcher) processBatch(ctx context.Context) {
	events, err := d.queries.ListPendingOutboxEvents(ctx, d.batchSize)
	if err != nil {
		d.logger.Error("failed to list pending outbox events", "error", err)
		return
	}

	for _, event := range events {
		link, err := d.queries.GetActiveAccountingLink(ctx, event.CompanyID)
		if err != nil {
			d.logger.Warn("no active accounting link for company, skipping",
				"company_id", event.CompanyID, "event_id", event.ID)
			continue
		}

		if err := d.deliver(ctx, link, event); err != nil {
			d.logger.Error("webhook delivery failed",
				"event_id", event.ID,
				"error", err,
			)
			_ = d.queries.MarkOutboxFailed(ctx, store.MarkOutboxFailedParams{
				ID:           event.ID,
				ErrorMessage: strPtr(err.Error()),
			})
		} else {
			_ = d.queries.MarkOutboxSent(ctx, event.ID)
			_ = d.queries.UpdateAccountingLinkSyncedAt(ctx, link.ID)
		}
	}
}

func (d *AccountingDispatcher) deliver(ctx context.Context, link store.AccountingLink, event store.AccountingOutbox) error {
	payload := WebhookPayload{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		EventType: event.EventType,
		Data:      json.RawMessage(event.Payload),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	// HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(link.WebhookSecret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	endpoint := link.ApiEndpoint + "/api/v1/webhooks/aigonhr"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Event-Type", event.EventType)
	req.Header.Set("X-Event-ID", payload.ID)
	req.Header.Set("Authorization", "Bearer "+link.ApiKeyEnc)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(respBody))
}

