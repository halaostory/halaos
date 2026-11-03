package leave

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/access"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/pagination"
	"github.com/halaostory/halaos/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	email   *email.Sender
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, emailSender *email.Sender) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, email: emailSender}
}

func (h *Handler) ListTypes(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	types, err := h.queries.ListLeaveTypes(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list leave types")
		return
	}
	response.OK(c, types)
}

func (h *Handler) CreateType(c *gin.Context) {
	var req struct {
		Code               string  `json:"code" binding:"required"`
		Name               string  `json:"name" binding:"required"`
		IsPaid             bool    `json:"is_paid"`
		DefaultDays        string  `json:"default_days"`
		IsConvertible      bool    `json:"is_convertible"`
		RequiresAttachment bool    `json:"requires_attachment"`
		MinDaysNotice      int32   `json:"min_days_notice"`
		AccrualType        string  `json:"accrual_type"`
		GenderSpecific     *string `json:"gender_specific"`
		IsStatutory        bool    `json:"is_statutory"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	accrualType := req.AccrualType
	if accrualType == "" {
		accrualType = "annual"
	}
	defaultDaysStr := req.DefaultDays
	if defaultDaysStr == "" {
		defaultDaysStr = "0"
	}
	var defaultDays pgtype.Numeric
	_ = defaultDays.Scan(defaultDaysStr)

	lt, err := h.queries.CreateLeaveType(c.Request.Context(), store.CreateLeaveTypeParams{
		CompanyID:          companyID,
		Code:               req.Code,
		Name:               req.Name,
		IsPaid:             req.IsPaid,
		DefaultDays:        defaultDays,
		IsConvertible:      req.IsConvertible,
		RequiresAttachment: req.RequiresAttachment,
		MinDaysNotice:      req.MinDaysNotice,
		AccrualType:        accrualType,
		GenderSpecific:     req.GenderSpecific,
		IsStatutory:        req.IsStatutory,
	})
	if err != nil {
		response.Conflict(c, "Leave type code already exists")
		return
	}
	response.Created(c, lt)
}

func (h *Handler) GetBalances(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		// Admin/manager users may not have an employee record — return empty list
		response.OK(c, []any{})
		return
	}

	year := time.Now().Year()
	balances, err := h.queries.ListLeaveBalances(c.Request.Context(), store.ListLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Year:       int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to get balances")
		return
	}
	response.OK(c, balances)
}

func (h *Handler) CreateRequest(c *gin.Context) {
	var req struct {
		LeaveTypeID    int64   `json:"leave_type_id" binding:"required"`
		StartDate      string  `json:"start_date" binding:"required"`
		EndDate        string  `json:"end_date" binding:"required"`
		Days           string  `json:"days"` // only "0.5" (a half day) is honoured; otherwise working days are counted
		Reason         *string `json:"reason"`
		AttachmentPath *string `json:"attachment_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		response.BadRequest(c, "Invalid start_date format")
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		response.BadRequest(c, "Invalid end_date format")
		return
	}

	if endDate.Before(startDate) {
		response.BadRequest(c, "The end date is before the start date")
		return
	}
	if endDate.Sub(startDate) > 366*24*time.Hour {
		response.BadRequest(c, "A leave request can cover at most one year")
		return
	}
	ctx := c.Request.Context()
	var lt *store.LeaveType
	if types, err := h.queries.ListLeaveTypes(ctx, companyID); err == nil {
		for i := range types {
			if types[i].ID == req.LeaveTypeID && types[i].IsActive {
				lt = &types[i]
			}
		}
	}
	if lt == nil {
		response.BadRequest(c, "Unknown leave type")
		return
	}
	// Days are counted here, not trusted from the client: working days
	// between the dates, without weekends and non-working holidays.
	nDays := workingDays(startDate, endDate, holidaysOff(ctx, h.queries, companyID, startDate, endDate))
	if nDays == 0 {
		response.BadRequest(c, "There are no working days in this range")
		return
	}
	if strings.TrimSpace(req.Days) == "0.5" && startDate.Equal(endDate) {
		nDays = 0.5
	}
	if n, err := h.queries.CountOverlappingLeave(ctx, store.CountOverlappingLeaveParams{CompanyID: companyID, EmployeeID: emp.ID, StartDate: startDate, EndDate: endDate}); err == nil && n > 0 {
		response.Conflict(c, "You already have leave on some of these days")
		return
	}
	if lt.IsPaid {
		bal, err := h.queries.GetLeaveBalance(ctx, store.GetLeaveBalanceParams{CompanyID: companyID, EmployeeID: emp.ID, LeaveTypeID: lt.ID, Year: int32(startDate.Year())})
		if err != nil {
			response.BadRequest(c, "You have no "+lt.Name+" balance for "+strconv.Itoa(startDate.Year()))
			return
		}
		left := numF(bal.Earned) + numF(bal.Carried) + numF(bal.Adjusted) - numF(bal.Used)
		if left+1e-9 < nDays {
			response.BadRequest(c, fmt.Sprintf("Not enough %s: %g day(s) left, %g requested", lt.Name, left, nDays))
			return
		}
	}
	var days pgtype.Numeric
	_ = days.Scan(strconv.FormatFloat(nDays, 'f', 1, 64))

	lr, err := h.queries.CreateLeaveRequest(c.Request.Context(), store.CreateLeaveRequestParams{
		CompanyID:      companyID,
		EmployeeID:     emp.ID,
		LeaveTypeID:    req.LeaveTypeID,
		StartDate:      startDate,
		EndDate:        endDate,
		Days:           days,
		Reason:         req.Reason,
		AttachmentPath: req.AttachmentPath,
	})
	if err != nil {
		h.logger.Error("failed to create leave request", "error", err)
		response.InternalError(c, "Failed to create leave request")
		return
	}

	// Create approval workflow entry for the approver
	var approverID int64
	if emp.ManagerID != nil && *emp.ManagerID > 0 {
		approverID = *emp.ManagerID
	} else {
		// No manager — assign to first admin employee
		adminID, err := h.queries.GetFirstAdminEmployeeID(c.Request.Context(), companyID)
		if err == nil {
			approverID = adminID
		}
	}
	if approverID > 0 {
		if _, err := h.queries.InsertApprovalWorkflow(c.Request.Context(), store.InsertApprovalWorkflowParams{
			CompanyID:  companyID,
			EntityType: "leave_request",
			EntityID:   lr.ID,
			ApproverID: approverID,
		}); err != nil {
			h.logger.Error("failed to create approval workflow", "leave_id", lr.ID, "error", err)
		}
	}

	// Emit leave.requested event for agentic workflow
	idempKey := fmt.Sprintf("leave.requested.%d", lr.ID)
	if _, err := h.queries.InsertHREvent(c.Request.Context(), store.InsertHREventParams{
		CompanyID:      companyID,
		AggregateType:  "leave_request",
		AggregateID:    lr.ID,
		EventType:      "leave.requested",
		EventVersion:   1,
		Payload:        json.RawMessage(`{}`),
		ActorUserID:    &userID,
		IdempotencyKey: &idempKey,
	}); err != nil {
		h.logger.Error("failed to emit leave.requested event", "leave_id", lr.ID, "error", err)
	}

	response.Created(c, lr)
}

