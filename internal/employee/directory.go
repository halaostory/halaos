package employee

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListDirectory(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	search := c.Query("search")
	deptFilter := c.Query("department_id")

	var searchVal string
	if search != "" {
		searchVal = "%" + search + "%"
	}

	var deptIDVal int64
	if deptFilter != "" {
		if id, err := strconv.ParseInt(deptFilter, 10, 64); err == nil {
			deptIDVal = id
		}
	}

	employees, err := h.queries.ListEmployeeDirectory(c.Request.Context(), store.ListEmployeeDirectoryParams{
		CompanyID: companyID,
		Column2:   searchVal,
		Column3:   deptIDVal,
	})
	if err != nil {
		response.InternalError(c, "Failed to list directory")
		return
	}
	response.OK(c, employees)
}

func (h *Handler) GetOrgChart(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	data, err := h.queries.GetOrgChartData(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get org chart data")
		return
	}
	response.OK(c, data)
}

func (h *Handler) ListExpiringDocuments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	docs, err := h.queries.ListExpiringDocuments(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list expiring documents")
		return
	}
	response.OK(c, docs)
}
