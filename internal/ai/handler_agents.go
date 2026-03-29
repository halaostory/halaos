package ai

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/agent"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// agentResponse is the JSON shape returned for agent list/detail endpoints.
type agentResponse struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Tools          []string `json:"tools"`
	CostMultiplier float64  `json:"cost_multiplier"`
	IsAutonomous   bool     `json:"is_autonomous"`
	MaxRounds      int      `json:"max_rounds"`
	MaxTokens      int      `json:"max_tokens"`
	Icon           string   `json:"icon"`
	Model          string   `json:"model"`
	IsActive       bool     `json:"is_active"`
	IsSystem       bool     `json:"is_system"`
	CompanyID      int64    `json:"company_id"`
}

func toAgentResponse(a agent.AgentConfig) agentResponse {
	return agentResponse{
		Slug:           a.Slug,
		Name:           a.Name,
		Description:    a.Description,
		Tools:          a.Tools,
		CostMultiplier: a.CostMultiplier,
		IsAutonomous:   a.IsAutonomous,
		MaxRounds:      a.MaxRounds,
		MaxTokens:      a.MaxTokens,
		Icon:           a.Icon,
		Model:          a.Model,
		IsActive:       true,
		IsSystem:       a.CompanyID == 0,
		CompanyID:      a.CompanyID,
	}
}

// ListAgents returns active agents visible to the user's company.
func (h *Handler) ListAgents(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	agents := h.registry.ListForCompany(c.Request.Context(), companyID)

	result := make([]agentResponse, len(agents))
	for i, a := range agents {
		result[i] = toAgentResponse(a)
	}

	response.OK(c, result)
}

// agentDetailResponse extends agentResponse with system_prompt for editing.
type agentDetailResponse struct {
	agentResponse
	SystemPrompt string `json:"system_prompt"`
}

// GetAgent returns a single agent by slug (includes system_prompt for editing).
func (h *Handler) GetAgent(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	slug := c.Param("slug")

	cfg, ok := h.registry.GetForCompany(c.Request.Context(), slug, companyID)
	if !ok {
		response.NotFound(c, "Agent not found")
		return
	}

	response.OK(c, agentDetailResponse{
		agentResponse: toAgentResponse(cfg),
		SystemPrompt:  cfg.SystemPrompt,
	})
}

// createAgentRequest is the JSON body for creating an agent.
type createAgentRequest struct {
	Slug           string   `json:"slug" binding:"required,min=2,max=50"`
	Name           string   `json:"name" binding:"required,min=1,max=100"`
	Description    string   `json:"description"`
	SystemPrompt   string   `json:"system_prompt"`
	Tools          []string `json:"tools"`
	CostMultiplier float64  `json:"cost_multiplier"`
	IsAutonomous   bool     `json:"is_autonomous"`
	MaxRounds      int32    `json:"max_rounds"`
	MaxTokens      int32    `json:"max_tokens"`
	Icon           string   `json:"icon"`
	Model          string   `json:"model"`
}

// CreateAgent creates a new company-specific agent.
func (h *Handler) CreateAgent(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	var req createAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Validate slug format (alphanumeric + hyphens)
	for _, ch := range req.Slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			response.BadRequest(c, "Slug must contain only lowercase letters, numbers, and hyphens")
			return
		}
	}

	// Defaults
	if req.CostMultiplier <= 0 {
		req.CostMultiplier = 1.0
	}
	if req.MaxRounds <= 0 {
		req.MaxRounds = 5
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 4096
	}
	if req.Tools == nil {
		req.Tools = []string{}
	}

	costNum := pgtype.Numeric{}
	if err := costNum.Scan(fmt.Sprintf("%.2f", req.CostMultiplier)); err != nil {
		response.BadRequest(c, "Invalid cost multiplier")
		return
	}

	created, err := h.queries.CreateAgent(c.Request.Context(), store.CreateAgentParams{
		CompanyID:      &companyID,
		Slug:           req.Slug,
		Name:           req.Name,
		Description:    req.Description,
		SystemPrompt:   req.SystemPrompt,
		Tools:          req.Tools,
		CostMultiplier: costNum,
		IsAutonomous:   req.IsAutonomous,
		MaxRounds:      req.MaxRounds,
		MaxTokens:      req.MaxTokens,
		Icon:           req.Icon,
		Model:          req.Model,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			response.BadRequest(c, "An agent with this slug already exists")
			return
		}
		response.InternalError(c, "Failed to create agent")
		return
	}

	// Invalidate registry cache so the new agent appears immediately
	h.registry.InvalidateCache()

	response.Created(c, toAgentResponseFromDB(created))
}

