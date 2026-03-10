package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/analytics/blindspot"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

// detectManagerBlindSpots runs weekly on Monday to identify patterns managers might miss.
func detectManagerBlindSpots(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	if time.Now().Weekday() != time.Monday {
		return
	}

	logger.Info("running weekly manager blind spot detection")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("blind spot: failed to list companies", "error", err)
		return
	}

	scorer := blindspot.NewScorer(pool, logger)
	weekDate := time.Now().Truncate(24 * time.Hour)
	totalSpots := 0

	for _, company := range companies {
		spots, err := scorer.DetectAll(ctx, company.ID)
		if err != nil {
			logger.Error("blind spot: failed to detect for company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		if len(spots) == 0 {
			continue
		}

		if err := scorer.UpsertSpots(ctx, company.ID, spots, weekDate); err != nil {
			logger.Error("blind spot: failed to persist",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}

		totalSpots += len(spots)

		// Notify each manager about their blind spots
		notifyManagerBlindSpots(ctx, queries, logger, company.ID, spots, weekDate)
	}

	if totalSpots > 0 {
		logger.Info("blind spot detection completed",
			"companies", len(companies),
			"total_spots", totalSpots,
		)
	}
}

// notifyManagerBlindSpots sends a summary notification to each manager.
func notifyManagerBlindSpots(
	ctx context.Context,
	queries *store.Queries,
	logger *slog.Logger,
	companyID int64,
	spots []blindspot.BlindSpot,
	weekDate time.Time,
) {
	// Group spots by manager
	byManager := make(map[int64][]blindspot.BlindSpot)
	for _, spot := range spots {
		byManager[spot.ManagerID] = append(byManager[spot.ManagerID], spot)
	}

	entityType := "blind_spot"

	for mgrID, mgrSpots := range byManager {
		// Resolve manager user ID
		mgr, err := queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
			ID:        mgrID,
			CompanyID: companyID,
		})
		if err != nil || mgr.UserID == nil {
			continue
		}

		// Dedup: check if already sent this week
		sent, _ := queries.HasReminderBeenSent(ctx, store.HasReminderBeenSentParams{
			CompanyID:     companyID,
			ReminderType:  "blind_spot_weekly",
			EntityType:    &entityType,
			Column4:       mgrID,
			ScheduledDate: weekDate,
		})
		if sent {
			continue
		}

		// Build summary
		highCount := 0
		for _, s := range mgrSpots {
			if s.Severity == "high" {
				highCount++
			}
		}

		title := "Weekly Team Insights"
		msg := fmt.Sprintf(
			"We detected %d pattern(s) in your team that may need attention",
			len(mgrSpots),
		)
		if highCount > 0 {
			msg += fmt.Sprintf(" (%d high priority)", highCount)
		}
		msg += ". Review them to stay ahead of potential issues."

		actions := []notification.NotificationAction{
			{Label: "View Insights", Route: "/org-intelligence"},
		}

		notification.Notify(ctx, queries, logger, companyID, *mgr.UserID, title, msg, "ai_reminder", &entityType, nil, actions)

		// Record dedup
		if _, err := queries.InsertAIReminder(ctx, store.InsertAIReminderParams{
			CompanyID:     companyID,
			UserID:        *mgr.UserID,
			ReminderType:  "blind_spot_weekly",
			EntityType:    &entityType,
			EntityID:      &mgrID,
			ScheduledDate: weekDate,
		}); err != nil {
			logger.Warn("failed to insert blind_spot_weekly reminder", "error", err)
		}
	}
}
