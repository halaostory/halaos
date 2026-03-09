package leave

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/email"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
	"github.com/tonypk/aigonhr/pkg/response"
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
		response.NotFound(c, "Employee not found")
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
		Days           string  `json:"days" binding:"required"`
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

	var days pgtype.Numeric
	_ = days.Scan(req.Days)

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
	userID := auth.GetUserID(c)

	emp, empErr := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})

	approveParams := store.ApproveLeaveRequestParams{
		ID:        id,
		CompanyID: companyID,
	}
	if empErr == nil {
		approveParams.ApproverID = &emp.ID
	}

	lr, err := h.queries.ApproveLeaveRequest(c.Request.Context(), approveParams)
	if err != nil {
		response.NotFound(c, "Leave request not found or already processed")
		return
	}

	// Deduct leave balance
	if err := h.queries.DeductLeaveBalance(c.Request.Context(), store.DeductLeaveBalanceParams{
		CompanyID:   companyID,
		EmployeeID:  lr.EmployeeID,
		LeaveTypeID: lr.LeaveTypeID,
		Year:        int32(lr.StartDate.Year()),
		Used:        lr.Days,
	}); err != nil {
		h.logger.Error("failed to deduct leave balance", "leave_request_id", lr.ID, "employee_id", lr.EmployeeID, "error", err)
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
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}

	lr, err := h.queries.RejectLeaveRequest(c.Request.Context(), store.RejectLeaveRequestParams{
		ID:              id,
		CompanyID:       companyID,
		ApproverID:      &emp.ID,
		RejectionReason: &req.Reason,
	})
	if err != nil {
		response.NotFound(c, "Leave request not found or already processed")
		return
	}

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
	response.OK(c, lr)
}
