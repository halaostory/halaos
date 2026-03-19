package integration

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// AccountingHandler handles accounting integration endpoints (SSO, export, status).
type AccountingHandler struct {
	queries *store.Queries
	sso     *SSOService
	logger  *slog.Logger
}

// NewAccountingHandler creates a new AccountingHandler.
func NewAccountingHandler(queries *store.Queries, sso *SSOService, logger *slog.Logger) *AccountingHandler {
	return &AccountingHandler{queries: queries, sso: sso, logger: logger}
}

// RegisterRoutes registers accounting integration routes.
func (h *AccountingHandler) RegisterRoutes(protected *gin.RouterGroup) {
	acct := protected.Group("/integrations/accounting")
	acct.POST("/link", h.CreateLink)
	acct.GET("/link", h.GetLink)
	acct.DELETE("/link/:id", h.DeleteLink)
	acct.GET("/sso-token", h.GetSSOToken)
	acct.GET("/export/employees", h.ExportEmployees)
	acct.GET("/export/payroll-runs", h.ExportPayrollRuns)
	acct.GET("/sync-status", h.GetSyncStatus)
}

// GetSSOToken generates a cross-app JWT for navigating to AIStarlight.
func (h *AccountingHandler) GetSSOToken(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	email := auth.GetEmail(c)
	role := string(auth.GetRole(c))

	// Get user details for the token
	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "Failed to get user details")
		return
	}

	// Check active accounting link exists
	link, err := h.queries.GetActiveAccountingLink(c.Request.Context(), companyID)
	if err != nil {
		response.NotFound(c, "No active accounting link. Connect AIStarlight first.")
		return
	}

	token, err := h.sso.GenerateToken(companyID, userID, email, role, user.FirstName, user.LastName)
	if err != nil {
		h.logger.Error("failed to generate SSO token", "error", err)
		response.InternalError(c, "Failed to generate SSO token")
		return
	}

	response.OK(c, gin.H{
		"sso_token":   token,
		"target_url":  link.ApiEndpoint,
		"remote_company_id": link.RemoteCompanyID,
		"expires_in":  300, // 5 minutes
	})
}

// GetSyncStatus returns the accounting sync status for the current company.
func (h *AccountingHandler) GetSyncStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	link, err := h.queries.GetActiveAccountingLink(c.Request.Context(), companyID)
	if err != nil {
		response.OK(c, gin.H{
			"connected":   false,
			"status":      "disconnected",
		})
		return
	}

	counts, _ := h.queries.CountOutboxByStatus(c.Request.Context(), companyID)
	statusCounts := make(map[string]int64)
	for _, sc := range counts {
		statusCounts[sc.Status] = sc.Count
	}

	response.OK(c, gin.H{
		"connected":      true,
		"status":         link.Status,
		"last_synced_at": link.LastSyncedAt,
		"provider":       link.Provider,
		"outbox": gin.H{
			"pending": statusCounts["pending"],
			"sent":    statusCounts["sent"],
			"failed":  statusCounts["failed"],
			"dead":    statusCounts["dead"],
		},
	})
}

