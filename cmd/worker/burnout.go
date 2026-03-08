package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/analytics/burnout"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

// calculateBurnoutScores scores all employees across all companies for burnout risk.
// Runs weekly on Monday only. Notifies managers when an employee crosses score 60.
func calculateBurnoutScores(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	if time.Now().Weekday() != time.Monday {
		return
	}

	logger.Info("running weekly burnout score calculation")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("burnout: failed to list companies", "error", err)
		return
	}

	scorer := burnout.NewScorer(pool, logger)
	totalScored := 0
	totalHighRisk := 0

	for _, company := range companies {
		scores, err := scorer.ScoreAll(ctx, company.ID)
		if err != nil {
			logger.Error("burnout: failed to score company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		if len(scores) == 0 {
			continue
		}

		if err := scorer.UpsertScores(ctx, company.ID, scores); err != nil {
			logger.Error("burnout: failed to upsert scores",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		totalScored += len(scores)

		// Notify managers for employees with burnout score >= 60
		highRisk := filterHighBurnout(scores, 60)
		if len(highRisk) == 0 {
			continue
		}

		totalHighRisk += len(highRisk)

		managers, err := queries.ListManagerUsersByCompany(ctx, company.ID)
		if err != nil || len(managers) == 0 {
			continue
		}

		for _, emp := range highRisk {
			entityType := "employee"
			entityID := emp.EmployeeID
			title := "Burnout Risk Alert"
			msg := fmt.Sprintf("%s (%s) has a burnout risk score of %d. Factors: %s",
				emp.Name, emp.EmployeeNo, emp.BurnoutScore, summarizeBurnoutFactors(emp.Factors))

			actions := []notification.NotificationAction{
				{Label: "View Employee", Route: fmt.Sprintf("/employees/%d", emp.EmployeeID), Action: "view_employee"},
			}

			for _, mgr := range managers {
				notification.Notify(ctx, queries, logger, company.ID, mgr.ID, title, msg, "ai_reminder", &entityType, &entityID, actions)
			}
		}
	}

	logger.Info("burnout score calculation completed",
		"companies", len(companies),
		"employees_scored", totalScored,
		"high_risk", totalHighRisk,
	)
}

func filterHighBurnout(scores []burnout.EmployeeBurnout, threshold int) []burnout.EmployeeBurnout {
	var result []burnout.EmployeeBurnout
	for _, s := range scores {
		if s.BurnoutScore >= threshold {
			result = append(result, s)
		}
	}
	return result
}

func summarizeBurnoutFactors(factors []burnout.BurnoutFactor) string {
	if len(factors) == 0 {
		return "none"
	}
	summary := ""
	for i, f := range factors {
		if i > 0 {
			summary += ", "
		}
		summary += f.Factor
	}
	return summary
}
