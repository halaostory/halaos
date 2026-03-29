package dashboard

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/numericutil"
	"github.com/halaostory/halaos/pkg/response"
)

// leavePrefill is returned when form_type=leave.
type leavePrefill struct {
	LeaveTypeID int64  `json:"leave_type_id"`
	LeaveType   string `json:"leave_type"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Days        int    `json:"days"`
	ReasonHint  string `json:"reason_hint"`
}

// expensePrefill is returned when form_type=expense.
type expensePrefill struct {
	CategoryID         int64    `json:"category_id"`
	CategoryName       string   `json:"category_name"`
	SuggestedAmount    float64  `json:"suggested_amount"`
	RecentDescriptions []string `json:"recent_descriptions"`
}

// overtimePrefill is returned when form_type=overtime.
type overtimePrefill struct {
	OTType          string  `json:"ot_type"`
	SuggestedHours  float64 `json:"suggested_hours"`
	CommonStartTime string  `json:"common_start_time"`
	CommonEndTime   string  `json:"common_end_time"`
}

// GetFormPrefill returns AI-powered form pre-fill suggestions based on the
// authenticated user's history.
func (h *Handler) GetFormPrefill(c *gin.Context) {
	ctx := c.Request.Context()
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	formType := c.Query("form_type")
	if formType == "" {
		response.BadRequest(c, "form_type query parameter is required (leave, expense, overtime)")
		return
	}

	// Look up the employee record for the current user.
	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		h.logger.Warn("form-prefill: employee not found", "user_id", userID, "err", err)
		response.NotFound(c, "Employee record not found for the current user")
		return
	}

	switch formType {
	case "leave":
		result := h.prefillLeave(ctx, companyID, emp.ID)
		response.OK(c, result)
	case "expense":
		result := h.prefillExpense(ctx, companyID, emp.ID)
		response.OK(c, result)
	case "overtime":
		result := h.prefillOvertime(ctx, companyID, emp.ID)
		response.OK(c, result)
	case "loan":
		result := h.prefillLoan(ctx, companyID, emp.ID)
		response.OK(c, result)
	case "disciplinary":
		result := h.prefillDisciplinary(ctx, companyID)
		response.OK(c, result)
	case "clearance":
		result := h.prefillClearance(ctx, companyID)
		response.OK(c, result)
	default:
		response.BadRequest(c, "Invalid form_type. Accepted values: leave, expense, overtime, loan, disciplinary, clearance")
	}
}

// prefillLeave suggests leave form fields based on the user's history.
func (h *Handler) prefillLeave(ctx context.Context, companyID, employeeID int64) leavePrefill {
	result := leavePrefill{
		Days: 1,
	}

	// Find the most commonly used leave type for this employee.
	var leaveTypeID int64
	var leaveTypeName string
	err := h.pool.QueryRow(ctx, `
		SELECT lr.leave_type_id, lt.name
		FROM leave_requests lr
		JOIN leave_types lt ON lt.id = lr.leave_type_id
		WHERE lr.company_id = $1
		  AND lr.employee_id = $2
		  AND lr.status IN ('approved', 'pending')
		GROUP BY lr.leave_type_id, lt.name
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, companyID, employeeID).Scan(&leaveTypeID, &leaveTypeName)
	if err == nil {
		result.LeaveTypeID = leaveTypeID
		result.LeaveType = leaveTypeName
	}

	// Get the most recent leave request for reason hint.
	var lastReason *string
	_ = h.pool.QueryRow(ctx, `
		SELECT reason
		FROM leave_requests
		WHERE company_id = $1 AND employee_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, companyID, employeeID).Scan(&lastReason)
	if lastReason != nil && *lastReason != "" {
		result.ReasonHint = *lastReason
	}

	// Suggest dates based on leave type.
	now := time.Now()
	isSickLeave := isSickType(leaveTypeName)

	if isSickLeave {
		// Sick leave: suggest tomorrow.
		tomorrow := now.AddDate(0, 0, 1)
		result.StartDate = tomorrow.Format("2006-01-02")
		result.EndDate = tomorrow.Format("2006-01-02")
		result.Days = 1
	} else {
		// Vacation / other: suggest next Monday through Friday.
		nextMon := nextWeekdayFrom(now, time.Monday)
		nextFri := nextMon.AddDate(0, 0, 4)
		result.StartDate = nextMon.Format("2006-01-02")
		result.EndDate = nextFri.Format("2006-01-02")
		result.Days = 5
	}

	return result
}

// prefillExpense suggests expense form fields based on the user's history.
func (h *Handler) prefillExpense(ctx context.Context, companyID, employeeID int64) expensePrefill {
	result := expensePrefill{
		RecentDescriptions: []string{},
	}

	// Find the most commonly used expense category.
	var categoryID int64
	var categoryName string
	err := h.pool.QueryRow(ctx, `
		SELECT ec.category_id, cat.name
		FROM expense_claims ec
		JOIN expense_categories cat ON cat.id = ec.category_id
		WHERE ec.company_id = $1 AND ec.employee_id = $2
		GROUP BY ec.category_id, cat.name
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, companyID, employeeID).Scan(&categoryID, &categoryName)
	if err == nil {
		result.CategoryID = categoryID
		result.CategoryName = categoryName

		// Calculate average amount for the most common category.
		var avgAmount pgtype.Numeric
		err2 := h.pool.QueryRow(ctx, `
			SELECT AVG(amount)
			FROM expense_claims
			WHERE company_id = $1 AND employee_id = $2 AND category_id = $3
		`, companyID, employeeID, categoryID).Scan(&avgAmount)
		if err2 == nil {
			result.SuggestedAmount = math.Round(numericutil.ToFloat(avgAmount)*100) / 100
		}
	}

	// Get recent descriptions from the last 5 expense claims.
	rows, err := h.pool.Query(ctx, `
		SELECT description
		FROM expense_claims
		WHERE company_id = $1 AND employee_id = $2
		ORDER BY created_at DESC
		LIMIT 5
	`, companyID, employeeID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var desc string
			if scanErr := rows.Scan(&desc); scanErr == nil && desc != "" {
				result.RecentDescriptions = append(result.RecentDescriptions, desc)
			}
		}
	}

	return result
}

