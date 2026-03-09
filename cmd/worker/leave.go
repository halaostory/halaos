package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

func accrueLeaveBalances(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	now := time.Now()
	if now.Day() != 1 {
		return // Only run on the 1st of each month
	}

	year := int32(now.Year())
	logger.Info("running monthly leave accrual", "month", fmt.Sprintf("%d-%02d", now.Year(), now.Month()))

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies", "error", err)
		return
	}

	totalAccrued := 0
	for _, company := range companies {
		count, err := accrueForCompany(ctx, queries, company.ID, year, logger)
		if err != nil {
			logger.Error("failed to accrue leaves for company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}
		totalAccrued += count
	}

	logger.Info("monthly leave accrual completed",
		"companies", len(companies),
		"balances_accrued", totalAccrued,
	)
}

func accrueForCompany(ctx context.Context, queries *store.Queries, companyID int64, year int32, logger *slog.Logger) (int, error) {
	leaveTypes, err := queries.ListLeaveTypes(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list leave types: %w", err)
	}

	// Filter to monthly accrual types
	monthlyTypes := make([]store.LeaveType, 0)
	for _, lt := range leaveTypes {
		if lt.AccrualType == "monthly" {
			monthlyTypes = append(monthlyTypes, lt)
		}
	}
	if len(monthlyTypes) == 0 {
		return 0, nil
	}

	employees, err := queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return 0, fmt.Errorf("list active employees: %w", err)
	}

	count := 0
	for _, emp := range employees {
		for _, lt := range monthlyTypes {
			// Check gender-specific eligibility
			if lt.GenderSpecific != nil && emp.Gender != nil && *lt.GenderSpecific != *emp.Gender {
				continue
			}

			// Calculate cumulative accrual: (default_days / 12) * months_elapsed
			defaultDays := numericToFloat(lt.DefaultDays)
			if defaultDays <= 0 {
				continue
			}
			monthsElapsed := float64(time.Now().Month())
			cumulativeEarned := (defaultDays / 12.0) * monthsElapsed

			// Round to 1 decimal
			cumulativeEarned = math.Round(cumulativeEarned*10) / 10

			var earned pgtype.Numeric
			_ = earned.Scan(fmt.Sprintf("%.1f", cumulativeEarned))

			var carried pgtype.Numeric
			_ = carried.Scan("0")

			_, err := queries.UpsertLeaveBalance(ctx, store.UpsertLeaveBalanceParams{
				CompanyID:   companyID,
				EmployeeID:  emp.ID,
				LeaveTypeID: lt.ID,
				Year:        year,
				Earned:      earned,
				Carried:     carried,
			})
			if err != nil {
				logger.Error("failed to upsert leave balance",
					"company_id", companyID,
					"employee_id", emp.ID,
					"leave_type_id", lt.ID,
					"error", err,
				)
				continue
			}
			count++
		}
	}

	return count, nil
}

func processLeaveCarryover(ctx context.Context, queries *store.Queries, logger *slog.Logger) {
	now := time.Now()
	// Run only in January (carry over from previous year)
	if now.Month() != time.January || now.Day() != 1 {
		return
	}

	prevYear := int32(now.Year() - 1)
	newYear := int32(now.Year())
	logger.Info("processing year-end leave carryover", "from_year", prevYear, "to_year", newYear)

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("failed to list companies for carryover", "error", err)
		return
	}

	totalCarried := 0
	for _, company := range companies {
		balances, err := queries.ListLeaveBalancesForCarryover(ctx, store.ListLeaveBalancesForCarryoverParams{
			CompanyID: company.ID,
			Year:      prevYear,
		})
		if err != nil {
			logger.Error("failed to list balances for carryover", "company_id", company.ID, "error", err)
			continue
		}

		for _, bal := range balances {
			earned := numericToFloat(bal.Earned)
			used := numericToFloat(bal.Used)
			carried := numericToFloat(bal.Carried)
			adjusted := numericToFloat(bal.Adjusted)
			remaining := earned + carried + adjusted - used

			if remaining <= 0 {
				continue
			}

			// Apply max carryover cap
			maxCarry := numericToFloat(bal.MaxCarryover)
			if maxCarry <= 0 {
				maxCarry = 5 // Default 5 days
			}
			carryAmount := math.Min(remaining, maxCarry)
			carryAmount = math.Round(carryAmount*10) / 10

			var carriedNum pgtype.Numeric
			_ = carriedNum.Scan(fmt.Sprintf("%.1f", carryAmount))
			var zeroNum pgtype.Numeric
			_ = zeroNum.Scan("0")

			_, err := queries.UpsertLeaveBalance(ctx, store.UpsertLeaveBalanceParams{
				CompanyID:   company.ID,
				EmployeeID:  bal.EmployeeID,
				LeaveTypeID: bal.LeaveTypeID,
				Year:        newYear,
				Earned:      zeroNum,
				Carried:     carriedNum,
			})
			if err != nil {
				logger.Error("failed to create carryover balance",
					"employee_id", bal.EmployeeID,
					"leave_type_id", bal.LeaveTypeID,
					"error", err,
				)
				continue
			}

			forfeited := remaining - carryAmount
			logger.Info("leave carryover processed",
				"employee", fmt.Sprintf("%s %s (%s)", bal.FirstName, bal.LastName, bal.EmployeeNo),
				"leave_type", bal.LeaveTypeName,
				"remaining", remaining,
				"carried", carryAmount,
				"forfeited", forfeited,
			)
			totalCarried++
		}
	}

	logger.Info("leave carryover completed", "balances_carried", totalCarried)
}
