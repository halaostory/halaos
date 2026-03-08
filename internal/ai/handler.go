package ai

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tonypk/aigonhr/internal/ai/agent"
	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Handler handles AI chat HTTP endpoints.
type Handler struct {
	service  *Service
	executor *agent.Executor
	registry *agent.Registry
	queries  *store.Queries
}

// NewHandler creates an AI handler.
func NewHandler(service *Service, executor *agent.Executor, registry *agent.Registry, queries *store.Queries) *Handler {
	return &Handler{
		service:  service,
		executor: executor,
		registry: registry,
		queries:  queries,
	}
}

// RegisterRoutes adds AI routes to the router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ai := rg.Group("/ai")
	{
		ai.POST("/chat", h.Chat)
		ai.POST("/chat/stream", h.StreamChat)
		ai.GET("/agents", h.ListAgents)
		ai.GET("/agents/:slug", h.GetAgent)
		ai.GET("/sessions", h.ListSessions)
		ai.GET("/sessions/:id/messages", h.GetSessionMessages)
		ai.DELETE("/sessions/:id", h.DeleteSession)
	}
}

// ListAgents returns all active agents.
func (h *Handler) ListAgents(c *gin.Context) {
	agents := h.registry.List(c.Request.Context())

	type agentResponse struct {
		Slug           string   `json:"slug"`
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		Tools          []string `json:"tools"`
		CostMultiplier float64  `json:"cost_multiplier"`
		IsAutonomous   bool     `json:"is_autonomous"`
		MaxRounds      int      `json:"max_rounds"`
		Icon           string   `json:"icon"`
		IsActive       bool     `json:"is_active"`
	}

	result := make([]agentResponse, len(agents))
	for i, a := range agents {
		result[i] = agentResponse{
			Slug:           a.Slug,
			Name:           a.Name,
			Description:    a.Description,
			Tools:          a.Tools,
			CostMultiplier: a.CostMultiplier,
			IsAutonomous:   a.IsAutonomous,
			MaxRounds:      a.MaxRounds,
			Icon:           a.Icon,
			IsActive:       true,
		}
	}

	response.OK(c, result)
}

// GetAgent returns a single agent by slug.
func (h *Handler) GetAgent(c *gin.Context) {
	slug := c.Param("slug")

	cfg, ok := h.registry.Get(c.Request.Context(), slug)
	if !ok {
		response.NotFound(c, "Agent not found")
		return
	}

	response.OK(c, gin.H{
		"slug":            cfg.Slug,
		"name":            cfg.Name,
		"description":     cfg.Description,
		"tools":           cfg.Tools,
		"cost_multiplier": cfg.CostMultiplier,
		"is_autonomous":   cfg.IsAutonomous,
		"max_rounds":      cfg.MaxRounds,
		"icon":            cfg.Icon,
	})
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
			// Send final done event with token info and session_id
			fmt.Fprintf(c.Writer, "data: {\"type\":\"done\",\"tokens_used\":%d,\"agent\":%q,\"session_id\":%q}\n\n", resp.TokensUsed, resp.Agent, resp.SessionID)
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
