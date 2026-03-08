package dashboard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/numericutil"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

func (h *Handler) GetStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	totalEmployees, _ := h.queries.CountEmployees(c.Request.Context(), store.CountEmployeesParams{
		CompanyID: companyID,
	})
	attendanceSummary, _ := h.queries.GetTodayAttendanceSummary(c.Request.Context(), companyID)
	var presentToday int64
	for _, s := range attendanceSummary {
		presentToday += s.Count
	}
	pendingLeaves, _ := h.queries.CountLeaveRequests(c.Request.Context(), store.CountLeaveRequestsParams{
		CompanyID: companyID,
		Column3:   "pending",
	})
	pendingOT, _ := h.queries.CountOvertimeRequests(c.Request.Context(), store.CountOvertimeRequestsParams{
		CompanyID: companyID,
		Column3:   "pending",
	})
	response.OK(c, gin.H{
		"total_employees":  totalEmployees,
		"present_today":    presentToday,
		"pending_leaves":   pendingLeaves,
		"pending_overtime": pendingOT,
	})
}

func (h *Handler) GetAttendance(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	summary, _ := h.queries.GetTodayAttendanceSummary(c.Request.Context(), companyID)
	response.OK(c, summary)
}

func (h *Handler) GetDepartmentDistribution(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT d.name, COUNT(e.id) as count
		FROM employees e
		JOIN departments d ON d.id = e.department_id
		WHERE e.company_id = $1 AND e.status = 'active'
		GROUP BY d.name
		ORDER BY count DESC
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()
	var result []gin.H
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err == nil {
			result = append(result, gin.H{"name": name, "count": count})
		}
	}
	response.OK(c, result)
}

func (h *Handler) GetPayrollTrend(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT pc.name, pr.total_gross, pr.total_deductions, pr.total_net, pr.total_employees
		FROM payroll_runs pr
		JOIN payroll_cycles pc ON pc.id = pr.cycle_id
		WHERE pr.company_id = $1 AND pr.status = 'completed'
		ORDER BY pc.period_start DESC
		LIMIT 12
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()
	var result []gin.H
	for rows.Next() {
		var name string
		var gross, deductions, net pgtype.Numeric
		var employees int32
		if err := rows.Scan(&name, &gross, &deductions, &net, &employees); err == nil {
			result = append(result, gin.H{
				"name":       name,
				"gross":      numericutil.ToFloat(gross),
				"deductions": numericutil.ToFloat(deductions),
				"net":        numericutil.ToFloat(net),
				"employees":  employees,
			})
		}
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	response.OK(c, result)
}

func (h *Handler) GetLeaveSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT lt.name, COUNT(lr.id) as count
		FROM leave_requests lr
		JOIN leave_types lt ON lt.id = lr.leave_type_id
		WHERE lr.company_id = $1
		  AND lr.status = 'approved'
		  AND EXTRACT(YEAR FROM lr.start_date) = EXTRACT(YEAR FROM NOW())
		GROUP BY lt.name
		ORDER BY count DESC
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()
	var result []gin.H
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err == nil {
			result = append(result, gin.H{"name": name, "count": count})
		}
	}
	response.OK(c, result)
}