// prefillOvertime suggests overtime form fields based on the user's history.
func (h *Handler) prefillOvertime(ctx context.Context, companyID, employeeID int64) overtimePrefill {
	result := overtimePrefill{
		OTType:          "regular",
		SuggestedHours:  2,
		CommonStartTime: "18:00",
		CommonEndTime:   "20:00",
	}

	// Find the most common OT type.
	var otType string
	err := h.pool.QueryRow(ctx, `
		SELECT ot_type
		FROM overtime_requests
		WHERE company_id = $1 AND employee_id = $2
		  AND status IN ('approved', 'pending')
		GROUP BY ot_type
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, companyID, employeeID).Scan(&otType)
	if err == nil {
		result.OTType = otType
	}

	// Calculate average hours from recent OT requests.
	var avgHours pgtype.Numeric
	err = h.pool.QueryRow(ctx, `
		SELECT AVG(hours)
		FROM overtime_requests
		WHERE company_id = $1 AND employee_id = $2
		  AND status IN ('approved', 'pending')
	`, companyID, employeeID).Scan(&avgHours)
	if err == nil {
		avg := math.Round(numericutil.ToFloat(avgHours)*100) / 100
		if avg > 0 {
			result.SuggestedHours = avg
		}
	}

	// Find most common start and end times from the most recent OT request.
	var startTime, endTime string
	err = h.pool.QueryRow(ctx, `
		SELECT
			TO_CHAR(start_at::time, 'HH24:MI') AS common_start,
			TO_CHAR(end_at::time, 'HH24:MI') AS common_end
		FROM overtime_requests
		WHERE company_id = $1 AND employee_id = $2
		  AND status IN ('approved', 'pending')
		ORDER BY created_at DESC
		LIMIT 1
	`, companyID, employeeID).Scan(&startTime, &endTime)
	if err == nil && startTime != "" && endTime != "" {
		result.CommonStartTime = startTime
		result.CommonEndTime = endTime
	}

	return result
}

// nextWeekdayFrom returns the next occurrence of the specified weekday after t.
// If t is already that weekday, it returns the following week.
func nextWeekdayFrom(t time.Time, day time.Weekday) time.Time {
	daysUntil := int(day - t.Weekday())
	if daysUntil <= 0 {
		daysUntil += 7
	}
	return t.AddDate(0, 0, daysUntil)
}

// isSickType returns true if the leave type name indicates sick leave.
func isSickType(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "sick") || lower == "sl"
}

// loanPrefill is returned when form_type=loan.
type loanPrefill struct {
	LoanTypeID      int64   `json:"loan_type_id"`
	LoanTypeName    string  `json:"loan_type_name"`
	MaxAmount       float64 `json:"max_amount"`
	SuggestedTerm   int     `json:"suggested_term"`
	HasExistingLoan bool    `json:"has_existing_loan"`
}

// prefillLoan suggests loan form fields based on salary and existing loans.
func (h *Handler) prefillLoan(ctx context.Context, companyID, employeeID int64) loanPrefill {
	result := loanPrefill{
		SuggestedTerm: 12,
	}

	// Get available loan types
	var loanTypeID int64
	var loanTypeName string
	_ = h.pool.QueryRow(ctx, `
		SELECT id, name FROM loan_types
		WHERE company_id = $1 AND is_active = true
		ORDER BY name LIMIT 1
	`, companyID).Scan(&loanTypeID, &loanTypeName)
	result.LoanTypeID = loanTypeID
	result.LoanTypeName = loanTypeName

	// Get current salary to compute max loan amount (3x monthly)
	var basicSalary pgtype.Numeric
	_ = h.pool.QueryRow(ctx, `
		SELECT basic_salary FROM employee_salaries
		WHERE company_id = $1 AND employee_id = $2
		  AND effective_from <= NOW()
		  AND (effective_to IS NULL OR effective_to >= NOW())
		ORDER BY effective_from DESC LIMIT 1
	`, companyID, employeeID).Scan(&basicSalary)

	salaryFloat := numericutil.ToFloat(basicSalary)
	maxAmount := salaryFloat * 3

	// Check existing active loans
	var existingBalance pgtype.Numeric
	_ = h.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(remaining_balance), 0) FROM loans
		WHERE company_id = $1 AND employee_id = $2 AND status IN ('active', 'approved')
	`, companyID, employeeID).Scan(&existingBalance)

	outstanding := numericutil.ToFloat(existingBalance)
	result.MaxAmount = math.Max(0, maxAmount-outstanding)
	result.HasExistingLoan = outstanding > 0

	return result
}

