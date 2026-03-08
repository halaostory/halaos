package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/ai/redact"
	"github.com/tonypk/aigonhr/internal/store"
)

// ToolRegistry is the interface for executing tools.
type ToolRegistry interface {
	Definitions() []provider.ToolDefinition
	DefinitionsForAgent(allowedTools []string) []provider.ToolDefinition
	Execute(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error)
}

// BillingService is the interface for token billing operations.
type BillingService interface {
	CheckBalance(ctx context.Context, companyID int64) (int64, error)
	CalculateTokenCost(inputTokens, outputTokens int, costMultiplier float64) int64
	DeductTokens(ctx context.Context, companyID, userID, amount int64, agentSlug, desc string) error
}

// ChatRequest is the input for an agent chat interaction.
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse is the result of an agent chat interaction.
type ChatResponse struct {
	RequestID  string `json:"request_id"`
	Message    string `json:"message"`
	SessionID  string `json:"session_id"`
	TokensUsed int64  `json:"tokens_used"`
	Agent      string `json:"agent"`
	MessageID  int64  `json:"message_id,omitempty"`
}

// ErrInsufficientBalance is returned when the company lacks token balance.
var ErrInsufficientBalance = fmt.Errorf("insufficient token balance")

const defaultAgentSlug = "general"

// Executor runs an agent with billing integration.
type Executor struct {
	provider provider.Provider
	tools    ToolRegistry
	billing  BillingService
	registry *Registry
	queries  *store.Queries
	redactor *redact.FieldRedactor
	logger   *slog.Logger
}

// NewExecutor creates an agent executor.
func NewExecutor(
	p provider.Provider,
	tools ToolRegistry,
	billing BillingService,
	registry *Registry,
	queries *store.Queries,
	logger *slog.Logger,
) *Executor {
	return &Executor{
		provider: p,
		tools:    tools,
		billing:  billing,
		registry: registry,
		queries:  queries,
		redactor: redact.NewFieldRedactor(),
		logger:   logger,
	}
}

