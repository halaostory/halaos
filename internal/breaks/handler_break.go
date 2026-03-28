package breaks

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type startBreakRequest struct {
	BreakType string  `json:"break_type" binding:"required"`
	Note      *string `json:"note"`
}

// StartBreak begins a new break for the authenticated employee.
func (h *Handler) StartBreak(c *gin.Context) {
	var req startBreakRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if !validBreakTypes[req.BreakType] {
		response.BadRequest(c, "Invalid break_type. Must be one of: meal, bathroom, rest, leave_post")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	// Find employee by user ID
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	// Must have an open attendance record (clocked in)
	attendance, err := h.queries.GetOpenAttendance(c.Request.Context(), store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.BadRequest(c, "Must clock in before starting a break")
		return
	}

	// Check no active break exists
	activeBreak, err := h.queries.GetActiveBreak(c.Request.Context(), store.GetActiveBreakParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err == nil {
		response.BadRequest(c, fmt.Sprintf("Already on break (type: %s). End it first.", activeBreak.BreakType))
		return
	}

	// Create the break log
	breakLog, err := h.queries.CreateBreakLog(c.Request.Context(), store.CreateBreakLogParams{
		CompanyID:       companyID,
		EmployeeID:      emp.ID,
		AttendanceLogID: attendance.ID,
		BreakType:       req.BreakType,
		Note:            req.Note,
	})
	if err != nil {
		h.logger.Error("failed to create break log", "error", err)
		response.InternalError(c, "Failed to start break")
		return
	}

	response.Created(c, breakLog)
}

// EndBreak ends the current active break for the authenticated employee.
func (h *Handler) EndBreak(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	// Get active break
	activeBreak, err := h.queries.GetActiveBreak(c.Request.Context(), store.GetActiveBreakParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.BadRequest(c, "No active break to end")
		return
	}

	// Look up break policy for overtime calculation
	var maxMinutes int32
	policy, err := h.queries.GetBreakPolicy(c.Request.Context(), store.GetBreakPolicyParams{
		CompanyID: companyID,
		BreakType: activeBreak.BreakType,
	})
	if err == nil {
		maxMinutes = policy.MaxMinutes
	}
	// If no policy found, maxMinutes stays 0 (no overtime calculation)

	// End the break
	endedBreak, err := h.queries.EndBreakLog(c.Request.Context(), store.EndBreakLogParams{
		ID:      activeBreak.ID,
		Column2: maxMinutes,
	})
	if err != nil {
		h.logger.Error("failed to end break", "error", err)
		response.InternalError(c, "Failed to end break")
		return
	}

	response.OK(c, endedBreak)
}

// ListBreaks returns all breaks for the authenticated employee on a given date.
func (h *Handler) ListBreaks(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee record not found")
		return
	}

	dateStr := c.Query("date")
	from, to, err := parseDateRange(dateStr)
	if err != nil {
		response.BadRequest(c, "Invalid date format. Use YYYY-MM-DD")
		return
	}

	breaks, err := h.queries.ListBreaksByDate(c.Request.Context(), store.ListBreaksByDateParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
		StartAt:    from,
		StartAt_2:  to,
	})
	if err != nil {
		h.logger.Error("failed to list breaks", "error", err)
		response.InternalError(c, "Failed to list breaks")
		return
	}

	response.OK(c, breaks)
}

// GetActiveBreak returns the current active break for the authenticated employee, or null.
func (h *Handler) GetActiveBreak(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, nil)
		return
	}

	activeBreak, err := h.queries.GetActiveBreak(c.Request.Context(), store.GetActiveBreakParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.OK(c, nil)
		return
	}

	response.OK(c, activeBreak)
}
