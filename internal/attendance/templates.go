package attendance

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) ListScheduleTemplates(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	templates, err := h.queries.ListScheduleTemplates(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list schedule templates")
		return
	}
	response.OK(c, templates)
}

func (h *Handler) CreateScheduleTemplate(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Days        []struct {
			DayOfWeek int   `json:"day_of_week"`
			ShiftID   int64 `json:"shift_id"`
			IsRestDay bool  `json:"is_rest_day"`
		} `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)

	tmpl, err := h.queries.CreateScheduleTemplate(c.Request.Context(), store.CreateScheduleTemplateParams{
		CompanyID:   companyID,
		Name:        req.Name,
		Description: &req.Description,
	})
	if err != nil {
		response.InternalError(c, "Failed to create schedule template")
		return
	}

	for _, d := range req.Days {
		var shiftID *int64
		if !d.IsRestDay && d.ShiftID > 0 {
			shiftID = &d.ShiftID
		}
		if _, err := h.queries.UpsertScheduleTemplateDay(c.Request.Context(), store.UpsertScheduleTemplateDayParams{
			TemplateID: tmpl.ID,
			DayOfWeek:  int32(d.DayOfWeek),
			ShiftID:    shiftID,
			IsRestDay:  d.IsRestDay,
		}); err != nil {
			h.logger.Error("failed to upsert schedule template day", "day", d.DayOfWeek, "error", err)
		}
	}

	response.Created(c, tmpl)
}

func (h *Handler) GetScheduleTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	tmpl, err := h.queries.GetScheduleTemplate(c.Request.Context(), store.GetScheduleTemplateParams{
		ID: id, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Template not found")
		return
	}
	days, _ := h.queries.ListScheduleTemplateDays(c.Request.Context(), id)
	response.OK(c, gin.H{"template": tmpl, "days": days})
}

func (h *Handler) UpdateScheduleTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Days        []struct {
			DayOfWeek int   `json:"day_of_week"`
			ShiftID   int64 `json:"shift_id"`
			IsRestDay bool  `json:"is_rest_day"`
		} `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tmpl, err := h.queries.UpdateScheduleTemplate(c.Request.Context(), store.UpdateScheduleTemplateParams{
		ID: id, CompanyID: companyID, Name: req.Name, Description: &req.Description,
	})
	if err != nil {
		response.NotFound(c, "Template not found")
		return
	}

	_ = h.queries.DeleteScheduleTemplateDays(c.Request.Context(), id)
	for _, d := range req.Days {
		var shiftID *int64
		if !d.IsRestDay && d.ShiftID > 0 {
			shiftID = &d.ShiftID
		}
		if _, err := h.queries.UpsertScheduleTemplateDay(c.Request.Context(), store.UpsertScheduleTemplateDayParams{
			TemplateID: tmpl.ID,
			DayOfWeek:  int32(d.DayOfWeek),
			ShiftID:    shiftID,
			IsRestDay:  d.IsRestDay,
		}); err != nil {
			h.logger.Error("failed to upsert schedule template day", "day", d.DayOfWeek, "error", err)
		}
	}

	response.OK(c, tmpl)
}

func (h *Handler) DeleteScheduleTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	_ = h.queries.DeleteScheduleTemplate(c.Request.Context(), store.DeleteScheduleTemplateParams{
		ID: id, CompanyID: companyID,
	})
	response.OK(c, gin.H{"message": "Deleted"})
}

func (h *Handler) AssignTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	var req struct {
		EmployeeIDs   []int64 `json:"employee_ids" binding:"required"`
		EffectiveFrom string  `json:"effective_from" binding:"required"`
		EffectiveTo   *string `json:"effective_to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	effFrom, _ := time.Parse("2006-01-02", req.EffectiveFrom)
	var effTo pgtype.Date
	if req.EffectiveTo != nil {
		parsed, _ := time.Parse("2006-01-02", *req.EffectiveTo)
		effTo = pgtype.Date{Time: parsed, Valid: true}
	}

	var assigned int
	for _, empID := range req.EmployeeIDs {
		_, err := h.queries.AssignScheduleTemplate(c.Request.Context(), store.AssignScheduleTemplateParams{
			CompanyID:     companyID,
			EmployeeID:    empID,
			TemplateID:    id,
			EffectiveFrom: effFrom,
			EffectiveTo:   effTo,
		})
		if err == nil {
			assigned++
		}
	}
	response.OK(c, gin.H{"assigned": assigned})
}

func (h *Handler) ListScheduleAssignments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	assignments, err := h.queries.ListEmployeeScheduleAssignments(c.Request.Context(), store.ListEmployeeScheduleAssignmentsParams{
		CompanyID: companyID,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list assignments")
		return
	}
	response.OK(c, assignments)
}