// updateAgentRequest is the JSON body for updating an agent.
type updateAgentRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SystemPrompt   string   `json:"system_prompt"`
	Tools          []string `json:"tools"`
	CostMultiplier float64  `json:"cost_multiplier"`
	IsAutonomous   bool     `json:"is_autonomous"`
	MaxRounds      int32    `json:"max_rounds"`
	MaxTokens      int32    `json:"max_tokens"`
	Icon           string   `json:"icon"`
	Model          string   `json:"model"`
}

// UpdateAgentEndpoint updates an existing agent.
func (h *Handler) UpdateAgentEndpoint(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	slug := c.Param("slug")

	// Verify agent exists and belongs to this company
	existing, err := h.queries.GetAgentBySlug(c.Request.Context(), slug)
	if err != nil {
		response.NotFound(c, "Agent not found")
		return
	}
	if existing.CompanyID == nil {
		response.BadRequest(c, "Cannot modify system agents")
		return
	}
	if *existing.CompanyID != companyID {
		response.NotFound(c, "Agent not found")
		return
	}

	var req updateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Tools == nil {
		req.Tools = []string{}
	}
	if req.CostMultiplier <= 0 {
		req.CostMultiplier = 1.0
	}
	if req.MaxRounds <= 0 {
		req.MaxRounds = 5
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 4096
	}

	costNum := pgtype.Numeric{}
	if err := costNum.Scan(fmt.Sprintf("%.2f", req.CostMultiplier)); err != nil {
		response.BadRequest(c, "Invalid cost multiplier")
		return
	}

	updated, err := h.queries.UpdateAgent(c.Request.Context(), store.UpdateAgentParams{
		Slug:           slug,
		Name:           req.Name,
		Description:    req.Description,
		SystemPrompt:   req.SystemPrompt,
		Tools:          req.Tools,
		CostMultiplier: costNum,
		IsAutonomous:   req.IsAutonomous,
		MaxRounds:      req.MaxRounds,
		MaxTokens:      req.MaxTokens,
		Icon:           req.Icon,
		Model:          req.Model,
	})
	if err != nil {
		response.InternalError(c, "Failed to update agent")
		return
	}

	h.registry.InvalidateCache()
	response.OK(c, toAgentResponseFromDB(updated))
}

// DeleteAgent soft-deletes (deactivates) a company-specific agent.
func (h *Handler) DeleteAgent(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	slug := c.Param("slug")

	// Verify agent exists and belongs to this company
	existing, err := h.queries.GetAgentBySlug(c.Request.Context(), slug)
	if err != nil {
		response.NotFound(c, "Agent not found")
		return
	}
	if existing.CompanyID == nil {
		response.BadRequest(c, "Cannot delete system agents")
		return
	}
	if *existing.CompanyID != companyID {
		response.NotFound(c, "Agent not found")
		return
	}

	if err := h.queries.DeactivateAgent(c.Request.Context(), store.DeactivateAgentParams{
		Slug:      slug,
		CompanyID: &companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete agent")
		return
	}

	h.registry.InvalidateCache()
	response.OK(c, gin.H{"deleted": true})
}

// ListTools returns all available tool definitions for the agent builder.
func (h *Handler) ListTools(c *gin.Context) {
	if h.toolRegistry == nil {
		response.OK(c, []any{})
		return
	}

	defs := h.toolRegistry.Definitions()
	type toolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	result := make([]toolInfo, len(defs))
	for i, d := range defs {
		result[i] = toolInfo{
			Name:        d.Name,
			Description: d.Description,
		}
	}
	response.OK(c, result)
}

// toAgentResponseFromDB converts a store.Agent to an agentResponse.
func toAgentResponseFromDB(a store.Agent) agentResponse {
	var companyID int64
	if a.CompanyID != nil {
		companyID = *a.CompanyID
	}
	return agentResponse{
		Slug:           a.Slug,
		Name:           a.Name,
		Description:    a.Description,
		Tools:          a.Tools,
		CostMultiplier: numericToFloat64(a.CostMultiplier),
		IsAutonomous:   a.IsAutonomous,
		MaxRounds:      int(a.MaxRounds),
		MaxTokens:      int(a.MaxTokens),
		Icon:           a.Icon,
		Model:          a.Model,
		IsActive:       a.IsActive,
		IsSystem:       a.CompanyID == nil,
		CompanyID:      companyID,
	}
}

// numericToFloat64 converts pgtype.Numeric to float64.
func numericToFloat64(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 1.0
	}
	return f.Float64
}
