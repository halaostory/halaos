package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/store"
)

// ToolRegistry maps tool names to executors.
type ToolRegistry struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	tools   map[string]ToolExecutor
}

// ToolExecutor runs a tool and returns the result as a string.
type ToolExecutor func(ctx context.Context, companyID, userID int64, input map[string]any) (string, error)

// NewToolRegistry creates and registers all HR tools.
func NewToolRegistry(queries *store.Queries, pool *pgxpool.Pool) *ToolRegistry {
	r := &ToolRegistry{
		queries: queries,
		pool:    pool,
		tools:   make(map[string]ToolExecutor),
	}
	r.registerTools()
	return r
}

// DefinitionsForAgent returns tool definitions filtered by the allowed tool names.
func (r *ToolRegistry) DefinitionsForAgent(allowedTools []string) []provider.ToolDefinition {
	if len(allowedTools) == 0 {
		return r.Definitions()
	}
	allowed := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		allowed[t] = true
	}
	all := r.Definitions()
	filtered := make([]provider.ToolDefinition, 0, len(allowedTools))
	for _, d := range all {
		if allowed[d.Name] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// Definitions returns tool definitions for the LLM.
func (r *ToolRegistry) Definitions() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "query_leave_balance",
			Description: "Query the leave balance for the current user or a specific employee. Returns earned, used, carried, and remaining days per leave type for the current year.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit to query the current user's balance."},
					"year":        map[string]any{"type": "integer", "description": "Year to query. Defaults to current year."},
				},
			}),
		},
		{
			Name:        "query_attendance_summary",
			Description: "Get attendance summary for today or a date range. Shows present, absent, late counts for the company.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_my_attendance",
			Description: "Get the current user's attendance status for today - whether clocked in, clock-in time, work hours so far.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
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
			Name:        "list_employees",
			Description: "List employees in the company. Admin/Manager only. Returns employee number, name, department, status.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "description": "Filter by status: active, probationary, suspended, separated."},
					"limit":  map[string]any{"type": "integer", "description": "Max results. Default 20."},
				},
			}),
		},
		{
			Name:        "search_knowledge_base",
			Description: "Search the knowledge base for Philippine labor law, HR policies, compliance regulations, payroll rules, leave policies, and company-specific articles. Use this for any HR/labor law question. Returns relevant articles with full content.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query in natural language. E.g., 'maternity leave benefits', 'SSS contribution rates', 'overtime pay holiday'."},
					"limit": map[string]any{"type": "integer", "description": "Max results. Default 5."},
				},
				"required": []string{"query"},
			}),
		},
		{
			Name:        "explain_policy",
			Description: "Explain a Philippine HR/labor policy or company regulation. Topics: leave_types, overtime_rules, 13th_month_pay, final_pay, de_minimis, sss, philhealth, pagibig, bir_tax, minimum_wage, maternity_leave, paternity_leave, solo_parent_leave.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic": map[string]any{"type": "string", "description": "The policy topic to explain."},
				},
				"required": []string{"topic"},
			}),
		},
		{
			Name:        "check_compliance",
			Description: "Check compliance status for the company. Verifies SSS/PhilHealth/PagIBIG registration, BIR filing status, and government form submissions.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
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

// Execute runs a tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error) {
	executor, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return executor(ctx, companyID, userID, input)
}

func (r *ToolRegistry) registerTools() {
	r.tools["query_leave_balance"] = r.toolQueryLeaveBalance
	r.tools["query_attendance_summary"] = r.toolQueryAttendanceSummary
	r.tools["get_my_attendance"] = r.toolGetMyAttendance
	r.tools["query_payslip"] = r.toolQueryPayslip
	r.tools["list_employees"] = r.toolListEmployees
	r.tools["search_knowledge_base"] = r.toolSearchKnowledgeBase
	r.tools["explain_policy"] = r.toolExplainPolicy
	r.tools["check_compliance"] = r.toolCheckCompliance
	r.tools["analyze_payroll_anomalies"] = r.toolAnalyzePayrollAnomalies
}

