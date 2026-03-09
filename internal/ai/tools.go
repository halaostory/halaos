package ai

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/ai/provider"
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
		// --- Salary Simulation ---
		{
			Name:        "simulate_salary",
			Description: "Simulate salary calculation with what-if scenarios. Input overtime hours, holiday work, night hours, late minutes, etc. to see estimated gross pay, deductions, and net pay. Useful for questions like 'How much would I earn with 10 hours overtime?'",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":          map[string]any{"type": "integer", "description": "Optional employee ID. Omit to simulate for current user."},
					"working_days":         map[string]any{"type": "number", "description": "Working days in period. Default 22."},
					"overtime_hours":       map[string]any{"type": "number", "description": "Regular overtime hours."},
					"rest_day_ot_hours":    map[string]any{"type": "number", "description": "Rest day overtime hours (169% rate)."},
					"holiday_ot_hours":     map[string]any{"type": "number", "description": "Holiday overtime hours (260% rate)."},
					"night_hours":          map[string]any{"type": "number", "description": "Night differential hours (10PM-6AM)."},
					"regular_holiday_days": map[string]any{"type": "number", "description": "Days worked on regular holidays."},
					"special_holiday_days": map[string]any{"type": "number", "description": "Days worked on special non-working holidays."},
					"late_minutes":         map[string]any{"type": "number", "description": "Total late minutes."},
					"unpaid_leave_days":    map[string]any{"type": "number", "description": "Unpaid leave days."},
				},
			}),
		},
		// --- Phase 1: Loan + Benefit + Encashment Tools ---
		{
			Name:        "query_my_loans",
			Description: "Query the current user's loans with repayment progress. Returns loan type, principal, remaining balance, monthly amortization, and status.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "list_pending_loans",
			Description: "List all pending loan applications awaiting approval. Manager/Admin only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "approve_loan",
			Description: "Approve a pending loan application. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"loan_id": map[string]any{"type": "integer", "description": "Loan ID to approve (from list_pending_loans)."},
				},
				"required": []string{"loan_id"},
			}),
		},
		{
			Name:        "reject_loan",
			Description: "Reject/cancel a pending loan application. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"loan_id": map[string]any{"type": "integer", "description": "Loan ID to reject (from list_pending_loans)."},
				},
				"required": []string{"loan_id"},
			}),
		},
		{
			Name:        "query_loan_eligibility",
			Description: "Check loan eligibility based on salary and existing loans. Returns max loan amount (3x monthly salary minus outstanding balances) and available loan types.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit to check current user."},
				},
			}),
		},
		{
			Name:        "query_my_benefits",
			Description: "Query the current user's benefit enrollments and pending claims. Returns plan names, categories, contribution shares, and claim status.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "list_pending_benefit_claims",
			Description: "List all pending benefit claims awaiting approval. Admin only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "approve_benefit_claim",
			Description: "Approve a pending benefit claim. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"claim_id": map[string]any{"type": "integer", "description": "Benefit claim ID to approve."},
				},
				"required": []string{"claim_id"},
			}),
		},
		{
			Name:        "reject_benefit_claim",
			Description: "Reject a pending benefit claim. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"claim_id": map[string]any{"type": "integer", "description": "Benefit claim ID to reject."},
					"reason":   map[string]any{"type": "string", "description": "Reason for rejection."},
				},
				"required": []string{"claim_id"},
			}),
		},
		{
			Name:        "query_encashment_eligibility",
			Description: "Query convertible leave balances and estimate encashment value. Returns leave types with remaining days and estimated PHP value based on daily rate.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "approve_leave_encashment",
			Description: "Approve a pending leave encashment request. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"encashment_id": map[string]any{"type": "integer", "description": "Leave encashment ID to approve."},
				},
				"required": []string{"encashment_id"},
			}),
		},
		// --- Phase 2: Performance + Training Tools ---
		{
			Name:        "list_review_cycles",
			Description: "List performance review cycles for the company. Returns cycle name, type, period, deadline, and status.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_my_performance_review",
			Description: "Get the current user's performance reviews. Returns review status, self-rating, final rating, and cycle information.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_goal",
			Description: "Create a performance goal from natural language. AI parses the input to extract title, category, weight, target value, and due date. Always confirm details with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":           map[string]any{"type": "string", "description": "Goal title."},
					"description":     map[string]any{"type": "string", "description": "Detailed goal description."},
					"category":        map[string]any{"type": "string", "description": "Category: individual, team, company. Default: individual."},
					"weight":          map[string]any{"type": "number", "description": "Goal weight percentage (0-100)."},
					"target_value":    map[string]any{"type": "string", "description": "Target value to achieve (e.g., '90%', '100 units')."},
					"due_date":        map[string]any{"type": "string", "description": "Due date in YYYY-MM-DD format."},
					"review_cycle_id": map[string]any{"type": "integer", "description": "Optional review cycle to link the goal to."},
				},
				"required": []string{"title"},
			}),
		},
		{
			Name:        "submit_self_review",
			Description: "Submit a self-assessment for a performance review. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"review_id": map[string]any{"type": "integer", "description": "Performance review ID (from get_my_performance_review)."},
					"rating":    map[string]any{"type": "integer", "description": "Self-rating 1-5 (1=Unsatisfactory, 3=Meets, 5=Outstanding)."},
					"comments":  map[string]any{"type": "string", "description": "Self-assessment comments."},
				},
				"required": []string{"review_id", "rating"},
			}),
		},
		{
			Name:        "submit_manager_review",
			Description: "Submit a manager review for an employee's performance. Manager/Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"review_id":      map[string]any{"type": "integer", "description": "Performance review ID."},
					"rating":         map[string]any{"type": "integer", "description": "Manager rating 1-5."},
					"comments":       map[string]any{"type": "string", "description": "Manager comments."},
					"strengths":      map[string]any{"type": "string", "description": "Employee strengths."},
					"improvements":   map[string]any{"type": "string", "description": "Areas for improvement."},
					"final_rating":   map[string]any{"type": "integer", "description": "Final overall rating 1-5. Defaults to manager rating if omitted."},
					"final_comments": map[string]any{"type": "string", "description": "Final review comments."},
				},
				"required": []string{"review_id", "rating"},
			}),
		},
		{
			Name:        "list_trainings",
			Description: "List available training programs. Returns title, type, trainer, dates, status, and participant count.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "list_my_certifications",
			Description: "Query the current user's professional certifications and flag expiring ones.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "enroll_in_training",
			Description: "Enroll the current user in a training program. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"training_id": map[string]any{"type": "integer", "description": "Training ID to enroll in (from list_trainings)."},
				},
				"required": []string{"training_id"},
			}),
		},
		{
			Name:        "mark_training_complete",
			Description: "Mark a training participant as completed. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"participant_id": map[string]any{"type": "integer", "description": "Participant ID."},
					"training_id":    map[string]any{"type": "integer", "description": "Training ID."},
					"score":          map[string]any{"type": "integer", "description": "Optional score 0-100."},
					"feedback":       map[string]any{"type": "string", "description": "Optional feedback."},
				},
				"required": []string{"participant_id", "training_id"},
			}),
		},
		// --- Phase 3: Disciplinary + Grievance Tools ---
		{
			Name:        "query_employee_disciplinary",
			Description: "Query an employee's disciplinary history including incident counts and action breakdown (warnings, suspensions). Manager/Admin only. Use this before creating actions to reference progressive discipline history.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Employee ID to query."},
				},
				"required": []string{"employee_id"},
			}),
		},
		{
			Name:        "create_disciplinary_incident",
			Description: "Create a disciplinary incident record from a verbal or written description. Admin/Manager only. AI extracts structured data: category, severity, date, witnesses. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":    map[string]any{"type": "integer", "description": "Employee ID involved."},
					"category":       map[string]any{"type": "string", "description": "Category: tardiness, absence, misconduct, insubordination, policy_violation, performance, safety."},
					"severity":       map[string]any{"type": "string", "description": "Severity: minor, moderate, major, grave. Default: minor."},
					"description":    map[string]any{"type": "string", "description": "Description of the incident."},
					"incident_date":  map[string]any{"type": "string", "description": "Incident date in YYYY-MM-DD format. Default: today."},
					"witnesses":      map[string]any{"type": "string", "description": "Names of witnesses, if any."},
					"evidence_notes": map[string]any{"type": "string", "description": "Notes about evidence collected."},
				},
				"required": []string{"employee_id", "category", "description"},
			}),
		},
		{
			Name:        "create_disciplinary_action",
			Description: "Create a disciplinary action (warning, suspension, etc.). Admin/Manager only. AI should suggest action level based on disciplinary history. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":     map[string]any{"type": "integer", "description": "Employee ID."},
					"action_type":     map[string]any{"type": "string", "description": "Action: verbal_warning, written_warning, final_warning, suspension, termination."},
					"description":     map[string]any{"type": "string", "description": "Action description."},
					"incident_id":     map[string]any{"type": "integer", "description": "Optional linked incident ID."},
					"suspension_days": map[string]any{"type": "integer", "description": "Number of suspension days (if suspension)."},
					"effective_date":  map[string]any{"type": "string", "description": "Effective date in YYYY-MM-DD format."},
					"end_date":        map[string]any{"type": "string", "description": "End date in YYYY-MM-DD format (for suspension)."},
					"notes":           map[string]any{"type": "string", "description": "Additional notes."},
				},
				"required": []string{"employee_id", "action_type", "description"},
			}),
		},
		{
			Name:        "list_recent_incidents",
			Description: "List recent disciplinary incidents across the company. Admin/Manager only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_grievance_summary",
			Description: "Get company-wide grievance/complaint statistics: open, under review, in mediation, resolved, and critical cases. Admin/Manager only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_grievance_detail",
			Description: "Get detailed information about a specific grievance case including comments thread. Admin/Manager only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"grievance_id": map[string]any{"type": "integer", "description": "Grievance case ID."},
				},
				"required": []string{"grievance_id"},
			}),
		},
		{
			Name:        "resolve_grievance",
			Description: "Resolve a grievance case with a resolution description. Admin only. AI can help draft the resolution. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"grievance_id": map[string]any{"type": "integer", "description": "Grievance case ID."},
					"resolution":   map[string]any{"type": "string", "description": "Resolution description."},
				},
				"required": []string{"grievance_id", "resolution"},
			}),
		},
		// --- Phase 4: Analytics + Tax Tools ---
		{
			Name:        "query_company_analytics",
			Description: "Get comprehensive company analytics: headcount, new hires, tenure, department costs. Use this when users ask about overall company status.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_headcount_trend",
			Description: "Get 12-month headcount trend showing active and separated employee counts per month.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_leave_utilization",
			Description: "Get leave utilization analysis for the current year. Shows total requests and days used by leave type.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "query_tax_filing_status",
			Description: "Get tax filing status for the current year: filed/overdue/upcoming counts, penalties, and detailed overdue items. Admin only.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "create_tax_filing_record",
			Description: "Create a tax filing record (BIR, SSS, PhilHealth, PagIBIG). Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filing_type":    map[string]any{"type": "string", "description": "Type: bir_1601c, sss_r3, philhealth_rf1, pagibig_ml1, bir_2316, bir_0619e."},
					"period_type":    map[string]any{"type": "string", "description": "Period: monthly, quarterly, annual. Default: monthly."},
					"period_year":    map[string]any{"type": "integer", "description": "Year. Default: current year."},
					"period_month":   map[string]any{"type": "integer", "description": "Month (1-12) for monthly filings."},
					"period_quarter": map[string]any{"type": "integer", "description": "Quarter (1-4) for quarterly filings."},
					"due_date":       map[string]any{"type": "string", "description": "Due date in YYYY-MM-DD format."},
					"amount":         map[string]any{"type": "number", "description": "Filing amount in PHP."},
				},
				"required": []string{"filing_type", "due_date"},
			}),
		},
		// --- Phase 5: Clearance + Final Pay Tools ---
		{
			Name:        "get_clearance_status",
			Description: "Get clearance/offboarding progress for an employee: items completed, pending departments, and overall status. Admin/Manager only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"clearance_id": map[string]any{"type": "integer", "description": "Clearance request ID."},
				},
				"required": []string{"clearance_id"},
			}),
		},
		{
			Name:        "update_clearance_item",
			Description: "Mark a clearance item as cleared or flagged. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"item_id": map[string]any{"type": "integer", "description": "Clearance item ID."},
					"status":  map[string]any{"type": "string", "description": "New status: cleared, flagged. Default: cleared."},
					"remarks": map[string]any{"type": "string", "description": "Optional remarks."},
				},
				"required": []string{"item_id"},
			}),
		},
		{
			Name:        "query_final_pay_components",
			Description: "Calculate final pay components for a separating employee: unpaid salary, leave encashment, pro-rated 13th month, minus outstanding loans. Admin only.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":     map[string]any{"type": "integer", "description": "Employee ID."},
					"separation_date": map[string]any{"type": "string", "description": "Separation date in YYYY-MM-DD. Default: today."},
					"unpaid_days":     map[string]any{"type": "number", "description": "Working days since last payroll. Default: 15."},
				},
				"required": []string{"employee_id"},
			}),
		},
		{
			Name:        "create_final_pay",
			Description: "Create a final pay payroll record for a separated employee. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Employee ID."},
					"amount":      map[string]any{"type": "number", "description": "Total final pay amount in PHP."},
					"notes":       map[string]any{"type": "string", "description": "Optional notes about the final pay."},
				},
				"required": []string{"employee_id", "amount"},
			}),
		},
		{
			Name:        "complete_clearance",
			Description: "Complete the clearance process for an employee. Requires all items to be cleared first. Admin only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"clearance_id": map[string]any{"type": "integer", "description": "Clearance request ID."},
				},
				"required": []string{"clearance_id"},
			}),
		},
		// --- Phase 6: Schedule Tools ---
		{
			Name:        "list_schedule_templates",
			Description: "List available schedule templates (shift patterns) for the company.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "get_employee_schedule",
			Description: "Get the current schedule assignment for the user or a specific employee. Returns the weekly schedule with shift times and rest days.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id": map[string]any{"type": "integer", "description": "Optional employee ID. Omit to check current user."},
				},
			}),
		},
		{
			Name:        "assign_schedule",
			Description: "Assign a schedule template to an employee. Admin/Manager only. Always confirm with the user before calling this tool.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"employee_id":    map[string]any{"type": "integer", "description": "Employee ID."},
					"template_id":    map[string]any{"type": "integer", "description": "Schedule template ID (from list_schedule_templates)."},
					"effective_date": map[string]any{"type": "string", "description": "Effective date in YYYY-MM-DD. Default: today."},
				},
				"required": []string{"employee_id", "template_id"},
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
	// Salary simulation
	r.tools["simulate_salary"] = r.toolSimulateSalary
	// Phase 1: Loan + Benefit + Leave Encashment
	r.tools["query_my_loans"] = r.toolQueryMyLoans
	r.tools["list_pending_loans"] = r.toolListPendingLoans
	r.tools["approve_loan"] = r.toolApproveLoan
	r.tools["reject_loan"] = r.toolRejectLoan
	r.tools["query_loan_eligibility"] = r.toolQueryLoanEligibility
	r.tools["query_my_benefits"] = r.toolQueryMyBenefits
	r.tools["list_pending_benefit_claims"] = r.toolListPendingBenefitClaims
	r.tools["approve_benefit_claim"] = r.toolApproveBenefitClaim
	r.tools["reject_benefit_claim"] = r.toolRejectBenefitClaim
	r.tools["query_encashment_eligibility"] = r.toolQueryEncashmentEligibility
	r.tools["approve_leave_encashment"] = r.toolApproveLeaveEncashment
	// Phase 2: Performance + Training
	r.tools["list_review_cycles"] = r.toolListReviewCycles
	r.tools["get_my_performance_review"] = r.toolGetMyPerformanceReview
	r.tools["create_goal"] = r.toolCreateGoal
	r.tools["submit_self_review"] = r.toolSubmitSelfReview
	r.tools["submit_manager_review"] = r.toolSubmitManagerReview
	r.tools["list_trainings"] = r.toolListTrainings
	r.tools["list_my_certifications"] = r.toolListMyCertifications
	r.tools["enroll_in_training"] = r.toolEnrollInTraining
	r.tools["mark_training_complete"] = r.toolMarkTrainingComplete
	// Phase 3: Disciplinary + Grievance
	r.tools["query_employee_disciplinary"] = r.toolQueryEmployeeDisciplinary
	r.tools["create_disciplinary_incident"] = r.toolCreateDisciplinaryIncident
	r.tools["create_disciplinary_action"] = r.toolCreateDisciplinaryAction
	r.tools["list_recent_incidents"] = r.toolListRecentIncidents
	r.tools["query_grievance_summary"] = r.toolQueryGrievanceSummary
	r.tools["get_grievance_detail"] = r.toolGetGrievanceDetail
	r.tools["resolve_grievance"] = r.toolResolveGrievance
	// Phase 4: Analytics + Tax
	r.tools["query_company_analytics"] = r.toolQueryCompanyAnalytics
	r.tools["query_headcount_trend"] = r.toolQueryHeadcountTrend
	r.tools["query_leave_utilization"] = r.toolQueryLeaveUtilization
	r.tools["query_tax_filing_status"] = r.toolQueryTaxFilingStatus
	r.tools["create_tax_filing_record"] = r.toolCreateTaxFilingRecord
	// Phase 5: Clearance + Final Pay
	r.tools["get_clearance_status"] = r.toolGetClearanceStatus
	r.tools["update_clearance_item"] = r.toolUpdateClearanceItem
	r.tools["query_final_pay_components"] = r.toolQueryFinalPayComponents
	r.tools["create_final_pay"] = r.toolCreateFinalPay
	r.tools["complete_clearance"] = r.toolCompleteClearance
	// Phase 6: Schedule
	r.tools["list_schedule_templates"] = r.toolListScheduleTemplates
	r.tools["get_employee_schedule"] = r.toolGetEmployeeSchedule
	r.tools["assign_schedule"] = r.toolAssignSchedule
}

