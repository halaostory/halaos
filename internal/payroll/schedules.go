package payroll

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

var scheduleFrequencies = map[string]bool{"monthly": true, "semi_monthly": true, "bi_weekly": true, "weekly": true}

type scheduleReq struct {
	Name      string `json:"name" binding:"required"`
	Frequency string `json:"frequency" binding:"required"`
	IsDefault bool   `json:"is_default"`
}

// ListPaySchedules handles GET /payroll/schedules.
func (h *Handler) ListPaySchedules(c *gin.Context) {
	rows, err := h.queries.ListPaySchedules(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to list pay schedules")
		return
	}
	response.OK(c, rows)
}

// CreatePaySchedule handles POST /payroll/schedules. The first schedule a
// company creates becomes its default.
func (h *Handler) CreatePaySchedule(c *gin.Context) {
	var r scheduleReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if !scheduleFrequencies[r.Frequency] {
		response.BadRequest(c, "frequency must be monthly, semi_monthly, bi_weekly or weekly")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	existing, _ := h.queries.ListPaySchedules(ctx, companyID)
	isDefault := r.IsDefault || len(existing) == 0
	if isDefault {
		_ = h.queries.ClearDefaultPaySchedule(ctx, companyID)
	}
	s, err := h.queries.CreatePaySchedule(ctx, store.CreatePayScheduleParams{CompanyID: companyID, Name: strings.TrimSpace(r.Name), Frequency: r.Frequency, IsDefault: isDefault})
	if err != nil {
		response.Conflict(c, "A schedule with this name already exists")
		return
	}
	response.Created(c, s)
}

// UpdatePaySchedule handles PUT /payroll/schedules/:id.
func (h *Handler) UpdatePaySchedule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	var r scheduleReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if !scheduleFrequencies[r.Frequency] {
		response.BadRequest(c, "invalid frequency")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	s, err := h.queries.UpdatePaySchedule(ctx, store.UpdatePayScheduleParams{ID: id, CompanyID: companyID, Name: strings.TrimSpace(r.Name), Frequency: r.Frequency})
	if err != nil {
		response.NotFound(c, "Pay schedule not found")
		return
	}
	if r.IsDefault && !s.IsDefault {
		_ = h.queries.ClearDefaultPaySchedule(ctx, companyID)
		_ = h.queries.SetDefaultPaySchedule(ctx, store.SetDefaultPayScheduleParams{ID: id, CompanyID: companyID})
		s.IsDefault = true
	}
	response.OK(c, s)
}

// AssignPaySchedule handles POST /payroll/schedules/:id/assign {employee_ids: [...]}.
func (h *Handler) AssignPaySchedule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	var r struct {
		EmployeeIDs []int64 `json:"employee_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	if _, err := h.queries.GetPaySchedule(ctx, store.GetPayScheduleParams{ID: id, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Pay schedule not found")
		return
	}
	n := 0
	for _, eid := range r.EmployeeIDs {
		if _, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: eid, CompanyID: companyID}); err != nil {
			continue // not in this company
		}
		if err := h.queries.AssignEmployeePaySchedule(ctx, store.AssignEmployeePayScheduleParams{EmployeeID: eid, PayScheduleID: id}); err == nil {
			n++
		}
	}
	response.OK(c, gin.H{"assigned": n})
}

// EmployeePaySchedules handles GET /payroll/schedules/assignments (employee_id → schedule_id).
func (h *Handler) EmployeePaySchedules(c *gin.Context) {
	rows, err := h.queries.ListEmployeePayScheduleMap(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to load assignments")
		return
	}
	response.OK(c, rows)
}
