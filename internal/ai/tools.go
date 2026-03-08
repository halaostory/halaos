package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
		// --- Write Tools ---
		{
			Name:        "list_leave_types",
			Description: "List all available leave types for the company (e.g., Vacation Leave, Sick Leave, Maternity Leave). Returns leave type IDs needed for create_leave_request.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_leave_request",
			Description: "Submit a leave request for the current user. You MUST call list_leave_types first to get the correct leave_type_id. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"leave_type_id": map[string]any{"type": "integer", "description": "Leave type ID (from list_leave_types)."},
					"start_date":    map[string]any{"type": "string", "description": "Start date in YYYY-MM-DD format."},
					"end_date":      map[string]any{"type": "string", "description": "End date in YYYY-MM-DD format."},
					"days":          map[string]any{"type": "number", "description": "Number of leave days (e.g., 1, 0.5, 2)."},
					"reason":        map[string]any{"type": "string", "description": "Optional reason for the leave request."},
				},
				"required": []string{"leave_type_id", "start_date", "end_date", "days"},
			}),
		},
		{
			Name:        "clock_in",
			Description: "Clock in (start work) for the current user. Records attendance with source='ai'. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "clock_out",
			Description: "Clock out (end work) for the current user. Closes the current open attendance record. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_overtime_request",
			Description: "Submit an overtime request for the current user. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ot_date":  map[string]any{"type": "string", "description": "Overtime date in YYYY-MM-DD format."},
					"start_at": map[string]any{"type": "string", "description": "OT start time in HH:MM format (24h)."},
					"end_at":   map[string]any{"type": "string", "description": "OT end time in HH:MM format (24h)."},
					"hours":    map[string]any{"type": "number", "description": "Total OT hours (e.g., 2, 1.5)."},
					"ot_type":  map[string]any{"type": "string", "description": "OT type: regular, rest_day, holiday, special_holiday. Default: regular."},
					"reason":   map[string]any{"type": "string", "description": "Optional reason for overtime."},
				},
				"required": []string{"ot_date", "start_at", "end_at", "hours"},
			}),
		},
		// --- Expense Tools ---
		{
			Name:        "list_expense_categories",
			Description: "List all active expense categories for the company (e.g., Transportation, Meals, Travel). Returns category IDs needed for create_expense_claim.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_expense_claim",
			Description: "Submit an expense reimbursement claim. You MUST call list_expense_categories first to get the correct category_id. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category_id":  map[string]any{"type": "integer", "description": "Expense category ID (from list_expense_categories)."},
					"description":  map[string]any{"type": "string", "description": "Brief description of the expense (e.g., 'Taxi to client meeting')."},
					"amount":       map[string]any{"type": "number", "description": "Expense amount in PHP."},
					"expense_date": map[string]any{"type": "string", "description": "Date of expense in YYYY-MM-DD format."},
					"notes":        map[string]any{"type": "string", "description": "Optional additional notes."},
				},
				"required": []string{"category_id", "description", "amount", "expense_date"},
			}),
		},
		// --- Approval Tools ---
		{
			Name:        "list_pending_approvals",
			Description: "List all pending leave and overtime requests awaiting approval. Manager/Admin only. Returns request IDs needed for approve_leave_request and approve_overtime_request.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "approve_leave_request",
			Description: "Approve a pending leave request. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Leave request ID (from list_pending_approvals)."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "approve_overtime_request",
			Description: "Approve a pending overtime request. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Overtime request ID (from list_pending_approvals)."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "reject_leave_request",
			Description: "Reject a pending leave request with a reason. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Leave request ID (from list_pending_approvals)."},
					"reason":     map[string]any{"type": "string", "description": "Reason for rejecting the leave request."},
				},
				"required": []string{"request_id"},
			}),
		},
		{
			Name:        "reject_overtime_request",
			Description: "Reject a pending overtime request with a reason. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "integer", "description": "Overtime request ID (from list_pending_approvals)."},
					"reason":     map[string]any{"type": "string", "description": "Reason for rejecting the overtime request."},
				},
				"required": []string{"request_id"},
			}),
		},
		// --- Employee Self-Service ---
		{
			Name:        "update_employee_profile",
			Description: "Update the current user's personal profile information. Only provided fields are updated; omitted fields remain unchanged. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address_line1":      map[string]any{"type": "string", "description": "Address line 1."},
					"address_line2":      map[string]any{"type": "string", "description": "Address line 2."},
					"city":               map[string]any{"type": "string", "description": "City."},
					"province":           map[string]any{"type": "string", "description": "Province."},
					"zip_code":           map[string]any{"type": "string", "description": "ZIP code."},
					"emergency_name":     map[string]any{"type": "string", "description": "Emergency contact name."},
					"emergency_phone":    map[string]any{"type": "string", "description": "Emergency contact phone."},
					"emergency_relation": map[string]any{"type": "string", "description": "Relationship to emergency contact."},
					"bank_name":          map[string]any{"type": "string", "description": "Bank name for payroll."},
					"bank_account_no":    map[string]any{"type": "string", "description": "Bank account number."},
					"bank_account_name":  map[string]any{"type": "string", "description": "Bank account holder name."},
					"tin":                map[string]any{"type": "string", "description": "Tax Identification Number."},
					"sss_no":             map[string]any{"type": "string", "description": "SSS number."},
					"philhealth_no":      map[string]any{"type": "string", "description": "PhilHealth number."},
					"pagibig_no":         map[string]any{"type": "string", "description": "Pag-IBIG number."},
				},
			}),
		},
		// --- Onboarding Tools ---
		{
			Name:        "onboard_employee",
			Description: "Create a new employee record from natural language input. Admin only. Extracts: first_name, last_name, department (name or ID), position (name or ID), hire_date (YYYY-MM-DD), basic_salary (monthly PHP). AI should parse the user's natural language and fill these fields. Always confirm all details with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"first_name":      map[string]any{"type": "string", "description": "Employee first name."},
					"last_name":       map[string]any{"type": "string", "description": "Employee last name."},
					"department":      map[string]any{"type": "string", "description": "Department name (will fuzzy-match to existing departments)."},
					"position":        map[string]any{"type": "string", "description": "Job position/title (will fuzzy-match to existing positions)."},
					"hire_date":       map[string]any{"type": "string", "description": "Hire/start date in YYYY-MM-DD format."},
					"basic_salary":    map[string]any{"type": "number", "description": "Monthly basic salary in PHP."},
					"employment_type": map[string]any{"type": "string", "description": "Employment type: regular, probationary, contractual. Default: probationary."},
					"email":           map[string]any{"type": "string", "description": "Optional work email address."},
				},
				"required": []string{"first_name", "last_name", "department", "hire_date"},
			}),
		},
		// --- Recruitment / ATS Tools ---
		{
			Name:        "list_job_postings",
			Description: "List job postings for the company. Optionally filter by status (draft, open, closed, on_hold). Returns title, department, applicant count, and status.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "description": "Filter by status: draft, open, closed, on_hold. Omit for all."},
				},
			}),
		},
		{
			Name:        "create_job_posting",
			Description: "Create a new job posting. Admin/manager only. Always confirm details with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":           map[string]any{"type": "string", "description": "Job title, e.g. 'Senior PHP Developer'."},
					"department":      map[string]any{"type": "string", "description": "Department name (will fuzzy-match)."},
					"description":     map[string]any{"type": "string", "description": "Full job description."},
					"requirements":    map[string]any{"type": "string", "description": "Job requirements/qualifications."},
					"salary_min":      map[string]any{"type": "number", "description": "Minimum monthly salary in PHP."},
					"salary_max":      map[string]any{"type": "number", "description": "Maximum monthly salary in PHP."},
					"employment_type": map[string]any{"type": "string", "description": "Employment type: regular, contractual, probationary, part_time, intern. Default: regular."},
					"location":        map[string]any{"type": "string", "description": "Work location, e.g. 'BGC, Taguig'."},
				},
				"required": []string{"title"},
			}),
		},
		{
			Name:        "screen_applicant",
			Description: "AI-screen an applicant: read their resume text, score them 0-100 against the job requirements, and save the AI score and summary. Admin/manager only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"applicant_id": map[string]any{"type": "integer", "description": "Applicant ID to screen."},
					"score":        map[string]any{"type": "integer", "description": "AI assessment score 0-100."},
					"summary":      map[string]any{"type": "string", "description": "Brief assessment summary (2-3 sentences)."},
				},
				"required": []string{"applicant_id", "score", "summary"},
			}),
		},
		{
			Name:        "rank_candidates",
			Description: "List and rank applicants for a specific job posting by their AI score. Returns applicants sorted by score descending with their status and summary.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_posting_id": map[string]any{"type": "integer", "description": "Job posting ID to rank candidates for."},
				},
				"required": []string{"job_posting_id"},
			}),
		},
		// --- Report Tools ---
		{
			Name:        "generate_attendance_report",
			Description: "Generate an attendance/DTR report for a date range. Returns summary with present days, late counts, overtime hours, etc. If no employee_id is specified, generates a company-wide report.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start_date":  map[string]any{"type": "string", "description": "Report start date in YYYY-MM-DD format."},
					"end_date":    map[string]any{"type": "string", "description": "Report end date in YYYY-MM-DD format."},
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit for company-wide report."},
				},
				"required": []string{"start_date", "end_date"},
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
	// Read tools
	r.tools["query_leave_balance"] = r.toolQueryLeaveBalance
	r.tools["query_attendance_summary"] = r.toolQueryAttendanceSummary
	r.tools["get_my_attendance"] = r.toolGetMyAttendance
	r.tools["query_payslip"] = r.toolQueryPayslip
	r.tools["list_employees"] = r.toolListEmployees
	r.tools["search_knowledge_base"] = r.toolSearchKnowledgeBase
	r.tools["explain_policy"] = r.toolExplainPolicy
	r.tools["check_compliance"] = r.toolCheckCompliance
	r.tools["analyze_payroll_anomalies"] = r.toolAnalyzePayrollAnomalies
	// Write tools
	r.tools["list_leave_types"] = r.toolListLeaveTypes
	r.tools["create_leave_request"] = r.toolCreateLeaveRequest
	r.tools["clock_in"] = r.toolClockIn
	r.tools["clock_out"] = r.toolClockOut
	r.tools["create_overtime_request"] = r.toolCreateOvertimeRequest
	// Expense tools
	r.tools["list_expense_categories"] = r.toolListExpenseCategories
	r.tools["create_expense_claim"] = r.toolCreateExpenseClaim
	// Approval tools
	r.tools["list_pending_approvals"] = r.toolListPendingApprovals
	r.tools["approve_leave_request"] = r.toolApproveLeaveRequest
	r.tools["approve_overtime_request"] = r.toolApproveOvertimeRequest
	r.tools["reject_leave_request"] = r.toolRejectLeaveRequest
	r.tools["reject_overtime_request"] = r.toolRejectOvertimeRequest
	// Employee self-service
	r.tools["update_employee_profile"] = r.toolUpdateEmployeeProfile
	// Recruitment / ATS
	r.tools["list_job_postings"] = r.toolListJobPostings
	r.tools["create_job_posting"] = r.toolCreateJobPosting
	r.tools["screen_applicant"] = r.toolScreenApplicant
	r.tools["rank_candidates"] = r.toolRankCandidates
	// Reports
	r.tools["generate_attendance_report"] = r.toolGenerateAttendanceReport
	// Onboarding
	r.tools["onboard_employee"] = r.toolOnboardEmployee
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

	// Tier 1: websearch_to_tsquery — best for structured keyword matches
	articles, err := r.queries.SearchKnowledgeArticles(ctx, store.SearchKnowledgeArticlesParams{
		CompanyID:          &companyID,
		WebsearchToTsquery: query,
		Limit:              limit,
	})
	if err != nil {
		articles = nil
	}

	// Tier 2: pg_trgm trigram similarity — good for fuzzy/typo matching
	if len(articles) == 0 {
		trigramRows, trgErr := r.queries.SearchKnowledgeByTrigram(ctx, store.SearchKnowledgeByTrigramParams{
			Query:      query,
			CompanyID:  &companyID,
			MaxResults: limit,
		})
		if trgErr == nil && len(trigramRows) > 0 {
			// Convert trigram rows to the same type as tsquery results
			articles = make([]store.SearchKnowledgeArticlesRow, len(trigramRows))
			for i, tr := range trigramRows {
				articles[i] = store.SearchKnowledgeArticlesRow{
					ID:        tr.ID,
					CompanyID: tr.CompanyID,
					Category:  tr.Category,
					Topic:     tr.Topic,
					Title:     tr.Title,
					Content:   tr.Content,
					Tags:      tr.Tags,
					Source:    tr.Source,
					IsActive:  tr.IsActive,
					CreatedAt: tr.CreatedAt,
					UpdatedAt: tr.UpdatedAt,
				}
			}
		}
	}

	// Tier 3: ILIKE substring fallback
	if len(articles) == 0 {
		fallbackRows, fbErr := r.queries.SearchKnowledgeArticlesByILIKE(ctx, store.SearchKnowledgeArticlesByILIKEParams{
			CompanyID: &companyID,
			Column2:   &query,
		})
		if fbErr == nil {
			articles = make([]store.SearchKnowledgeArticlesRow, len(fallbackRows))
			for i, fr := range fallbackRows {
				articles[i] = store.SearchKnowledgeArticlesRow{
					ID: fr.ID, CompanyID: fr.CompanyID, Category: fr.Category,
					Topic: fr.Topic, Title: fr.Title, Content: fr.Content,
					Tags: fr.Tags, Source: fr.Source, IsActive: fr.IsActive,
					CreatedAt: fr.CreatedAt, UpdatedAt: fr.UpdatedAt,
				}
			}
		}
	}

	if len(articles) == 0 {
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

// --- Write Tool Implementations ---

func (r *ToolRegistry) toolListLeaveTypes(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	types, err := r.queries.ListLeaveTypes(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list leave types: %w", err)
	}

	type leaveTypeResult struct {
		ID          int64  `json:"id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		IsPaid      bool   `json:"is_paid"`
		DefaultDays string `json:"default_days"`
	}

	results := make([]leaveTypeResult, len(types))
	for i, lt := range types {
		results[i] = leaveTypeResult{
			ID:          lt.ID,
			Code:        lt.Code,
			Name:        lt.Name,
			IsPaid:      lt.IsPaid,
			DefaultDays: numericToString(lt.DefaultDays),
		}
	}

	return toJSON(results)
}

func (r *ToolRegistry) toolCreateLeaveRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	leaveTypeID, ok := input["leave_type_id"].(float64)
	if !ok || leaveTypeID <= 0 {
		return "", fmt.Errorf("leave_type_id is required")
	}

	startDateStr, _ := input["start_date"].(string)
	endDateStr, _ := input["end_date"].(string)
	daysFloat, _ := input["days"].(float64)

	if startDateStr == "" || endDateStr == "" || daysFloat <= 0 {
		return "", fmt.Errorf("start_date, end_date, and days are required")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid start_date format, use YYYY-MM-DD")
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid end_date format, use YYYY-MM-DD")
	}

	if endDate.Before(startDate) {
		return "", fmt.Errorf("end_date must not be before start_date")
	}

	var days pgtype.Numeric
	_ = days.Scan(fmt.Sprintf("%.1f", daysFloat))

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.CreateLeaveRequest(ctx, store.CreateLeaveRequestParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		LeaveTypeID: int64(leaveTypeID),
		StartDate:   startDate,
		EndDate:     endDate,
		Days:        days,
		Reason:      reason,
	})
	if err != nil {
		return "", fmt.Errorf("create leave request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"start_date": startDateStr,
		"end_date":   endDateStr,
		"days":       daysFloat,
		"message":    "Leave request submitted successfully. It is now pending approval.",
	})
}

func (r *ToolRegistry) toolClockIn(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Check if already clocked in
	_, err = r.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err == nil {
		return toJSON(map[string]any{
			"success": false,
			"message": "You are already clocked in. Please clock out first.",
		})
	}

	att, err := r.queries.ClockIn(ctx, store.ClockInParams{
		CompanyID:     companyID,
		EmployeeID:    emp.ID,
		ClockInSource: "ai",
		ClockInLat:    pgtype.Numeric{Valid: false},
		ClockInLng:    pgtype.Numeric{Valid: false},
	})
	if err != nil {
		return "", fmt.Errorf("clock in: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"attendance_id": att.ID,
		"clock_in_at":   att.ClockInAt.Time.Format(time.RFC3339),
		"source":        "ai",
		"message":       "Successfully clocked in.",
	})
}

func (r *ToolRegistry) toolClockOut(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Find open attendance record
	open, err := r.queries.GetOpenAttendance(ctx, store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		return toJSON(map[string]any{
			"success": false,
			"message": "No open attendance record found. You need to clock in first.",
		})
	}

	source := "ai"
	att, err := r.queries.ClockOut(ctx, store.ClockOutParams{
		ID:             open.ID,
		EmployeeID:     emp.ID,
		ClockOutSource: &source,
		ClockOutLat:    pgtype.Numeric{Valid: false},
		ClockOutLng:    pgtype.Numeric{Valid: false},
	})
	if err != nil {
		return "", fmt.Errorf("clock out: %w", err)
	}

	return toJSON(map[string]any{
		"success":       true,
		"attendance_id": att.ID,
		"clock_in_at":   att.ClockInAt.Time.Format(time.RFC3339),
		"clock_out_at":  att.ClockOutAt.Time.Format(time.RFC3339),
		"source":        "ai",
		"message":       "Successfully clocked out.",
	})
}

func (r *ToolRegistry) toolCreateOvertimeRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	otDateStr, _ := input["ot_date"].(string)
	startAtStr, _ := input["start_at"].(string)
	endAtStr, _ := input["end_at"].(string)
	hoursFloat, _ := input["hours"].(float64)

	if otDateStr == "" || startAtStr == "" || endAtStr == "" || hoursFloat <= 0 {
		return "", fmt.Errorf("ot_date, start_at, end_at, and hours are required")
	}

	otDate, err := time.Parse("2006-01-02", otDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid ot_date format, use YYYY-MM-DD")
	}

	startAt, err := time.Parse("2006-01-02 15:04", otDateStr+" "+startAtStr)
	if err != nil {
		return "", fmt.Errorf("invalid start_at format, use HH:MM")
	}
	endAt, err := time.Parse("2006-01-02 15:04", otDateStr+" "+endAtStr)
	if err != nil {
		return "", fmt.Errorf("invalid end_at format, use HH:MM")
	}

	if !endAt.After(startAt) {
		return "", fmt.Errorf("end_at must be after start_at")
	}

	var hours pgtype.Numeric
	_ = hours.Scan(fmt.Sprintf("%.1f", hoursFloat))

	otType := "regular"
	if t, ok := input["ot_type"].(string); ok && t != "" {
		otType = t
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.CreateOvertimeRequest(ctx, store.CreateOvertimeRequestParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		OtDate:     otDate,
		StartAt:    startAt,
		EndAt:      endAt,
		Hours:      hours,
		OtType:     otType,
		Reason:     reason,
	})
	if err != nil {
		return "", fmt.Errorf("create overtime request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"ot_date":    otDateStr,
		"hours":      hoursFloat,
		"ot_type":    otType,
		"message":    "Overtime request submitted successfully. It is now pending approval.",
	})
}

// --- Expense Tool Implementations ---

func (r *ToolRegistry) toolListExpenseCategories(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	categories, err := r.queries.ListActiveExpenseCategories(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list expense categories: %w", err)
	}

	type categoryResult struct {
		ID             int64  `json:"id"`
		Name           string `json:"name"`
		Description    string `json:"description,omitempty"`
		MaxAmount      string `json:"max_amount,omitempty"`
		RequiresReceipt bool  `json:"requires_receipt"`
	}

	results := make([]categoryResult, len(categories))
	for i, c := range categories {
		desc := ""
		if c.Description != nil {
			desc = *c.Description
		}
		results[i] = categoryResult{
			ID:              c.ID,
			Name:            c.Name,
			Description:     desc,
			MaxAmount:       numericToString(c.MaxAmount),
			RequiresReceipt: c.RequiresReceipt,
		}
	}

	return toJSON(results)
}

func (r *ToolRegistry) toolCreateExpenseClaim(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	categoryID, ok := input["category_id"].(float64)
	if !ok || categoryID <= 0 {
		return "", fmt.Errorf("category_id is required")
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}

	amountFloat, ok := input["amount"].(float64)
	if !ok || amountFloat <= 0 {
		return "", fmt.Errorf("amount must be greater than 0")
	}

	expenseDateStr, _ := input["expense_date"].(string)
	if expenseDateStr == "" {
		return "", fmt.Errorf("expense_date is required")
	}
	expenseDate, err := time.Parse("2006-01-02", expenseDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid expense_date format, use YYYY-MM-DD")
	}

	var amount pgtype.Numeric
	_ = amount.Scan(fmt.Sprintf("%.2f", amountFloat))

	// Generate claim number
	nextNum, err := r.queries.NextExpenseClaimNumber(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("generate claim number: %w", err)
	}
	claimNumber := fmt.Sprintf("EXP-%05d", nextNum)

	var notes *string
	if n, ok := input["notes"].(string); ok && n != "" {
		notes = &n
	}

	claim, err := r.queries.CreateExpenseClaim(ctx, store.CreateExpenseClaimParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		ClaimNumber: claimNumber,
		CategoryID:  int64(categoryID),
		Description: description,
		Amount:      amount,
		Currency:    "PHP",
		ExpenseDate: expenseDate,
		Status:      "submitted",
		Notes:       notes,
	})
	if err != nil {
		return "", fmt.Errorf("create expense claim: %w", err)
	}

	return toJSON(map[string]any{
		"success":      true,
		"claim_id":     claim.ID,
		"claim_number": claim.ClaimNumber,
		"status":       claim.Status,
		"amount":       amountFloat,
		"message":      fmt.Sprintf("Expense claim %s submitted successfully for ₱%.2f.", claimNumber, amountFloat),
	})
}

// --- Approval Tool Implementations ---

func (r *ToolRegistry) toolListPendingApprovals(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	// Check user role and company scope
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can view pending approvals")
	}

	leaveApprovals, err := r.queries.ListPendingLeaveApprovals(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list pending leave approvals: %w", err)
	}

	overtimeApprovals, err := r.queries.ListPendingOvertimeApprovals(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list pending overtime approvals: %w", err)
	}

	type approvalItem struct {
		ID           int64  `json:"id"`
		Type         string `json:"type"`
		EmployeeName string `json:"employee_name"`
		Details      any    `json:"details"`
	}

	items := make([]approvalItem, 0, len(leaveApprovals)+len(overtimeApprovals))

	for _, la := range leaveApprovals {
		items = append(items, approvalItem{
			ID:           la.ID,
			Type:         "leave",
			EmployeeName: fmt.Sprintf("%v", la.EmployeeName),
			Details: map[string]any{
				"leave_type": la.LeaveTypeName,
				"start_date": la.StartDate.Format("2006-01-02"),
				"end_date":   la.EndDate.Format("2006-01-02"),
				"days":       numericToString(la.Days),
			},
		})
	}

	for _, oa := range overtimeApprovals {
		items = append(items, approvalItem{
			ID:           oa.ID,
			Type:         "overtime",
			EmployeeName: fmt.Sprintf("%v", oa.EmployeeName),
			Details: map[string]any{
				"ot_date": oa.OtDate.Format("2006-01-02"),
				"hours":   numericToString(oa.Hours),
				"ot_type": oa.OtType,
			},
		})
	}

	return toJSON(map[string]any{
		"total_pending": len(items),
		"items":         items,
	})
}

func (r *ToolRegistry) toolApproveLeaveRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	// Check user role and company scope
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can approve leave requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	// Get the approver's employee record
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	req, err := r.queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
		ID:         int64(requestID),
		CompanyID:  companyID,
		ApproverID: &emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("approve leave request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Leave request approved successfully.",
	})
}

func (r *ToolRegistry) toolApproveOvertimeRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	// Check user role and company scope
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can approve overtime requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	req, err := r.queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
		ID:         int64(requestID),
		CompanyID:  companyID,
		ApproverID: &emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("approve overtime request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Overtime request approved successfully.",
	})
}

func (r *ToolRegistry) toolRejectLeaveRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can reject leave requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.RejectLeaveRequest(ctx, store.RejectLeaveRequestParams{
		ID:              int64(requestID),
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("reject leave request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Leave request rejected.",
	})
}

func (r *ToolRegistry) toolRejectOvertimeRequest(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return "", fmt.Errorf("access denied")
	}
	if user.Role != "admin" && user.Role != "manager" {
		return "", fmt.Errorf("only managers and admins can reject overtime requests")
	}

	requestID, ok := input["request_id"].(float64)
	if !ok || requestID <= 0 {
		return "", fmt.Errorf("request_id is required")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("approver employee not found: %w", err)
	}

	var reason *string
	if r, ok := input["reason"].(string); ok && r != "" {
		reason = &r
	}

	req, err := r.queries.RejectOvertimeRequest(ctx, store.RejectOvertimeRequestParams{
		ID:              int64(requestID),
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("reject overtime request: %w", err)
	}

	return toJSON(map[string]any{
		"success":    true,
		"request_id": req.ID,
		"status":     req.Status,
		"message":    "Overtime request rejected.",
	})
}

// --- Employee Self-Service Implementation ---

func (r *ToolRegistry) toolUpdateEmployeeProfile(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Get existing profile (may not exist yet)
	existing, err := r.queries.GetEmployeeProfile(ctx, emp.ID)
	if err != nil {
		// No existing profile — start with empty
		existing = store.EmployeeProfile{EmployeeID: emp.ID}
	}

	// Merge: input overrides existing
	merged := store.UpsertEmployeeProfileParams{
		EmployeeID:        emp.ID,
		AddressLine1:      coalesceStrPtr(input, "address_line1", existing.AddressLine1),
		AddressLine2:      coalesceStrPtr(input, "address_line2", existing.AddressLine2),
		City:              coalesceStrPtr(input, "city", existing.City),
		Province:          coalesceStrPtr(input, "province", existing.Province),
		ZipCode:           coalesceStrPtr(input, "zip_code", existing.ZipCode),
		EmergencyName:     coalesceStrPtr(input, "emergency_name", existing.EmergencyName),
		EmergencyPhone:    coalesceStrPtr(input, "emergency_phone", existing.EmergencyPhone),
		EmergencyRelation: coalesceStrPtr(input, "emergency_relation", existing.EmergencyRelation),
		BankName:          coalesceStrPtr(input, "bank_name", existing.BankName),
		BankAccountNo:     coalesceStrPtr(input, "bank_account_no", existing.BankAccountNo),
		BankAccountName:   coalesceStrPtr(input, "bank_account_name", existing.BankAccountName),
		Tin:               coalesceStrPtr(input, "tin", existing.Tin),
		SssNo:             coalesceStrPtr(input, "sss_no", existing.SssNo),
		PhilhealthNo:      coalesceStrPtr(input, "philhealth_no", existing.PhilhealthNo),
		PagibigNo:         coalesceStrPtr(input, "pagibig_no", existing.PagibigNo),
	}

	profile, err := r.queries.UpsertEmployeeProfile(ctx, merged)
	if err != nil {
		return "", fmt.Errorf("update employee profile: %w", err)
	}

	// Build list of updated fields
	updated := make([]string, 0)
	for _, key := range []string{"address_line1", "address_line2", "city", "province", "zip_code",
		"emergency_name", "emergency_phone", "emergency_relation",
		"bank_name", "bank_account_no", "bank_account_name",
		"tin", "sss_no", "philhealth_no", "pagibig_no"} {
		if v, ok := input[key].(string); ok && v != "" {
			updated = append(updated, key)
		}
	}

	return toJSON(map[string]any{
		"success":        true,
		"employee_id":    profile.EmployeeID,
		"updated_fields": updated,
		"message":        "Profile updated successfully.",
	})
}

// --- Report Tool Implementation ---

func (r *ToolRegistry) toolGenerateAttendanceReport(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	startDateStr, _ := input["start_date"].(string)
	endDateStr, _ := input["end_date"].(string)

	if startDateStr == "" || endDateStr == "" {
		return "", fmt.Errorf("start_date and end_date are required")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid start_date format, use YYYY-MM-DD")
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid end_date format, use YYYY-MM-DD")
	}

	if endDate.Before(startDate) {
		return "", fmt.Errorf("end_date must not be before start_date")
	}

	// Cap date range to 93 days (one quarter)
	if endDate.Sub(startDate) > 93*24*time.Hour {
		return "", fmt.Errorf("date range cannot exceed 93 days")
	}

	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	// Check permissions: non-managers can only query their own data
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	isManager := user.Role == "admin" || user.Role == "manager"

	// End date should be exclusive (next day start)
	endDateExclusive := endDate.Add(24 * time.Hour)

	startTS := pgtype.Timestamptz{Time: startDate, Valid: true}
	endTS := pgtype.Timestamptz{Time: endDateExclusive, Valid: true}

	summaryMap := make(map[int64]*attendanceSummaryEntry)

	if eid, ok := input["employee_id"].(float64); ok && eid > 0 {
		targetEmpID := int64(eid)
		// Non-managers can only query their own attendance
		if !isManager && targetEmpID != emp.ID {
			return "", fmt.Errorf("you can only view your own attendance report")
		}
		records, err := r.queries.GetDTR(ctx, store.GetDTRParams{
			CompanyID:   companyID,
			EmployeeID:  targetEmpID,
			ClockInAt:   startTS,
			ClockInAt_2: endTS,
		})
		if err != nil {
			return "", fmt.Errorf("get DTR: %w", err)
		}
		for _, rec := range records {
			s := getOrCreateSummary(summaryMap, rec.EmployeeID, rec.EmployeeNo, rec.FirstName+" "+rec.LastName, rec.DepartmentName)
			accumulateDTR(s, rec.WorkHours, rec.OvertimeHours, rec.LateMinutes)
		}
	} else if isManager {
		// Company-wide report: managers/admins only
		records, err := r.queries.GetDTRAllEmployees(ctx, store.GetDTRAllEmployeesParams{
			CompanyID:   companyID,
			ClockInAt:   startTS,
			ClockInAt_2: endTS,
		})
		if err != nil {
			return "", fmt.Errorf("get DTR all employees: %w", err)
		}
		for _, rec := range records {
			s := getOrCreateSummary(summaryMap, rec.EmployeeID, rec.EmployeeNo, rec.FirstName+" "+rec.LastName, rec.DepartmentName)
			accumulateDTR(s, rec.WorkHours, rec.OvertimeHours, rec.LateMinutes)
		}
	} else {
		// Non-manager without employee_id: default to own data
		records, err := r.queries.GetDTR(ctx, store.GetDTRParams{
			CompanyID:   companyID,
			EmployeeID:  emp.ID,
			ClockInAt:   startTS,
			ClockInAt_2: endTS,
		})
		if err != nil {
			return "", fmt.Errorf("get DTR: %w", err)
		}
		for _, rec := range records {
			s := getOrCreateSummary(summaryMap, rec.EmployeeID, rec.EmployeeNo, rec.FirstName+" "+rec.LastName, rec.DepartmentName)
			accumulateDTR(s, rec.WorkHours, rec.OvertimeHours, rec.LateMinutes)
		}
	}

	employees := make([]attendanceSummaryEntry, 0, len(summaryMap))
	totalPresent := 0
	totalLate := 0
	for _, s := range summaryMap {
		employees = append(employees, *s)
		totalPresent += s.PresentDays
		totalLate += s.LateDays
	}

	return toJSON(map[string]any{
		"period":              startDateStr + " to " + endDateStr,
		"total_employees":     len(summaryMap),
		"total_present_days":  totalPresent,
		"total_late_instances": totalLate,
		"employees":           employees,
	})
}

type attendanceSummaryEntry struct {
	EmployeeNo     string  `json:"employee_no"`
	Name           string  `json:"name"`
	Department     string  `json:"department"`
	PresentDays    int     `json:"present_days"`
	LateDays       int     `json:"late_days"`
	TotalWorkHours float64 `json:"total_work_hours"`
	TotalOTHours   float64 `json:"total_ot_hours"`
	TotalLateMin   int     `json:"total_late_minutes"`
}

func getOrCreateSummary(m map[int64]*attendanceSummaryEntry, empID int64, empNo, name, dept string) *attendanceSummaryEntry {
	if s, ok := m[empID]; ok {
		return s
	}
	s := &attendanceSummaryEntry{
		EmployeeNo: empNo,
		Name:       name,
		Department: dept,
	}
	m[empID] = s
	return s
}

func accumulateDTR(s *attendanceSummaryEntry, workHours, overtimeHours pgtype.Numeric, lateMinutes *int32) {
	s.PresentDays++
	if wh, err := workHours.Float64Value(); err == nil && wh.Valid {
		s.TotalWorkHours += wh.Float64
	}
	if oh, err := overtimeHours.Float64Value(); err == nil && oh.Valid {
		s.TotalOTHours += oh.Float64
	}
	if lateMinutes != nil && *lateMinutes > 0 {
		s.LateDays++
		s.TotalLateMin += int(*lateMinutes)
	}
}

// coalesceStrPtr returns the input value if present, otherwise the existing value.
func coalesceStrPtr(input map[string]any, key string, existing *string) *string {
	if v, ok := input[key].(string); ok && v != "" {
		return &v
	}
	return existing
}

func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "0"
	}
	return fmt.Sprintf("%.1f", f.Float64)
}

// --- Onboarding Tool Implementation ---

func (r *ToolRegistry) toolOnboardEmployee(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	// 1. Check admin/manager role
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.Role != "admin" && user.Role != "super_admin" && user.Role != "manager" {
		return "", fmt.Errorf("only admins and managers can onboard employees")
	}

	// 2. Extract required fields
	firstName, _ := input["first_name"].(string)
	lastName, _ := input["last_name"].(string)
	deptName, _ := input["department"].(string)
	if firstName == "" || lastName == "" {
		return "", fmt.Errorf("first_name and last_name are required")
	}

	hireDateStr, _ := input["hire_date"].(string)
	if hireDateStr == "" {
		hireDateStr = time.Now().Format("2006-01-02")
	}
	hireDate, err := time.Parse("2006-01-02", hireDateStr)
	if err != nil {
		return "", fmt.Errorf("invalid hire_date format, use YYYY-MM-DD")
	}

	// 3. Fuzzy-match department
	var deptID int64
	var matchedDeptName string
	if deptName != "" {
		err := r.pool.QueryRow(ctx, `
			SELECT id, name FROM departments
			WHERE company_id = $1 AND is_active = true AND name ILIKE '%' || $2 || '%'
			ORDER BY CASE WHEN LOWER(name) = LOWER($2) THEN 0 ELSE 1 END, id
			LIMIT 1
		`, companyID, deptName).Scan(&deptID, &matchedDeptName)
		if err != nil {
			return "", fmt.Errorf("department '%s' not found. Please check available departments", deptName)
		}
	}

	// 4. Fuzzy-match position (optional)
	posName, _ := input["position"].(string)
	var posID int64
	var matchedPosName string
	if posName != "" {
		_ = r.pool.QueryRow(ctx, `
			SELECT id, title FROM positions
			WHERE company_id = $1 AND is_active = true AND title ILIKE '%' || $2 || '%'
			ORDER BY CASE WHEN LOWER(title) = LOWER($2) THEN 0 ELSE 1 END, id
			LIMIT 1
		`, companyID, posName).Scan(&posID, &matchedPosName)
	}

	// 5. Generate employee number (EMP-XXXXX format)
	var maxNum int
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTRING(employee_no FROM 5) AS INTEGER)), 0)
		FROM employees WHERE company_id = $1 AND employee_no LIKE 'EMP-%'
	`, companyID).Scan(&maxNum)
	employeeNo := fmt.Sprintf("EMP-%05d", maxNum+1)

	empType := "probationary"
	if t, ok := input["employment_type"].(string); ok && t != "" {
		empType = t
	}

	email, _ := input["email"].(string)

	// 6. Create employee via raw SQL
	var empID int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO employees (
			company_id, employee_no, first_name, last_name,
			department_id, position_id, hire_date, employment_type,
			status, email
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6::bigint, 0), $7, $8, 'active', NULLIF($9, ''))
		RETURNING id
	`, companyID, employeeNo, firstName, lastName,
		deptID, posID, hireDate, empType, email).Scan(&empID)
	if err != nil {
		return "", fmt.Errorf("create employee: %w", err)
	}

	// 7. Create employment history record
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO employment_history (
			company_id, employee_id, action_type, effective_date,
			to_department_id, to_position_id, remarks, created_by
		) VALUES ($1, $2, 'hire', $3, NULLIF($4::bigint, 0), NULLIF($5::bigint, 0), $6, $7)
	`, companyID, empID, hireDate, deptID, posID,
		fmt.Sprintf("Onboarded via AI assistant by user %d", userID), userID)

	// 8. Assign salary if provided
	salaryMsg := ""
	if salary, ok := input["basic_salary"].(float64); ok && salary > 0 {
		_, salErr := r.pool.Exec(ctx, `
			INSERT INTO employee_salaries (
				company_id, employee_id, basic_salary, effective_from, remarks, created_by
			) VALUES ($1, $2, $3, $4, 'Initial salary - onboarded via AI', $5)
		`, companyID, empID, salary, hireDate, userID)
		if salErr == nil {
			salaryMsg = fmt.Sprintf(" with PHP %.2f/month salary", salary)
		}
	}

	result := map[string]any{
		"success":         true,
		"employee_id":     empID,
		"employee_no":     employeeNo,
		"name":            firstName + " " + lastName,
		"department":      matchedDeptName,
		"hire_date":       hireDateStr,
		"employment_type": empType,
		"message": fmt.Sprintf("Employee %s (%s) has been successfully onboarded to %s department starting %s%s.",
			firstName+" "+lastName, employeeNo, matchedDeptName, hireDateStr, salaryMsg),
	}
	if matchedPosName != "" {
		result["position"] = matchedPosName
	}

	return toJSON(result)
}

