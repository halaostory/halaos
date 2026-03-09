package leave

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/numericutil"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) GetConvertibleBalances(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}
	year := int32(time.Now().Year())
	if y := c.Query("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = int32(v)
		}
	}
	balances, err := h.queries.GetConvertibleLeaveBalances(c.Request.Context(), store.GetConvertibleLeaveBalancesParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Year:       year,
	})
	if err != nil {
		response.InternalError(c, "Failed to get convertible balances")
		return
	}
	response.OK(c, balances)
}

func (h *Handler) CreateEncashment(c *gin.Context) {
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
	empID := emp.ID

	var req struct {
		LeaveTypeID int64   `json:"leave_type_id" binding:"required"`
		Year        int32   `json:"year" binding:"required"`
		Days        float64 `json:"days" binding:"required"`
		Remarks     *string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	salary, err := h.queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    empID,
		EffectiveFrom: time.Now(),
	})
	if err != nil {
		response.BadRequest(c, "No active salary found")
		return
	}
	monthlyF := numericutil.ToFloat(salary.BasicSalary)
	dailyRate := monthlyF / 26.0
	totalAmount := dailyRate * req.Days

	var days, dr, ta pgtype.Numeric
	_ = days.Scan(fmt.Sprintf("%.1f", req.Days))
	_ = dr.Scan(fmt.Sprintf("%.2f", dailyRate))
	_ = ta.Scan(fmt.Sprintf("%.2f", totalAmount))

	enc, err := h.queries.CreateLeaveEncashment(c.Request.Context(), store.CreateLeaveEncashmentParams{
		CompanyID:   companyID,
		EmployeeID:  empID,
		LeaveTypeID: req.LeaveTypeID,
		Year:        req.Year,
		Days:        days,
		DailyRate:   dr,
		TotalAmount: ta,
		Remarks:     req.Remarks,
	})
	if err != nil {
		h.logger.Error("failed to create leave encashment", "error", err)
		response.InternalError(c, "Failed to create encashment request")
		return
	}
	response.OK(c, enc)
}

func (h *Handler) ListEncashments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	var empID int64
	if e := c.Query("employee_id"); e != "" {
		empID, _ = strconv.ParseInt(e, 10, 64)
	}
	limit, offset := 50, 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	items, err := h.queries.ListLeaveEncashments(c.Request.Context(), store.ListLeaveEncashmentsParams{
		CompanyID: companyID,
		Column2:   status,
		Column3:   empID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list encashments")
		return
	}
	count, _ := h.queries.CountLeaveEncashments(c.Request.Context(), store.CountLeaveEncashmentsParams{
		CompanyID: companyID,
		Column2:   status,
		Column3:   empID,
	})
	response.Paginated(c, items, count, offset/limit+1, limit)
}

func (h *Handler) ApproveEncashment(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	enc, err := h.queries.ApproveLeaveEncashment(c.Request.Context(), store.ApproveLeaveEncashmentParams{
		ID:         id,
		CompanyID:  companyID,
		ApprovedBy: &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to approve encashment")
		return
	}
	response.OK(c, enc)
}

func (h *Handler) RejectEncashment(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Remarks *string `json:"remarks"`
	}
	_ = c.ShouldBindJSON(&req)
	enc, err := h.queries.RejectLeaveEncashment(c.Request.Context(), store.RejectLeaveEncashmentParams{
		ID:         id,
		CompanyID:  companyID,
		ApprovedBy: &userID,
		Remarks:    req.Remarks,
	})
	if err != nil {
		response.InternalError(c, "Failed to reject encashment")
		return
	}
	response.OK(c, enc)
}

func (h *Handler) MarkEncashmentPaid(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	enc, err := h.queries.MarkLeaveEncashmentPaid(c.Request.Context(), store.MarkLeaveEncashmentPaidParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to mark encashment as paid")
		return
	}
	response.OK(c, enc)
}
