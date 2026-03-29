package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

func scanContractMilestones(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	logger.Info("scanning contract milestones")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies for milestone scan", "error", err)
		return
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	totalCreated := 0

	for _, company := range companies {
		// 1. Probation ending (Philippine law: 6 months max)
		probationary, err := queries.ListProbationaryEmployees(ctx, company.ID)
		if err != nil {
			logger.Error("failed to list probationary employees", "company_id", company.ID, "error", err)
			continue
		}
		for _, emp := range probationary {
			// Probation ends 6 months after hire
			probationEnd := emp.HireDate.AddDate(0, 6, 0)
			daysRemaining := int(probationEnd.Sub(today).Hours() / 24)

			// Alert if within 30 days
			if daysRemaining <= 30 && daysRemaining >= -7 {
				_, err := queries.UpsertContractMilestone(ctx, store.UpsertContractMilestoneParams{
					CompanyID:     company.ID,
					EmployeeID:    emp.ID,
					MilestoneType: "probation_ending",
					MilestoneDate: probationEnd,
					DaysRemaining: int32(daysRemaining),
				})
				if err != nil {
					logger.Error("failed to upsert probation milestone", "employee_id", emp.ID, "error", err)
					continue
				}
				totalCreated++
			}
		}

		// 2. Contract expiring
		contractual, err := queries.ListContractualEmployees(ctx, company.ID)
		if err != nil {
			logger.Error("failed to list contractual employees", "company_id", company.ID, "error", err)
			continue
		}
		for _, emp := range contractual {
			if !emp.ContractEndDate.Valid {
				continue
			}
			contractEnd := emp.ContractEndDate.Time
			daysRemaining := int(contractEnd.Sub(today).Hours() / 24)

			// Alert if within 60 days
			if daysRemaining <= 60 && daysRemaining >= -7 {
				_, err := queries.UpsertContractMilestone(ctx, store.UpsertContractMilestoneParams{
					CompanyID:     company.ID,
					EmployeeID:    emp.ID,
					MilestoneType: "contract_expiring",
					MilestoneDate: contractEnd,
					DaysRemaining: int32(daysRemaining),
				})
				if err != nil {
					logger.Error("failed to upsert contract milestone", "employee_id", emp.ID, "error", err)
					continue
				}
				totalCreated++
			}
		}

		// 3. Upcoming work anniversaries (this month)
		month := int32(now.Month())
		anniversaries, err := queries.ListUpcomingAnniversaries(ctx, store.ListUpcomingAnniversariesParams{
			CompanyID: company.ID,
			Column2:   month,
			Column3:   1,
			Column4:   31,
		})
		if err != nil {
			logger.Error("failed to list anniversaries", "company_id", company.ID, "error", err)
			continue
		}
		for _, emp := range anniversaries {
			yearsOfService := now.Year() - emp.HireDate.Year()
			if yearsOfService <= 0 {
				continue
			}
			// Only milestone years: 1, 3, 5, 10, 15, 20, 25, 30
			if !isAnniversaryMilestone(yearsOfService) {
				continue
			}
			anniversaryDate := emp.HireDate.AddDate(yearsOfService, 0, 0)
			daysRemaining := int(anniversaryDate.Sub(today).Hours() / 24)

			_, err := queries.UpsertContractMilestone(ctx, store.UpsertContractMilestoneParams{
				CompanyID:     company.ID,
				EmployeeID:    emp.ID,
				MilestoneType: "anniversary",
				MilestoneDate: anniversaryDate,
				DaysRemaining: int32(daysRemaining),
			})
			if err != nil {
				logger.Error("failed to upsert anniversary milestone", "employee_id", emp.ID, "error", err)
				continue
			}
			totalCreated++
		}
	}

	logger.Info("contract milestone scan completed", "milestones_created_or_updated", totalCreated)
}

func isAnniversaryMilestone(years int) bool {
	switch years {
	case 1, 3, 5, 10, 15, 20, 25, 30:
		return true
	default:
		return false
	}
}