// --- Recruitment / ATS Tool Implementations ---

func (r *ToolRegistry) toolListJobPostings(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	status, _ := input["status"].(string)

	rows, err := r.pool.Query(ctx, `
		SELECT jp.id, jp.title, COALESCE(d.name, '') AS department,
		       jp.employment_type, jp.location, jp.status, jp.created_at,
		       (SELECT COUNT(*) FROM applicants a WHERE a.job_posting_id = jp.id) AS applicant_count
		FROM job_postings jp
		LEFT JOIN departments d ON d.id = jp.department_id
		WHERE jp.company_id = $1
		  AND ($2 = '' OR jp.status = $2)
		ORDER BY jp.created_at DESC
		LIMIT 50
	`, companyID, status)
	if err != nil {
		return "", fmt.Errorf("list job postings: %w", err)
	}
	defer rows.Close()

	type jobResult struct {
		ID             int64  `json:"id"`
		Title          string `json:"title"`
		Department     string `json:"department"`
		EmploymentType string `json:"employment_type"`
		Location       string `json:"location"`
		Status         string `json:"status"`
		CreatedAt      string `json:"created_at"`
		ApplicantCount int64  `json:"applicant_count"`
	}
	var results []jobResult
	for rows.Next() {
		var j jobResult
		var createdAt time.Time
		if err := rows.Scan(&j.ID, &j.Title, &j.Department,
			&j.EmploymentType, &j.Location, &j.Status, &createdAt, &j.ApplicantCount); err != nil {
			continue
		}
		j.CreatedAt = createdAt.Format("2006-01-02")
		results = append(results, j)
	}
	if results == nil {
		results = []jobResult{}
	}
	return toJSON(map[string]any{
		"total": len(results),
		"jobs":  results,
	})
}

