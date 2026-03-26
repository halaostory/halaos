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
	"strings"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

// BrainDispatcher polls the brain_outbox and delivers webhooks to AI Management Brain.
type BrainDispatcher struct {
	queries    *store.Queries
	httpClient *http.Client
	logger     *slog.Logger
	batchSize  int32
	interval   time.Duration
}

// NewBrainDispatcher creates a new dispatcher.
func NewBrainDispatcher(queries *store.Queries, logger *slog.Logger) *BrainDispatcher {
	return &BrainDispatcher{
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
func (d *BrainDispatcher) Run(ctx context.Context) {
	d.logger.Info("brain dispatcher started")
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("brain dispatcher stopped")
			return
		case <-ticker.C:
			d.processBatch(ctx)
		}
	}
}

func (d *BrainDispatcher) processBatch(ctx context.Context) {
	events, err := d.queries.ListPendingBrainOutbox(ctx, d.batchSize)
	if err != nil {
		d.logger.Error("failed to list pending brain outbox events", "error", err)
		return
	}

	for _, event := range events {
		link, err := d.queries.GetActiveBrainLink(ctx, event.CompanyID)
		if err != nil {
			d.logger.Warn("no active brain link for company, skipping",
				"company_id", event.CompanyID, "event_id", event.ID)
			continue
		}

		if err := d.deliver(ctx, link, event); err != nil {
			d.logger.Error("brain webhook delivery failed",
				"event_id", event.ID,
				"error", err,
			)
			_ = d.queries.MarkBrainOutboxFailed(ctx, store.MarkBrainOutboxFailedParams{
				ID:           event.ID,
				ErrorMessage: strPtr(err.Error()),
			})
		} else {
			_ = d.queries.MarkBrainOutboxSent(ctx, event.ID)
			_ = d.queries.UpdateBrainLinkSyncedAt(ctx, link.ID)
		}
	}
}

func (d *BrainDispatcher) deliver(ctx context.Context, link store.BrainLink, event store.BrainOutbox) error {
	envelope := WebhookPayload{
		ID:        fmt.Sprintf("%d", event.ID),
		Timestamp: event.CreatedAt,
		EventType: event.EventType,
		Data:      json.RawMessage(event.Payload),
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	// HMAC-SHA256 signature in Brain convention: "sha256=<hex_digest>"
	mac := hmac.New(sha256.New, []byte(link.WebhookSecret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	endpoint := strings.TrimRight(link.ApiEndpoint, "/") + "/webhooks/halaos"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-256", signature)
	req.Header.Set("X-Event-Type", event.EventType)
	req.Header.Set("X-Event-ID", fmt.Sprintf("%d", event.ID))
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
	return fmt.Errorf("brain webhook returned %d: %s", resp.StatusCode, string(respBody))
}
