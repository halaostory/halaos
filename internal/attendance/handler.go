package attendance

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
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
	Source string  `json:"source"` // web, mobile
	Lat    *string `json:"lat"`
	Lng    *string `json:"lng"`
	Note   *string `json:"note"`
}

// haversineDistance calculates the distance between two GPS coordinates in meters.
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

type geofenceResult struct {
	matchedID int64
	status    string // "inside", "outside", "not_checked"
}

// checkGeofence validates GPS coordinates against active geofences.
// Returns the matched geofence (closest within radius) or "outside" if none match.
func (h *Handler) checkGeofence(c *gin.Context, companyID int64, lat, lng float64, enforceClockIn bool) geofenceResult {
	geofences, err := h.queries.ListActiveGeofences(c.Request.Context(), companyID)
	if err != nil || len(geofences) == 0 {
		return geofenceResult{status: "not_checked"}
	}

	var closestID int64
	closestDist := math.MaxFloat64
	for _, gf := range geofences {
		// Check enforcement direction
		if enforceClockIn && !gf.EnforceOnClockIn {
			continue
		}
		if !enforceClockIn && !gf.EnforceOnClockOut {
			continue
		}

		gfLat, _ := gf.Latitude.Float64Value()
		gfLng, _ := gf.Longitude.Float64Value()
		if !gfLat.Valid || !gfLng.Valid {
			continue
		}
		dist := haversineDistance(lat, lng, gfLat.Float64, gfLng.Float64)
		if dist <= float64(gf.RadiusMeters) && dist < closestDist {
			closestID = gf.ID
			closestDist = dist
		}
	}

	if closestID > 0 {
		return geofenceResult{matchedID: closestID, status: "inside"}
	}
	return geofenceResult{status: "outside"}
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
		_, _ = h.pool.Exec(c.Request.Context(),
			"UPDATE attendance_logs SET clock_in_geofence_id = $1, clock_in_geofence_status = $2 WHERE id = $3",
			nilIfZero(gfResult.matchedID), gfStatus, log.ID)
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
		_, _ = h.pool.Exec(c.Request.Context(),
			"UPDATE attendance_logs SET clock_out_geofence_id = $1, clock_out_geofence_status = $2 WHERE id = $3",
			nilIfZero(gfResult.matchedID), gfStatus, log.ID)
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
		EmployeeIDs []int64 `json:"employee_ids" binding:"required"`
		ShiftID     int64   `json:"shift_id" binding:"required"`
		Dates       []string `json:"dates" binding:"required"`
		IsRestDay   bool    `json:"is_rest_day"`
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

func nilIfZero(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// Geofence CRUD handlers

func (h *Handler) ListGeofences(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	geofences, err := h.queries.ListGeofences(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list geofences")
		return
	}
	response.OK(c, geofences)
}

func (h *Handler) CreateGeofence(c *gin.Context) {
	var req struct {
		Name              string  `json:"name" binding:"required"`
		Address           *string `json:"address"`
		Latitude          float64 `json:"latitude" binding:"required"`
		Longitude         float64 `json:"longitude" binding:"required"`
		RadiusMeters      int32   `json:"radius_meters"`
		EnforceOnClockIn  bool    `json:"enforce_on_clock_in"`
		EnforceOnClockOut bool    `json:"enforce_on_clock_out"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	if req.RadiusMeters <= 0 {
		req.RadiusMeters = 200
	}

	var lat, lng pgtype.Numeric
	_ = lat.Scan(fmt.Sprintf("%.7f", req.Latitude))
	_ = lng.Scan(fmt.Sprintf("%.7f", req.Longitude))

	gf, err := h.queries.CreateGeofence(c.Request.Context(), store.CreateGeofenceParams{
		CompanyID:         companyID,
		Name:              req.Name,
		Address:           req.Address,
		Latitude:          lat,
		Longitude:         lng,
		RadiusMeters:      req.RadiusMeters,
		EnforceOnClockIn:  req.EnforceOnClockIn,
		EnforceOnClockOut: req.EnforceOnClockOut,
		CreatedBy:         &userID,
	})
	if err != nil {
		h.logger.Error("failed to create geofence", "error", err)
		response.InternalError(c, "Failed to create geofence")
		return
	}
	response.Created(c, gf)
}

func (h *Handler) UpdateGeofence(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid geofence ID")
		return
	}

	var req struct {
		Name              string  `json:"name"`
		Address           *string `json:"address"`
		Latitude          float64 `json:"latitude" binding:"required"`
		Longitude         float64 `json:"longitude" binding:"required"`
		RadiusMeters      int32   `json:"radius_meters"`
		IsActive          bool    `json:"is_active"`
		EnforceOnClockIn  bool    `json:"enforce_on_clock_in"`
		EnforceOnClockOut bool    `json:"enforce_on_clock_out"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	var lat, lng pgtype.Numeric
	_ = lat.Scan(fmt.Sprintf("%.7f", req.Latitude))
	_ = lng.Scan(fmt.Sprintf("%.7f", req.Longitude))

	gf, err := h.queries.UpdateGeofence(c.Request.Context(), store.UpdateGeofenceParams{
		ID:                id,
		CompanyID:         companyID,
		Name:              req.Name,
		Address:           req.Address,
		Latitude:          lat,
		Longitude:         lng,
		RadiusMeters:      req.RadiusMeters,
		IsActive:          req.IsActive,
		EnforceOnClockIn:  req.EnforceOnClockIn,
		EnforceOnClockOut: req.EnforceOnClockOut,
	})
	if err != nil {
		response.NotFound(c, "Geofence not found")
		return
	}
	response.OK(c, gf)
}

func (h *Handler) DeleteGeofence(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid geofence ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteGeofence(c.Request.Context(), store.DeleteGeofenceParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete geofence")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

func (h *Handler) GetGeofenceSettings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	enabled, _ := h.queries.IsGeofenceEnabled(c.Request.Context(), companyID)
	response.OK(c, gin.H{"geofence_enabled": enabled})
}

func (h *Handler) SetGeofenceSettings(c *gin.Context) {
	var req struct {
		Enabled bool `json:"geofence_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.SetGeofenceEnabled(c.Request.Context(), store.SetGeofenceEnabledParams{
		ID:              companyID,
		GeofenceEnabled: req.Enabled,
	}); err != nil {
		response.InternalError(c, "Failed to update geofence settings")
		return
	}
	response.OK(c, gin.H{"geofence_enabled": req.Enabled})
}
