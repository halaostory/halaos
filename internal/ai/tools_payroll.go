package ai

import (
	"context"
	"fmt"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/store"
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

// payrollDefs returns tool definitions for payroll-related tools.
func payrollDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_payslip",
			Description: "Get payslip details for the current user. Returns the most recent payslip with gross pay, deductions, net pay, and breakdown.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "Number of recent payslips to return. Default 1."},
				},
			}),
		},
		{
			Name:        "analyze_payroll_anomalies",
			Description: "Run anomaly detection on a payroll cycle. Detects: pay deviations vs. history, zero contributions, excessive overtime, missing tax, negative net pay, work day anomalies, and salary jumps. Returns categorized anomalies with severity levels.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cycle_id": map[string]any{"type": "integer", "description": "The payroll cycle ID to analyze. If omitted, analyzes the most recent cycle."},
				},
			}),
		},
	}
}

// registerPayrollTools registers payroll-related tool executors.
func (r *ToolRegistry) registerPayrollTools() {
	r.tools["query_payslip"] = r.toolQueryPayslip
	r.tools["analyze_payroll_anomalies"] = r.toolAnalyzePayrollAnomalies
}