// CreateLink creates a new accounting link to AIStarlight.
func (h *AccountingHandler) CreateLink(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var req struct {
		RemoteCompanyID string `json:"remote_company_id" binding:"required"`
		APIEndpoint     string `json:"api_endpoint" binding:"required,url"`
		Jurisdiction    string `json:"jurisdiction" binding:"required,oneof=PH LK SG"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check for existing active link
	existing, err := h.queries.GetActiveAccountingLink(c.Request.Context(), companyID)
	if err == nil && existing.ID > 0 {
		response.Error(c, http.StatusConflict, "CONFLICT", "An active accounting link already exists. Delete it first to create a new one.")
		return
	}

	// Generate webhook secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		response.InternalError(c, "Failed to generate webhook secret")
		return
	}
	webhookSecret := hex.EncodeToString(secretBytes)

	link, err := h.queries.CreateAccountingLink(c.Request.Context(), store.CreateAccountingLinkParams{
		CompanyID:       companyID,
		Provider:        "aistarlight",
		RemoteCompanyID: req.RemoteCompanyID,
		ApiEndpoint:     req.APIEndpoint,
		ApiKeyEnc:       "", // managed externally
		WebhookSecret:   webhookSecret,
		Jurisdiction:    req.Jurisdiction,
		Status:          "active",
	})
	if err != nil {
		h.logger.Error("failed to create accounting link", "error", err)
		response.InternalError(c, "Failed to create accounting link")
		return
	}

	response.Created(c, gin.H{
		"id":                link.ID,
		"provider":          link.Provider,
		"remote_company_id": link.RemoteCompanyID,
		"api_endpoint":      link.ApiEndpoint,
		"jurisdiction":      link.Jurisdiction,
		"webhook_secret":    webhookSecret,
		"status":            link.Status,
		"created_at":        link.CreatedAt,
	})
}

// GetLink returns the current accounting link for the company.
func (h *AccountingHandler) GetLink(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	link, err := h.queries.GetActiveAccountingLink(c.Request.Context(), companyID)
	if err != nil {
		response.OK(c, gin.H{
			"connected": false,
		})
		return
	}

	response.OK(c, gin.H{
		"connected":         true,
		"id":                link.ID,
		"provider":          link.Provider,
		"remote_company_id": link.RemoteCompanyID,
		"api_endpoint":      link.ApiEndpoint,
		"jurisdiction":      link.Jurisdiction,
		"status":            link.Status,
		"last_synced_at":    link.LastSyncedAt,
		"created_at":        link.CreatedAt,
	})
}

// DeleteLink removes an accounting link.
func (h *AccountingHandler) DeleteLink(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid link ID")
		return
	}

	if err := h.queries.DeleteAccountingLink(c.Request.Context(), store.DeleteAccountingLinkParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		h.logger.Error("failed to delete accounting link", "error", err)
		response.InternalError(c, "Failed to delete accounting link")
		return
	}

	response.OK(c, gin.H{"message": "Accounting link removed"})
}

// ExportEmployees exports employees as JSON for AIStarlight backfill.
func (h *AccountingHandler) ExportEmployees(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	employees, err := h.queries.ListActiveEmployees(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list employees")
		return
	}

	// Get profiles for TIN/SSS/PhilHealth/PagIBIG
	type employeeExport struct {
		EmployeeID     int64   `json:"employee_id"`
		EmployeeNo     string  `json:"employee_no"`
		FirstName      string  `json:"first_name"`
		LastName       string  `json:"last_name"`
		Email          *string `json:"email"`
		Status         string  `json:"status"`
		EmploymentType string  `json:"employment_type"`
		DepartmentID   *int64  `json:"department_id,omitempty"`
		PositionID     *int64  `json:"position_id,omitempty"`
		Nationality    *string `json:"nationality,omitempty"`
		TIN            string  `json:"tin,omitempty"`
		SSS            string  `json:"sss,omitempty"`
		PhilHealth     string  `json:"philhealth,omitempty"`
		PagIBIG        string  `json:"pagibig,omitempty"`
	}

	result := make([]employeeExport, 0, len(employees))
	for _, emp := range employees {
		export := employeeExport{
			EmployeeID:     emp.ID,
			EmployeeNo:     emp.EmployeeNo,
			FirstName:      emp.FirstName,
			LastName:       emp.LastName,
			Email:          emp.Email,
			Status:         emp.Status,
			EmploymentType: emp.EmploymentType,
			DepartmentID:   emp.DepartmentID,
			PositionID:     emp.PositionID,
			Nationality:    emp.Nationality,
		}

		// Try to get profile for statutory numbers
		profile, err := h.queries.GetEmployeeProfile(c.Request.Context(), store.GetEmployeeProfileParams{
			EmployeeID: emp.ID,
			CompanyID:  companyID,
		})
		if err == nil {
			if profile.Tin != nil {
				export.TIN = *profile.Tin
			}
			if profile.SssNo != nil {
				export.SSS = *profile.SssNo
			}
			if profile.PhilhealthNo != nil {
				export.PhilHealth = *profile.PhilhealthNo
			}
			if profile.PagibigNo != nil {
				export.PagIBIG = *profile.PagibigNo
			}
		}

		result = append(result, export)
	}

	response.OK(c, result)
}

// ExportPayrollRuns exports completed payroll runs for AIStarlight backfill.
func (h *AccountingHandler) ExportPayrollRuns(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.ParseInt(limitStr, 10, 32)
	offset, _ := strconv.ParseInt(offsetStr, 10, 32)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	runs, err := h.queries.ListCompletedPayrollRuns(c.Request.Context(), store.ListCompletedPayrollRunsParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list payroll runs")
		return
	}

	response.OK(c, runs)
}
