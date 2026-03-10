package approval

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// ApprovalContext holds the structured context data for a pending approval.
type ApprovalContext struct {
	RequestInfo    RequestInfo    `json:"request_info"`
	EmployeeInfo   EmployeeInfo   `json:"employee_info"`
	LeaveHistory   []LeaveRecord  `json:"leave_history,omitempty"`
	BalanceImpact  []BalanceEntry `json:"balance_impact,omitempty"`
	TeamConflicts  []Conflict     `json:"team_conflicts,omitempty"`
	Recommendation *string        `json:"recommendation,omitempty"`
}

// RequestInfo describes the pending request.
type RequestInfo struct {
	ID            int64  `json:"id"`
	EntityType    string `json:"entity_type"`
	StartDate     string `json:"start_date,omitempty"`
	EndDate       string `json:"end_date,omitempty"`
	Days          string `json:"days,omitempty"`
	Hours         string `json:"hours,omitempty"`
	LeaveTypeName string `json:"leave_type_name,omitempty"`
	OTType        string `json:"ot_type,omitempty"`
	Reason        string `json:"reason,omitempty"`
	PendingSince  string `json:"pending_since"`
}

// EmployeeInfo describes the requesting employee.
type EmployeeInfo struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Department string `json:"department,omitempty"`
	Position   string `json:"position,omitempty"`
	HireDate   string `json:"hire_date"`
	TenureStr  string `json:"tenure"`
}

