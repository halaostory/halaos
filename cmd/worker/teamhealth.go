package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/analytics/teamhealth"
	"github.com/tonypk/aigonhr/internal/store"
)

// calculateTeamHealth scores each department's health weekly (Monday only).
func calculateTeamHealth(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	if time.Now().Weekday() != time.Monday {
		return
	}

	logger.Info("running weekly team health calculation")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("team health: failed to list companies", "error", err)
		return
	}

	scorer := teamhealth.NewScorer(pool, logger)
	totalDepts := 0

	for _, company := range companies {
		scores, err := scorer.ScoreAll(ctx, company.ID)
		if err != nil {
			logger.Error("team health: failed to score company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		if len(scores) == 0 {
			continue
		}

		if err := scorer.UpsertScores(ctx, company.ID, scores); err != nil {
			logger.Error("team health: failed to upsert scores",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		totalDepts += len(scores)
	}

	logger.Info("team health calculation complete", "departments_scored", totalDepts)
}