// disciplinaryPrefill is returned when form_type=disciplinary.
type disciplinaryPrefill struct {
	SuggestedSeverity string   `json:"suggested_severity"`
	Categories        []string `json:"categories"`
	ActionTypes       []string `json:"action_types"`
}

// prefillDisciplinary provides disciplinary form defaults.
func (h *Handler) prefillDisciplinary(_ context.Context, _ int64) disciplinaryPrefill {
	return disciplinaryPrefill{
		SuggestedSeverity: "minor",
		Categories: []string{
			"tardiness", "absence", "misconduct", "insubordination",
			"policy_violation", "performance", "safety",
		},
		ActionTypes: []string{
			"verbal_warning", "written_warning", "final_warning",
			"suspension", "termination",
		},
	}
}

// clearancePrefill is returned when form_type=clearance.
type clearancePrefill struct {
	Departments    []string `json:"departments"`
	TemplateItems  int      `json:"template_items"`
}

// prefillClearance provides clearance form defaults with template items.
func (h *Handler) prefillClearance(ctx context.Context, companyID int64) clearancePrefill {
	result := clearancePrefill{
		Departments: []string{"IT", "HR", "Finance", "Admin", "Manager"},
	}

	// Count template items
	templates, err := h.queries.ListClearanceTemplates(ctx, companyID)
	if err == nil {
		result.TemplateItems = len(templates)
	}

	return result
}
