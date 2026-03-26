package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tonypk/aigonhr/internal/store"
)

// BrainOutbox writes events into brain_outbox for reliable delivery to AI Management Brain.
type BrainOutbox struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewBrainOutbox creates a new outbox writer.
func NewBrainOutbox(queries *store.Queries, logger *slog.Logger) *BrainOutbox {
	return &BrainOutbox{queries: queries, logger: logger}
}

// Enqueue inserts an event into the brain outbox. It is idempotent via a
// date-scoped idempotency key, allowing recurring weekly score recalculations
// to overwrite stale same-day entries without creating duplicates.
func (o *BrainOutbox) Enqueue(ctx context.Context, companyID int64, eventType, aggregateType string, aggregateID int64, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	idempotencyKey := fmt.Sprintf("%s:%s:%d:%s", eventType, aggregateType, aggregateID, time.Now().Format("2006-01-02"))

	_, err = o.queries.InsertBrainOutbox(ctx, store.InsertBrainOutboxParams{
		CompanyID:      companyID,
		EventType:      eventType,
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		Payload:        data,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			o.logger.Debug("brain outbox event already enqueued today, skipping",
				"event_type", eventType,
				"aggregate_type", aggregateType,
				"aggregate_id", aggregateID,
				"company_id", companyID,
				"idempotency_key", idempotencyKey,
			)
			return nil
		}
		return fmt.Errorf("insert brain outbox: %w", err)
	}

	o.logger.Debug("brain event enqueued",
		"event_type", eventType,
		"aggregate_type", aggregateType,
		"aggregate_id", aggregateID,
		"company_id", companyID,
		"idempotency_key", idempotencyKey,
	)
	return nil
}
