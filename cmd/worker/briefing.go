package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/orgintel"
	"github.com/tonypk/aigonhr/internal/store"
)

// generateExecutiveBriefings generates weekly AI briefings for all companies.
// Runs on Monday only, after snapshotScoreHistory. Skips if AI is not enabled.
func generateExecutiveBriefings(ctx context.Context, queries *store.Queries, prov provider.Provider, logger *slog.Logger) {
	if time.Now().Weekday() != time.Monday {
		return
	}

	if prov == nil {
		logger.Info("executive briefing: skipped (AI provider not configured)")
		return
	}

	logger.Info("running weekly executive briefing generation")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("executive briefing: failed to list companies", "error", err)
		return
	}

	weekDate := mondayOfWeek(time.Now())
	gen := orgintel.NewBriefingGenerator(queries, prov, logger)
	generated := 0

	for _, company := range companies {
		// Skip if briefing already exists for this week
		existing, err := queries.GetExecutiveBriefingByWeek(ctx, store.GetExecutiveBriefingByWeekParams{
			CompanyID: company.ID,
			WeekDate:  weekDate,
		})
		if err == nil && existing.ID > 0 {
			continue
		}

		result, err := gen.Generate(ctx, company.ID)
		if err != nil {
			logger.Error("executive briefing: failed for company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		generated++
		logger.Info("executive briefing generated",
			"company_id", company.ID,
			"tokens_used", result.TokensUsed,
		)
	}

	logger.Info("executive briefing generation completed",
		"companies", len(companies),
		"generated", generated,
	)
}
