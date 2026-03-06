package company

import (
	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListCostCenters(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	centers, err := h.queries.ListCostCenters(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list cost centers")
		return
	}
	response.OK(c, centers)
}

func (h *Handler) CreateCostCenter(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Code and name are required")
		return
	}
	companyID := auth.GetCompanyID(c)
	center, err := h.queries.CreateCostCenter(c.Request.Context(), store.CreateCostCenterParams{
		CompanyID: companyID,
		Code:      req.Code,
		Name:      req.Name,
	})
	if err != nil {
		response.InternalError(c, "Failed to create cost center")
		return
	}
	response.Created(c, center)
}
