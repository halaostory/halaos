package workflow

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
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

// --- Trigger CRUD ---

// ListTriggers returns all workflow triggers for the company.
func (h *Handler) ListTriggers(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	triggers, err := h.queries.ListWorkflowTriggers(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list triggers")
		return
	}
	response.OK(c, triggers)
}

// CreateTrigger creates a new workflow trigger.
func (h *Handler) CreateTrigger(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req struct {
		Name          string          `json:"name" binding:"required"`
		Description   *string         `json:"description"`
		TriggerType   string          `json:"trigger_type" binding:"required"`
		EntityType    string          `json:"entity_type" binding:"required"`
		TriggerConfig json.RawMessage `json:"trigger_config"`
		ActionType    string          `json:"action_type" binding:"required"`
		ActionConfig  json.RawMessage `json:"action_config"`
		Priority      int32           `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Priority == 0 {
		req.Priority = 100
	}
	if req.TriggerConfig == nil {
		req.TriggerConfig = json.RawMessage(`{}`)
	}
	if req.ActionConfig == nil {
		req.ActionConfig = json.RawMessage(`{}`)
	}

	trigger, err := h.queries.CreateWorkflowTrigger(c.Request.Context(), store.CreateWorkflowTriggerParams{
		CompanyID:     companyID,
		Name:          req.Name,
		Description:   req.Description,
		TriggerType:   req.TriggerType,
		EntityType:    req.EntityType,
		TriggerConfig: req.TriggerConfig,
		ActionType:    req.ActionType,
		ActionConfig:  req.ActionConfig,
		Priority:      req.Priority,
		CreatedBy:     &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to create trigger")
		return
	}
	response.Created(c, trigger)
}

// UpdateTrigger updates an existing workflow trigger.
func (h *Handler) UpdateTrigger(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid trigger ID")
		return
	}

	var req struct {
		Name          string          `json:"name" binding:"required"`
		Description   *string         `json:"description"`
		TriggerType   string          `json:"trigger_type" binding:"required"`
		EntityType    string          `json:"entity_type" binding:"required"`
		TriggerConfig json.RawMessage `json:"trigger_config"`
		ActionType    string          `json:"action_type" binding:"required"`
		ActionConfig  json.RawMessage `json:"action_config"`
		Priority      int32           `json:"priority"`
		IsActive      bool            `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.TriggerConfig == nil {
		req.TriggerConfig = json.RawMessage(`{}`)
	}
	if req.ActionConfig == nil {
		req.ActionConfig = json.RawMessage(`{}`)
	}

	trigger, err := h.queries.UpdateWorkflowTrigger(c.Request.Context(), store.UpdateWorkflowTriggerParams{
		ID:            id,
		CompanyID:     companyID,
		Name:          req.Name,
		Description:   req.Description,
		TriggerType:   req.TriggerType,
		EntityType:    req.EntityType,
		TriggerConfig: req.TriggerConfig,
		ActionType:    req.ActionType,
		ActionConfig:  req.ActionConfig,
		Priority:      req.Priority,
		IsActive:      req.IsActive,
	})
	if err != nil {
		response.NotFound(c, "Trigger not found")
		return
	}
	response.OK(c, trigger)
}

// DeactivateTrigger deactivates a workflow trigger.
func (h *Handler) DeactivateTrigger(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid trigger ID")
		return
	}

	if err := h.queries.DeactivateWorkflowTrigger(c.Request.Context(), store.DeactivateWorkflowTriggerParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.NotFound(c, "Trigger not found")
		return
	}
	response.OK(c, gin.H{"message": "Trigger deactivated"})
}

// --- Decision Endpoints ---

// ListDecisions returns AI decision history with pagination and filters.
func (h *Handler) ListDecisions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	entityType := c.DefaultQuery("entity_type", "")
	var entityID int64
	if eid := c.Query("entity_id"); eid != "" {
		entityID, _ = strconv.ParseInt(eid, 10, 64)
	}
	onlyOverridden := c.Query("only_overridden") == "true"

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize > 100 {
		pageSize = 100
	}

	decisions, err := h.queries.ListWorkflowDecisions(c.Request.Context(), store.ListWorkflowDecisionsParams{
		CompanyID: companyID,
		Column2:   entityType,
		Column3:   entityID,
		Column4:   onlyOverridden,
		Limit:     int32(pageSize),
		Offset:    int32((page - 1) * pageSize),
	})
	if err != nil {
		response.InternalError(c, "Failed to list decisions")
		return
	}

	total, err := h.queries.CountWorkflowDecisions(c.Request.Context(), store.CountWorkflowDecisionsParams{
		CompanyID: companyID,
		Column2:   entityType,
		Column3:   entityID,
		Column4:   onlyOverridden,
	})
	if err != nil {
		total = 0
	}

	response.Paginated(c, decisions, total, page, pageSize)
}

// GetDecision returns a single decision by ID.
func (h *Handler) GetDecision(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid decision ID")
		return
	}

	decision, err := h.queries.GetWorkflowDecision(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Decision not found")
		return
	}

	// Verify company access
	companyID := auth.GetCompanyID(c)
	if decision.CompanyID != companyID {
		response.NotFound(c, "Decision not found")
		return
	}

	response.OK(c, decision)
}

// OverrideDecision records a manager's override of an AI decision.
func (h *Handler) OverrideDecision(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid decision ID")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	// Verify decision exists and belongs to company
	decision, err := h.queries.GetWorkflowDecision(c.Request.Context(), id)
	if err != nil || decision.CompanyID != companyID {
		response.NotFound(c, "Decision not found")
		return
	}

	var req struct {
		Action string  `json:"action" binding:"required"`
		Reason *string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.queries.RecordDecisionOverride(c.Request.Context(), store.RecordDecisionOverrideParams{
		ID:             id,
		OverriddenBy:   &userID,
		OverrideAction: &req.Action,
		OverrideReason: req.Reason,
	}); err != nil {
		response.InternalError(c, "Failed to record override")
		return
	}

	response.OK(c, gin.H{"message": "Override recorded"})
}