func (r *ToolRegistry) toolCreateJobPosting(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.Role != "admin" && user.Role != "super_admin" && user.Role != "manager" {
		return "", fmt.Errorf("only admins and managers can create job postings")
	}

	title, _ := input["title"].(string)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	description, _ := input["description"].(string)
	requirements, _ := input["requirements"].(string)
	employmentType, _ := input["employment_type"].(string)
	if employmentType == "" {
		employmentType = "regular"
	}
	location, _ := input["location"].(string)

	// Fuzzy-match department
	var deptID *int64
	if deptName, ok := input["department"].(string); ok && deptName != "" {
		var did int64
		err := r.pool.QueryRow(ctx, `
			SELECT id FROM departments
			WHERE company_id = $1 AND is_active = true AND name ILIKE '%' || $2 || '%'
			ORDER BY CASE WHEN LOWER(name) = LOWER($2) THEN 0 ELSE 1 END, id
			LIMIT 1
		`, companyID, deptName).Scan(&did)
		if err == nil {
			deptID = &did
		}
	}

	// Salary
	var salMin, salMax *string
	if v, ok := input["salary_min"].(float64); ok && v > 0 {
		s := fmt.Sprintf("%.2f", v)
		salMin = &s
	}
	if v, ok := input["salary_max"].(float64); ok && v > 0 {
		s := fmt.Sprintf("%.2f", v)
		salMax = &s
	}

	var id int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO job_postings (company_id, title, department_id, description, requirements,
			salary_min, salary_max, employment_type, location, status, created_by)
		VALUES ($1, $2, $3, $4, $5,
			CASE WHEN $6 IS NULL THEN NULL ELSE $6::NUMERIC END,
			CASE WHEN $7 IS NULL THEN NULL ELSE $7::NUMERIC END,
			$8, $9, 'draft', $10)
		RETURNING id
	`, companyID, title, deptID, description, requirements,
		salMin, salMax, employmentType, location, userID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create job posting: %w", err)
	}

	return toJSON(map[string]any{
		"success": true,
		"id":      id,
		"title":   title,
		"status":  "draft",
		"message": fmt.Sprintf("Job posting '%s' created as draft. Use the Recruitment page to publish it.", title),
	})
}

func (r *ToolRegistry) toolScreenApplicant(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if user.Role != "admin" && user.Role != "super_admin" && user.Role != "manager" {
		return "", fmt.Errorf("only admins and managers can screen applicants")
	}

	applicantID, ok := input["applicant_id"].(float64)
	if !ok || applicantID <= 0 {
		return "", fmt.Errorf("applicant_id is required")
	}

	score, ok := input["score"].(float64)
	if !ok || score < 0 || score > 100 {
		return "", fmt.Errorf("score must be between 0 and 100")
	}

	summary, _ := input["summary"].(string)
	if summary == "" {
		return "", fmt.Errorf("summary is required")
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE applicants
		SET ai_score = $3, ai_summary = $4, status = CASE WHEN status = 'new' THEN 'screening' ELSE status END, updated_at = now()
		WHERE id = $1 AND company_id = $2
	`, int64(applicantID), companyID, int(score), summary)
	if err != nil {
		return "", fmt.Errorf("screen applicant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", fmt.Errorf("applicant not found")
	}

	return toJSON(map[string]any{
		"success":      true,
		"applicant_id": int64(applicantID),
		"ai_score":     int(score),
		"message":      fmt.Sprintf("Applicant screened with score %d/100.", int(score)),
	})
}

