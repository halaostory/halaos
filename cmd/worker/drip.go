package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/tonypk/aigonhr/internal/email"
	"github.com/tonypk/aigonhr/internal/store"
)

// sendDripEmails checks for companies eligible for each drip step and sends emails.
// Runs hourly as part of periodic jobs.
func sendDripEmails(ctx context.Context, queries *store.Queries, emailSvc *email.Service, logger *slog.Logger) {
	if emailSvc == nil || !emailSvc.IsEnabled() {
		return
	}

	steps := []email.DripStep{
		email.DripGettingStarted,
		email.DripFirstEmployee,
		email.DripFirstPayroll,
		email.DripExploreFeatures,
	}

	totalSent := 0

	for _, step := range steps {
		delay := email.DripStepDelay(step)
		companies, err := queries.ListCompaniesForDrip(ctx, int32(step))
		if err != nil {
			logger.Error("failed to list companies for drip", "step", step, "error", err)
			continue
		}

		for _, co := range companies {
			// Check if company is old enough for this step
			age := time.Since(co.RegisteredAt)
			if age < time.Duration(delay)*24*time.Hour {
				continue
			}

			if err := emailSvc.SendDripEmail(co.AdminEmail, co.AdminFirstName, co.CompanyName, step); err != nil {
				logger.Error("failed to send drip email",
					"company_id", co.CompanyID,
					"step", step,
					"email", co.AdminEmail,
					"error", err,
				)
				continue
			}

			if err := queries.InsertDripEmail(ctx, store.InsertDripEmailParams{
				CompanyID: co.CompanyID,
				Step:      int32(step),
			}); err != nil {
				logger.Error("failed to record drip email",
					"company_id", co.CompanyID,
					"step", step,
					"error", err,
				)
			}

			totalSent++
		}
	}

	if totalSent > 0 {
		logger.Info("drip campaign completed", "emails_sent", totalSent)
	}
}
