package compliance

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListTaxFilings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))
	status := c.Query("status")
	filingType := c.Query("type")

	filings, err := h.queries.ListTaxFilings(c.Request.Context(), store.ListTaxFilingsParams{
		CompanyID:  companyID,
		Status:     status,
		FilingType: filingType,
		PeriodYear: int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to list tax filings")
		return
	}
	summary, _ := h.queries.GetFilingSummary(c.Request.Context(), store.GetFilingSummaryParams{
		CompanyID:  companyID,
		PeriodYear: int32(year),
	})
	response.OK(c, gin.H{"data": filings, "summary": summary, "year": year})
}

func (h *Handler) CreateTaxFiling(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		FilingType    string  `json:"filing_type" binding:"required"`
		PeriodType    string  `json:"period_type" binding:"required"`
		PeriodYear    int32   `json:"period_year" binding:"required"`
		PeriodMonth   *int32  `json:"period_month"`
		PeriodQuarter *int32  `json:"period_quarter"`
		DueDate       string  `json:"due_date" binding:"required"`
		Amount        float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		response.BadRequest(c, "Invalid due date")
		return
	}
	var amountNum pgtype.Numeric
	_ = amountNum.Scan(fmt.Sprintf("%.2f", req.Amount))
	filing, err := h.queries.CreateTaxFiling(c.Request.Context(), store.CreateTaxFilingParams{
		CompanyID:     companyID,
		FilingType:    req.FilingType,
		PeriodType:    req.PeriodType,
		PeriodYear:    req.PeriodYear,
		PeriodMonth:   req.PeriodMonth,
		PeriodQuarter: req.PeriodQuarter,
		DueDate:       dueDate,
		Amount:        amountNum,
		Status:        "pending",
	})
	if err != nil {
		response.InternalError(c, "Failed to create tax filing")
		return
	}
	response.Created(c, filing)
}

func (h *Handler) UpdateTaxFilingStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Status      string `json:"status" binding:"required"`
		ReferenceNo string `json:"reference_no"`
		Notes       string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	filing, err := h.queries.UpdateTaxFilingStatus(c.Request.Context(), store.UpdateTaxFilingStatusParams{
		ID:          id,
		CompanyID:   companyID,
		Status:      req.Status,
		FiledBy:     &userID,
		ReferenceNo: &req.ReferenceNo,
		Notes:       &req.Notes,
	})
	if err != nil {
		response.InternalError(c, "Failed to update filing")
		return
	}
	response.OK(c, filing)
}

func (h *Handler) ListOverdueFilings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	filings, err := h.queries.ListOverdueFilings(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list overdue filings")
		return
	}
	response.OK(c, filings)
}

func (h *Handler) ListUpcomingFilings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	filings, err := h.queries.ListUpcomingFilings(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list upcoming filings")
		return
	}
	response.OK(c, filings)
}

func (h *Handler) GenerateAnnualFilings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		Year int32 `json:"year" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Year is required")
		return
	}
	err := h.queries.GenerateAnnualFilings(c.Request.Context(), store.GenerateAnnualFilingsParams{
		CompanyID:  companyID,
		PeriodYear: req.Year,
	})
	if err != nil {
		response.InternalError(c, "Failed to generate annual filings")
		return
	}
	response.OK(c, gin.H{"message": "Annual filings generated", "year": req.Year})
}

func (h *Handler) ListRemittanceRecords(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))
	records, err := h.queries.ListRemittanceRecords(c.Request.Context(), store.ListRemittanceRecordsParams{
		CompanyID:  companyID,
		PeriodYear: int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to list remittance records")
		return
	}
	response.OK(c, records)
}