func (h *Handler) GetActionItems(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	type ActionItem struct {
		Category string `json:"category"`
		Label    string `json:"label"`
		Count    int64  `json:"count"`
		Route    string `json:"route"`
	}
	var items []ActionItem

	pendingLeaves, _ := h.queries.CountLeaveRequests(ctx, store.CountLeaveRequestsParams{
		CompanyID: companyID, Column3: "pending",
	})
	if pendingLeaves > 0 {
		items = append(items, ActionItem{"approvals", "Pending Leave Requests", pendingLeaves, "/approvals"})
	}
	pendingOT, _ := h.queries.CountOvertimeRequests(ctx, store.CountOvertimeRequestsParams{
		CompanyID: companyID, Column3: "pending",
	})
	if pendingOT > 0 {
		items = append(items, ActionItem{"approvals", "Pending Overtime Requests", pendingOT, "/approvals"})
	}
	var pendingLoans int64
	_ = h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM loans WHERE company_id = $1 AND status = 'pending'`, companyID).Scan(&pendingLoans)
	if pendingLoans > 0 {
		items = append(items, ActionItem{"approvals", "Pending Loan Applications", pendingLoans, "/loans"})
	}
	var draftPayroll int64
	_ = h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM payroll_cycles WHERE company_id = $1 AND status IN ('draft', 'computed')`, companyID).Scan(&draftPayroll)
	if draftPayroll > 0 {
		items = append(items, ActionItem{"payroll", "Payroll Cycles Pending", draftPayroll, "/payroll"})
	}
	var noSalary int64
	_ = h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM employees e
		WHERE e.company_id = $1 AND e.status = 'active'
		AND NOT EXISTS (SELECT 1 FROM employee_salaries es WHERE es.employee_id = e.id)
	`, companyID).Scan(&noSalary)
	if noSalary > 0 {
		items = append(items, ActionItem{"data_gaps", "Employees Without Salary", noSalary, "/employees"})
	}
	var expiringDocs int64
	_ = h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM employee_documents
		WHERE company_id = $1 AND expiry_date IS NOT NULL
		AND expiry_date BETWEEN NOW()::date AND (NOW() + INTERVAL '30 days')::date
	`, companyID).Scan(&expiringDocs)
	if expiringDocs > 0 {
		items = append(items, ActionItem{"compliance", "Documents Expiring Soon", expiringDocs, "/employees"})
	}
	response.OK(c, items)
}

func (h *Handler) GetCelebrations(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	daysAhead := 7

	bRows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, employee_no, first_name, last_name, birth_date
		FROM employees
		WHERE company_id = $1 AND status = 'active' AND birth_date IS NOT NULL
		  AND (
		    (EXTRACT(MONTH FROM birth_date) = EXTRACT(MONTH FROM CURRENT_DATE)
		     AND EXTRACT(DAY FROM birth_date) BETWEEN EXTRACT(DAY FROM CURRENT_DATE) AND EXTRACT(DAY FROM CURRENT_DATE) + $2)
		    OR
		    (EXTRACT(MONTH FROM birth_date) = EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
		     AND EXTRACT(DAY FROM birth_date) <= EXTRACT(DAY FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
		     AND EXTRACT(MONTH FROM CURRENT_DATE) != EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL))
		  )
		ORDER BY EXTRACT(MONTH FROM birth_date), EXTRACT(DAY FROM birth_date)
		LIMIT 20
	`, companyID, daysAhead)
	var birthdays []gin.H
	if err == nil {
		defer bRows.Close()
		for bRows.Next() {
			var id int64
			var empNo, firstName, lastName string
			var birthDate pgtype.Date
			if err := bRows.Scan(&id, &empNo, &firstName, &lastName, &birthDate); err == nil {
				bd := ""
				if birthDate.Valid {
					bd = birthDate.Time.Format("01-02")
				}
				birthdays = append(birthdays, gin.H{
					"id": id, "employee_no": empNo,
					"name": firstName + " " + lastName,
					"date": bd,
				})
			}
		}
	}

	aRows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, employee_no, first_name, last_name, hire_date,
		       EXTRACT(YEAR FROM AGE(CURRENT_DATE, hire_date))::int as years
		FROM employees
		WHERE company_id = $1 AND status = 'active'
		  AND EXTRACT(YEAR FROM AGE(CURRENT_DATE, hire_date)) >= 1
		  AND (
		    (EXTRACT(MONTH FROM hire_date) = EXTRACT(MONTH FROM CURRENT_DATE)
		     AND EXTRACT(DAY FROM hire_date) BETWEEN EXTRACT(DAY FROM CURRENT_DATE) AND EXTRACT(DAY FROM CURRENT_DATE) + $2)
		    OR
		    (EXTRACT(MONTH FROM hire_date) = EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
		     AND EXTRACT(DAY FROM hire_date) <= EXTRACT(DAY FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
		     AND EXTRACT(MONTH FROM CURRENT_DATE) != EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL))
		  )
		ORDER BY EXTRACT(MONTH FROM hire_date), EXTRACT(DAY FROM hire_date)
		LIMIT 20
	`, companyID, daysAhead)
	var anniversaries []gin.H
	if err == nil {
		defer aRows.Close()
		for aRows.Next() {
			var id int64
			var empNo, firstName, lastName string
			var hireDate time.Time
			var years int
			if err := aRows.Scan(&id, &empNo, &firstName, &lastName, &hireDate, &years); err == nil {
				anniversaries = append(anniversaries, gin.H{
					"id": id, "employee_no": empNo,
					"name":  firstName + " " + lastName,
					"date":  hireDate.Format("01-02"),
					"years": years,
				})
			}
		}
	}
	response.OK(c, gin.H{
		"birthdays":     birthdays,
		"anniversaries": anniversaries,
	})
}

