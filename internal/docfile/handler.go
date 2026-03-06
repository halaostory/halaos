package docfile

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

func (h *Handler) ListCategories(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	_ = h.queries.EnsureDefaultCategories(c.Request.Context(), companyID)
	cats, err := h.queries.ListDocumentCategories(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list categories")
		return
	}
	response.OK(c, cats)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		Name        string `json:"name" binding:"required"`
		Slug        string `json:"slug" binding:"required"`
		Description string `json:"description"`
		SortOrder   int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cat, err := h.queries.CreateDocumentCategory(c.Request.Context(), store.CreateDocumentCategoryParams{
		CompanyID:   companyID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: &req.Description,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		response.InternalError(c, "Failed to create category")
		return
	}
	response.Created(c, cat)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	categoryID, _ := strconv.ParseInt(c.Query("category_id"), 10, 64)
	status := c.Query("status")
	docs, err := h.queries.List201Documents(c.Request.Context(), store.List201DocumentsParams{
		CompanyID:  companyID,
		EmployeeID: empID,
		CategoryID: categoryID,
		Status:     status,
	})
	if err != nil {
		response.InternalError(c, "Failed to list documents")
		return
	}
	response.OK(c, docs)
}

func (h *Handler) GetStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	stats, err := h.queries.GetEmployee201Stats(c.Request.Context(), store.GetEmployee201StatsParams{
		CompanyID:  companyID,
		EmployeeID: empID,
	})
	if err != nil {
		response.InternalError(c, "Failed to get stats")
		return
	}
	response.OK(c, stats)
}

func (h *Handler) Upload(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "File is required")
		return
	}
	defer file.Close()

	title := c.PostForm("title")
	docType := c.PostForm("doc_type")
	if docType == "" {
		docType = "general"
	}
	categoryIDStr := c.PostForm("category_id")

	uploadDir := fmt.Sprintf("uploads/201file/%d/%d", companyID, empID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		response.InternalError(c, "Failed to create upload directory")
		return
	}

	fileName := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), header.Filename)
	filePath := filepath.Join(uploadDir, fileName)

	out, err := os.Create(filePath)
	if err != nil {
		response.InternalError(c, "Failed to save file")
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		response.InternalError(c, "Failed to write file")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	var expiryDate pgtype.Date
	if ed := c.PostForm("expiry_date"); ed != "" {
		if parsed, err := time.Parse("2006-01-02", ed); err == nil {
			expiryDate = pgtype.Date{Time: parsed, Valid: true}
		}
	}

	var catID *int64
	if cid, err := strconv.ParseInt(categoryIDStr, 10, 64); err == nil && cid > 0 {
		catID = &cid
	}

	var titlePtr *string
	if title != "" {
		titlePtr = &title
	}

	var notes *string
	if n := c.PostForm("notes"); n != "" {
		notes = &n
	}

	doc, err := h.queries.Upload201Document(c.Request.Context(), store.Upload201DocumentParams{
		CompanyID:  companyID,
		EmployeeID: empID,
		CategoryID: catID,
		Title:      titlePtr,
		DocType:    docType,
		FileName:   header.Filename,
		FilePath:   filePath,
		FileSize:   written,
		MimeType:   &mimeType,
		Version:    1,
		ExpiryDate: expiryDate,
		UploadedBy: &userID,
		Notes:      notes,
	})
	if err != nil {
		response.InternalError(c, "Failed to save document record")
		return
	}
	response.Created(c, doc)
}

func (h *Handler) Download(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	docID, err := uuid.Parse(c.Param("doc_id"))
	if err != nil {
		response.BadRequest(c, "Invalid document ID")
		return
	}
	doc, err := h.queries.Get201Document(c.Request.Context(), store.Get201DocumentParams{
		ID: docID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Document not found")
		return
	}
	c.FileAttachment(doc.FilePath, doc.FileName)
}

func (h *Handler) UpdateDocument(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	docID, err := uuid.Parse(c.Param("doc_id"))
	if err != nil {
		response.BadRequest(c, "Invalid document ID")
		return
	}
	var req struct {
		Title      string `json:"title"`
		CategoryID *int64 `json:"category_id"`
		ExpiryDate string `json:"expiry_date"`
		Notes      string `json:"notes"`
		Status     string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var expiryDate pgtype.Date
	if req.ExpiryDate != "" {
		if parsed, err := time.Parse("2006-01-02", req.ExpiryDate); err == nil {
			expiryDate = pgtype.Date{Time: parsed, Valid: true}
		}
	}
	var titlePtr, notesPtr *string
	if req.Title != "" {
		titlePtr = &req.Title
	}
	if req.Notes != "" {
		notesPtr = &req.Notes
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	doc, err := h.queries.Update201Document(c.Request.Context(), store.Update201DocumentParams{
		ID:         docID,
		CompanyID:  companyID,
		Title:      titlePtr,
		CategoryID: req.CategoryID,
		ExpiryDate: expiryDate,
		Notes:      notesPtr,
		Status:     status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update document")
		return
	}
	response.OK(c, doc)
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	docID, err := uuid.Parse(c.Param("doc_id"))
	if err != nil {
		response.BadRequest(c, "Invalid document ID")
		return
	}
	doc, err := h.queries.Get201Document(c.Request.Context(), store.Get201DocumentParams{
		ID: docID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Document not found")
		return
	}
	_ = os.Remove(doc.FilePath)
	if err := h.queries.Delete201Document(c.Request.Context(), store.Delete201DocumentParams{
		ID: docID, CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete document")
		return
	}
	response.OK(c, gin.H{"message": "Document deleted"})
}

func (h *Handler) ListExpiring(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	docs, err := h.queries.List201ExpiringDocuments(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list expiring documents")
		return
	}
	response.OK(c, docs)
}

func (h *Handler) GetCompliance(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	checklist, err := h.queries.GetComplianceChecklist(c.Request.Context(), store.GetComplianceChecklistParams{
		CompanyID:  companyID,
		EmployeeID: empID,
	})
	if err != nil {
		response.InternalError(c, "Failed to get compliance checklist")
		return
	}
	response.OK(c, checklist)
}

func (h *Handler) ListRequirements(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	reqs, err := h.queries.ListDocumentRequirements(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list requirements")
		return
	}
	response.OK(c, reqs)
}

func (h *Handler) CreateRequirement(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		CategoryID   int64  `json:"category_id" binding:"required"`
		DocumentName string `json:"document_name" binding:"required"`
		IsRequired   bool   `json:"is_required"`
		AppliesTo    string `json:"applies_to"`
		ExpiryMonths *int32 `json:"expiry_months"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	appliesTo := req.AppliesTo
	if appliesTo == "" {
		appliesTo = "all"
	}
	r, err := h.queries.CreateDocumentRequirement(c.Request.Context(), store.CreateDocumentRequirementParams{
		CompanyID:    companyID,
		CategoryID:   req.CategoryID,
		DocumentName: req.DocumentName,
		IsRequired:   req.IsRequired,
		AppliesTo:    appliesTo,
		ExpiryMonths: req.ExpiryMonths,
	})
	if err != nil {
		response.InternalError(c, "Failed to create requirement")
		return
	}
	response.Created(c, r)
}

func (h *Handler) DeleteRequirement(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	reqID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.queries.DeleteDocumentRequirement(c.Request.Context(), store.DeleteDocumentRequirementParams{
		ID: reqID, CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete requirement")
		return
	}
	response.OK(c, gin.H{"message": "Requirement deleted"})
}
