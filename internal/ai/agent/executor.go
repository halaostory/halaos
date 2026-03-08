package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
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

	requestID := uuid.New().String()
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	start := time.Now()
	redactedInput := redact.RedactText(req.Message)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: req.Message},
	}

	toolDefs := e.tools.DefinitionsForAgent(agentCfg.Tools)

	var finalResponse string
	var totalInput, totalOutput int

	for round := 0; round < agentCfg.MaxRounds; round++ {
		resp, err := e.provider.Generate(ctx, provider.Request{
			System:    agentCfg.SystemPrompt,
			Messages:  messages,
			Tools:     toolDefs,
			MaxTokens: agentCfg.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("llm generate: %w", err)
		}

		totalInput += resp.Usage.InputTokens
		totalOutput += resp.Usage.OutputTokens

		if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 {
			messages = append(messages, provider.Message{
				Role:    provider.RoleAssistant,
				Content: resp.Content,
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

	requestID := uuid.New().String()
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	start := time.Now()
	redactedInput := redact.RedactText(req.Message)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: req.Message},
	}

	toolDefs := e.tools.DefinitionsForAgent(agentCfg.Tools)

	var totalInput, totalOutput int
	var finalText string

	for round := 0; round < agentCfg.MaxRounds; round++ {
		isLastRound := round == agentCfg.MaxRounds-1

		if round > 0 || len(messages) > 1 {
			// After tool execution, use non-streaming for tool rounds
			resp, err := e.provider.Generate(ctx, provider.Request{
				System:    agentCfg.SystemPrompt,
				Messages:  messages,
				Tools:     toolDefs,
				MaxTokens: agentCfg.MaxTokens,
			})
			if err != nil {
				return nil, fmt.Errorf("llm generate round %d: %w", round, err)
			}
			totalInput += resp.Usage.InputTokens
			totalOutput += resp.Usage.OutputTokens

			if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 && !isLastRound {
				messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: resp.Content})
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

			finalText = resp.Content
			onChunk(provider.StreamChunk{Type: "text_delta", Text: resp.Content})
			onChunk(provider.StreamChunk{Type: "message_stop", Usage: &resp.Usage})
			break
		}

		// First round: stream directly
		resp, err := e.provider.Stream(ctx, provider.Request{
			System:    agentCfg.SystemPrompt,
			Messages:  messages,
			Tools:     toolDefs,
			MaxTokens: agentCfg.MaxTokens,
		}, onChunk)
		if err != nil {
			return nil, fmt.Errorf("llm stream: %w", err)
		}
		totalInput += resp.Usage.InputTokens
		totalOutput += resp.Usage.OutputTokens

		if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 {
			messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: resp.Content})
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
	}, nil
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

// CalculateTokenCostDefault provides a default token cost calculation.
// Cost = ceil((inputTokens + outputTokens) * costMultiplier)
func CalculateTokenCostDefault(inputTokens, outputTokens int, costMultiplier float64) int64 {
	total := float64(inputTokens+outputTokens) * costMultiplier
	return int64(math.Ceil(total))
}
