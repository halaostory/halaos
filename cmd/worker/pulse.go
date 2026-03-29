package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

// distributePulseSurveys creates new rounds for active pulse surveys that need distribution.
// Runs hourly but checks frequency to decide if a new round is needed.
func distributePulseSurveys(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	// Only run at 9 AM
	hour := time.Now().Hour()
	if hour != 9 {
		return
	}

	// Get all companies
	rows, err := pool.Query(ctx, `SELECT DISTINCT id FROM companies`)
	if err != nil {
		logger.Error("pulse: failed to list companies", "error", err)
		return
	}
	defer rows.Close()

	var companyIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			companyIDs = append(companyIDs, id)
		}
	}

	totalCreated := 0
	for _, companyID := range companyIDs {
		surveys, err := queries.ListActiveSurveysForDistribution(ctx, companyID)
		if err != nil {
			logger.Error("pulse: failed to list surveys for distribution", "company_id", companyID, "error", err)
			continue
		}

		for _, survey := range surveys {
			if !shouldDistribute(survey.Frequency) {
				continue
			}

			// Count active employees
			count, err := queries.CountActiveEmployees(ctx, companyID)
			if err != nil || count == 0 {
				continue
			}

			// Create new round
			_, err = queries.CreatePulseRound(ctx, store.CreatePulseRoundParams{
				SurveyID:  survey.ID,
				CompanyID: companyID,
				RoundDate: time.Now(),
				TotalSent: int32(count),
			})
			if err != nil {
				logger.Error("pulse: failed to create round", "survey_id", survey.ID, "error", err)
				continue
			}

			totalCreated++
			logger.Info("pulse: created new round", "survey_id", survey.ID, "survey_title", survey.Title, "employees", count)
		}
	}

	if totalCreated > 0 {
		logger.Info("pulse: distribution complete", "rounds_created", totalCreated)
	}
}

// shouldDistribute checks if today matches the survey's frequency schedule.
func shouldDistribute(frequency string) bool {
	now := time.Now()
	weekday := now.Weekday()
	day := now.Day()

	switch frequency {
	case "weekly":
		return weekday == time.Monday
	case "biweekly":
		// Every other Monday (week number is even)
		_, week := now.ISOWeek()
		return weekday == time.Monday && week%2 == 0
	case "monthly":
		return day == 1
	case "one_time":
		return true // distribute once when first activated
	default:
		return false
	}
}

// closeExpiredPulseRounds closes rounds that have been open for more than 7 days.
func closeExpiredPulseRounds(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	rows, err := pool.Query(ctx,
		`SELECT id FROM pulse_rounds WHERE status = 'open' AND round_date < CURRENT_DATE - INTERVAL '7 days'`)
	if err != nil {
		logger.Error("pulse: failed to find expired rounds", "error", err)
		return
	}
	defer rows.Close()

	closed := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			if err := queries.ClosePulseRound(ctx, id); err == nil {
				closed++
			}
		}
	}

	if closed > 0 {
		logger.Info("pulse: closed expired rounds", "count", closed)
	}
}
