package payroll

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) ListCycleItems(c *gin.Context) {
	cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid cycle ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	var runID int64
	row := h.pool.QueryRow(c.Request.Context(),
		"SELECT id FROM payroll_runs WHERE cycle_id = $1 AND company_id = $2 ORDER BY created_at DESC LIMIT 1",
		cycleID, companyID)
	if err := row.Scan(&runID); err != nil {
		response.OK(c, []any{})
		return
	}
	items, err := h.queries.ListPayrollItems(c.Request.Context(), runID)
	if err != nil {
		response.InternalError(c, "Failed to list payroll items")
		return
	}
	response.OK(c, items)
}

func (h *Handler) LockCycle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	if err := h.queries.LockPayrollCycle(c.Request.Context(), store.LockPayrollCycleParams{
		ID:        id,
		CompanyID: companyID,
		LockedBy:  &userID,
	}); err != nil {
		response.InternalError(c, "Failed to lock cycle")
		return
	}
	response.OK(c, map[string]bool{"locked": true})
}

func (h *Handler) UnlockCycle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	if err := h.queries.UnlockPayrollCycle(c.Request.Context(), store.UnlockPayrollCycleParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to unlock cycle")
		return
	}
	response.OK(c, map[string]bool{"locked": false})
}

func (h *Handler) GetRunAnomalies(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid run ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	calculator := NewCalculator(h.queries, h.pool, h.logger)
	report, err := calculator.DetectAnomalies(c.Request.Context(), runID, companyID)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("Anomaly detection failed: %s", err.Error()))
		return
	}
	response.OK(c, report)
}

func (h *Handler) GetCycleAnomalies(c *gin.Context) {
	cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid cycle ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	runID, err := h.queries.GetLatestCompletedRunForCycle(c.Request.Context(), store.GetLatestCompletedRunForCycleParams{
		CycleID:   cycleID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, map[string]any{"anomalies": []any{}, "total_items": 0})
		return
	}
	calculator := NewCalculator(h.queries, h.pool, h.logger)
	report, err := calculator.DetectAnomalies(c.Request.Context(), runID, companyID)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("Anomaly detection failed: %s", err.Error()))
		return
	}
	response.OK(c, report)
}

func (h *Handler) List13thMonthPay(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", 0))
	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid year parameter")
		return
	}
	if year == 0 {
		year = int64(h.currentYear())
	}
	records, err := h.queries.List13thMonthPay(c.Request.Context(), store.List13thMonthPayParams{
		CompanyID: companyID,
		Year:      int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to list 13th month pay")
		return
	}
	response.OK(c, records)
}

func (h *Handler) Calculate13thMonth(c *gin.Context) {
	var req struct {
		Year int32 `json:"year" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Year is required")
		return
	}
	companyID := auth.GetCompanyID(c)
	calculator := NewCalculator(h.queries, h.pool, h.logger)
	results, err := calculator.Calculate13thMonthPay(c.Request.Context(), companyID, req.Year)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("Calculation failed: %s", err.Error()))
		return
	}
	response.OK(c, results)
}

func (h *Handler) currentYear() int {
	return time.Now().Year()
}