func (h *Handler) GetSuggestions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	today := time.Now()

	type suggestion struct {
		Type        string      `json:"type"`
		Priority    string      `json:"priority"`
		Title       string      `json:"title"`
		Description string      `json:"description"`
		Count       int         `json:"count,omitempty"`
		Items       interface{} `json:"items,omitempty"`
	}

	var suggestions []suggestion

	regDue, _ := h.queries.ListEmployeesDueForRegularization(c.Request.Context(), store.ListEmployeesDueForRegularizationParams{
		CompanyID: companyID, Column2: today,
	})
	if len(regDue) > 0 {
		suggestions = append(suggestions, suggestion{
			Type: "regularization", Priority: "high",
			Title:       fmt.Sprintf("%d employee(s) due for regularization", len(regDue)),
			Description: "These probationary employees are approaching or past their regularization date.",
			Count: len(regDue), Items: regDue,
		})
	}

	expiring, _ := h.queries.ListExpiringContracts(c.Request.Context(), store.ListExpiringContractsParams{
		CompanyID: companyID, Column2: today,
	})
	if len(expiring) > 0 {
		suggestions = append(suggestions, suggestion{
			Type: "contract_expiry", Priority: "high",
			Title:       fmt.Sprintf("%d contract(s) expiring within 60 days", len(expiring)),
			Description: "Review these contractual employees for renewal or separation.",
			Count: len(expiring), Items: expiring,
		})
	}

	birthdays, _ := h.queries.ListUpcomingBirthdays(c.Request.Context(), store.ListUpcomingBirthdaysParams{
		CompanyID: companyID, Column2: today,
	})
	if len(birthdays) > 0 {
		suggestions = append(suggestions, suggestion{
			Type: "birthday", Priority: "low",
			Title:       fmt.Sprintf("%d upcoming birthday(s) in the next 30 days", len(birthdays)),
			Description: "Send greetings to celebrate your team members.",
			Count: len(birthdays), Items: birthdays,
		})
	}

	pendingTasks, _ := h.queries.ListPendingOnboardingTasks(c.Request.Context(), companyID)
	if len(pendingTasks) > 0 {
		suggestions = append(suggestions, suggestion{
			Type: "onboarding", Priority: "medium",
			Title:       fmt.Sprintf("%d pending onboarding task(s)", len(pendingTasks)),
			Description: "Complete these onboarding tasks to ensure smooth employee integration.",
			Count: len(pendingTasks), Items: pendingTasks,
		})
	}

	noSalaryCount, _ := h.queries.CountEmployeesWithNoSalary(c.Request.Context(), store.CountEmployeesWithNoSalaryParams{
		CompanyID: companyID, EffectiveFrom: today,
	})
	if noSalaryCount > 0 {
		suggestions = append(suggestions, suggestion{
			Type: "missing_salary", Priority: "high",
			Title:       fmt.Sprintf("%d employee(s) have no salary record", noSalaryCount),
			Description: "Assign salary to these employees to include them in payroll.",
			Count: int(noSalaryCount),
		})
	}

	response.OK(c, suggestions)
}

// GetFlightRisk returns the top 10 employees with the highest flight risk scores.
func (h *Handler) GetFlightRisk(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT ers.employee_id, ers.risk_score, ers.factors, ers.calculated_at,
		       e.employee_no, e.first_name, e.last_name,
		       COALESCE(d.name, '') as department
		FROM employee_risk_scores ers
		JOIN employees e ON e.id = ers.employee_id
		LEFT JOIN departments d ON d.id = e.department_id
		WHERE ers.company_id = $1
		ORDER BY ers.risk_score DESC
		LIMIT 10
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()

	type riskFactor struct {
		Factor string `json:"factor"`
		Points int    `json:"points"`
		Detail string `json:"detail"`
	}

	var result []gin.H
	for rows.Next() {
		var employeeID int64
		var riskScore int
		var factorsJSON []byte
		var calculatedAt time.Time
		var employeeNo, firstName, lastName, department string

		if err := rows.Scan(
			&employeeID, &riskScore, &factorsJSON, &calculatedAt,
			&employeeNo, &firstName, &lastName, &department,
		); err != nil {
			continue
		}

		var factors []riskFactor
		_ = json.Unmarshal(factorsJSON, &factors)

		result = append(result, gin.H{
			"employee_id":   employeeID,
			"employee_no":   employeeNo,
			"name":          firstName + " " + lastName,
			"department":    department,
			"risk_score":    riskScore,
			"factors":       factors,
			"calculated_at": calculatedAt,
		})
	}

	response.OK(c, result)
}

