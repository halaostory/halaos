package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

// autoRegularize converts probationary employees to regular status when
// their 6-month probation period has elapsed. Philippine Labor Code
// Article 296 mandates regularization after 6 months.
func autoRegularize(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	logger.Info("checking for probationary employees due for regularization")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies for regularization", "error", err)
		return
	}

	today := time.Now().Truncate(24 * time.Hour)
	totalRegularized := 0

	for _, company := range companies {
		probationary, err := queries.ListProbationaryEmployees(ctx, company.ID)
		if err != nil {
			logger.Error("failed to list probationary employees", "company_id", company.ID, "error", err)
			continue
		}

		for _, emp := range probationary {
			probationEnd := emp.HireDate.AddDate(0, 6, 0)
			if today.Before(probationEnd) {
				continue // not yet due
			}

			// Auto-regularize
			result, err := queries.RegularizeEmployee(ctx, store.RegularizeEmployeeParams{
				ID:        emp.ID,
				CompanyID: company.ID,
			})
			if err != nil {
				logger.Error("failed to regularize employee",
					"employee_id", emp.ID,
					"employee_no", emp.EmployeeNo,
					"error", err,
				)
				continue
			}

			totalRegularized++
			logger.Info("auto-regularized employee",
				"employee_id", result.ID,
				"employee_no", result.EmployeeNo,
				"name", fmt.Sprintf("%s %s", result.FirstName, result.LastName),
				"hire_date", emp.HireDate.Format("2006-01-02"),
				"probation_end", probationEnd.Format("2006-01-02"),
			)

			// Mark the milestone as actioned
			milestones, _ := queries.ListPendingMilestonesByCompany(ctx, company.ID)
			for _, m := range milestones {
				if m.EmployeeID == emp.ID && m.MilestoneType == "probation_ending" {
					if _, err := queries.ActionMilestone(ctx, store.ActionMilestoneParams{
						ID:        m.ID,
						CompanyID: company.ID,
						Notes:     strPtr("Auto-regularized by system"),
					}); err != nil {
						logger.Warn("failed to action milestone", "milestone_id", m.ID, "error", err)
					}
				}
			}

			// Create notification for HR/admin
			if _, err := pool.Exec(ctx,
				`INSERT INTO notifications (company_id, user_id, title, body, category, priority)
				 SELECT $1, u.id, $2, $3, 'hr', 'info'
				 FROM users u WHERE u.company_id = $1 AND u.role IN ('admin', 'super_admin')`,
				company.ID,
				fmt.Sprintf("Employee Regularized: %s %s", result.FirstName, result.LastName),
				fmt.Sprintf("%s %s (%s) has been automatically regularized after completing the 6-month probation period.",
					result.FirstName, result.LastName, result.EmployeeNo),
			); err != nil {
				logger.Warn("failed to create regularization notification", "employee_id", emp.ID, "error", err)
			}
		}
	}

	if totalRegularized > 0 {
		logger.Info("auto-regularization completed", "regularized", totalRegularized)
	}
}
