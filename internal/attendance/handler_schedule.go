package attendance

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListShifts(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	shifts, err := h.queries.ListShifts(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list shifts")
		return
	}
	response.OK(c, shifts)
}

func (h *Handler) CreateShift(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		StartTime    string `json:"start_time" binding:"required"`
		EndTime      string `json:"end_time" binding:"required"`
		BreakMinutes int32  `json:"break_minutes"`
		GraceMinutes int32  `json:"grace_minutes"`
		IsOvernight  bool   `json:"is_overnight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	startTimeParsed, _ := time.Parse("15:04", req.StartTime)
	endTimeParsed, _ := time.Parse("15:04", req.EndTime)

	startMicros := int64(startTimeParsed.Hour())*3600000000 + int64(startTimeParsed.Minute())*60000000
	endMicros := int64(endTimeParsed.Hour())*3600000000 + int64(endTimeParsed.Minute())*60000000

	shift, err := h.queries.CreateShift(c.Request.Context(), store.CreateShiftParams{
		CompanyID:    companyID,
		Name:         req.Name,
		StartTime:    pgtype.Time{Microseconds: startMicros, Valid: true},
		EndTime:      pgtype.Time{Microseconds: endMicros, Valid: true},
		BreakMinutes: req.BreakMinutes,
		GraceMinutes: req.GraceMinutes,
		IsOvernight:  req.IsOvernight,
	})
	if err != nil {
		response.InternalError(c, "Failed to create shift")
		return
	}
	response.Created(c, shift)
}

func (h *Handler) UpdateShift(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid shift ID")
		return
	}

	var req struct {
		Name         string `json:"name"`
		StartTime    string `json:"start_time"`
		EndTime      string `json:"end_time"`
		BreakMinutes int32  `json:"break_minutes"`
		GraceMinutes int32  `json:"grace_minutes"`
		IsOvernight  bool   `json:"is_overnight"`
		IsActive     bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	var startTime, endTime pgtype.Time
	if req.StartTime != "" {
		parsed, _ := time.Parse("15:04", req.StartTime)
		startTime = pgtype.Time{
			Microseconds: int64(parsed.Hour())*3600000000 + int64(parsed.Minute())*60000000,
			Valid:        true,
		}
	}
	if req.EndTime != "" {
		parsed, _ := time.Parse("15:04", req.EndTime)
		endTime = pgtype.Time{
			Microseconds: int64(parsed.Hour())*3600000000 + int64(parsed.Minute())*60000000,
			Valid:        true,
		}
	}

	shift, err := h.queries.UpdateShift(c.Request.Context(), store.UpdateShiftParams{
		ID:           id,
		CompanyID:    companyID,
		Name:         req.Name,
		StartTime:    startTime,
		EndTime:      endTime,
		BreakMinutes: req.BreakMinutes,
		GraceMinutes: req.GraceMinutes,
		IsOvernight:  req.IsOvernight,
		IsActive:     req.IsActive,
	})
	if err != nil {
		response.NotFound(c, "Shift not found")
		return
	}
	response.OK(c, shift)
}

func (h *Handler) AssignSchedule(c *gin.Context) {
	var req struct {
		EmployeeID int64  `json:"employee_id" binding:"required"`
		ShiftID    int64  `json:"shift_id" binding:"required"`
		WorkDate   string `json:"work_date" binding:"required"`
		IsRestDay  bool   `json:"is_rest_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	workDate, err := time.Parse("2006-01-02", req.WorkDate)
	if err != nil {
		response.BadRequest(c, "Invalid work_date format, use YYYY-MM-DD")
		return
	}

	schedule, err := h.queries.AssignSchedule(c.Request.Context(), store.AssignScheduleParams{
		CompanyID:  companyID,
		EmployeeID: req.EmployeeID,
		ShiftID:    req.ShiftID,
		WorkDate:   workDate,
		IsRestDay:  req.IsRestDay,
	})
	if err != nil {
		h.logger.Error("failed to assign schedule", "error", err)
		response.InternalError(c, "Failed to assign schedule")
		return
	}
	response.Created(c, schedule)
}

func (h *Handler) ListAllSchedules(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr == "" || endStr == "" {
		// Default to current week
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := now.AddDate(0, 0, -weekday+1)
		end := start.AddDate(0, 0, 6)
		startStr = start.Format("2006-01-02")
		endStr = end.Format("2006-01-02")
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		response.BadRequest(c, "Invalid start date")
		return
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		response.BadRequest(c, "Invalid end date")
		return
	}

	schedules, err := h.queries.ListAllSchedules(c.Request.Context(), store.ListAllSchedulesParams{
		CompanyID:  companyID,
		WorkDate:   startDate,
		WorkDate_2: endDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to list schedules")
		return
	}
	response.OK(c, schedules)
}

func (h *Handler) BulkAssignSchedule(c *gin.Context) {
	var req struct {
		EmployeeIDs []int64  `json:"employee_ids" binding:"required"`
		ShiftID     int64    `json:"shift_id" binding:"required"`
		Dates       []string `json:"dates" binding:"required"`
		IsRestDay   bool     `json:"is_rest_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	count := 0
	for _, empID := range req.EmployeeIDs {
		for _, dateStr := range req.Dates {
			workDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}
			if err := h.queries.BulkAssignSchedule(c.Request.Context(), store.BulkAssignScheduleParams{
				CompanyID:  companyID,
				EmployeeID: empID,
				ShiftID:    req.ShiftID,
				WorkDate:   workDate,
				IsRestDay:  req.IsRestDay,
			}); err != nil {
				h.logger.Error("failed to assign schedule", "employee_id", empID, "date", dateStr, "error", err)
				continue
			}
			count++
		}
	}
	response.OK(c, gin.H{"assigned": count})
}

func (h *Handler) DeleteSchedule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("schedule_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid schedule ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteSchedule(c.Request.Context(), store.DeleteScheduleParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete schedule")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}