func (h *Handler) ListRequests(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	var employeeIDVal int64
	if eid := c.Query("employee_id"); eid != "" {
		if id, err := strconv.ParseInt(eid, 10, 64); err == nil {
			employeeIDVal = id
		}
	}

	statusFilter := c.Query("status")

	requests, err := h.queries.ListLeaveRequests(c.Request.Context(), store.ListLeaveRequestsParams{
		CompanyID: companyID,
		Column2:   employeeIDVal,
		Column3:   statusFilter,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list requests")
		return
	}

	count, _ := h.queries.CountLeaveRequests(c.Request.Context(), store.CountLeaveRequestsParams{
		CompanyID: companyID,
		Column2:   employeeIDVal,
		Column3:   statusFilter,
	})

	response.Paginated(c, requests, count, pg.Page, pg.Limit)
}

func (h *Handler) ApproveRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid request ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	approver, ok := h.decider(c, id)
	if !ok {
		return
	}
	approveParams := store.ApproveLeaveRequestParams{
		ID:        id,
		CompanyID: companyID,
	}
	if approver != nil {
		approveParams.ApproverID = &approver.ID
	}

	// Approve and deduct in one transaction, so a request is never approved
	// without its days being taken from the balance.
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to approve leave request")
		return
	}
	defer tx.Rollback(c.Request.Context()) //nolint:errcheck
	qt := h.queries.WithTx(tx)
	lr, err := qt.ApproveLeaveRequest(c.Request.Context(), approveParams)
	if err != nil {
		response.NotFound(c, "Leave request not found or already processed")
		return
	}
	paid := true
	if types, err := qt.ListLeaveTypes(c.Request.Context(), companyID); err == nil {
		for _, t := range types {
			if t.ID == lr.LeaveTypeID {
				paid = t.IsPaid
			}
		}
	}
	if paid {
		if _, err := qt.GetLeaveBalance(c.Request.Context(), store.GetLeaveBalanceParams{CompanyID: companyID, EmployeeID: lr.EmployeeID, LeaveTypeID: lr.LeaveTypeID, Year: int32(lr.StartDate.Year())}); err != nil {
			response.BadRequest(c, "The employee has no balance for this leave type and year — add one in Time off → Balances first")
			return
		}
	}
	if err := qt.DeductLeaveBalance(c.Request.Context(), store.DeductLeaveBalanceParams{
		CompanyID:   companyID,
		EmployeeID:  lr.EmployeeID,
		LeaveTypeID: lr.LeaveTypeID,
		Year:        int32(lr.StartDate.Year()),
		Used:        lr.Days,
	}); err != nil {
		h.logger.Error("failed to deduct leave balance", "leave_request_id", lr.ID, "employee_id", lr.EmployeeID, "error", err)
		response.InternalError(c, "Failed to approve leave request")
		return
	}
	if err := qt.CloseApprovalWorkflows(c.Request.Context(), store.CloseApprovalWorkflowsParams{Status: "approved", CompanyID: companyID, EntityType: "leave_request", EntityID: lr.ID}); err != nil {
		response.InternalError(c, "Failed to approve leave request")
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.InternalError(c, "Failed to approve leave request")
		return
	}

	// Notify employee
	if reqEmp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: lr.EmployeeID, CompanyID: companyID}); err == nil && reqEmp.UserID != nil {
		entityType := "leave_request"
		notification.Notify(c.Request.Context(), h.queries, h.logger, companyID, *reqEmp.UserID,
			"Leave Approved",
			fmt.Sprintf("Your leave request (%s - %s) has been approved.", lr.StartDate.Format("Jan 2"), lr.EndDate.Format("Jan 2")),
			"leave", &entityType, &lr.ID)

		// Email notification
		if reqEmp.Email != nil && *reqEmp.Email != "" {
			empName := reqEmp.FirstName + " " + reqEmp.LastName
			leaveTypeName := "Leave"
			if types, err := h.queries.ListLeaveTypes(c.Request.Context(), companyID); err == nil {
				for _, lt := range types {
					if lt.ID == lr.LeaveTypeID {
						leaveTypeName = lt.Name
						break
					}
				}
			}
			subj, body := email.LeaveApprovedEmail(empName, leaveTypeName, lr.StartDate.Format("Jan 2, 2006"), lr.EndDate.Format("Jan 2, 2006"))
			h.email.SendAsync(*reqEmp.Email, subj, body)
		}
	}

	response.OK(c, lr)
}

