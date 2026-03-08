package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/ai/agent"
	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Handler handles AI chat HTTP endpoints.
type Handler struct {
	service      *Service
	executor     *agent.Executor
	registry     *agent.Registry
	toolRegistry *ToolRegistry
	queries      *store.Queries
}

// NewHandler creates an AI handler.
func NewHandler(service *Service, executor *agent.Executor, registry *agent.Registry, toolRegistry *ToolRegistry, queries *store.Queries) *Handler {
	return &Handler{
		service:      service,
		executor:     executor,
		registry:     registry,
		toolRegistry: toolRegistry,
		queries:      queries,
	}
}

// RegisterRoutes adds AI routes to the router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ai := rg.Group("/ai")
	{
		ai.POST("/chat", h.Chat)
		ai.POST("/chat/stream", h.StreamChat)
		ai.POST("/command", h.Command)
		ai.GET("/agents", h.ListAgents)
		ai.GET("/agents/tools", h.ListTools)
		ai.GET("/agents/:slug", h.GetAgent)
		ai.POST("/agents", auth.AdminOnly(), h.CreateAgent)
		ai.PUT("/agents/:slug", auth.AdminOnly(), h.UpdateAgentEndpoint)
		ai.DELETE("/agents/:slug", auth.AdminOnly(), h.DeleteAgent)
		ai.GET("/sessions", h.ListSessions)
		ai.GET("/sessions/:id/messages", h.GetSessionMessages)
		ai.DELETE("/sessions/:id", h.DeleteSession)
		ai.POST("/messages/:id/feedback", h.SubmitFeedback)
		ai.GET("/feedback/stats", auth.AdminOnly(), h.GetFeedbackStats)
		ai.GET("/feedback/recent", auth.AdminOnly(), h.ListRecentFeedback)
		ai.GET("/audit-log", auth.AdminOnly(), h.ListAIAuditLog)
	}
}

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

// Chat handles synchronous chat requests.
func (h *Handler) Chat(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if req.Message == "" {
		response.BadRequest(c, "Message is required")
		return
	}

	// Use executor if available (agent-based with billing)
	if h.executor != nil {
		agentSlug := req.Agent
		if agentSlug == "" {
			agentSlug = "general"
		}

		agentReq := agent.ChatRequest{
			Message:   req.Message,
			SessionID: req.SessionID,
		}

		resp, err := h.executor.Chat(c.Request.Context(), companyID, userID, agentSlug, agentReq)
		if err != nil {
			if errors.Is(err, agent.ErrInsufficientBalance) {
				c.JSON(http.StatusPaymentRequired, gin.H{
					"success": false,
					"error":   "Insufficient token balance. Please purchase more tokens.",
				})
				return
			}
			response.InternalError(c, fmt.Sprintf("AI chat error: %s", err.Error()))
			return
		}

		response.OK(c, resp)
		return
	}

	// Fallback to legacy service (no billing)
	resp, err := h.service.Chat(c.Request.Context(), companyID, userID, req)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("AI chat error: %s", err.Error()))
		return
	}

	response.OK(c, resp)
}

