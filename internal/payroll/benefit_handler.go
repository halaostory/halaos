package payroll

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListBenefitDeductions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	employeeIDStr := c.Query("employee_id")

	if employeeIDStr != "" {
		employeeID, err := strconv.ParseInt(employeeIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid employee_id")
			return
		}
		deductions, err := h.queries.ListEmployeeBenefitDeductions(c.Request.Context(), store.ListEmployeeBenefitDeductionsParams{
			CompanyID:     companyID,
			EmployeeID:    employeeID,
			EffectiveDate: time.Now(),
		})
		if err != nil {
			response.InternalError(c, "Failed to list benefit deductions")
			return
		}
		response.OK(c, deductions)
		return
	}

	deductions, err := h.queries.ListBenefitDeductionsByCompany(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list benefit deductions")
		return
	}
	response.OK(c, deductions)
}

func (h *Handler) CreateBenefitDeduction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		EmployeeID      int64   `json:"employee_id" binding:"required"`
		DeductionType   string  `json:"deduction_type" binding:"required"`
		AmountPerPeriod float64 `json:"amount_per_period" binding:"required"`
		AnnualLimit     float64 `json:"annual_limit"`
		ReducesFica     bool    `json:"reduces_fica"`
		EffectiveDate   string  `json:"effective_date" binding:"required"`
		EndDate         string  `json:"end_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	effDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		response.BadRequest(c, "invalid effective_date format, expected YYYY-MM-DD")
		return
	}
	var endDate pgtype.Date
	if req.EndDate != "" {
		ed, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			response.BadRequest(c, "invalid end_date format, expected YYYY-MM-DD")
			return
		}
		endDate = pgtype.Date{Valid: true, Time: ed}
	}

	deduction, err := h.queries.CreateBenefitDeduction(c.Request.Context(), store.CreateBenefitDeductionParams{
		CompanyID:       companyID,
		EmployeeID:      req.EmployeeID,
		DeductionType:   req.DeductionType,
		AmountPerPeriod: numericFromFloat(req.AmountPerPeriod),
		AnnualLimit:     numericFromFloat(req.AnnualLimit),
		ReducesFica:     req.ReducesFica,
		EffectiveDate:   effDate,
		EndDate:         endDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to create benefit deduction")
		return
	}
	response.Created(c, deduction)
}

func (h *Handler) UpdateBenefitDeduction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		DeductionType   string  `json:"deduction_type" binding:"required"`
		AmountPerPeriod float64 `json:"amount_per_period" binding:"required"`
		AnnualLimit     float64 `json:"annual_limit"`
		ReducesFica     bool    `json:"reduces_fica"`
		EffectiveDate   string  `json:"effective_date" binding:"required"`
		EndDate         string  `json:"end_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	effDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		response.BadRequest(c, "invalid effective_date format, expected YYYY-MM-DD")
		return
	}
	var endDate pgtype.Date
	if req.EndDate != "" {
		ed, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			response.BadRequest(c, "invalid end_date format, expected YYYY-MM-DD")
			return
		}
		endDate = pgtype.Date{Valid: true, Time: ed}
	}

	deduction, err := h.queries.UpdateBenefitDeduction(c.Request.Context(), store.UpdateBenefitDeductionParams{
		ID:              id,
		CompanyID:       companyID,
		DeductionType:   req.DeductionType,
		AmountPerPeriod: numericFromFloat(req.AmountPerPeriod),
		AnnualLimit:     numericFromFloat(req.AnnualLimit),
		ReducesFica:     req.ReducesFica,
		EffectiveDate:   effDate,
		EndDate:         endDate,
	})
	if err != nil {
		response.InternalError(c, "Failed to update benefit deduction")
		return
	}
	response.OK(c, deduction)
}

func (h *Handler) DeleteBenefitDeduction(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.queries.DeleteBenefitDeduction(c.Request.Context(), store.DeleteBenefitDeductionParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete benefit deduction")
		return
	}
	response.OK(c, nil)
}
