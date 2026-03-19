package compliance

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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

// Tax Tables

func (h *Handler) ListSSSTable(c *gin.Context) {
	asOf := parseAsOfDate(c)
	table, err := h.queries.ListSSSTable(c.Request.Context(), asOf)
	if err != nil {
		response.InternalError(c, "Failed to list SSS table")
		return
	}
	response.OK(c, table)
}

func (h *Handler) ListPhilHealthTable(c *gin.Context) {
	asOf := parseAsOfDate(c)
	table, err := h.queries.ListPhilHealthTable(c.Request.Context(), asOf)
	if err != nil {
		response.InternalError(c, "Failed to list PhilHealth table")
		return
	}
	response.OK(c, table)
}

func (h *Handler) ListPagIBIGTable(c *gin.Context) {
	asOf := parseAsOfDate(c)
	table, err := h.queries.ListPagIBIGTable(c.Request.Context(), asOf)
	if err != nil {
		response.InternalError(c, "Failed to list Pag-IBIG table")
		return
	}
	response.OK(c, table)
}

func (h *Handler) ListBIRTaxTable(c *gin.Context) {
	asOf := parseAsOfDate(c)
	frequency := c.DefaultQuery("frequency", "semi_monthly")
	table, err := h.queries.ListBIRTaxTable(c.Request.Context(), store.ListBIRTaxTableParams{
		Frequency:     frequency,
		EffectiveFrom: asOf,
	})
	if err != nil {
		response.InternalError(c, "Failed to list BIR tax table")
		return
	}
	response.OK(c, table)
}

// Government Forms

func (h *Handler) ListGovernmentForms(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	forms, err := h.queries.ListGovernmentForms(c.Request.Context(), store.ListGovernmentFormsParams{
		CompanyID: companyID,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list government forms")
		return
	}
	response.OK(c, forms)
}

func (h *Handler) CreateGovernmentForm(c *gin.Context) {
	var req struct {
		FormType string          `json:"form_type" binding:"required"`
		TaxYear  int32           `json:"tax_year" binding:"required"`
		Period   *string         `json:"period"`
		Payload  json.RawMessage `json:"payload" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	form, err := h.queries.CreateGovernmentForm(c.Request.Context(), store.CreateGovernmentFormParams{
		CompanyID: companyID,
		FormType:  req.FormType,
		TaxYear:   req.TaxYear,
		Period:    req.Period,
		Payload:   req.Payload,
	})
	if err != nil {
		h.logger.Error("failed to create government form", "error", err)
		response.InternalError(c, "Failed to create government form")
		return
	}
	response.Created(c, form)
}

// Salary Structures

func (h *Handler) ListSalaryStructures(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	structures, err := h.queries.ListSalaryStructures(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list salary structures")
		return
	}
	response.OK(c, structures)
}

func (h *Handler) CreateSalaryStructure(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	structure, err := h.queries.CreateSalaryStructure(c.Request.Context(), store.CreateSalaryStructureParams{
		CompanyID:   companyID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		response.Conflict(c, "Salary structure already exists")
		return
	}
	response.Created(c, structure)
}

func (h *Handler) UpdateSalaryStructure(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}

	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	structure, err := h.queries.UpdateSalaryStructure(c.Request.Context(), store.UpdateSalaryStructureParams{
		ID:          id,
		CompanyID:   companyID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		response.NotFound(c, "Salary structure not found")
		return
	}
	response.OK(c, structure)
}

func (h *Handler) DeleteSalaryStructure(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteSalaryStructure(c.Request.Context(), store.DeleteSalaryStructureParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.NotFound(c, "Salary structure not found")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

// Salary Components

func (h *Handler) ListSalaryComponents(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	components, err := h.queries.ListSalaryComponents(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list salary components")
		return
	}
	response.OK(c, components)
}

func (h *Handler) CreateSalaryComponent(c *gin.Context) {
	var req struct {
		Code          string `json:"code" binding:"required"`
		Name          string `json:"name" binding:"required"`
		ComponentType string `json:"component_type" binding:"required"`
		IsTaxable     bool   `json:"is_taxable"`
		IsStatutory   bool   `json:"is_statutory"`
		IsFixed       bool   `json:"is_fixed"`
		Formula       []byte `json:"formula"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	component, err := h.queries.CreateSalaryComponent(c.Request.Context(), store.CreateSalaryComponentParams{
		CompanyID:     companyID,
		Code:          req.Code,
		Name:          req.Name,
		ComponentType: req.ComponentType,
		IsTaxable:     req.IsTaxable,
		IsStatutory:   req.IsStatutory,
		IsFixed:       req.IsFixed,
		Formula:       req.Formula,
	})
	if err != nil {
		response.Conflict(c, "Salary component code already exists")
		return
	}
	response.Created(c, component)
}

func (h *Handler) UpdateSalaryComponent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid component ID")
		return
	}

	var req struct {
		Code          string `json:"code" binding:"required"`
		Name          string `json:"name" binding:"required"`
		ComponentType string `json:"component_type" binding:"required"`
		IsTaxable     bool   `json:"is_taxable"`
		IsStatutory   bool   `json:"is_statutory"`
		IsFixed       bool   `json:"is_fixed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	component, err := h.queries.UpdateSalaryComponent(c.Request.Context(), store.UpdateSalaryComponentParams{
		ID:            id,
		CompanyID:     companyID,
		Code:          req.Code,
		Name:          req.Name,
		ComponentType: req.ComponentType,
		IsTaxable:     req.IsTaxable,
		IsStatutory:   req.IsStatutory,
		IsFixed:       req.IsFixed,
	})
	if err != nil {
		response.NotFound(c, "Salary component not found")
		return
	}
	response.OK(c, component)
}

func (h *Handler) DeleteSalaryComponent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid component ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteSalaryComponent(c.Request.Context(), store.DeleteSalaryComponentParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.NotFound(c, "Salary component not found")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

func parseAsOfDate(c *gin.Context) time.Time {
	if dateStr := c.Query("as_of"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			return t
		}
	}
	return time.Now()
}
