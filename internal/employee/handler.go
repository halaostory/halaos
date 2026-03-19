package employee

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
	"github.com/tonypk/aigonhr/pkg/response"
)

// AccountingEmitter emits employee lifecycle events to the accounting outbox.
type AccountingEmitter interface {
	EmitEmployeeUpserted(ctx context.Context, companyID, employeeID int64) error
	EmitEmployeeTerminated(ctx context.Context, companyID, employeeID int64, reason string) error
}

type Handler struct {
	queries    *store.Queries
	pool       *pgxpool.Pool
	logger     *slog.Logger
	accounting AccountingEmitter
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// SetAccountingEmitter configures the accounting event emitter (optional).
func (h *Handler) SetAccountingEmitter(emitter AccountingEmitter) {
	h.accounting = emitter
}

type createEmployeeRequest struct {
	EmployeeNo     string  `json:"employee_no" binding:"required"`
	FirstName      string  `json:"first_name" binding:"required"`
	LastName       string  `json:"last_name" binding:"required"`
	MiddleName     *string `json:"middle_name"`
	Suffix         *string `json:"suffix"`
	DisplayName    *string `json:"display_name"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
	BirthDate      *string `json:"birth_date"`
	Gender         *string `json:"gender"`
	CivilStatus    *string `json:"civil_status"`
	Nationality    *string `json:"nationality"`
	DepartmentID   *int64  `json:"department_id"`
	PositionID     *int64  `json:"position_id"`
	CostCenterID   *int64  `json:"cost_center_id"`
	ManagerID      *int64  `json:"manager_id"`
	HireDate       string  `json:"hire_date" binding:"required"`
	EmploymentType string  `json:"employment_type"`
}

func (h *Handler) ListEmployees(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	statusFilter := c.Query("status")
	deptFilter := c.Query("department_id")

	var deptIDVal int64
	if deptFilter != "" {
		if id, err := strconv.ParseInt(deptFilter, 10, 64); err == nil {
			deptIDVal = id
		}
	}

	employees, err := h.queries.ListEmployees(c.Request.Context(), store.ListEmployeesParams{
		CompanyID: companyID,
		Column2:   statusFilter,
		Column3:   deptIDVal,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list employees")
		return
	}

	count, _ := h.queries.CountEmployees(c.Request.Context(), store.CountEmployeesParams{
		CompanyID: companyID,
		Column2:   statusFilter,
		Column3:   deptIDVal,
	})

	response.Paginated(c, employees, count, pg.Page, pg.Limit)
}

func (h *Handler) CreateEmployee(c *gin.Context) {
	var req createEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	hireDate, err := time.Parse("2006-01-02", req.HireDate)
	if err != nil {
		response.BadRequest(c, "Invalid hire_date format, use YYYY-MM-DD")
		return
	}

	var birthDate pgtype.Date
	if req.BirthDate != nil {
		if bd, err := time.Parse("2006-01-02", *req.BirthDate); err == nil {
			birthDate = pgtype.Date{Time: bd, Valid: true}
		}
	}

	empType := req.EmploymentType
	if empType == "" {
		empType = "regular"
	}

	emp, err := h.queries.CreateEmployee(c.Request.Context(), store.CreateEmployeeParams{
		CompanyID:      companyID,
		EmployeeNo:     req.EmployeeNo,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		MiddleName:     req.MiddleName,
		Suffix:         req.Suffix,
		DisplayName:    req.DisplayName,
		Email:          req.Email,
		Phone:          req.Phone,
		BirthDate:      birthDate,
		Gender:         req.Gender,
		CivilStatus:    req.CivilStatus,
		Nationality:    req.Nationality,
		DepartmentID:   req.DepartmentID,
		PositionID:     req.PositionID,
		CostCenterID:   req.CostCenterID,
		ManagerID:      req.ManagerID,
		HireDate:       hireDate,
		EmploymentType: empType,
	})
	if err != nil {
		h.logger.Error("failed to create employee", "error", err)
		response.Conflict(c, "Employee number already exists")
		return
	}

	// Emit employee.hired event for Integration Hub provisioning
	h.emitEmployeeEvent(c, companyID, emp.ID, "employee.hired", map[string]any{
		"email":           derefStr(req.Email),
		"first_name":      req.FirstName,
		"last_name":       req.LastName,
		"employment_type": empType,
		"department_id":   req.DepartmentID,
	})

	// Emit accounting event (async, non-blocking)
	if h.accounting != nil {
		go func() {
			if err := h.accounting.EmitEmployeeUpserted(context.Background(), companyID, emp.ID); err != nil {
				h.logger.Error("failed to emit employee.upserted accounting event", "employee_id", emp.ID, "error", err)
			}
		}()
	}

	response.Created(c, emp)
}

func (h *Handler) GetEmployee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	emp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	response.OK(c, emp)
}

func (h *Handler) UpdateEmployee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	var req struct {
		FirstName      *string `json:"first_name"`
		LastName       *string `json:"last_name"`
		MiddleName     *string `json:"middle_name"`
		DisplayName    *string `json:"display_name"`
		Email          *string `json:"email"`
		Phone          *string `json:"phone"`
		DepartmentID   *int64  `json:"department_id"`
		PositionID     *int64  `json:"position_id"`
		CostCenterID   *int64  `json:"cost_center_id"`
		ManagerID      *int64  `json:"manager_id"`
		EmploymentType *string `json:"employment_type"`
		Status         *string `json:"status"`
		Nationality    *string `json:"nationality"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	firstName := ""
	if req.FirstName != nil {
		firstName = *req.FirstName
	}
	lastName := ""
	if req.LastName != nil {
		lastName = *req.LastName
	}
	employmentType := ""
	if req.EmploymentType != nil {
		employmentType = *req.EmploymentType
	}
	empStatus := ""
	if req.Status != nil {
		empStatus = *req.Status
	}

	emp, err := h.queries.UpdateEmployee(c.Request.Context(), store.UpdateEmployeeParams{
		ID:             id,
		CompanyID:      companyID,
		FirstName:      firstName,
		LastName:       lastName,
		MiddleName:     req.MiddleName,
		DisplayName:    req.DisplayName,
		Email:          req.Email,
		Phone:          req.Phone,
		DepartmentID:   req.DepartmentID,
		PositionID:     req.PositionID,
		CostCenterID:   req.CostCenterID,
		ManagerID:      req.ManagerID,
		EmploymentType: employmentType,
		Status:         empStatus,
		Nationality:    req.Nationality,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	// Emit lifecycle events for status changes
	if empStatus == "separated" || empStatus == "terminated" {
		h.emitEmployeeEvent(c, companyID, id, "employee.terminated", map[string]any{
			"status": empStatus,
		})
	}
	if req.DepartmentID != nil {
		h.emitEmployeeEvent(c, companyID, id, "employee.transferred", map[string]any{
			"department_id": *req.DepartmentID,
		})
	}

	// Emit accounting events (async, non-blocking)
	if h.accounting != nil {
		go func() {
			if empStatus == "separated" || empStatus == "terminated" {
				reason := empStatus
				if err := h.accounting.EmitEmployeeTerminated(context.Background(), companyID, id, reason); err != nil {
					h.logger.Error("failed to emit employee.terminated accounting event", "employee_id", id, "error", err)
				}
			} else {
				if err := h.accounting.EmitEmployeeUpserted(context.Background(), companyID, id); err != nil {
					h.logger.Error("failed to emit employee.upserted accounting event", "employee_id", id, "error", err)
				}
			}
		}()
	}

	response.OK(c, emp)
}

func (h *Handler) GetProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	profile, err := h.queries.GetEmployeeProfile(c.Request.Context(), store.GetEmployeeProfileParams{
		EmployeeID: id,
		CompanyID:  companyID,
	})
	if err != nil {
		response.NotFound(c, "Profile not found")
		return
	}
	response.OK(c, profile)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	var req store.UpsertEmployeeProfileParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req.EmployeeID = id

	profile, err := h.queries.UpsertEmployeeProfile(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to update profile", "error", err)
		response.InternalError(c, "Failed to update profile")
		return
	}
	response.OK(c, profile)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	docs, err := h.queries.ListEmployeeDocuments(c.Request.Context(), store.ListEmployeeDocumentsParams{
		EmployeeID: id,
		CompanyID:  companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list documents")
		return
	}
	response.OK(c, docs)
}

func (h *Handler) UploadDocument(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	const maxEmpDocSize = 20 << 20 // 20MB

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "File is required")
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > maxEmpDocSize {
		response.BadRequest(c, "File size exceeds 20MB limit")
		return
	}

	// Detect actual MIME type from file content
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	detectedMIME := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		response.InternalError(c, "Failed to process file")
		return
	}
	allowedMIME := map[string]bool{
		"application/pdf": true, "image/png": true, "image/jpeg": true, "image/webp": true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
		"text/plain": true, "text/csv": true,
	}
	if !allowedMIME[detectedMIME] {
		response.BadRequest(c, "File type not allowed. Accepted: PDF, images, Office documents, CSV, text")
		return
	}

	docType := c.PostForm("doc_type")
	if docType == "" {
		docType = "general"
	}

	// Save file to upload directory
	uploadDir := fmt.Sprintf("uploads/documents/%d/%d", companyID, id)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("failed to create upload dir", "error", err)
		response.InternalError(c, "Failed to upload document")
		return
	}

	// Use sanitized UUID filename to prevent path traversal
	ext := strings.ToLower(filepath.Ext(header.Filename))
	fileName := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), uuid.New().String()[:8], ext)
	filePath := filepath.Join(uploadDir, fileName)

	out, err := os.Create(filePath)
	if err != nil {
		h.logger.Error("failed to create file", "error", err)
		response.InternalError(c, "Failed to upload document")
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		h.logger.Error("failed to write file", "error", err)
		response.InternalError(c, "Failed to upload document")
		return
	}

	mimeType := detectedMIME

	var expiryDate pgtype.Date
	if ed := c.PostForm("expiry_date"); ed != "" {
		if parsed, err := time.Parse("2006-01-02", ed); err == nil {
			expiryDate = pgtype.Date{Time: parsed, Valid: true}
		}
	}

	doc, err := h.queries.CreateEmployeeDocument(c.Request.Context(), store.CreateEmployeeDocumentParams{
		CompanyID:  companyID,
		EmployeeID: id,
		DocType:    docType,
		FileName:   header.Filename,
		FilePath:   filePath,
		FileSize:   written,
		MimeType:   &mimeType,
		UploadedBy: &userID,
		ExpiryDate: expiryDate,
	})
	if err != nil {
		h.logger.Error("failed to create document record", "error", err)
		response.InternalError(c, "Failed to save document record")
		return
	}
	response.Created(c, doc)
}

