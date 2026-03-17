package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/tonypk/aigonhr/internal/store"
)

// AccountingOutbox writes events into accounting_outbox for reliable delivery.
type AccountingOutbox struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewAccountingOutbox creates a new outbox writer.
func NewAccountingOutbox(queries *store.Queries, logger *slog.Logger) *AccountingOutbox {
	return &AccountingOutbox{queries: queries, logger: logger}
}

// Enqueue inserts an event into the outbox. It is idempotent via idempotency_key.
func (o *AccountingOutbox) Enqueue(ctx context.Context, companyID int64, eventType, aggregateType string, aggregateID int64, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	idempotencyKey := fmt.Sprintf("%s:%s:%d", eventType, aggregateType, aggregateID)

	_, err = o.queries.InsertAccountingOutbox(ctx, store.InsertAccountingOutboxParams{
		CompanyID:      companyID,
		EventType:      eventType,
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		Payload:        data,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	o.logger.Info("accounting event enqueued",
		"event_type", eventType,
		"aggregate_type", aggregateType,
		"aggregate_id", aggregateID,
		"company_id", companyID,
	)
	return nil
}
