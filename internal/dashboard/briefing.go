package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/numericutil"
	"github.com/tonypk/aigonhr/pkg/response"
)

// leaveBalanceItem is a serializable summary of a single leave type balance.
type leaveBalanceItem struct {
	LeaveType string  `json:"leave_type"`
	Code      string  `json:"code"`
	Earned    float64 `json:"earned"`
	Used      float64 `json:"used"`
	Remaining float64 `json:"remaining"`
}

// scheduleItem is a serializable representation of today's shift.
type scheduleItem struct {
	ShiftName string `json:"shift_name"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	IsRestDay bool   `json:"is_rest_day"`
}

// managerBriefing contains additional data visible to managers and admins.
type managerBriefing struct {
	PendingLeaveApprovals    int64           `json:"pending_leave_approvals"`
	PendingOvertimeApprovals int64           `json:"pending_overtime_approvals"`
	TodayPresent             int64           `json:"today_present"`
	TodayTotal               int64           `json:"today_total"`
	TodayLate                int64           `json:"today_late"`
	UpcomingPayrollDeadline  *payrollInfo    `json:"upcoming_payroll_deadline"`
	Alerts                   []briefingAlert `json:"alerts"`
}

// payrollInfo summarises the next payroll cycle.
type payrollInfo struct {
	CycleName   string `json:"cycle_name"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	PayDate     string `json:"pay_date"`
	Status      string `json:"status"`
}

// briefingAlert represents a single actionable alert for managers.
type briefingAlert struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Count   int64  `json:"count"`
}

// briefingResponse is the top-level response for GET /ai/briefing.
type briefingResponse struct {
	Greeting             string             `json:"greeting"`
	Date                 string             `json:"date"`
	Schedule             *scheduleItem      `json:"schedule"`
	LeaveBalances        []leaveBalanceItem `json:"leave_balances"`
	NextPayday           *payrollInfo       `json:"next_payday"`
	PendingExpenseClaims int64              `json:"pending_expense_claims"`
	UnreadNotifications  int64              `json:"unread_notifications"`
	Manager              *managerBriefing   `json:"manager,omitempty"`
}

// GetBriefing returns a personalised morning briefing for the authenticated user.
func (h *Handler) GetBriefing(c *gin.Context) {
	ctx := c.Request.Context()
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)
	role := auth.GetRole(c)
	today := time.Now()

	// Look up the employee record for the current user.
	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		h.logger.Warn("briefing: employee not found for user", "user_id", userID, "err", err)
		// Admin/manager users may not have an employee record — return basic briefing
		basicBriefing := briefingResponse{
			Greeting: "Good morning!",
			Date:     today.Format("2006-01-02"),
		}
		if role == auth.RoleSuperAdmin || role == auth.RoleAdmin || role == auth.RoleManager {
			basicBriefing.Manager = h.fetchManagerBriefing(ctx, companyID, today)
		}
		response.OK(c, basicBriefing)
		return
	}

	// Build greeting.
	displayName := emp.FirstName
	if emp.DisplayName != nil && *emp.DisplayName != "" {
		displayName = *emp.DisplayName
	}
	greeting := fmt.Sprintf("Good morning, %s!", displayName)

	// Today's schedule / shift.
	schedule := h.fetchTodaySchedule(ctx, companyID, emp.ID, today)

	// Leave balances for current year.
	leaveBalances := h.fetchLeaveBalances(ctx, companyID, emp.ID, today)

	// Next payday (next payroll cycle with pay_date >= today).
	nextPayday := h.fetchNextPayday(ctx, companyID, today)

	// Pending expense claims for this employee.
	pendingExpenses, _ := h.queries.CountExpenseClaims(ctx, store.CountExpenseClaimsParams{
		CompanyID:  companyID,
		Status:     "submitted",
		EmployeeID: emp.ID,
	})

	// Unread notifications.
	unreadNotifs, _ := h.queries.CountUnreadNotifications(ctx, userID)

	briefing := briefingResponse{
		Greeting:             greeting,
		Date:                 today.Format("2006-01-02"),
		Schedule:             schedule,
		LeaveBalances:        leaveBalances,
		NextPayday:           nextPayday,
		PendingExpenseClaims: pendingExpenses,
		UnreadNotifications:  unreadNotifs,
	}

	// Manager / Admin data.
	if role == auth.RoleSuperAdmin || role == auth.RoleAdmin || role == auth.RoleManager {
		briefing.Manager = h.fetchManagerBriefing(ctx, companyID, today)
	}

	response.OK(c, briefing)
}

