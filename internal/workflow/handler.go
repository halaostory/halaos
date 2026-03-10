package workflow

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Handler handles workflow rule CRUD operations.
type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewHandler creates a new workflow handler.
func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// List returns all workflow rules for the company.
func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	rules, err := h.queries.ListWorkflowRules(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list workflow rules")
		return
	}
	response.OK(c, rules)
}

// Create creates a new workflow rule.
func (h *Handler) Create(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Name        string          `json:"name" binding:"required"`
		Description *string         `json:"description"`
		EntityType  string          `json:"entity_type" binding:"required"`
		RuleType    string          `json:"rule_type" binding:"required"`
		Conditions  json.RawMessage `json:"conditions" binding:"required"`
		Priority    int32           `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Priority == 0 {
		req.Priority = 100
	}

	rule, err := h.queries.CreateWorkflowRule(c.Request.Context(), store.CreateWorkflowRuleParams{
		CompanyID:   companyID,
		Name:        req.Name,
		Description: req.Description,
		EntityType:  req.EntityType,
		RuleType:    req.RuleType,
		Conditions:  req.Conditions,
		Priority:    req.Priority,
		CreatedBy:   &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to create workflow rule")
		return
	}
	response.Created(c, rule)
}

// Update updates an existing workflow rule.
func (h *Handler) Update(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid rule ID")
		return
	}

	var req struct {
		Name        string          `json:"name" binding:"required"`
		Description *string         `json:"description"`
		EntityType  string          `json:"entity_type" binding:"required"`
		RuleType    string          `json:"rule_type" binding:"required"`
		Conditions  json.RawMessage `json:"conditions" binding:"required"`
		Priority    int32           `json:"priority"`
		IsActive    bool            `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rule, err := h.queries.UpdateWorkflowRule(c.Request.Context(), store.UpdateWorkflowRuleParams{
		ID:          id,
		CompanyID:   companyID,
		Name:        req.Name,
		Description: req.Description,
		EntityType:  req.EntityType,
		RuleType:    req.RuleType,
		Conditions:  req.Conditions,
		Priority:    req.Priority,
		IsActive:    req.IsActive,
	})
	if err != nil {
		response.NotFound(c, "Workflow rule not found")
		return
	}
	response.OK(c, rule)
}

// Deactivate soft-deletes a workflow rule.
func (h *Handler) Deactivate(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid rule ID")
		return
	}

	if err := h.queries.DeactivateWorkflowRule(c.Request.Context(), store.DeactivateWorkflowRuleParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.NotFound(c, "Workflow rule not found")
		return
	}
	response.OK(c, gin.H{"message": "Rule deactivated"})
}

// ListExecutions returns rule execution history.
func (h *Handler) ListExecutions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var ruleID int64
	if idStr := c.Param("id"); idStr != "" {
		ruleID, _ = strconv.ParseInt(idStr, 10, 64)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize > 100 {
		pageSize = 100
	}

	executions, err := h.queries.ListRuleExecutions(c.Request.Context(), store.ListRuleExecutionsParams{
		CompanyID: companyID,
		Column2:   ruleID,
		Limit:     int32(pageSize),
		Offset:    int32((page - 1) * pageSize),
	})
	if err != nil {
		response.InternalError(c, "Failed to list executions")
		return
	}

	total, err := h.queries.CountRuleExecutions(c.Request.Context(), store.CountRuleExecutionsParams{
		CompanyID: companyID,
		Column2:   ruleID,
	})
	if err != nil {
		total = 0
	}

	response.Paginated(c, executions, total, page, pageSize)
}

// ListSLAConfigs returns SLA configurations for the company.
func (h *Handler) ListSLAConfigs(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	configs, err := h.queries.ListSLAConfigs(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list SLA configs")
		return
	}
	response.OK(c, configs)
}

// UpsertSLAConfig creates or updates an SLA config.
func (h *Handler) UpsertSLAConfig(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var req struct {
		EntityType         string `json:"entity_type" binding:"required"`
		ReminderAfterHours int32  `json:"reminder_after_hours"`
		SecondReminderHours int32 `json:"second_reminder_hours"`
		EscalateAfterHours int32  `json:"escalate_after_hours"`
		AutoActionHours    int32  `json:"auto_action_hours"`
		AutoAction         string `json:"auto_action"`
		EscalationRole     string `json:"escalation_role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.ReminderAfterHours == 0 {
		req.ReminderAfterHours = 12
	}
	if req.SecondReminderHours == 0 {
		req.SecondReminderHours = 24
	}
	if req.EscalateAfterHours == 0 {
		req.EscalateAfterHours = 48
	}
	if req.AutoActionHours == 0 {
		req.AutoActionHours = 72
	}
	if req.AutoAction == "" {
		req.AutoAction = "approve"
	}
	if req.EscalationRole == "" {
		req.EscalationRole = "admin"
	}

	config, err := h.queries.UpsertSLAConfig(c.Request.Context(), store.UpsertSLAConfigParams{
		CompanyID:           companyID,
		EntityType:          req.EntityType,
		ReminderAfterHours:  req.ReminderAfterHours,
		SecondReminderHours: req.SecondReminderHours,
		EscalateAfterHours:  req.EscalateAfterHours,
		AutoActionHours:     req.AutoActionHours,
		AutoAction:          req.AutoAction,
		EscalationRole:      req.EscalationRole,
	})
	if err != nil {
		response.InternalError(c, "Failed to save SLA config")
		return
	}
	c.JSON(http.StatusOK, config)
}
