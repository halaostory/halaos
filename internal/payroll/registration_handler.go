package payroll

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListRegistrationNumbers(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	company, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}

	numbers, err := h.queries.ListCompanyRegistrationNumbers(c.Request.Context(), store.ListCompanyRegistrationNumbersParams{
		CompanyID: companyID,
		Country:   company.Country,
	})
	if err != nil {
		response.InternalError(c, "Failed to list registration numbers")
		return
	}
	response.OK(c, numbers)
}

func (h *Handler) UpsertRegistrationNumber(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		RegistrationType  string `json:"registration_type" binding:"required"`
		RegistrationValue string `json:"registration_value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	company, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}

	result, err := h.queries.UpsertCompanyRegistrationNumber(c.Request.Context(), store.UpsertCompanyRegistrationNumberParams{
		CompanyID:         companyID,
		Country:           company.Country,
		RegistrationType:  req.RegistrationType,
		RegistrationValue: req.RegistrationValue,
	})
	if err != nil {
		response.InternalError(c, "Failed to save registration number")
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteRegistrationNumber(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.queries.DeleteCompanyRegistrationNumber(c.Request.Context(), store.DeleteCompanyRegistrationNumberParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete registration number")
		return
	}
	response.OK(c, nil)
}