// fetchTodaySchedule returns the employee's schedule for today, or nil.
func (h *Handler) fetchTodaySchedule(ctx context.Context, companyID, employeeID int64, today time.Time) *scheduleItem {
	todayDate := truncateToDate(today)
	schedules, err := h.queries.ListSchedules(ctx, store.ListSchedulesParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		WorkDate:   todayDate,
		WorkDate_2: todayDate,
	})
	if err != nil || len(schedules) == 0 {
		return nil
	}
	s := schedules[0]
	return &scheduleItem{
		ShiftName: s.ShiftName,
		StartTime: formatPgTime(s.StartTime),
		EndTime:   formatPgTime(s.EndTime),
		IsRestDay: s.IsRestDay,
	}
}

// fetchLeaveBalances returns a summary of leave balances for the current year.
func (h *Handler) fetchLeaveBalances(ctx context.Context, companyID, employeeID int64, today time.Time) []leaveBalanceItem {
	balances, err := h.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		Year:       int32(today.Year()),
	})
	if err != nil {
		return nil
	}
	items := make([]leaveBalanceItem, 0, len(balances))
	for _, b := range balances {
		earned := numericutil.ToFloat(b.Earned)
		used := numericutil.ToFloat(b.Used)
		carried := numericutil.ToFloat(b.Carried)
		adjusted := numericutil.ToFloat(b.Adjusted)
		remaining := earned + carried + adjusted - used
		items = append(items, leaveBalanceItem{
			LeaveType: b.LeaveTypeName,
			Code:      b.Code,
			Earned:    earned,
			Used:      used,
			Remaining: remaining,
		})
	}
	return items
}

// fetchNextPayday finds the next upcoming payroll cycle.
func (h *Handler) fetchNextPayday(ctx context.Context, companyID int64, today time.Time) *payrollInfo {
	var name, status string
	var periodStart, periodEnd, payDate time.Time

	err := h.pool.QueryRow(ctx, `
		SELECT name, period_start, period_end, pay_date, status
		FROM payroll_cycles
		WHERE company_id = $1 AND pay_date >= $2::date
		ORDER BY pay_date ASC
		LIMIT 1
	`, companyID, today).Scan(&name, &periodStart, &periodEnd, &payDate, &status)
	if err != nil {
		return nil
	}
	return &payrollInfo{
		CycleName:   name,
		PeriodStart: periodStart.Format("2006-01-02"),
		PeriodEnd:   periodEnd.Format("2006-01-02"),
		PayDate:     payDate.Format("2006-01-02"),
		Status:      status,
	}
}