// LeaveRecord is a summary of a past leave request.
type LeaveRecord struct {
	LeaveType string `json:"leave_type"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Days      string `json:"days"`
	Status    string `json:"status"`
}

// BalanceEntry represents a leave balance.
type BalanceEntry struct {
	LeaveType string  `json:"leave_type"`
	Earned    float64 `json:"earned"`
	Used      float64 `json:"used"`
	Remaining float64 `json:"remaining"`
}

// Conflict represents a team member on leave during the requested period.
type Conflict struct {
	Name      string `json:"name"`
	LeaveType string `json:"leave_type"`
	Dates     string `json:"dates"`
}

// GetApprovalContext returns enriched context for a leave or OT request.
func (h *Handler) GetApprovalContext(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	entityType := c.Query("entity_type")
	entityIDStr := c.Query("entity_id")
	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil || entityType == "" {
		response.BadRequest(c, "entity_type and entity_id are required")
		return
	}

	ctx := c.Request.Context()
	var ac ApprovalContext

	switch entityType {
	case "leave_request":
		ac, err = h.buildLeaveContext(ctx, companyID, entityID)
	case "overtime_request":
		ac, err = h.buildOTContext(ctx, companyID, entityID)
	default:
		response.BadRequest(c, "Unsupported entity type")
		return
	}

	if err != nil {
		h.logger.Error("failed to build approval context", "entity_type", entityType, "entity_id", entityID, "error", err)
		response.InternalError(c, "Failed to build context")
		return
	}

	// AI recommendation (optional)
	if h.aiProvider != nil {
		rec := h.generateRecommendation(ctx, ac)
		if rec != "" {
			ac.Recommendation = &rec
		}
	}

	response.OK(c, ac)
}

func (h *Handler) buildLeaveContext(ctx context.Context, companyID, leaveID int64) (ApprovalContext, error) {
	var ac ApprovalContext

	lr, err := h.queries.GetLeaveRequest(ctx, store.GetLeaveRequestParams{
		ID:        leaveID,
		CompanyID: companyID,
	})
	if err != nil {
		return ac, fmt.Errorf("leave request not found: %w", err)
	}

	emp, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
		ID:        lr.EmployeeID,
		CompanyID: companyID,
	})
	if err != nil {
		return ac, fmt.Errorf("employee not found: %w", err)
	}

	// Request info
	reason := ""
	if lr.Reason != nil {
		reason = *lr.Reason
	}
	ac.RequestInfo = RequestInfo{
		ID:           lr.ID,
		EntityType:   "leave_request",
		StartDate:    lr.StartDate.Format("2006-01-02"),
		EndDate:      lr.EndDate.Format("2006-01-02"),
		Days:         numericStr(lr.Days),
		Reason:       reason,
		PendingSince: lr.CreatedAt.Format(time.RFC3339),
	}

	// Get leave type name
	leaveTypes, err := h.queries.ListLeaveTypes(ctx, companyID)
	if err == nil {
		for _, lt := range leaveTypes {
			if lt.ID == lr.LeaveTypeID {
				ac.RequestInfo.LeaveTypeName = lt.Name
				break
			}
		}
	}

	// Employee info
	ac.EmployeeInfo = buildEmployeeInfo(emp)

	// Department name
	if emp.DepartmentID != nil {
		dept, err := h.queries.GetDepartmentByID(ctx, store.GetDepartmentByIDParams{
			ID:        *emp.DepartmentID,
			CompanyID: companyID,
		})
		if err == nil {
			ac.EmployeeInfo.Department = dept.Name
		}
	}

	// Leave history (last 6 months)
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	leaveRequests, err := h.queries.ListLeaveRequests(ctx, store.ListLeaveRequestsParams{
		CompanyID: companyID,
		Column2:   lr.EmployeeID,
		Column3:   "", // all statuses
		Limit:     20,
		Offset:    0,
	})
	if err == nil {
		for _, r := range leaveRequests {
			if r.CreatedAt.Before(sixMonthsAgo) || r.ID == lr.ID {
				continue
			}
			ac.LeaveHistory = append(ac.LeaveHistory, LeaveRecord{
				LeaveType: r.LeaveTypeName,
				StartDate: r.StartDate.Format("2006-01-02"),
				EndDate:   r.EndDate.Format("2006-01-02"),
				Days:      numericStr(r.Days),
				Status:    r.Status,
			})
		}
	}

	// Balance impact
	year := int32(time.Now().Year())
	balances, err := h.queries.ListLeaveBalances(ctx, store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: lr.EmployeeID,
		Year:       year,
	})
	if err == nil {
		for _, b := range balances {
			earned := numericFloat(b.Earned)
			used := numericFloat(b.Used)
			carried := numericFloat(b.Carried)
			adjusted := numericFloat(b.Adjusted)
			remaining := earned + carried + adjusted - used
			ac.BalanceImpact = append(ac.BalanceImpact, BalanceEntry{
				LeaveType: b.LeaveTypeName,
				Earned:    earned,
				Used:      used,
				Remaining: remaining,
			})
		}
	}

	// Team conflicts (same department, overlapping dates)
	if emp.DepartmentID != nil {
		conflicts, err := h.queries.ListApprovedLeavesForCalendar(ctx, store.ListApprovedLeavesForCalendarParams{
			CompanyID: companyID,
			EndDate:   lr.StartDate, // $2: leaves that end on or after this date
			StartDate: lr.EndDate,   // $3: leaves that start on or before this date
		})
		if err == nil {
			for _, c := range conflicts {
				if c.EmployeeID == lr.EmployeeID {
					continue
				}
				name := c.FirstName + " " + c.LastName
				ac.TeamConflicts = append(ac.TeamConflicts, Conflict{
					Name:      name,
					LeaveType: c.LeaveTypeName,
					Dates:     c.StartDate.Format("Jan 2") + " - " + c.EndDate.Format("Jan 2"),
				})
			}
		}
	}

	return ac, nil
}

func (h *Handler) buildOTContext(ctx context.Context, companyID, otID int64) (ApprovalContext, error) {
	var ac ApprovalContext

	ot, err := h.queries.GetOvertimeRequest(ctx, store.GetOvertimeRequestParams{
		ID:        otID,
		CompanyID: companyID,
	})
	if err != nil {
		return ac, fmt.Errorf("overtime request not found: %w", err)
	}

	emp, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
		ID:        ot.EmployeeID,
		CompanyID: companyID,
	})
	if err != nil {
		return ac, fmt.Errorf("employee not found: %w", err)
	}

	reason := ""
	if ot.Reason != nil {
		reason = *ot.Reason
	}
	ac.RequestInfo = RequestInfo{
		ID:           ot.ID,
		EntityType:   "overtime_request",
		StartDate:    ot.OtDate.Format("2006-01-02"),
		Hours:        numericStr(ot.Hours),
		OTType:       ot.OtType,
		Reason:       reason,
		PendingSince: ot.CreatedAt.Format(time.RFC3339),
	}

	ac.EmployeeInfo = buildEmployeeInfo(emp)

	if emp.DepartmentID != nil {
		dept, err := h.queries.GetDepartmentByID(ctx, store.GetDepartmentByIDParams{
			ID:        *emp.DepartmentID,
			CompanyID: companyID,
		})
		if err == nil {
			ac.EmployeeInfo.Department = dept.Name
		}
	}

	return ac, nil
}

func (h *Handler) generateRecommendation(ctx context.Context, ac ApprovalContext) string {
	if h.aiProvider == nil {
		return ""
	}

	prompt := fmt.Sprintf(
		"You are an HR assistant. Based on the following context, provide a brief 1-2 sentence recommendation (Approve, Review with Caution, or Reject) with reasoning.\n\n"+
			"Request: %s #%d — %s, %s days, reason: %s\n"+
			"Employee: %s, %s, tenure: %s\n"+
			"Leave balance impact: %d types tracked\n"+
			"Team conflicts during period: %d\n"+
			"Recent leave history (6mo): %d requests\n\n"+
			"Give a concise recommendation.",
		ac.RequestInfo.EntityType, ac.RequestInfo.ID,
		ac.RequestInfo.LeaveTypeName, ac.RequestInfo.Days, ac.RequestInfo.Reason,
		ac.EmployeeInfo.Name, ac.EmployeeInfo.Department, ac.EmployeeInfo.TenureStr,
		len(ac.BalanceImpact), len(ac.TeamConflicts), len(ac.LeaveHistory),
	)

	resp, err := h.aiProvider.Generate(ctx, provider.Request{
		System: "You are a concise HR recommendation engine. Reply with APPROVE, CAUTION, or REJECT followed by one brief sentence of reasoning.",
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 100,
	})
	if err != nil {
		h.logger.Error("AI recommendation failed", "error", err)
		return ""
	}

	return resp.Content
}

func buildEmployeeInfo(emp store.Employee) EmployeeInfo {
	name := emp.FirstName + " " + emp.LastName
	tenure := time.Since(emp.HireDate)
	months := int(tenure.Hours() / 24 / 30)
	years := months / 12
	remMonths := months % 12
	tenureStr := fmt.Sprintf("%dy %dm", years, remMonths)

	return EmployeeInfo{
		ID:       emp.ID,
		Name:     name,
		HireDate: emp.HireDate.Format("2006-01-02"),
		TenureStr: tenureStr,
	}
}

func numericStr(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "0"
	}
	return strconv.FormatFloat(f.Float64, 'f', -1, 64)
}

func numericFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
