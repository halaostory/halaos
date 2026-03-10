package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/store"
)

// snapshotScoreHistory copies current scores into history tables and computes org-level aggregates.
// Runs weekly on Monday only, after individual scorers have completed.
func snapshotScoreHistory(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	if time.Now().Weekday() != time.Monday {
		return
	}

	logger.Info("running weekly score history snapshot")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("score history: failed to list companies", "error", err)
		return
	}

	weekDate := mondayOfWeek(time.Now())

	for _, company := range companies {
		if err := snapshotCompany(ctx, queries, pool, company.ID, weekDate, logger); err != nil {
			logger.Error("score history: failed for company",
				"company_id", company.ID,
				"error", err,
			)
		}
	}

	logger.Info("score history snapshot completed", "companies", len(companies))
}

func snapshotCompany(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, companyID int64, weekDate time.Time, logger *slog.Logger) error {
	// Snapshot flight risk scores
	flightRisks, err := queries.ListAllFlightRiskScores(ctx, companyID)
	if err != nil {
		logger.Warn("score history: no flight risk data", "company_id", companyID, "error", err)
		flightRisks = nil
	}

	var totalFlightRisk float64
	highRiskCount := 0
	for _, fr := range flightRisks {
		if err := queries.InsertFlightRiskHistory(ctx, store.InsertFlightRiskHistoryParams{
			CompanyID:  companyID,
			EmployeeID: fr.EmployeeID,
			RiskScore:  fr.RiskScore,
			Factors:    fr.Factors,
			WeekDate:   weekDate,
		}); err != nil {
			logger.Warn("score history: failed to insert flight risk",
				"employee_id", fr.EmployeeID, "error", err)
		}
		totalFlightRisk += float64(fr.RiskScore)
		if fr.RiskScore >= 70 {
			highRiskCount++
		}
	}

	// Snapshot burnout scores
	burnouts, err := queries.ListAllBurnoutScores(ctx, companyID)
	if err != nil {
		logger.Warn("score history: no burnout data", "company_id", companyID, "error", err)
		burnouts = nil
	}

	var totalBurnout float64
	highBurnoutCount := 0
	for _, b := range burnouts {
		if err := queries.InsertBurnoutHistory(ctx, store.InsertBurnoutHistoryParams{
			CompanyID:    companyID,
			EmployeeID:   b.EmployeeID,
			BurnoutScore: b.BurnoutScore,
			Factors:      b.Factors,
			WeekDate:     weekDate,
		}); err != nil {
			logger.Warn("score history: failed to insert burnout",
				"employee_id", b.EmployeeID, "error", err)
		}
		totalBurnout += float64(b.BurnoutScore)
		if b.BurnoutScore >= 60 {
			highBurnoutCount++
		}
	}

	// Snapshot team health scores
	teamHealths, err := queries.ListAllTeamHealthScores(ctx, companyID)
	if err != nil {
		logger.Warn("score history: no team health data", "company_id", companyID, "error", err)
		teamHealths = nil
	}

	var totalTeamHealth float64
	lowHealthCount := 0
	for _, th := range teamHealths {
		if err := queries.InsertTeamHealthHistory(ctx, store.InsertTeamHealthHistoryParams{
			CompanyID:      companyID,
			DepartmentID:   th.DepartmentID,
			DepartmentName: th.DepartmentName,
			HealthScore:    th.HealthScore,
			Factors:        th.Factors,
			WeekDate:       weekDate,
		}); err != nil {
			logger.Warn("score history: failed to insert team health",
				"department_id", th.DepartmentID, "error", err)
		}
		totalTeamHealth += float64(th.HealthScore)
		if th.HealthScore < 50 {
			lowHealthCount++
		}
	}

	// Compute org-level averages
	totalEmployees := len(flightRisks)
	if len(burnouts) > totalEmployees {
		totalEmployees = len(burnouts)
	}
	totalDepts := len(teamHealths)

	avgFR := avgScore(totalFlightRisk, len(flightRisks))
	avgBO := avgScore(totalBurnout, len(burnouts))
	avgTH := avgScore(totalTeamHealth, len(teamHealths))

	metadata, _ := json.Marshal(map[string]any{
		"snapshot_time":   time.Now().Format(time.RFC3339),
		"flight_risk_n":   len(flightRisks),
		"burnout_n":       len(burnouts),
		"team_health_n":   len(teamHealths),
	})

	return queries.UpsertOrgScoreSnapshot(ctx, store.UpsertOrgScoreSnapshotParams{
		CompanyID:          companyID,
		WeekDate:           weekDate,
		AvgFlightRisk:      floatToNumeric(avgFR),
		AvgBurnout:         floatToNumeric(avgBO),
		AvgTeamHealth:      floatToNumeric(avgTH),
		HighRiskCount:      int32(highRiskCount),
		HighBurnoutCount:   int32(highBurnoutCount),
		LowHealthDeptCount: int32(lowHealthCount),
		TotalEmployees:     int32(totalEmployees),
		TotalDepartments:   int32(totalDepts),
		Metadata:           metadata,
	})
}

// mondayOfWeek returns the Monday of the current ISO week.
func mondayOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := t.AddDate(0, 0, -int(weekday-time.Monday))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

func avgScore(total float64, count int) float64 {
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func floatToNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", f))
	return n
}