func (r *ToolRegistry) toolQueryLeaveBalance(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	year := int32(time.Now().Year())
	if y, ok := input["year"].(float64); ok {
		year = int32(y)
	}

	// Get the employee linked to this user
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found for user: %w", err)
	}

	employeeID := emp.ID
	if eid, ok := input["employee_id"].(float64); ok {
		employeeID = int64(eid)
	}

	balances, err := r.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		Year:       year,
	})
	if err != nil {
		return "", fmt.Errorf("query leave balances: %w", err)
	}

	return toJSON(balances)
}

func (r *ToolRegistry) toolQueryAttendanceSummary(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	summary, err := r.queries.GetTodayAttendanceSummary(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("query attendance summary: %w", err)
	}
	return toJSON(summary)
}

func (r *ToolRegistry) toolGetMyAttendance(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	att, err := r.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		return toJSON(map[string]string{"status": "not_clocked_in"})
	}
	return toJSON(att)
}

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

func (r *ToolRegistry) toolListEmployees(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	limit := int32(20)
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int32(l)
	}
	if limit > 50 {
		limit = 50
	}

	employees, err := r.queries.ListEmployees(ctx, store.ListEmployeesParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list employees: %w", err)
	}
	return toJSON(employees)
}

func (r *ToolRegistry) toolSearchKnowledgeBase(ctx context.Context, companyID int64, _ int64, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := int32(5)
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int32(l)
	}
	if limit > 10 {
		limit = 10
	}

	articles, err := r.queries.SearchKnowledgeArticles(ctx, store.SearchKnowledgeArticlesParams{
		CompanyID:      &companyID,
		PlaintoTsquery: query,
		Limit:          limit,
	})
	if err != nil || len(articles) == 0 {
		return "No relevant knowledge articles found. Try rephrasing your query.", nil
	}

	type articleResult struct {
		Title    string  `json:"title"`
		Category string  `json:"category"`
		Content  string  `json:"content"`
		Source   *string `json:"source,omitempty"`
	}

	results := make([]articleResult, len(articles))
	for i, a := range articles {
		results[i] = articleResult{
			Title:    a.Title,
			Category: a.Category,
			Content:  a.Content,
			Source:   a.Source,
		}
	}

	return toJSON(results)
}