// fetchManagerBriefing assembles extra data for manager/admin users.
func (h *Handler) fetchManagerBriefing(ctx context.Context, companyID int64, today time.Time) *managerBriefing {
	mb := &managerBriefing{}

	// Pending leave approvals.
	mb.PendingLeaveApprovals, _ = h.queries.CountLeaveRequests(ctx, store.CountLeaveRequestsParams{
		CompanyID: companyID,
		Column3:   "pending",
	})

	// Pending overtime approvals.
	mb.PendingOvertimeApprovals, _ = h.queries.CountOvertimeRequests(ctx, store.CountOvertimeRequestsParams{
		CompanyID: companyID,
		Column3:   "pending",
	})

	// Today's attendance summary.
	totalEmployees, _ := h.queries.CountEmployees(ctx, store.CountEmployeesParams{
		CompanyID: companyID,
	})
	mb.TodayTotal = totalEmployees

	attSummary, _ := h.queries.GetTodayAttendanceSummary(ctx, companyID)
	for _, row := range attSummary {
		mb.TodayPresent += row.Count
		if row.Status == "late" {
			mb.TodayLate = row.Count
		}
	}

	// Upcoming payroll deadline (next non-completed cycle).
	var pName, pStatus string
	var pStart, pEnd, pPayDate time.Time
	err := h.pool.QueryRow(ctx, `
		SELECT name, period_start, period_end, pay_date, status
		FROM payroll_cycles
		WHERE company_id = $1 AND status NOT IN ('completed', 'cancelled')
		ORDER BY pay_date ASC
		LIMIT 1
	`, companyID).Scan(&pName, &pStart, &pEnd, &pPayDate, &pStatus)
	if err == nil {
		mb.UpcomingPayrollDeadline = &payrollInfo{
			CycleName:   pName,
			PeriodStart: pStart.Format("2006-01-02"),
			PeriodEnd:   pEnd.Format("2006-01-02"),
			PayDate:     pPayDate.Format("2006-01-02"),
			Status:      pStatus,
		}
	}

	// Alerts.
	mb.Alerts = h.fetchAlerts(ctx, companyID, today)

	return mb
}

// fetchAlerts builds a list of actionable alerts for managers.
func (h *Handler) fetchAlerts(ctx context.Context, companyID int64, today time.Time) []briefingAlert {
	var alerts []briefingAlert

	// Consecutive absences: employees with no attendance for 3+ consecutive work days
	// who have previously clocked in at least once.
	var absentCount int64
	_ = h.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT e.id)
		FROM employees e
		WHERE e.company_id = $1 AND e.status = 'active'
		AND NOT EXISTS (
			SELECT 1 FROM attendance_logs al
			WHERE al.employee_id = e.id
			AND al.clock_in_at >= ($2::date - INTERVAL '3 days')
			AND al.clock_in_at < ($2::date + INTERVAL '1 day')
		)
		AND EXISTS (
			SELECT 1 FROM attendance_logs al2
			WHERE al2.employee_id = e.id
		)
	`, companyID, today).Scan(&absentCount)
	if absentCount > 0 {
		alerts = append(alerts, briefingAlert{
			Type:    "consecutive_absences",
			Message: fmt.Sprintf("%d employee(s) absent for 3+ consecutive days", absentCount),
			Count:   absentCount,
		})
	}

	// Expiring contracts (within 30 days).
	var expiringContracts int64
	_ = h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM employees
		WHERE company_id = $1 AND status = 'active'
		AND contract_end_date IS NOT NULL
		AND contract_end_date BETWEEN $2::date AND ($2::date + INTERVAL '30 days')
	`, companyID, today).Scan(&expiringContracts)
	if expiringContracts > 0 {
		alerts = append(alerts, briefingAlert{
			Type:    "expiring_contracts",
			Message: fmt.Sprintf("%d contract(s) expiring within 30 days", expiringContracts),
			Count:   expiringContracts,
		})
	}

	// Expiring documents (within 30 days).
	var expiringDocs int64
	_ = h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM employee_documents
		WHERE company_id = $1 AND expiry_date IS NOT NULL
		AND expiry_date BETWEEN $2::date AND ($2::date + INTERVAL '30 days')
	`, companyID, today).Scan(&expiringDocs)
	if expiringDocs > 0 {
		alerts = append(alerts, briefingAlert{
			Type:    "expiring_documents",
			Message: fmt.Sprintf("%d document(s) expiring within 30 days", expiringDocs),
			Count:   expiringDocs,
		})
	}

	if alerts == nil {
		alerts = []briefingAlert{}
	}

	return alerts
}

// truncateToDate returns the given time truncated to midnight (date only).
func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// formatPgTime converts a pgtype.Time to an "HH:MM" string.
func formatPgTime(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	// Microseconds since midnight.
	totalSecs := t.Microseconds / 1_000_000
	hours := totalSecs / 3600
	minutes := (totalSecs % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