func (h *Handler) RejectRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid request ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	companyID := auth.GetCompanyID(c)
	approver, ok := h.decider(c, id)
	if !ok {
		return
	}
	var approverID *int64
	if approver != nil {
		approverID = &approver.ID
	}

	lr, err := h.queries.RejectLeaveRequest(c.Request.Context(), store.RejectLeaveRequestParams{
		ID:              id,
		CompanyID:       companyID,
		ApproverID:      approverID,
		RejectionReason: &req.Reason,
	})
	if err != nil {
		response.NotFound(c, "Leave request not found or already processed")
		return
	}
	_ = h.queries.CloseApprovalWorkflows(c.Request.Context(), store.CloseApprovalWorkflowsParams{Status: "rejected", CompanyID: companyID, EntityType: "leave_request", EntityID: lr.ID})

	// Notify employee
	if reqEmp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: lr.EmployeeID, CompanyID: companyID}); err == nil && reqEmp.UserID != nil {
		entityType := "leave_request"
		msg := fmt.Sprintf("Your leave request (%s - %s) has been rejected.", lr.StartDate.Format("Jan 2"), lr.EndDate.Format("Jan 2"))
		if req.Reason != "" {
			msg += " Reason: " + req.Reason
		}
		notification.Notify(c.Request.Context(), h.queries, h.logger, companyID, *reqEmp.UserID,
			"Leave Rejected", msg, "leave", &entityType, &lr.ID)

		// Email notification
		if reqEmp.Email != nil && *reqEmp.Email != "" {
			empName := reqEmp.FirstName + " " + reqEmp.LastName
			leaveTypeName := "Leave"
			if types, err := h.queries.ListLeaveTypes(c.Request.Context(), companyID); err == nil {
				for _, lt := range types {
					if lt.ID == lr.LeaveTypeID {
						leaveTypeName = lt.Name
						break
					}
				}
			}
			subj, body := email.LeaveRejectedEmail(empName, leaveTypeName, lr.StartDate.Format("Jan 2, 2006"), lr.EndDate.Format("Jan 2, 2006"), req.Reason)
			h.email.SendAsync(*reqEmp.Email, subj, body)
		}
	}

	response.OK(c, lr)
}

func (h *Handler) CancelRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid request ID")
		return
	}

	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: auth.GetCompanyID(c),
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}

	lr, err := h.queries.CancelLeaveRequest(c.Request.Context(), store.CancelLeaveRequestParams{
		ID:         id,
		EmployeeID: emp.ID,
		CompanyID:  auth.GetCompanyID(c),
	})
	if err != nil {
		response.NotFound(c, "Leave request not found or cannot be cancelled")
		return
	}
	_ = h.queries.CloseApprovalWorkflows(c.Request.Context(), store.CloseApprovalWorkflowsParams{Status: "cancelled", CompanyID: auth.GetCompanyID(c), EntityType: "leave_request", EntityID: lr.ID})
	response.OK(c, lr)
}

// decider checks the caller may approve or reject leave request id (see
// access.Decide: HR anyone's, a manager their direct reports', never your
// own).
func (h *Handler) decider(c *gin.Context, id int64) (*store.Employee, bool) {
	lr, err := h.queries.GetLeaveRequest(c.Request.Context(), store.GetLeaveRequestParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.NotFound(c, "Leave request not found or already processed")
		return nil, false
	}
	return access.Decide(c, h.queries, lr.EmployeeID)
}

func numF(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