// StreamChat handles SSE streaming chat requests.
func (h *Handler) StreamChat(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if req.Message == "" {
		response.BadRequest(c, "Message is required")
		return
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "Streaming not supported")
		return
	}

	// Use executor if available (agent-based with billing)
	if h.executor != nil {
		agentSlug := req.Agent
		if agentSlug == "" {
			agentSlug = "general"
		}

		agentReq := agent.ChatRequest{
			Message:   req.Message,
			SessionID: req.SessionID,
		}

		resp, err := h.executor.StreamChat(c.Request.Context(), companyID, userID, agentSlug, agentReq,
			func(chunk provider.StreamChunk) {
				switch chunk.Type {
				case "text_delta":
					fmt.Fprintf(c.Writer, "data: {\"type\":\"text\",\"text\":%q}\n\n", chunk.Text)
					flusher.Flush()
				case "tool_use":
					if chunk.ToolCall != nil {
						fmt.Fprintf(c.Writer, "data: {\"type\":\"tool\",\"name\":%q}\n\n", chunk.ToolCall.Name)
						flusher.Flush()
					}
				}
			},
		)

		if err != nil {
			if errors.Is(err, agent.ErrInsufficientBalance) {
				fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"code\":402,\"message\":\"Insufficient token balance\"}\n\n")
			} else {
				fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":%q}\n\n", err.Error())
			}
			flusher.Flush()
		} else if resp != nil {
			// Send final done event with token info, session_id and message_id
			fmt.Fprintf(c.Writer, "data: {\"type\":\"done\",\"tokens_used\":%d,\"agent\":%q,\"session_id\":%q,\"message_id\":%d}\n\n", resp.TokensUsed, resp.Agent, resp.SessionID, resp.MessageID)
			flusher.Flush()
		}

		_, _ = io.WriteString(c.Writer, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	// Fallback to legacy service
	_, err := h.service.StreamChat(c.Request.Context(), companyID, userID, req,
		func(chunk provider.StreamChunk) {
			switch chunk.Type {
			case "text_delta":
				fmt.Fprintf(c.Writer, "data: {\"type\":\"text\",\"text\":%q}\n\n", chunk.Text)
				flusher.Flush()
			case "tool_use":
				if chunk.ToolCall != nil {
					fmt.Fprintf(c.Writer, "data: {\"type\":\"tool\",\"name\":%q}\n\n", chunk.ToolCall.Name)
					flusher.Flush()
				}
			case "message_stop":
				fmt.Fprintf(c.Writer, "data: {\"type\":\"done\"}\n\n")
				flusher.Flush()
			}
		},
	)

	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":%q}\n\n", err.Error())
		flusher.Flush()
	}

	_, _ = io.WriteString(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// ListSessions returns the user's chat sessions.
func (h *Handler) ListSessions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	sessions, err := h.queries.ListUserChatSessions(c.Request.Context(), store.ListUserChatSessionsParams{
		UserID:    userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list sessions")
		return
	}

	response.OK(c, sessions)
}

// GetSessionMessages returns messages for a specific session.
func (h *Handler) GetSessionMessages(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	sessionIDStr := c.Param("id")

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid session ID")
		return
	}

	// Verify session belongs to user's company
	_, err = h.queries.GetChatSession(c.Request.Context(), store.GetChatSessionParams{
		ID:        sessionID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Session not found")
		return
	}

	messages, err := h.queries.ListChatMessages(c.Request.Context(), sessionID)
	if err != nil {
		response.InternalError(c, "Failed to list messages")
		return
	}

	response.OK(c, messages)
}

// DeleteSession deletes a chat session.
func (h *Handler) DeleteSession(c *gin.Context) {
	userID := auth.GetUserID(c)
	sessionIDStr := c.Param("id")

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid session ID")
		return
	}

	if err := h.queries.DeleteChatSession(c.Request.Context(), store.DeleteChatSessionParams{
		ID:     sessionID,
		UserID: userID,
	}); err != nil {
		response.InternalError(c, "Failed to delete session")
		return
	}

	response.OK(c, gin.H{"deleted": true})
}

// feedbackRequest is the JSON body for submitting feedback.
type feedbackRequest struct {
	Rating  string  `json:"rating" binding:"required"`
	Comment *string `json:"comment"`
}

// SubmitFeedback creates or updates feedback for a chat message.
func (h *Handler) SubmitFeedback(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	messageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid message ID")
		return
	}

	var req feedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if req.Rating != "positive" && req.Rating != "negative" {
		response.BadRequest(c, "Rating must be 'positive' or 'negative'")
		return
	}

	feedback, err := h.queries.InsertChatFeedback(c.Request.Context(), store.InsertChatFeedbackParams{
		MessageID: messageID,
		CompanyID: companyID,
		UserID:    userID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	})
	if err != nil {
		response.InternalError(c, "Failed to save feedback")
		return
	}

	response.OK(c, feedback)
}

// GetFeedbackStats returns aggregated feedback counts by rating (admin only).
func (h *Handler) GetFeedbackStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	stats, err := h.queries.GetFeedbackStats(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get feedback stats")
		return
	}

	response.OK(c, stats)
}

// ListRecentFeedback returns paginated recent feedback with message content (admin only).
func (h *Handler) ListRecentFeedback(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	feedbackList, err := h.queries.ListRecentFeedback(c.Request.Context(), store.ListRecentFeedbackParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list feedback")
		return
	}

	response.OK(c, feedbackList)
}

// ListAIAuditLog returns paginated AI audit logs (admin only).
func (h *Handler) ListAIAuditLog(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	logs, err := h.queries.ListAIAuditLogs(c.Request.Context(), store.ListAIAuditLogsParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list AI audit logs")
		return
	}

	response.OK(c, logs)
}

