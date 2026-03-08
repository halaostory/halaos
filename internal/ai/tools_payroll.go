package ai

import (
	"context"
	"fmt"

	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolQueryPayslip(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	limit := int32(1)
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int32(l)
	}
	if limit > 5 {
		limit = 5
	}

	payslips, err := r.queries.ListPayslips(ctx, store.ListPayslipsParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Limit:      limit,
		Offset:     0,
	})
	if err != nil {
		return "", fmt.Errorf("query payslips: %w", err)
	}
	return toJSON(payslips)
}

func (r *ToolRegistry) toolAnalyzePayrollAnomalies(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	var runID int64

	if cycleID, ok := input["cycle_id"].(float64); ok && cycleID > 0 {
		// Get the latest completed run for this cycle
		rid, err := r.queries.GetLatestCompletedRunForCycle(ctx, store.GetLatestCompletedRunForCycleParams{
			CycleID:   int64(cycleID),
			CompanyID: companyID,
		})
		if err != nil {
			return "", fmt.Errorf("no completed run found for cycle %d", int64(cycleID))
		}
		runID = rid
	} else {
		// Find the most recent completed run for this company
		cycles, err := r.queries.ListPayrollCycles(ctx, store.ListPayrollCyclesParams{
			CompanyID: companyID,
			Limit:     5,
			Offset:    0,
		})
		if err != nil || len(cycles) == 0 {
			return "No payroll cycles found", nil
		}
		for _, c := range cycles {
			rid, err := r.queries.GetLatestCompletedRunForCycle(ctx, store.GetLatestCompletedRunForCycleParams{
				CycleID:   c.ID,
				CompanyID: companyID,
			})
			if err == nil {
				runID = rid
				break
			}
		}
		if runID == 0 {
			return "No completed payroll runs found", nil
		}
	}

	calculator := payroll.NewCalculator(r.queries, r.pool, nil)
	report, err := calculator.DetectAnomalies(ctx, runID, companyID)
	if err != nil {
		return "", fmt.Errorf("anomaly detection failed: %w", err)
	}

	return toJSON(report)
}
