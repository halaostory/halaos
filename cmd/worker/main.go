package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/config"
	"github.com/tonypk/aigonhr/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		logger.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	defer rdb.Close()

	queries := store.New(pool)

	logger.Info("worker started")

	// Event outbox processor
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processEvents(ctx, queries, logger)
			}
		}
	}()

	// Periodic jobs
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runPeriodicJobs(ctx, queries, rdb, logger)
			}
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig.String())
	cancel()
	time.Sleep(2 * time.Second)
	logger.Info("worker stopped")
}

func processEvents(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	events, err := queries.GetPendingEvents(ctx, 50)
	if err != nil {
		logger.Error("failed to get pending events", "error", err)
		return
	}

	for _, ev := range events {
		if err := dispatchEvent(ctx, queries, ev, logger); err != nil {
			logger.Error("event dispatch failed",
				"event_id", ev.ID,
				"event_type", ev.EventType,
				"error", err,
			)
			_ = queries.MarkEventFailed(ctx, store.MarkEventFailedParams{
				ID:           ev.ID,
				ErrorMessage: strPtr(err.Error()),
			})
			continue
		}
		_ = queries.MarkEventProcessed(ctx, ev.ID)
	}
}

func dispatchEvent(ctx context.Context, queries *store.Queries, ev store.HrEvent, logger *slog.Logger) error {
	logger.Info("processing event", "type", ev.EventType, "aggregate", ev.AggregateType, "id", ev.AggregateID)
	// TODO: route to specific handlers based on event type
	return nil
}

func runPeriodicJobs(ctx context.Context, queries *store.Queries, rdb *redis.Client, logger *slog.Logger) {
	logger.Info("running periodic jobs")
	// TODO: attendance auto-close, leave accrual, compliance monitoring
}

func strPtr(s string) *string { return &s }
