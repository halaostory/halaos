package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
)

// generateComplianceAlerts scans all companies for compliance risks and upserts
// them into the compliance_alerts table. Runs daily.
// Alert types: document_expiry, contract_expiry, filing_overdue, filing_upcoming
func generateComplianceAlerts(ctx context.Context, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	logger.Info("running daily compliance alerts generation")

	companies, err := queries.ListAllCompanies(ctx)
	if err != nil {
		logger.Error("compliance: failed to list companies", "error", err)
		return
	}

	totalAlerts := 0
	totalCritical := 0

	for _, company := range companies {
		var alerts []complianceAlert

		// 1. Expiring documents
		docAlerts := checkExpiringDocuments(ctx, queries, company.ID, logger)
		alerts = append(alerts, docAlerts...)

		// 2. Expiring contracts
		contractAlerts := checkExpiringContracts(ctx, queries, company.ID, logger)
		alerts = append(alerts, contractAlerts...)

		// 3. Overdue filings
		overdueAlerts := checkOverdueFilings(ctx, queries, company.ID, logger)
		alerts = append(alerts, overdueAlerts...)

		// 4. Upcoming filings
		upcomingAlerts := checkUpcomingFilings(ctx, queries, company.ID, logger)
		alerts = append(alerts, upcomingAlerts...)

		if len(alerts) == 0 {
			continue
		}

		// Upsert alerts
		upserted := upsertComplianceAlerts(ctx, pool, company.ID, alerts, logger)
		totalAlerts += upserted

		// Notify admins for critical/high alerts
		critical := filterCriticalAlerts(alerts)
		totalCritical += len(critical)

		if len(critical) == 0 {
			continue
		}

		admins, err := queries.ListAdminUsersByCompany(ctx, company.ID)
		if err != nil || len(admins) == 0 {
			continue
		}

		for _, alert := range critical {
			entityType := alert.entityType
			title := fmt.Sprintf("[%s] %s", severityLabel(alert.severity), alert.title)

			for _, admin := range admins {
				notification.Notify(ctx, queries, logger, company.ID, admin.ID,
					title, alert.description, "compliance_alert", &entityType, alert.entityID)
			}
		}
	}

	logger.Info("compliance alerts generation completed",
		"companies", len(companies),
		"total_alerts", totalAlerts,
		"critical_high", totalCritical,
	)
}

// complianceAlert is an internal struct for compliance alert data.
type complianceAlert struct {
	alertType   string
	severity    string // critical, high, medium, low
	title       string
	description string
	entityType  string
	entityID    *int64
	dueDate     *time.Time
	daysRemain  int
}

// checkExpiringDocuments generates alerts for employee documents nearing expiry.
func checkExpiringDocuments(ctx context.Context, queries *store.Queries, companyID int64, logger *slog.Logger) []complianceAlert {
	docs, err := queries.List201ExpiringDocuments(ctx, companyID)
	if err != nil {
		logger.Warn("compliance: failed to list expiring documents", "company_id", companyID, "error", err)
		return nil
	}

	var alerts []complianceAlert
	now := time.Now().Truncate(24 * time.Hour)

	for _, doc := range docs {
		if !doc.ExpiryDate.Valid {
			continue
		}
		expiry := doc.ExpiryDate.Time
		days := int(math.Ceil(expiry.Sub(now).Hours() / 24))
		sev := severityByDays(days)

		empName := doc.FirstName + " " + doc.LastName
		entityID := doc.EmployeeID

		catName := "unknown"
		if doc.CategoryName != nil {
			catName = *doc.CategoryName
		}

		alerts = append(alerts, complianceAlert{
			alertType:   "document_expiry",
			severity:    sev,
			title:       fmt.Sprintf("Document Expiring: %s", empName),
			description: fmt.Sprintf("%s (%s) - %s document expires in %d days (due %s)", empName, doc.EmployeeNo, catName, days, expiry.Format("2006-01-02")),
			entityType:  "document",
			entityID:    &entityID,
			dueDate:     &expiry,
			daysRemain:  days,
		})
	}
	return alerts
}

// checkExpiringContracts generates alerts for contractual employees with upcoming separation dates.
func checkExpiringContracts(ctx context.Context, queries *store.Queries, companyID int64, logger *slog.Logger) []complianceAlert {
	now := time.Now().Truncate(24 * time.Hour)
	contracts, err := queries.ListExpiringContracts(ctx, store.ListExpiringContractsParams{
		CompanyID: companyID,
		Column2:   now,
	})
	if err != nil {
		logger.Warn("compliance: failed to list expiring contracts", "company_id", companyID, "error", err)
		return nil
	}

	var alerts []complianceAlert
	for _, c := range contracts {
		if !c.SeparationDate.Valid {
			continue
		}
		sepDate := c.SeparationDate.Time
		days := int(math.Ceil(sepDate.Sub(now).Hours() / 24))
		sev := severityByDays(days)

		empName := c.FirstName + " " + c.LastName
		entityID := c.ID

		alerts = append(alerts, complianceAlert{
			alertType:   "contract_expiry",
			severity:    sev,
			title:       fmt.Sprintf("Contract Expiring: %s", empName),
			description: fmt.Sprintf("%s (%s) - %s, contract ends in %d days (%s)", empName, c.EmployeeNo, c.PositionTitle, days, sepDate.Format("2006-01-02")),
			entityType:  "employee",
			entityID:    &entityID,
			dueDate:     &sepDate,
			daysRemain:  days,
		})
	}
	return alerts
}

