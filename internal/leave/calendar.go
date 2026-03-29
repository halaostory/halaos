package leave

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) GetCalendar(c *gin.Context) {
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
	leaves, err := h.queries.ListApprovedLeavesForCalendar(c.Request.Context(), store.ListApprovedLeavesForCalendarParams{
		CompanyID: companyID,
		EndDate:   startDate,
		StartDate: endDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to list leave calendar")
		return
	}
	response.OK(c, leaves)
}
