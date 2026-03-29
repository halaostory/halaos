package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/ai/agent"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/pkg/response"
)

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
