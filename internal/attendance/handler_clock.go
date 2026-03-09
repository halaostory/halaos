package attendance

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
	"github.com/tonypk/aigonhr/pkg/response"
)

type clockRequest struct {
	Source string  `json:"source"` // web, mobile
	Lat    *string `json:"lat"`
	Lng    *string `json:"lng"`
	Note   *string `json:"note"`
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

	var clockInLat, clockInLng pgtype.Numeric
	if req.Lat != nil {
		_ = clockInLat.Scan(*req.Lat)
	}
	if req.Lng != nil {
		_ = clockInLng.Scan(*req.Lng)
	}

	// Geofence validation
	gfEnabled, _ := h.queries.IsGeofenceEnabled(c.Request.Context(), companyID)
	var gfResult geofenceResult
	if gfEnabled && req.Lat != nil && req.Lng != nil {
		lat, errLat := strconv.ParseFloat(*req.Lat, 64)
		lng, errLng := strconv.ParseFloat(*req.Lng, 64)
		if errLat != nil || errLng != nil {
			response.BadRequest(c, "Invalid GPS coordinates")
			return
		}
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			response.BadRequest(c, "GPS coordinates out of range")
			return
		}
		gfResult = h.checkGeofence(c, companyID, lat, lng, true)
		if gfResult.status == "outside" {
			response.Forbidden(c, "You are outside all allowed geofence areas. Please move to an approved location.")
			return
		}
	}

	log, err := h.queries.ClockIn(c.Request.Context(), store.ClockInParams{
		CompanyID:     companyID,
		EmployeeID:    emp.ID,
		ClockInSource: source,
		ClockInLat:    clockInLat,
		ClockInLng:    clockInLng,
		ClockInNote:   req.Note,
	})
	if err != nil {
		h.logger.Error("failed to clock in", "error", err)
		response.InternalError(c, "Failed to clock in")
		return
	}

	// Update geofence status on the attendance log
	if gfEnabled && gfResult.status != "" {
		gfStatus := gfResult.status
		if req.Lat == nil || req.Lng == nil {
			gfStatus = "not_checked"
		}
		if _, err := h.pool.Exec(c.Request.Context(),
			"UPDATE attendance_logs SET clock_in_geofence_id = $1, clock_in_geofence_status = $2 WHERE id = $3",
			nilIfZero(gfResult.matchedID), gfStatus, log.ID); err != nil {
			h.logger.Error("failed to update clock-in geofence status", "log_id", log.ID, "error", err)
		}
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

	var clockOutLat, clockOutLng pgtype.Numeric
	if req.Lat != nil {
		_ = clockOutLat.Scan(*req.Lat)
	}
	if req.Lng != nil {
		_ = clockOutLng.Scan(*req.Lng)
	}

	// Geofence validation for clock-out
	gfEnabled, _ := h.queries.IsGeofenceEnabled(c.Request.Context(), companyID)
	var gfResult geofenceResult
	if gfEnabled && req.Lat != nil && req.Lng != nil {
		lat, errLat := strconv.ParseFloat(*req.Lat, 64)
		lng, errLng := strconv.ParseFloat(*req.Lng, 64)
		if errLat != nil || errLng != nil {
			response.BadRequest(c, "Invalid GPS coordinates")
			return
		}
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			response.BadRequest(c, "GPS coordinates out of range")
			return
		}
		gfResult = h.checkGeofence(c, companyID, lat, lng, false)
		if gfResult.status == "outside" {
			response.Forbidden(c, "You are outside all allowed geofence areas. Please move to an approved location.")
			return
		}
	}

	log, err := h.queries.ClockOut(c.Request.Context(), store.ClockOutParams{
		ID:             open.ID,
		EmployeeID:     emp.ID,
		ClockOutSource: &source,
		ClockOutLat:    clockOutLat,
		ClockOutLng:    clockOutLng,
		ClockOutNote:   req.Note,
	})
	if err != nil {
		h.logger.Error("failed to clock out", "error", err)
		response.InternalError(c, "Failed to clock out")
		return
	}

	// Update geofence status on the attendance log
	if gfEnabled && gfResult.status != "" {
		gfStatus := gfResult.status
		if req.Lat == nil || req.Lng == nil {
			gfStatus = "not_checked"
		}
		if _, err := h.pool.Exec(c.Request.Context(),
			"UPDATE attendance_logs SET clock_out_geofence_id = $1, clock_out_geofence_status = $2 WHERE id = $3",
			nilIfZero(gfResult.matchedID), gfStatus, log.ID); err != nil {
			h.logger.Error("failed to update clock-out geofence status", "log_id", log.ID, "error", err)
		}
	}

	response.OK(c, log)
}

func (h *Handler) ListRecords(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	fromTime, _ := time.Parse("2006-01-02", c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02")))
	toTime, _ := time.Parse("2006-01-02", c.DefaultQuery("to", time.Now().AddDate(0, 0, 1).Format("2006-01-02")))

	var employeeIDVal int64
	if eid := c.Query("employee_id"); eid != "" {
		if id, err := strconv.ParseInt(eid, 10, 64); err == nil {
			employeeIDVal = id
		}
	}

	fromTS := pgtype.Timestamptz{Time: fromTime, Valid: true}
	toTS := pgtype.Timestamptz{Time: toTime, Valid: true}

	records, err := h.queries.ListAttendanceLogs(c.Request.Context(), store.ListAttendanceLogsParams{
		CompanyID:   companyID,
		Column2:     employeeIDVal,
		ClockInAt:   fromTS,
		ClockInAt_2: toTS,
		Limit:       int32(pg.Limit),
		Offset:      int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list records")
		return
	}

	count, _ := h.queries.CountAttendanceLogs(c.Request.Context(), store.CountAttendanceLogsParams{
		CompanyID:   companyID,
		Column2:     employeeIDVal,
		ClockInAt:   fromTS,
		ClockInAt_2: toTS,
	})

	response.Paginated(c, records, count, pg.Page, pg.Limit)
}

func (h *Handler) GetSummary(c *gin.Context) {
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
	if err == nil {
		response.OK(c, open)
		return
	}

	// No open attendance — check if there's a completed record today
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	fromTS := pgtype.Timestamptz{Time: startOfDay, Valid: true}
	toTS := pgtype.Timestamptz{Time: endOfDay, Valid: true}

	records, err := h.queries.ListAttendanceLogs(c.Request.Context(), store.ListAttendanceLogsParams{
		CompanyID:   companyID,
		Column2:     emp.ID,
		ClockInAt:   fromTS,
		ClockInAt_2: toTS,
		Limit:       1,
		Offset:      0,
	})
	if err != nil || len(records) == 0 {
		response.NotFound(c, "No attendance record today")
		return
	}
	response.OK(c, records[0])
}
