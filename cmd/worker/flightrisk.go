package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/analytics/flightrisk"
	"github.com/tonypk/aigonhr/internal/integration"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

// calculateFlightRisk scores all employees across all companies and upserts results.
// Runs weekly on Monday only. Notifies managers when an employee crosses score 70.
func calculateFlightRisk(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, brainEmitter *integration.BrainEmitter, logger *slog.Logger) {
	if time.Now().Weekday() != time.Monday {
		return
	}

	logger.Info("running weekly flight risk calculation")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("flight risk: failed to list companies", "error", err)
		return
	}

	scorer := flightrisk.NewScorer(pool, logger)
	totalScored := 0
	totalHighRisk := 0

	for _, company := range companies {
		risks, err := scorer.ScoreAll(ctx, company.ID)
		if err != nil {
			logger.Error("flight risk: failed to score company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		if len(risks) == 0 {
			continue
		}

		if err := scorer.UpsertScores(ctx, company.ID, risks); err != nil {
			logger.Error("flight risk: failed to upsert scores",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		// Emit brain events for each scored employee
		for _, risk := range risks {
			factors := make([]integration.EventFactor, len(risk.Factors))
			for i, f := range risk.Factors {
				factors[i] = integration.EventFactor{
					Factor: f.Factor,
					Points: f.Points,
					Detail: f.Detail,
				}
			}
			_ = brainEmitter.EmitRiskUpdated(ctx, company.ID, risk.EmployeeID, risk.EmployeeNo, risk.Name, risk.Department, risk.RiskScore, factors, 0)
		}

		totalScored += len(risks)

		// Notify managers for employees crossing score 70
		highRisk := filterHighRisk(risks, 70)
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
			title := "High Flight Risk Alert"
			msg := fmt.Sprintf("%s (%s) has a flight risk score of %d. Factors: %s",
				emp.Name, emp.EmployeeNo, emp.RiskScore, summarizeFactors(emp.Factors))

			actions := []notification.NotificationAction{
				{Label: "View Employee", Route: fmt.Sprintf("/employees/%d", emp.EmployeeID), Action: "view_employee"},
			}

			for _, mgr := range managers {
				notification.Notify(ctx, queries, logger, company.ID, mgr.ID, title, msg, "ai_reminder", &entityType, &entityID, actions)
			}
		}
	}

	logger.Info("flight risk calculation completed",
		"companies", len(companies),
		"employees_scored", totalScored,
		"high_risk", totalHighRisk,
	)
}

// filterHighRisk returns employees with risk score >= threshold.
func filterHighRisk(risks []flightrisk.EmployeeRisk, threshold int) []flightrisk.EmployeeRisk {
	var result []flightrisk.EmployeeRisk
	for _, r := range risks {
		if r.RiskScore >= threshold {
			result = append(result, r)
		}
	}
	return result
}

// summarizeFactors builds a short human-readable summary of risk factors.
func summarizeFactors(factors []flightrisk.RiskFactor) string {
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
