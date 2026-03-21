package company

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

func (h *Handler) ListUserCompanies(c *gin.Context) {
	userID := auth.GetUserID(c)

	companies, err := h.queries.GetUserCompanies(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list user companies", "error", err, "user_id", userID)
		response.InternalError(c, "failed to load companies")
		return
	}

	response.OK(c, companies)
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

func (h *Handler) UpdateCompany(c *gin.Context) {
	var req struct {
		Name            string  `json:"name"`
		LegalName       *string `json:"legal_name"`
		TIN             *string `json:"tin"`
		BIRRDO          *string `json:"bir_rdo"`
		Address         *string `json:"address"`
		City            *string `json:"city"`
		Province        *string `json:"province"`
		ZipCode         *string `json:"zip_code"`
		Timezone        string  `json:"timezone"`
		PayFrequency    string  `json:"pay_frequency"`
		LogoURL         *string `json:"logo_url"`
		SSSErNo         *string `json:"sss_er_no"`
		PhilhealthErNo  *string `json:"philhealth_er_no"`
		PagibigErNo     *string `json:"pagibig_er_no"`
		BankName        *string `json:"bank_name"`
		BankBranch      *string `json:"bank_branch"`
		BankAccountNo   *string `json:"bank_account_no"`
		BankAccountName *string `json:"bank_account_name"`
		ContactPerson   *string `json:"contact_person"`
		ContactEmail    *string `json:"contact_email"`
		ContactPhone    *string `json:"contact_phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	company, err := h.queries.UpdateCompany(c.Request.Context(), store.UpdateCompanyParams{
		ID:              companyID,
		Name:            req.Name,
		LegalName:       req.LegalName,
		Tin:             req.TIN,
		BirRdo:          req.BIRRDO,
		Address:         req.Address,
		City:            req.City,
		Province:        req.Province,
		ZipCode:         req.ZipCode,
		Timezone:        req.Timezone,
		PayFrequency:    req.PayFrequency,
		LogoUrl:         req.LogoURL,
		SssErNo:         req.SSSErNo,
		PhilhealthErNo:  req.PhilhealthErNo,
		PagibigErNo:     req.PagibigErNo,
		BankName:        req.BankName,
		BankBranch:      req.BankBranch,
		BankAccountNo:   req.BankAccountNo,
		BankAccountName: req.BankAccountName,
		ContactPerson:   req.ContactPerson,
		ContactEmail:    req.ContactEmail,
		ContactPhone:    req.ContactPhone,
	})
	if err != nil {
		h.logger.Error("failed to update company", "error", err)
		response.InternalError(c, "Failed to update company")
		return
	}
	response.OK(c, company)
}

func (h *Handler) UploadLogo(c *gin.Context) {
	file, header, err := c.Request.FormFile("logo")
	if err != nil {
		response.BadRequest(c, "Logo file is required")
		return
	}
	defer file.Close()

	// Validate file type
	ext := filepath.Ext(header.Filename)
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".svg": true, ".webp": true}
	if !allowed[ext] {
		response.BadRequest(c, "Only PNG, JPG, SVG, and WebP files are allowed")
		return
	}

	companyID := auth.GetCompanyID(c)
	uploadDir := fmt.Sprintf("uploads/logos/%d", companyID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("failed to create logo dir", "error", err)
		response.InternalError(c, "Failed to upload logo")
		return
	}

	fileName := fmt.Sprintf("logo_%d%s", time.Now().UnixMilli(), ext)
	filePath := filepath.Join(uploadDir, fileName)

	out, err := os.Create(filePath)
	if err != nil {
		h.logger.Error("failed to create logo file", "error", err)
		response.InternalError(c, "Failed to upload logo")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		h.logger.Error("failed to write logo file", "error", err)
		response.InternalError(c, "Failed to upload logo")
		return
	}

	// Update company logo_url
	logoURL := "/" + filePath
	company, err := h.queries.UpdateCompany(c.Request.Context(), store.UpdateCompanyParams{
		ID:      companyID,
		LogoUrl: &logoURL,
	})
	if err != nil {
		h.logger.Error("failed to update logo url", "error", err)
		response.InternalError(c, "Failed to update logo")
		return
	}
	response.OK(c, company)
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
	var req struct {
		Code     string `json:"code" binding:"required"`
		Name     string `json:"name" binding:"required"`
		ParentID *int64 `json:"parent_id"`
	}
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
		Name           string `json:"name"`
		ParentID       *int64 `json:"parent_id"`
		HeadEmployeeID *int64 `json:"head_employee_id"`
		IsActive       bool   `json:"is_active"`
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
	var req struct {
		Code         string  `json:"code" binding:"required"`
		Title        string  `json:"title" binding:"required"`
		DepartmentID *int64  `json:"department_id"`
		Grade        *string `json:"grade"`
	}
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
		Title        string  `json:"title"`
		DepartmentID *int64  `json:"department_id"`
		Grade        *string `json:"grade"`
		IsActive     bool    `json:"is_active"`
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
