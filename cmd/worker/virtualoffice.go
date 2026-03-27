package main

import (
	"context"
	"log/slog"

	"github.com/tonypk/aigonhr/internal/store"
)

// clearStaleVOStatuses clears virtual office manual_status fields that were
// set before today. This prevents stale "focused" / "in_meeting" badges from
// carrying over to the next workday. Idempotent — safe to call hourly.
func clearStaleVOStatuses(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	if err := queries.ClearStaleManualStatuses(ctx); err != nil {
		logger.Error("failed to clear stale VO manual statuses", "error", err)
		return
	}
	logger.Info("cleared stale virtual office manual statuses")
}
