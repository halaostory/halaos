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

func (h *Handler) CreateCorrection(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}

	var req struct {
		AttendanceID      *int64  `json:"attendance_id"`
		CorrectionDate    string  `json:"correction_date" binding:"required"`
		RequestedClockIn  *string `json:"requested_clock_in"`
		RequestedClockOut *string `json:"requested_clock_out"`
		Reason            string  `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	corrDate, err := time.Parse("2006-01-02", req.CorrectionDate)
	if err != nil {
		response.BadRequest(c, "Invalid correction date")
		return
	}

	params := store.CreateAttendanceCorrectionParams{
		CompanyID:      companyID,
		EmployeeID:     emp.ID,
		AttendanceID:   req.AttendanceID,
		CorrectionDate: corrDate,
		Reason:         req.Reason,
	}

	if req.RequestedClockIn != nil {
		t, err := time.Parse(time.RFC3339, *req.RequestedClockIn)
		if err == nil {
			params.RequestedClockIn = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if req.RequestedClockOut != nil {
		t, err := time.Parse(time.RFC3339, *req.RequestedClockOut)
		if err == nil {
			params.RequestedClockOut = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	if req.AttendanceID != nil {
		origLog, err := h.queries.GetAttendanceByID(c.Request.Context(), store.GetAttendanceByIDParams{
			ID:        *req.AttendanceID,
			CompanyID: companyID,
		})
		if err == nil {
			params.OriginalClockIn = origLog.ClockInAt
			params.OriginalClockOut = origLog.ClockOutAt
		}
	}

	correction, err := h.queries.CreateAttendanceCorrection(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "Failed to create correction request")
		return
	}
	response.Created(c, correction)
}

func (h *Handler) ListCorrections(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	corrections, err := h.queries.ListAttendanceCorrections(c.Request.Context(), store.ListAttendanceCorrectionsParams{
		CompanyID: companyID,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list corrections")
		return
	}
	response.OK(c, corrections)
}

func (h *Handler) ListPendingCorrections(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	corrections, err := h.queries.ListPendingCorrections(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list pending corrections")
		return
	}
	response.OK(c, corrections)
}

func (h *Handler) ListMyCorrections(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}
	corrections, err := h.queries.ListMyCorrections(c.Request.Context(), store.ListMyCorrectionsParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
		Limit:      50,
		Offset:     0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list corrections")
		return
	}
	response.OK(c, corrections)
}

func (h *Handler) ApproveCorrection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid correction ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)

	correction, err := h.queries.ApproveAttendanceCorrection(c.Request.Context(), store.ApproveAttendanceCorrectionParams{
		ID:         id,
		CompanyID:  companyID,
		ReviewedBy: &userID,
		ReviewNote: &req.Note,
	})
	if err != nil {
		response.InternalError(c, "Failed to approve correction")
		return
	}

	if correction.AttendanceID != nil {
		if correction.RequestedClockIn.Valid {
			_, _ = h.pool.Exec(c.Request.Context(),
				"UPDATE attendance_logs SET clock_in_at = $1, is_corrected = true, corrected_by = $2 WHERE id = $3",
				correction.RequestedClockIn.Time, userID, *correction.AttendanceID)
		}
		if correction.RequestedClockOut.Valid {
			_, _ = h.pool.Exec(c.Request.Context(),
				"UPDATE attendance_logs SET clock_out_at = $1, is_corrected = true, corrected_by = $2 WHERE id = $3",
				correction.RequestedClockOut.Time, userID, *correction.AttendanceID)
		}
	}

	response.OK(c, correction)
}

func (h *Handler) RejectCorrection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid correction ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)

	correction, err := h.queries.RejectAttendanceCorrection(c.Request.Context(), store.RejectAttendanceCorrectionParams{
		ID:         id,
		CompanyID:  companyID,
		ReviewedBy: &userID,
		ReviewNote: &req.Note,
	})
	if err != nil {
		response.InternalError(c, "Failed to reject correction")
		return
	}
	response.OK(c, correction)
}
