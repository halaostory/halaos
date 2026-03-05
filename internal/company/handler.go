package company

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
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

func (h *Handler) GetCompany(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	company, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.NotFound(c, "Company not found")
		return
	}
	response.OK(c, company)
}

type updateCompanyRequest struct {
	Name         *string `json:"name"`
	LegalName    *string `json:"legal_name"`
	TIN          *string `json:"tin"`
	BIRRDO       *string `json:"bir_rdo"`
	Address      *string `json:"address"`
	City         *string `json:"city"`
	Province     *string `json:"province"`
	ZipCode      *string `json:"zip_code"`
	Timezone     *string `json:"timezone"`
	PayFrequency *string `json:"pay_frequency"`
	LogoURL      *string `json:"logo_url"`
}

func (h *Handler) UpdateCompany(c *gin.Context) {
	var req updateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	company, err := h.queries.UpdateCompany(c.Request.Context(), store.UpdateCompanyParams{
		ID:           companyID,
		Name:         req.Name,
		LegalName:    req.LegalName,
		Tin:          req.TIN,
		BirRdo:       req.BIRRDO,
		Address:      req.Address,
		City:         req.City,
		Province:     req.Province,
		ZipCode:      req.ZipCode,
		Timezone:     req.Timezone,
		PayFrequency: req.PayFrequency,
		LogoUrl:      req.LogoURL,
	})
	if err != nil {
		h.logger.Error("failed to update company", "error", err)
		response.InternalError(c, "Failed to update company")
		return
	}
	response.OK(c, company)
}

type createDepartmentRequest struct {
	Code     string `json:"code" binding:"required"`
	Name     string `json:"name" binding:"required"`
	ParentID *int64 `json:"parent_id"`
}

func (h *Handler) ListDepartments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	depts, err := h.queries.ListDepartments(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list departments")
		return
	}
	response.OK(c, depts)
}

func (h *Handler) CreateDepartment(c *gin.Context) {
	var req createDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	dept, err := h.queries.CreateDepartment(c.Request.Context(), store.CreateDepartmentParams{
		CompanyID: companyID,
		Code:      req.Code,
		Name:      req.Name,
		ParentID:  req.ParentID,
	})
	if err != nil {
		h.logger.Error("failed to create department", "error", err)
		response.Conflict(c, "Department code already exists")
		return
	}
	response.Created(c, dept)
}

func (h *Handler) UpdateDepartment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid department ID")
		return
	}

	var req struct {
		Name           *string `json:"name"`
		ParentID       *int64  `json:"parent_id"`
		HeadEmployeeID *int64  `json:"head_employee_id"`
		IsActive       *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	dept, err := h.queries.UpdateDepartment(c.Request.Context(), store.UpdateDepartmentParams{
		ID:             id,
		CompanyID:      companyID,
		Name:           req.Name,
		ParentID:       req.ParentID,
		HeadEmployeeID: req.HeadEmployeeID,
		IsActive:       req.IsActive,
	})
	if err != nil {
		response.NotFound(c, "Department not found")
		return
	}
	response.OK(c, dept)
}

type createPositionRequest struct {
	Code         string `json:"code" binding:"required"`
	Title        string `json:"title" binding:"required"`
	DepartmentID *int64 `json:"department_id"`
	Grade        *string `json:"grade"`
}

func (h *Handler) ListPositions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	positions, err := h.queries.ListPositions(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list positions")
		return
	}
	response.OK(c, positions)
}

func (h *Handler) CreatePosition(c *gin.Context) {
	var req createPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	pos, err := h.queries.CreatePosition(c.Request.Context(), store.CreatePositionParams{
		CompanyID:    companyID,
		Code:         req.Code,
		Title:        req.Title,
		DepartmentID: req.DepartmentID,
		Grade:        req.Grade,
	})
	if err != nil {
		response.Conflict(c, "Position code already exists")
		return
	}
	response.Created(c, pos)
}

func (h *Handler) UpdatePosition(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid position ID")
		return
	}

	var req struct {
		Title        *string `json:"title"`
		DepartmentID *int64  `json:"department_id"`
		Grade        *string `json:"grade"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	pos, err := h.queries.UpdatePosition(c.Request.Context(), store.UpdatePositionParams{
		ID:           id,
		CompanyID:    companyID,
		Title:        req.Title,
		DepartmentID: req.DepartmentID,
		Grade:        req.Grade,
		IsActive:     req.IsActive,
	})
	if err != nil {
		response.NotFound(c, "Position not found")
		return
	}
	response.OK(c, pos)
}

func intParam(c *gin.Context, name string) int64 {
	v, _ := strconv.ParseInt(c.Param(name), 10, 64)
	return v
}