// Chat performs a synchronous chat using the specified agent with billing.
func (e *Executor) Chat(ctx context.Context, companyID, userID int64, agentSlug string, req ChatRequest) (*ChatResponse, error) {
	agentCfg, err := e.resolveAgent(ctx, agentSlug)
	if err != nil {
		return nil, err
	}

	// Pre-check balance before calling LLM
	balance, err := e.billing.CheckBalance(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("check balance: %w", err)
	}
	if balance <= 0 {
		return nil, ErrInsufficientBalance
	}

	requestID := uuid.New().String()
	start := time.Now()
	redactedInput := redact.RedactText(req.Message)

	// Resolve or create session
	sessionID, isNewSession := e.resolveSession(ctx, companyID, userID, agentCfg.Slug, req)

	// Load history + append current user message
	messages := e.loadSessionMessages(ctx, sessionID, req.Message)

	toolDefs := e.tools.DefinitionsForAgent(agentCfg.Tools)

	var finalResponse string
	var totalInput, totalOutput int

	for round := 0; round < agentCfg.MaxRounds; round++ {
		resp, err := e.provider.Generate(ctx, provider.Request{
			System:    agentCfg.SystemPrompt,
			Messages:  messages,
			Tools:     toolDefs,
			MaxTokens: agentCfg.MaxTokens,
			Model:     agentCfg.Model,
		})
		if err != nil {
			return nil, fmt.Errorf("llm generate: %w", err)
		}

		totalInput += resp.Usage.InputTokens
		totalOutput += resp.Usage.OutputTokens

		if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 {
			messages = append(messages, provider.Message{
				Role:      provider.RoleAssistant,
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			for _, tc := range resp.ToolCalls {
				e.logger.Info("executing tool", "agent", agentCfg.Slug, "name", tc.Name, "input", tc.Input)

				result, execErr := e.tools.Execute(ctx, tc.Name, companyID, userID, tc.Input)
				if execErr != nil {
					result = fmt.Sprintf("Error: %s", execErr.Error())
				}

				messages = append(messages, provider.Message{
					Role: provider.RoleUser,
					Tool: &provider.ToolResult{
						ToolUseID: tc.ID,
						Content:   result,
						IsError:   execErr != nil,
					},
				})
			}
			continue
		}

		finalResponse = resp.Content
		break
	}

	// Calculate and deduct tokens
	tokenCost := e.billing.CalculateTokenCost(totalInput, totalOutput, agentCfg.CostMultiplier)
	desc := fmt.Sprintf("AI chat with %s agent", agentCfg.Slug)
	if err := e.billing.DeductTokens(ctx, companyID, userID, tokenCost, agentCfg.Slug, desc); err != nil {
		return nil, ErrInsufficientBalance
	}

	latency := time.Since(start)

	// Save messages to session
	e.saveSessionMessages(ctx, sessionID, req.Message, finalResponse, int32(tokenCost), isNewSession)

	// Audit log
	redactedOutput := redact.RedactText(finalResponse)
	promptHash := fmt.Sprintf("%x", sha256.Sum256([]byte(redactedInput)))

	e.writeAuditLog(ctx, companyID, userID, requestID, sessionID,
		"agent.chat", agentCfg.Slug, promptHash, redactedInput, redactedOutput,
		totalInput, totalOutput, int(latency.Milliseconds()))

	return &ChatResponse{
		RequestID:  requestID,
		Message:    finalResponse,
		SessionID:  sessionID,
		TokensUsed: tokenCost,
		Agent:      agentCfg.Slug,
	}, nil
}

// StreamChat performs a streaming agent chat with billing.
// The onChunk callback receives text chunks for SSE forwarding.
func (e *Executor) StreamChat(ctx context.Context, companyID, userID int64, agentSlug string, req ChatRequest, onChunk func(provider.StreamChunk)) (*ChatResponse, error) {
	agentCfg, err := e.resolveAgent(ctx, agentSlug)
	if err != nil {
		return nil, err
	}

	// Pre-check balance before calling LLM
	balance, err := e.billing.CheckBalance(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("check balance: %w", err)
	}
	if balance <= 0 {
		return nil, ErrInsufficientBalance
	}

	requestID := uuid.New().String()
	start := time.Now()
	redactedInput := redact.RedactText(req.Message)

	// Resolve or create session
	sessionID, isNewSession := e.resolveSession(ctx, companyID, userID, agentCfg.Slug, req)

	// Load history + append current user message
	messages := e.loadSessionMessages(ctx, sessionID, req.Message)

	toolDefs := e.tools.DefinitionsForAgent(agentCfg.Tools)

	var totalInput, totalOutput int
	var finalText string

	for round := 0; round < agentCfg.MaxRounds; round++ {
		isLastRound := round == agentCfg.MaxRounds-1

		var resp *provider.Response

		if round == 0 {
			// First round: always stream for user-visible output
			var err error
			resp, err = e.provider.Stream(ctx, provider.Request{
				System:    agentCfg.SystemPrompt,
				Messages:  messages,
				Tools:     toolDefs,
				MaxTokens: agentCfg.MaxTokens,
				Model:     agentCfg.Model,
			}, onChunk)
			if err != nil {
				return nil, fmt.Errorf("llm stream: %w", err)
			}
		} else {
			// Subsequent rounds (after tool use): non-streaming
			var err error
			resp, err = e.provider.Generate(ctx, provider.Request{
				System:    agentCfg.SystemPrompt,
				Messages:  messages,
				Tools:     toolDefs,
				MaxTokens: agentCfg.MaxTokens,
				Model:     agentCfg.Model,
			})
			if err != nil {
				return nil, fmt.Errorf("llm generate round %d: %w", round, err)
			}
		}

		totalInput += resp.Usage.InputTokens
		totalOutput += resp.Usage.OutputTokens

		if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 && !isLastRound {
			messages = append(messages, provider.Message{
				Role:      provider.RoleAssistant,
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})
			for _, tc := range resp.ToolCalls {
				result, execErr := e.tools.Execute(ctx, tc.Name, companyID, userID, tc.Input)
				if execErr != nil {
					result = fmt.Sprintf("Error: %s", execErr.Error())
				}
				messages = append(messages, provider.Message{
					Role: provider.RoleUser,
					Tool: &provider.ToolResult{ToolUseID: tc.ID, Content: result, IsError: execErr != nil},
				})
			}
			continue
		}

		// Final response
		if round > 0 {
			// Non-streamed rounds: send text as a single chunk
			onChunk(provider.StreamChunk{Type: "text_delta", Text: resp.Content})
			onChunk(provider.StreamChunk{Type: "message_stop", Usage: &resp.Usage})
		}

		finalText = resp.Content
		break
	}

	// Calculate and deduct tokens
	tokenCost := e.billing.CalculateTokenCost(totalInput, totalOutput, agentCfg.CostMultiplier)
	desc := fmt.Sprintf("AI chat with %s agent (stream)", agentCfg.Slug)
	if err := e.billing.DeductTokens(ctx, companyID, userID, tokenCost, agentCfg.Slug, desc); err != nil {
		return nil, ErrInsufficientBalance
	}

	latency := time.Since(start)

	// Save messages to session
	msgID := e.saveSessionMessages(ctx, sessionID, req.Message, finalText, int32(tokenCost), isNewSession)

	redactedOutput := redact.RedactText(finalText)
	promptHash := fmt.Sprintf("%x", sha256.Sum256([]byte(redactedInput)))

	e.writeAuditLog(ctx, companyID, userID, requestID, sessionID,
		"agent.chat.stream", agentCfg.Slug, promptHash, redactedInput, redactedOutput,
		totalInput, totalOutput, int(latency.Milliseconds()))

	return &ChatResponse{
		RequestID:  requestID,
		Message:    finalText,
		SessionID:  sessionID,
		TokensUsed: tokenCost,
		Agent:      agentCfg.Slug,
		MessageID:  msgID,
	}, nil
}

// resolveSession returns an existing session ID or creates a new one.
// Returns (sessionID string, isNewSession bool).
func (e *Executor) resolveSession(ctx context.Context, companyID, userID int64, agentSlug string, req ChatRequest) (string, bool) {
	if req.SessionID != "" {
		// Validate the session exists and belongs to this user
		sid, err := uuid.Parse(req.SessionID)
		if err == nil {
			_, err := e.queries.GetChatSession(ctx, store.GetChatSessionParams{
				ID:        sid,
				CompanyID: companyID,
				UserID:    userID,
			})
			if err == nil {
				return req.SessionID, false
			}
		}
		e.logger.Warn("invalid session_id, creating new session", "session_id", req.SessionID)
	}

	// Create new session
	sess, err := e.queries.CreateChatSession(ctx, store.CreateChatSessionParams{
		CompanyID: companyID,
		UserID:    userID,
		AgentSlug: agentSlug,
		Title:     "",
	})
	if err != nil {
		e.logger.Error("failed to create chat session", "error", err)
		return uuid.New().String(), true
	}
	return sess.ID.String(), true
}

// maxHistoryMessages is the maximum number of past messages to load into context.
// 40 messages ≈ 20 conversation turns, keeping well within the context window.
const maxHistoryMessages = 40

// loadSessionMessages loads recent history from DB and appends the current user message.
// Older messages beyond maxHistoryMessages are dropped to prevent context overflow.
func (e *Executor) loadSessionMessages(ctx context.Context, sessionID, currentMessage string) []provider.Message {
	var messages []provider.Message

	sid, err := uuid.Parse(sessionID)
	if err == nil {
		history, err := e.queries.ListChatMessages(ctx, sid)
		if err == nil && len(history) > 0 {
			// Keep only the most recent messages
			start := 0
			if len(history) > maxHistoryMessages {
				start = len(history) - maxHistoryMessages
				e.logger.Info("truncating session history",
					"session_id", sessionID,
					"total_messages", len(history),
					"keeping", maxHistoryMessages,
				)
			}
			for _, msg := range history[start:] {
				messages = append(messages, provider.Message{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}

	// Append current user message
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: currentMessage,
	})

	return messages
}

// saveSessionMessages persists the user message and assistant response to the DB.
// Returns the assistant message ID (0 if saving failed).
func (e *Executor) saveSessionMessages(ctx context.Context, sessionID, userMsg, assistantMsg string, tokensUsed int32, isNewSession bool) int64 {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return 0
	}

	// Save user message
	_, err = e.queries.InsertChatMessage(ctx, store.InsertChatMessageParams{
		SessionID:  sid,
		Role:       "user",
		Content:    userMsg,
		TokensUsed: 0,
	})
	if err != nil {
		e.logger.Error("failed to save user message", "error", err)
	}

	// Save assistant message
	asstMsg, err := e.queries.InsertChatMessage(ctx, store.InsertChatMessageParams{
		SessionID:  sid,
		Role:       "assistant",
		Content:    assistantMsg,
		TokensUsed: tokensUsed,
	})
	if err != nil {
		e.logger.Error("failed to save assistant message", "error", err)
		return 0
	}
	var msgID int64 = asstMsg.ID

	// Update session title from first user message (first 30 chars)
	if isNewSession && userMsg != "" {
		title := userMsg
		if len(title) > 30 {
			title = title[:30] + "..."
		}
		_ = e.queries.UpdateChatSessionTitle(ctx, store.UpdateChatSessionTitleParams{
			ID:    sid,
			Title: title,
		})
	} else {
		// Touch updated_at
		_ = e.queries.TouchChatSession(ctx, sid)
	}

	return msgID
}

// resolveAgent looks up the agent config, falling back to the default agent.
func (e *Executor) resolveAgent(ctx context.Context, slug string) (AgentConfig, error) {
	if slug == "" {
		slug = defaultAgentSlug
	}

	cfg, ok := e.registry.Get(ctx, slug)
	if !ok {
		return AgentConfig{}, fmt.Errorf("agent not found: %s", slug)
	}
	return cfg, nil
}

// writeAuditLog records the interaction in the AI audit log.
func (e *Executor) writeAuditLog(ctx context.Context, companyID, userID int64,
	requestID, sessionID, intent, agentSlug, promptHash, redactedInput, redactedOutput string,
	inputTokens, outputTokens, latencyMs int) {

	intentWithAgent := fmt.Sprintf("%s[%s]", intent, agentSlug)

	_, err := e.queries.InsertAIAuditLog(ctx, store.InsertAIAuditLogParams{
		CompanyID:      companyID,
		UserID:         userID,
		RequestID:      requestID,
		SessionID:      &sessionID,
		Intent:         intentWithAgent,
		Model:          e.provider.Name(),
		PromptHash:     &promptHash,
		RiskLevel:      "low",
		InputTokens:    int32(inputTokens),
		OutputTokens:   int32(outputTokens),
		LatencyMs:      int32(latencyMs),
		RedactedInput:  &redactedInput,
		RedactedOutput: &redactedOutput,
	})
	if err != nil {
		e.logger.Error("failed to write AI audit log", "error", err)
	}
}