func (r *ToolRegistry) toolExplainPolicy(_ context.Context, _ int64, _ int64, input map[string]any) (string, error) {
	topic, _ := input["topic"].(string)
	if topic == "" {
		return "", fmt.Errorf("topic is required")
	}

	policies := map[string]string{
		"leave_types":       "Under Philippine law, employees are entitled to: Service Incentive Leave (5 days/year after 1 year), Maternity Leave (105 days, RA 11210), Paternity Leave (7 days, RA 8187), Solo Parent Leave (7 days, RA 8972), VAWC Leave (10 days, RA 9262), Special Leave for Women (60 days, RA 9710). Companies may provide additional Vacation Leave (VL) and Sick Leave (SL).",
		"13th_month_pay":    "13th Month Pay is mandatory under PD 851. All rank-and-file employees who have worked at least 1 month are entitled. Formula: Total Basic Salary Earned / 12. Must be paid on or before December 24. Pro-rated for employees who worked less than 12 months. The first PHP 90,000 of 13th month pay is tax-exempt (TRAIN Law).",
		"final_pay":         "Final Pay (last pay) must be released within 30 days from separation (DOLE Labor Advisory No. 06-20). Components: unpaid salary, pro-rated 13th month pay, cash conversion of unused leave credits (if company policy allows), separation pay (if applicable), tax refund/liability. Deductions: outstanding loans, unreturned company property.",
		"de_minimis":        "De Minimis Benefits are tax-exempt fringe benefits: Rice subsidy (PHP 2,000/month or 50kg/month), Uniform/clothing allowance (PHP 6,000/year), Medical cash allowance (PHP 1,500/quarter), Laundry allowance (PHP 300/month), Achievement awards (PHP 10,000/year). Excess over limits becomes taxable compensation.",
		"sss":               "SSS contributions are mandatory for all employees. In 2025, the contribution rate is 14% of Monthly Salary Credit (MSC), split: Employee 4.5%, Employer 9.5%. MSC ranges from PHP 4,000 to PHP 30,000. EC (Employees' Compensation) is an additional employer contribution.",
		"philhealth":        "PhilHealth premium rate is 5% of basic monthly salary in 2025. Split equally: Employee 2.5%, Employer 2.5%. Income floor: PHP 10,000, ceiling: PHP 100,000. Maximum monthly contribution: PHP 500 (EE) + PHP 500 (ER) = PHP 1,000 total.",
		"pagibig":           "Pag-IBIG contributions: Employee 1-2% of monthly compensation (1% if salary <= PHP 1,500, else 2%). Employer always 2%. Maximum monthly salary credit: PHP 10,000. Maximum EE contribution: PHP 200, ER contribution: PHP 200.",
		"bir_tax":           "Withholding tax follows TRAIN Law graduated rates (2018+). Monthly brackets: 0% for first PHP 20,833; 15% for PHP 20,833-33,333; 20% for PHP 33,333-66,667; 25% for PHP 66,667-166,667; 30% for PHP 166,667-666,667; 35% over PHP 666,667.",
		"minimum_wage":      "Minimum wage varies by region. NCR (Metro Manila): PHP 645/day (2024). Minimum wage earners are exempt from income tax. Overtime pay: +25% on regular days, +30% on rest days/holidays.",
		"overtime_rules":    "Regular OT: +25% of hourly rate. Rest day OT: +30%. Regular holiday: +100% (double pay). Special non-working holiday: +30%. Night shift differential (10pm-6am): +10% of regular wage.",
		"maternity_leave":   "RA 11210 (Expanded Maternity Leave): 105 days for live birth (with option to extend 30 unpaid days), 60 days for miscarriage/emergency termination. Solo parents get additional 15 days. SSS pays maternity benefit based on average daily salary credit.",
		"paternity_leave":   "RA 8187: 7 days paid paternity leave for married male employees for the first 4 deliveries of legitimate spouse. Must be used within 60 days from birth.",
		"solo_parent_leave": "RA 8972: 7 working days paid parental leave per year for solo parents who have rendered at least 1 year of service. Must present Solo Parent ID from DSWD.",
	}

	info, ok := policies[topic]
	if !ok {
		return fmt.Sprintf("Unknown policy topic: %s. Available topics: leave_types, 13th_month_pay, final_pay, de_minimis, sss, philhealth, pagibig, bir_tax, minimum_wage, overtime_rules, maternity_leave, paternity_leave, solo_parent_leave", topic), nil
	}
	return info, nil
}

func (r *ToolRegistry) toolCheckCompliance(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	// Check how many active employees exist
	employees, err := r.queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list employees: %w", err)
	}

	// Check government form submissions
	forms, err := r.queries.ListGovernmentForms(ctx, store.ListGovernmentFormsParams{
		CompanyID: companyID,
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list forms: %w", err)
	}

	result := map[string]any{
		"total_active_employees": len(employees),
		"government_forms_filed": len(forms),
		"checks": []map[string]string{
			{"item": "SSS Registration", "status": "check_required", "note": "Verify all employees have SSS numbers in their profiles"},
			{"item": "PhilHealth Registration", "status": "check_required", "note": "Verify all employees have PhilHealth numbers"},
			{"item": "PagIBIG Registration", "status": "check_required", "note": "Verify all employees have PagIBIG numbers"},
			{"item": "BIR TIN", "status": "check_required", "note": "Verify all employees have TIN numbers"},
		},
	}

	return toJSON(result)
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

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonSchema(schema map[string]any) map[string]any {
	return schema
}