// commandRequest is the JSON body for the Command Palette endpoint.
type commandRequest struct {
	Query string `json:"query" binding:"required"`
}

// CommandAction represents a follow-up action the UI can offer.
type CommandAction struct {
	Label  string         `json:"label"`
	Route  string         `json:"route,omitempty"`
	Action string         `json:"action,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// CommandResult is the structured response from the Command Palette.
type CommandResult struct {
	Type    string          `json:"type"`
	Title   string          `json:"title"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
	Actions []CommandAction `json:"actions,omitempty"`
}

// commandSystemPromptWrapper wraps the user query with instructions for
// structured command-palette output. The AI is told to respond with JSON
// so we can parse it into a CommandResult.
const commandSystemPromptWrapper = `You are a Command Palette assistant for an HR system. The user typed a quick command.
Your job: identify the intent, execute the right tool, and return a CONCISE structured result.

RULES:
- Identify intent: "action" (approve, reject, create), "query" (look up data), "info" (general help), or "navigation" (go to page).
- Execute the appropriate tool if needed.
- Respond with ONLY a JSON object (no markdown fences, no extra text) in this exact format:
{
  "type": "action|query|info|navigation",
  "title": "Short title of what happened",
  "message": "One-sentence summary of the result",
  "data": {},
  "actions": [
    {"label": "Button Label", "route": "/path/to/page"},
    {"label": "Action Label", "action": "action_name", "params": {"key": "value"}}
  ]
}

GUIDELINES for "actions" array:
- For query results: include a "View Details" action with a route
- For completed actions: include "View Details" and optionally "Undo" if reversible
- For navigation: include the target route
- For info: include relevant page links
- Keep actions to 1-3 items max

Common routes: /employees, /employees/:id, /leaves, /leaves/:id, /attendance, /payroll, /payslips, /approvals, /dashboard

Be brief and direct. This is a command palette, not a conversation.`

// Command handles one-shot Command Palette requests.
// It reuses the agent executor with a command-specific prompt wrapper,
// then parses the AI response into structured ActionCard data.
func (h *Handler) Command(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req commandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		response.BadRequest(c, "Query is required")
		return
	}

	if h.executor == nil {
		response.InternalError(c, "AI executor not configured")
		return
	}

	// Build the command message: embed the structured-output instructions
	// in the user message so the existing executor uses them alongside
	// the general agent's system prompt.
	commandMessage := fmt.Sprintf(
		"[COMMAND PALETTE MODE]\n%s\n\nUser command: %s",
		commandSystemPromptWrapper,
		req.Query,
	)

	agentReq := agent.ChatRequest{
		Message: commandMessage,
		// No SessionID — commands are one-shot, no session persistence
	}

	resp, err := h.executor.Chat(c.Request.Context(), companyID, userID, "general", agentReq)
	if err != nil {
		if errors.Is(err, agent.ErrInsufficientBalance) {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"success": false,
				"error":   "Insufficient token balance. Please purchase more tokens.",
			})
			return
		}
		response.InternalError(c, fmt.Sprintf("Command execution error: %s", err.Error()))
		return
	}

	// Parse the AI response into structured CommandResult
	result := parseCommandResponse(resp.Message)
	result.Data = ensureValidJSON(result.Data)

	response.OK(c, gin.H{
		"result":      result,
		"tokens_used": resp.TokensUsed,
	})
}

// parseCommandResponse attempts to extract a CommandResult from the AI's
// text response. It handles cases where the AI wraps JSON in markdown
// fences or includes extra text around the JSON.
func parseCommandResponse(raw string) CommandResult {
	cleaned := extractJSON(raw)

	var result CommandResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		// Fallback: return the raw text as an info response
		return CommandResult{
			Type:    "info",
			Title:   "Result",
			Message: raw,
		}
	}

	// Validate the type field
	switch result.Type {
	case "action", "query", "info", "navigation":
		// valid
	default:
		result.Type = "info"
	}

	return result
}

// extractJSON finds the first JSON object in the text, handling markdown
// code fences and surrounding prose.
func extractJSON(s string) string {
	// Strip markdown code fences if present
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence (```json or ```)
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	// Find the outermost { ... } pair
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	// Unbalanced braces — return from start to end
	return s[start:]
}

// ensureValidJSON returns the input if it's valid JSON, or null otherwise.
// This prevents invalid data from being serialized into the response.
func ensureValidJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if json.Valid(raw) {
		return raw
	}
	return nil
}
