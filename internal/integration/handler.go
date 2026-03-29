package integration

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/pagination"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler handles HTTP requests for integration management.
type Handler struct {
	svc      *Service
	provSvc  *ProvisioningService
	queries  *store.Queries
}

// NewHandler creates an integration HTTP handler.
func NewHandler(svc *Service, provSvc *ProvisioningService, queries *store.Queries) *Handler {
	return &Handler{svc: svc, provSvc: provSvc, queries: queries}
}

// ListConnections returns all connections for the company.
func (h *Handler) ListConnections(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	conns, err := h.queries.ListIntegrationConnections(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list connections")
		return
	}

	// Strip encrypted_creds from response
	type safeConn struct {
		store.IntegrationConnection
		EncryptedCreds []byte `json:"encrypted_creds,omitempty"`
	}
	safe := make([]safeConn, len(conns))
	for i, conn := range conns {
		safe[i] = safeConn{IntegrationConnection: conn}
		safe[i].EncryptedCreds = nil
	}

	response.OK(c, safe)
}

// CreateConnection creates a new SaaS connection.
func (h *Handler) CreateConnection(c *gin.Context) {
	var req CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	conn, err := h.svc.CreateConnection(c.Request.Context(), companyID, userID, req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, conn)
}

// GetConnection returns a single connection.
func (h *Handler) GetConnection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid connection ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	conn, err := h.queries.GetIntegrationConnection(c.Request.Context(), store.GetIntegrationConnectionParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Connection not found")
		return
	}

	// Strip credentials
	conn.EncryptedCreds = nil
	response.OK(c, conn)
}

// UpdateConnection updates a connection.
func (h *Handler) UpdateConnection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid connection ID")
		return
	}

	var req struct {
		DisplayName string         `json:"display_name"`
		Status      string         `json:"status"`
		OAuthScope  string         `json:"oauth_scope"`
		Config      map[string]any `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	configJSON, _ := json.Marshal(req.Config)
	if configJSON == nil {
		configJSON = []byte("{}")
	}

	conn, err := h.queries.UpdateIntegrationConnection(c.Request.Context(), store.UpdateIntegrationConnectionParams{
		ID:          id,
		CompanyID:   companyID,
		DisplayName: req.DisplayName,
		Status:      req.Status,
		OauthScope:  req.OAuthScope,
		Config:      configJSON,
	})
	if err != nil {
		response.InternalError(c, "Failed to update connection")
		return
	}

	conn.EncryptedCreds = nil
	response.OK(c, conn)
}

// DeleteConnection removes a connection.
func (h *Handler) DeleteConnection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid connection ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteIntegrationConnection(c.Request.Context(), store.DeleteIntegrationConnectionParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete connection")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// TestConnection tests a connection's health.
func (h *Handler) TestConnection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid connection ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.svc.TestConnection(c.Request.Context(), companyID, id); err != nil {
		response.OK(c, gin.H{"healthy": false, "error": err.Error()})
		return
	}

	response.OK(c, gin.H{"healthy": true})
}

// ListTemplates returns all provisioning templates for the company.
func (h *Handler) ListTemplates(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	templates, err := h.queries.ListProvisioningTemplates(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list templates")
		return
	}
	response.OK(c, templates)
}

// CreateTemplate creates a new provisioning template.
func (h *Handler) CreateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	paramsJSON, _ := json.Marshal(req.Params)
	if paramsJSON == nil {
		paramsJSON = []byte("{}")
	}

	deprovisionMode := req.DeprovisionMode
	if deprovisionMode == "" {
		deprovisionMode = "disable"
	}

	tmpl, err := h.queries.CreateProvisioningTemplate(c.Request.Context(), store.CreateProvisioningTemplateParams{
		CompanyID:            companyID,
		ConnectionID:         req.ConnectionID,
		Provider:             req.Provider,
		EventTrigger:         req.EventTrigger,
		ActionType:           req.ActionType,
		FilterDepartmentID:   req.FilterDepartmentID,
		FilterEmploymentType: req.FilterEmploymentType,
		Params:               paramsJSON,
		DeprovisionMode:      deprovisionMode,
		RequiresApproval:     req.RequiresApproval,
		IsActive:             req.IsActive,
	})
	if err != nil {
		response.InternalError(c, "Failed to create template")
		return
	}

	response.Created(c, tmpl)
}

// UpdateTemplate updates a provisioning template.
func (h *Handler) UpdateTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid template ID")
		return
	}

	var req UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	paramsJSON, _ := json.Marshal(req.Params)
	if paramsJSON == nil {
		paramsJSON = []byte("{}")
	}

	requiresApproval := false
	if req.RequiresApproval != nil {
		requiresApproval = *req.RequiresApproval
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	tmpl, err := h.queries.UpdateProvisioningTemplate(c.Request.Context(), store.UpdateProvisioningTemplateParams{
		ID:                   id,
		CompanyID:            companyID,
		EventTrigger:         req.EventTrigger,
		ActionType:           req.ActionType,
		FilterDepartmentID:   req.FilterDepartmentID,
		FilterEmploymentType: req.FilterEmploymentType,
		Params:               paramsJSON,
		DeprovisionMode:      req.DeprovisionMode,
		RequiresApproval:     requiresApproval,
		IsActive:             isActive,
	})
	if err != nil {
		response.InternalError(c, "Failed to update template")
		return
	}

	response.OK(c, tmpl)
}

// DeleteTemplate removes a provisioning template.
func (h *Handler) DeleteTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid template ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteProvisioningTemplate(c.Request.Context(), store.DeleteProvisioningTemplateParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete template")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListJobs returns provisioning jobs for the company.
func (h *Handler) ListJobs(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	jobs, err := h.queries.ListProvisioningJobs(c.Request.Context(), store.ListProvisioningJobsParams{
		CompanyID: companyID,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list jobs")
		return
	}

	response.OK(c, jobs)
}

// RetryJob retries a failed provisioning job.
func (h *Handler) RetryJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid job ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.RetryProvisioningJob(c.Request.Context(), store.RetryProvisioningJobParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to retry job")
		return
	}

	response.OK(c, gin.H{"status": "retrying"})
}

// SkipJob skips a provisioning job.
func (h *Handler) SkipJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid job ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.SkipProvisioningJob(c.Request.Context(), store.SkipProvisioningJobParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to skip job")
		return
	}

	response.OK(c, gin.H{"status": "skipped"})
}

// ListEmployeeIntegrations returns external account mappings for an employee.
func (h *Handler) ListEmployeeIntegrations(c *gin.Context) {
	empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	identities, err := h.queries.ListEmployeeIntegrations(c.Request.Context(), store.ListEmployeeIntegrationsParams{
		EmployeeID: empID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list integrations")
		return
	}

	response.OK(c, identities)
}

// ListAuditLogs returns integration audit logs.
func (h *Handler) ListAuditLogs(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	logs, err := h.queries.ListIntegrationAuditLogs(c.Request.Context(), store.ListIntegrationAuditLogsParams{
		CompanyID: companyID,
		Limit:     int32(pg.Limit),
		Offset:    int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list audit logs")
		return
	}

	response.OK(c, logs)
}
