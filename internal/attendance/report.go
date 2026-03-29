package attendance

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) GetReport(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		response.BadRequest(c, "start and end dates are required")
		return
	}
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		response.BadRequest(c, "Invalid start date format")
		return
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		response.BadRequest(c, "Invalid end date format")
		return
	}
	endDate = endDate.AddDate(0, 0, 1)
	report, err := h.queries.GetAttendanceReport(c.Request.Context(), store.GetAttendanceReportParams{
		CompanyID:   companyID,
		ClockInAt:   pgtype.Timestamptz{Time: startDate, Valid: true},
		ClockInAt_2: pgtype.Timestamptz{Time: endDate, Valid: true},
	})
	if err != nil {
		h.logger.Error("failed to get attendance report", "error", err)
		response.InternalError(c, "Failed to generate report")
		return
	}
	response.OK(c, report)
}