func (h *Handler) GetTeamHealth(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT department_id, department_name, health_score, factors, calculated_at
		FROM team_health_scores
		WHERE company_id = $1
		ORDER BY health_score ASC
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()

	type healthFactor struct {
		Name   string `json:"name"`
		Score  int    `json:"score"`
		Detail string `json:"detail"`
	}

	var result []gin.H
	for rows.Next() {
		var departmentID int64
		var healthScore int
		var factorsJSON []byte
		var calculatedAt time.Time
		var departmentName string

		if err := rows.Scan(
			&departmentID, &departmentName, &healthScore, &factorsJSON, &calculatedAt,
		); err != nil {
			continue
		}

		var factors []healthFactor
		_ = json.Unmarshal(factorsJSON, &factors)

		result = append(result, gin.H{
			"department_id":   departmentID,
			"department_name": departmentName,
			"health_score":    healthScore,
			"factors":         factors,
			"calculated_at":   calculatedAt,
		})
	}

	response.OK(c, result)
}

func (h *Handler) GetBurnoutRisk(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT ebs.employee_id, ebs.burnout_score, ebs.factors, ebs.calculated_at,
		       e.employee_no, e.first_name, e.last_name,
		       COALESCE(d.name, '') as department
		FROM employee_burnout_scores ebs
		JOIN employees e ON e.id = ebs.employee_id
		LEFT JOIN departments d ON d.id = e.department_id
		WHERE ebs.company_id = $1
		ORDER BY ebs.burnout_score DESC
		LIMIT 10
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()

	type burnoutFactor struct {
		Factor string `json:"factor"`
		Points int    `json:"points"`
		Detail string `json:"detail"`
	}

	var result []gin.H
	for rows.Next() {
		var employeeID int64
		var burnoutScore int
		var factorsJSON []byte
		var calculatedAt time.Time
		var employeeNo, firstName, lastName, department string

		if err := rows.Scan(
			&employeeID, &burnoutScore, &factorsJSON, &calculatedAt,
			&employeeNo, &firstName, &lastName, &department,
		); err != nil {
			continue
		}

		var factors []burnoutFactor
		_ = json.Unmarshal(factorsJSON, &factors)

		result = append(result, gin.H{
			"employee_id":   employeeID,
			"employee_no":   employeeNo,
			"name":          firstName + " " + lastName,
			"department":    department,
			"burnout_score": burnoutScore,
			"factors":       factors,
			"calculated_at": calculatedAt,
		})
	}

	response.OK(c, result)
}

func (h *Handler) GetComplianceAlerts(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, alert_type, severity, title, description,
		       entity_type, entity_id, due_date, days_remaining, calculated_at
		FROM compliance_alerts
		WHERE company_id = $1 AND is_resolved = false
		ORDER BY
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				ELSE 4
			END,
			days_remaining ASC
		LIMIT 20
	`, companyID)
	if err != nil {
		response.OK(c, []any{})
		return
	}
	defer rows.Close()

	var result []gin.H
	for rows.Next() {
		var id, entityID int64
		var daysRemaining int
		var alertType, severity, title, description string
		var entityType *string
		var dueDate *time.Time
		var calculatedAt time.Time

		if err := rows.Scan(
			&id, &alertType, &severity, &title, &description,
			&entityType, &entityID, &dueDate, &daysRemaining, &calculatedAt,
		); err != nil {
			continue
		}

		result = append(result, gin.H{
			"id":             id,
			"alert_type":     alertType,
			"severity":       severity,
			"title":          title,
			"description":    description,
			"entity_type":    entityType,
			"entity_id":      entityID,
			"due_date":       dueDate,
			"days_remaining": daysRemaining,
			"calculated_at":  calculatedAt,
		})
	}

	response.OK(c, result)
}