func (r *ToolRegistry) toolRankCandidates(ctx context.Context, companyID, _ int64, input map[string]any) (string, error) {
	jobPostingID, ok := input["job_posting_id"].(float64)
	if !ok || jobPostingID <= 0 {
		return "", fmt.Errorf("job_posting_id is required")
	}

	// Verify job posting belongs to company
	var jobTitle string
	err := r.pool.QueryRow(ctx, `
		SELECT title FROM job_postings WHERE id = $1 AND company_id = $2
	`, int64(jobPostingID), companyID).Scan(&jobTitle)
	if err != nil {
		return "", fmt.Errorf("job posting not found")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, first_name, last_name, email, ai_score, ai_summary, status, source
		FROM applicants
		WHERE job_posting_id = $1 AND company_id = $2
		ORDER BY COALESCE(ai_score, 0) DESC, applied_at ASC
	`, int64(jobPostingID), companyID)
	if err != nil {
		return "", fmt.Errorf("rank candidates: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		Rank      int    `json:"rank"`
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AIScore   *int   `json:"ai_score"`
		AISummary string `json:"ai_summary,omitempty"`
		Status    string `json:"status"`
		Source    string `json:"source"`
	}
	var candidates []candidate
	rank := 1
	for rows.Next() {
		var c candidate
		var firstName, lastName string
		var aiSummary *string
		if err := rows.Scan(&c.ID, &firstName, &lastName, &c.Email,
			&c.AIScore, &aiSummary, &c.Status, &c.Source); err != nil {
			continue
		}
		c.Name = firstName + " " + lastName
		c.Rank = rank
		if aiSummary != nil {
			c.AISummary = *aiSummary
		}
		candidates = append(candidates, c)
		rank++
	}
	if candidates == nil {
		candidates = []candidate{}
	}

	return toJSON(map[string]any{
		"job_title":  jobTitle,
		"total":      len(candidates),
		"candidates": candidates,
	})
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
