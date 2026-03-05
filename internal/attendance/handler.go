package attendance

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

type clockRequest struct {
	Source string   `json:"source"` // web, mobile
	Lat    *string  `json:"lat"`
	Lng    *string  `json:"lng"`
	Note   *string  `json:"note"`
}

func (h *Handler) ClockIn(c *gin.Context) {
	var req clockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
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

	// Check for open attendance
	_, err = h.queries.GetOpenAttendance(c.Request.Context(), store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err == nil {
		response.Conflict(c, "Already clocked in. Please clock out first.")
		return
	}

	source := req.Source
	if source == "" {
		source = "web"
	}

	log, err := h.queries.ClockIn(c.Request.Context(), store.ClockInParams{
		CompanyID:     companyID,
		EmployeeID:    emp.ID,
		ClockInSource: source,
		ClockInLat:    req.Lat,
		ClockInLng:    req.Lng,
		ClockInNote:   req.Note,
	})
	if err != nil {
		h.logger.Error("failed to clock in", "error", err)
		response.InternalError(c, "Failed to clock in")
		return
	}

	response.Created(c, log)
}

func (h *Handler) ClockOut(c *gin.Context) {
	var req clockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

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

	open, err := h.queries.GetOpenAttendance(c.Request.Context(), store.GetOpenAttendanceParams{
		EmployeeID: emp.ID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.NotFound(c, "No open attendance record found")
		return
	}

	source := req.Source
	if source == "" {
		source = "web"
	}

	log, err := h.queries.ClockOut(c.Request.Context(), store.ClockOutParams{
		ID:             open.ID,
		EmployeeID:     emp.ID,
		ClockOutSource: &source,
		ClockOutLat:    req.Lat,
		ClockOutLng:    req.Lng,
		ClockOutNote:   req.Note,
	})
	if err != nil {
		h.logger.Error("failed to clock out", "error", err)
		response.InternalError(c, "Failed to clock out")
		return
	}

	response.OK(c, log)
}

func (h *Handler) ListRecords(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	from, _ := time.Parse("2006-01-02", c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02")))
	to, _ := time.Parse("2006-01-02", c.DefaultQuery("to", time.Now().AddDate(0, 0, 1).Format("2006-01-02")))

	var employeeID *int64
	if eid := c.Query("employee_id"); eid != "" {
		if id, err := strconv.ParseInt(eid, 10, 64); err == nil {
			employeeID = &id
		}
	}

	records, err := h.queries.ListAttendanceLogs(c.Request.Context(), store.ListAttendanceLogsParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		ClockInAt:  from,
		ClockInAt_2: to,
		Limit:      int32(pg.Limit),
		Offset:     int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list records")
		return
	}

	count, _ := h.queries.CountAttendanceLogs(c.Request.Context(), store.CountAttendanceLogsParams{
		CompanyID:  companyID,
		EmployeeID: employeeID,
		ClockInAt:  from,
		ClockInAt_2: to,
	})

	response.Paginated(c, records, count, pg.Page, pg.Limit)
}

func (h *Handler) GetSummary(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	summary, err := h.queries.GetTodayAttendanceSummary(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get summary")
		return
	}
	response.OK(c, summary)
}

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
	startTime, _ := time.Parse("15:04", req.StartTime)
	endTime, _ := time.Parse("15:04", req.EndTime)

	shift, err := h.queries.CreateShift(c.Request.Context(), store.CreateShiftParams{
		CompanyID:    companyID,
		Name:         req.Name,
		StartTime:    startTime,
		EndTime:      endTime,
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
	response.OK(c, gin.H{"message": "update shift placeholder"})
}

func (h *Handler) AssignSchedule(c *gin.Context) {
	response.OK(c, gin.H{"message": "assign schedule placeholder"})
}
