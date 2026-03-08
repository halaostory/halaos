package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolQueryCompanyAnalytics(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	summary, err := r.queries.GetAnalyticsSummary(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("get analytics summary: %w", err)
	}

	deptCosts, err := r.queries.GetDepartmentCostAnalysis(ctx, companyID)
	if err != nil {
		deptCosts = nil
	}

	type deptCost struct {
		Department    string `json:"department"`
		EmployeeCount int64  `json:"employee_count"`
		TotalSalary   string `json:"total_salary"`
	}
	departments := make([]deptCost, len(deptCosts))
	for i, d := range deptCosts {
		departments[i] = deptCost{
			Department:    d.DepartmentName,
			EmployeeCount: d.EmployeeCount,
			TotalSalary:   fmt.Sprintf("%v", d.TotalSalaryCost),
		}
	}

	return toJSON(map[string]any{
		"active_employees":     summary.ActiveEmployees,
		"separated_employees":  summary.SeparatedEmployees,
		"new_hires_this_month": summary.NewHiresThisMonth,
		"probationary_count":   summary.ProbationaryCount,
		"avg_tenure_years":     numericToString(summary.AvgTenureYears),
		"departments":          departments,
	})
}

func (r *ToolRegistry) toolQueryHeadcountTrend(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	startDate := time.Now().AddDate(-1, 0, 0)
	trends, err := r.queries.GetHeadcountTrend(ctx, store.GetHeadcountTrendParams{
		CompanyID: companyID,
		HireDate:  startDate,
	})
	if err != nil {
		return "", fmt.Errorf("get headcount trend: %w", err)
	}

	return toJSON(map[string]any{
		"period": fmt.Sprintf("%s to %s", startDate.Format("2006-01"), time.Now().Format("2006-01")),
		"months": trends,
	})
}

func (r *ToolRegistry) toolQueryLeaveUtilization(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	startDate := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	utilization, err := r.queries.GetLeaveUtilization(ctx, store.GetLeaveUtilizationParams{
		CompanyID: companyID,
		StartDate: startDate,
	})
	if err != nil {
		return "", fmt.Errorf("get leave utilization: %w", err)
	}

	return toJSON(map[string]any{
		"year":        time.Now().Year(),
		"utilization": utilization,
	})
}

func (r *ToolRegistry) toolQueryTaxFilingStatus(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	year := int32(time.Now().Year())
	summary, err := r.queries.GetFilingSummary(ctx, store.GetFilingSummaryParams{
		CompanyID:  companyID,
		PeriodYear: year,
	})
	if err != nil {
		return "", fmt.Errorf("get filing summary: %w", err)
	}

	overdue, _ := r.queries.ListOverdueFilings(ctx, companyID)
	upcoming, _ := r.queries.ListUpcomingFilings(ctx, companyID)

	type filingItem struct {
		ID         int64  `json:"id"`
		FilingType string `json:"filing_type"`
		DueDate    string `json:"due_date"`
		Status     string `json:"status"`
	}

	overdueItems := make([]filingItem, len(overdue))
	for i, f := range overdue {
		overdueItems[i] = filingItem{
			ID: f.ID, FilingType: f.FilingType,
			DueDate: f.DueDate.Format("2006-01-02"), Status: f.Status,
		}
	}

	upcomingItems := make([]filingItem, len(upcoming))
	for i, f := range upcoming {
		upcomingItems[i] = filingItem{
			ID: f.ID, FilingType: f.FilingType,
			DueDate: f.DueDate.Format("2006-01-02"), Status: f.Status,
		}
	}

	return toJSON(map[string]any{
		"year":            year,
		"total_filings":   summary.Total,
		"filed":           summary.Filed,
		"overdue_count":   summary.Overdue,
		"upcoming_count":  summary.Upcoming,
		"total_amount":    summary.TotalAmount,
		"total_penalties": summary.TotalPenalties,
		"overdue_items":   overdueItems,
		"upcoming_items":  upcomingItems,
	})
}

func (r *ToolRegistry) toolCreateTaxFilingRecord(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	filingType, _ := input["filing_type"].(string)
	if filingType == "" {
		return "", fmt.Errorf("filing_type is required (bir_1601c, sss_r3, philhealth_rf1, pagibig_ml1, bir_2316, bir_0619e)")
	}

	periodType := "monthly"
	if pt, ok := input["period_type"].(string); ok && pt != "" {
		periodType = pt
	}

	periodYear := int32(time.Now().Year())
	if py, ok := input["period_year"].(float64); ok && py > 0 {
		periodYear = int32(py)
	}

	var periodMonth, periodQuarter *int32
	if pm, ok := input["period_month"].(float64); ok && pm > 0 {
		m := int32(pm)
		periodMonth = &m
	}
	if pq, ok := input["period_quarter"].(float64); ok && pq > 0 {
		q := int32(pq)
		periodQuarter = &q
	}

	dueDateStr, _ := input["due_date"].(string)
	if dueDateStr == "" {
		return "", fmt.Errorf("due_date is required in YYYY-MM-DD format")
	}
	dueDate, err := time.Parse("2006-01-02", dueDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid due_date format")
	}

	var amount pgtype.Numeric
	if a, ok := input["amount"].(float64); ok && a > 0 {
		_ = amount.Scan(fmt.Sprintf("%.2f", a))
	}

	filing, err := r.queries.CreateTaxFiling(ctx, store.CreateTaxFilingParams{
		CompanyID:     companyID,
		FilingType:    filingType,
		PeriodType:    periodType,
		PeriodYear:    periodYear,
		PeriodMonth:   periodMonth,
		PeriodQuarter: periodQuarter,
		DueDate:       dueDate,
		Amount:        amount,
		Status:        "pending",
	})
	if err != nil {
		return "", fmt.Errorf("create tax filing: %w", err)
	}

	return toJSON(map[string]any{
		"success":   true,
		"filing_id": filing.ID,
		"type":      filing.FilingType,
		"due_date":  dueDateStr,
		"message":   fmt.Sprintf("Tax filing record '%s' created for %d.", filingType, periodYear),
	})
}