func (h *Handler) DownloadDocument(c *gin.Context) {
	docID, err := uuid.Parse(c.Param("doc_id"))
	if err != nil {
		response.BadRequest(c, "Invalid document ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	doc, err := h.queries.GetEmployeeDocument(c.Request.Context(), store.GetEmployeeDocumentParams{
		ID:        docID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Document not found")
		return
	}

	c.FileAttachment(doc.FilePath, doc.FileName)
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	docID, err := uuid.Parse(c.Param("doc_id"))
	if err != nil {
		response.BadRequest(c, "Invalid document ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	doc, err := h.queries.GetEmployeeDocument(c.Request.Context(), store.GetEmployeeDocumentParams{
		ID:        docID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Document not found")
		return
	}

	// Delete file from disk
	_ = os.Remove(doc.FilePath)

	if err := h.queries.DeleteEmployeeDocument(c.Request.Context(), store.DeleteEmployeeDocumentParams{
		ID:        docID,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete document")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

// emitEmployeeEvent fires an hr_event for integration provisioning.
func (h *Handler) emitEmployeeEvent(c *gin.Context, companyID, employeeID int64, eventType string, payload map[string]any) {
	payloadJSON, _ := json.Marshal(payload)
	userID := auth.GetUserID(c)
	idempotencyKey := fmt.Sprintf("%s:%d:%d", eventType, employeeID, time.Now().UnixNano())

	_, err := h.queries.InsertHREvent(c.Request.Context(), store.InsertHREventParams{
		CompanyID:      companyID,
		AggregateType:  "employee",
		AggregateID:    employeeID,
		EventType:      eventType,
		EventVersion:   1,
		Payload:        payloadJSON,
		ActorUserID:    &userID,
		IdempotencyKey: &idempotencyKey,
	})
	if err != nil {
		h.logger.Error("failed to emit employee event",
			"event_type", eventType,
			"employee_id", employeeID,
			"error", err,
		)
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