// checkOverdueFilings generates critical alerts for overdue government filings.
func checkOverdueFilings(ctx context.Context, queries *store.Queries, companyID int64, logger *slog.Logger) []complianceAlert {
	filings, err := queries.ListOverdueFilings(ctx, companyID)
	if err != nil {
		logger.Warn("compliance: failed to list overdue filings", "company_id", companyID, "error", err)
		return nil
	}

	var alerts []complianceAlert
	now := time.Now().Truncate(24 * time.Hour)

	for _, f := range filings {
		dueDate := f.DueDate
		daysOverdue := int(math.Ceil(now.Sub(dueDate).Hours() / 24))
		entityID := f.ID
		period := formatFilingPeriod(f.PeriodType, f.PeriodYear, f.PeriodMonth, f.PeriodQuarter)

		alerts = append(alerts, complianceAlert{
			alertType:   "filing_overdue",
			severity:    "critical",
			title:       fmt.Sprintf("Overdue Filing: %s", f.FilingType),
			description: fmt.Sprintf("%s for %s is %d days overdue (due %s)", f.FilingType, period, daysOverdue, dueDate.Format("2006-01-02")),
			entityType:  "tax_filing",
			entityID:    &entityID,
			dueDate:     &dueDate,
			daysRemain:  -daysOverdue,
		})
	}
	return alerts
}

// checkUpcomingFilings generates alerts for upcoming government filings with graduated severity.
func checkUpcomingFilings(ctx context.Context, queries *store.Queries, companyID int64, logger *slog.Logger) []complianceAlert {
	filings, err := queries.ListUpcomingFilings(ctx, companyID)
	if err != nil {
		logger.Warn("compliance: failed to list upcoming filings", "company_id", companyID, "error", err)
		return nil
	}

	var alerts []complianceAlert
	now := time.Now().Truncate(24 * time.Hour)

	for _, f := range filings {
		dueDate := f.DueDate
		days := int(math.Ceil(dueDate.Sub(now).Hours() / 24))
		sev := severityByDays(days)
		entityID := f.ID
		period := formatFilingPeriod(f.PeriodType, f.PeriodYear, f.PeriodMonth, f.PeriodQuarter)

		alerts = append(alerts, complianceAlert{
			alertType:   "filing_upcoming",
			severity:    sev,
			title:       fmt.Sprintf("Upcoming Filing: %s", f.FilingType),
			description: fmt.Sprintf("%s for %s due in %d days (%s)", f.FilingType, period, days, dueDate.Format("2006-01-02")),
			entityType:  "tax_filing",
			entityID:    &entityID,
			dueDate:     &dueDate,
			daysRemain:  days,
		})
	}
	return alerts
}

// upsertComplianceAlerts persists alerts into the compliance_alerts table.
func upsertComplianceAlerts(ctx context.Context, pool *pgxpool.Pool, companyID int64, alerts []complianceAlert, logger *slog.Logger) int {
	count := 0
	for _, a := range alerts {
		var entityID int64
		if a.entityID != nil {
			entityID = *a.entityID
		}

		var dueDate *time.Time
		if a.dueDate != nil {
			d := *a.dueDate
			dueDate = &d
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO compliance_alerts (company_id, alert_type, severity, title, description,
				entity_type, entity_id, due_date, days_remaining, calculated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
			ON CONFLICT (company_id, alert_type, entity_type, COALESCE(entity_id, 0), due_date)
			DO UPDATE SET severity = $3, title = $4, description = $5,
				days_remaining = $9, calculated_at = NOW()
		`, companyID, a.alertType, a.severity, a.title, a.description,
			a.entityType, entityID, dueDate, a.daysRemain)
		if err != nil {
			logger.Error("compliance: failed to upsert alert",
				"company_id", companyID,
				"alert_type", a.alertType,
				"error", err,
			)
			continue
		}
		count++
	}
	return count
}

// severityByDays returns severity based on days remaining.
func severityByDays(days int) string {
	switch {
	case days <= 3:
		return "critical"
	case days <= 7:
		return "high"
	case days <= 14:
		return "medium"
	default:
		return "low"
	}
}

// severityLabel returns a human-readable severity label.
func severityLabel(sev string) string {
	switch sev {
	case "critical":
		return "CRITICAL"
	case "high":
		return "HIGH"
	case "medium":
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// filterCriticalAlerts returns only critical and high severity alerts.
func filterCriticalAlerts(alerts []complianceAlert) []complianceAlert {
	var result []complianceAlert
	for _, a := range alerts {
		if a.severity == "critical" || a.severity == "high" {
			result = append(result, a)
		}
	}
	return result
}

// formatFilingPeriod formats the filing period for display.
func formatFilingPeriod(periodType string, year int32, month, quarter *int32) string {
	switch periodType {
	case "monthly":
		if month != nil {
			return fmt.Sprintf("%s %d", time.Month(*month).String(), year)
		}
		return fmt.Sprintf("%d", year)
	case "quarterly":
		if quarter != nil {
			return fmt.Sprintf("Q%d %d", *quarter, year)
		}
		return fmt.Sprintf("%d", year)
	case "annual":
		return fmt.Sprintf("%d", year)
	default:
		return fmt.Sprintf("%d", year)
	}
}
