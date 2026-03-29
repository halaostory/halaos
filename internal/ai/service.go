package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/ai/redact"
	"github.com/halaostory/halaos/internal/store"
)

const systemPrompt = `You are AigoNHR AI Assistant, an expert in Philippine HR, payroll, and labor compliance.
You help employees and HR managers with questions about:
- Leave balances and policies
- Attendance and time tracking
- Payroll and compensation
- Government contributions (SSS, PhilHealth, PagIBIG, BIR)
- Philippine labor laws (DOLE regulations, TRAIN Law, Labor Code)
- Company policies and HR best practices

Rules:
- Always use tools to get real data before answering. Never guess numbers.
- For labor law, policy, or compliance questions, ALWAYS use the search_knowledge_base tool first. It contains comprehensive Philippine labor law, DOLE regulations, and HR policy articles. Cite the legal source (e.g., "Under RA 11210..." or "Per DOLE Labor Advisory...") when available.
- Use explain_policy as a fallback for quick policy summaries.
- Be concise but thorough. Use tables for numerical data.
- Respond in the same language the user writes in.
- When showing monetary amounts, use PHP (Philippine Peso) format.
- Never reveal sensitive personal information (TIN, SSS numbers, bank accounts).
- If you cannot answer a question, suggest who to contact (HR, Finance, etc.).`

// Service orchestrates AI interactions with tool calling.
type Service struct {
	provider provider.Provider
	tools    *ToolRegistry
	queries  *store.Queries
	redactor *redact.FieldRedactor
	logger   *slog.Logger
}

// NewService creates the AI service.
func NewService(p provider.Provider, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{
		provider: p,
		tools:    NewToolRegistry(queries, pool),
		queries:  queries,
		redactor: redact.NewFieldRedactor(),
		logger:   logger,
	}
}

// ChatRequest is the input for a chat interaction.
type ChatRequest struct {
	Message     string         `json:"message"`
	SessionID   string         `json:"session_id,omitempty"`
	Agent       string         `json:"agent,omitempty"`
	PageContext *PageContextDTO `json:"page_context,omitempty"`
}

// PageContextDTO passes page context from the frontend.
type PageContextDTO struct {
	Section string `json:"section,omitempty"`
	Action  string `json:"action,omitempty"`
}

// ChatResponse is the result of a chat interaction.
type ChatResponse struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

// Chat performs a synchronous chat with tool calling loop.
func (s *Service) Chat(ctx context.Context, companyID, userID int64, req ChatRequest) (*ChatResponse, error) {
	requestID := uuid.New().String()
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	start := time.Now()

	// Redact user message before sending to LLM
	redactedInput := redact.RedactText(req.Message)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: req.Message},
	}

	var finalResponse string
	var totalInput, totalOutput int
	maxRounds := 5 // max tool-calling rounds

	for round := 0; round < maxRounds; round++ {
		resp, err := s.provider.Generate(ctx, provider.Request{
			System:    systemPrompt,
			Messages:  messages,
			Tools:     s.tools.Definitions(),
			MaxTokens: 4096,
		})
		if err != nil {
			return nil, fmt.Errorf("llm generate: %w", err)
		}

		totalInput += resp.Usage.InputTokens
		totalOutput += resp.Usage.OutputTokens

		if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 {
			// Add assistant message with tool calls
			messages = append(messages, provider.Message{
				Role:    provider.RoleAssistant,
				Content: resp.Content,
			})

			// Execute each tool
			for _, tc := range resp.ToolCalls {
				s.logger.Info("executing tool", "name", tc.Name, "input", tc.Input)

				result, err := s.tools.Execute(ctx, tc.Name, companyID, userID, tc.Input)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				}

				messages = append(messages, provider.Message{
					Role: provider.RoleUser,
					Tool: &provider.ToolResult{
						ToolUseID: tc.ID,
						Content:   result,
						IsError:   err != nil,
					},
				})
			}
			continue
		}

		// End turn - we have the final response
		finalResponse = resp.Content
		break
	}

	latency := time.Since(start)

	// Audit log
	redactedOutput := redact.RedactText(finalResponse)
	promptHash := fmt.Sprintf("%x", sha256.Sum256([]byte(redactedInput)))

	_ = s.writeAuditLog(ctx, companyID, userID, requestID, sessionID,
		"chat", promptHash, redactedInput, redactedOutput,
		totalInput, totalOutput, int(latency.Milliseconds()))

	return &ChatResponse{
		RequestID: requestID,
		Message:   finalResponse,
		SessionID: sessionID,
	}, nil
}

// StreamChat performs a streaming chat. The onChunk callback receives text chunks.
func (s *Service) StreamChat(ctx context.Context, companyID, userID int64, req ChatRequest, onChunk func(provider.StreamChunk)) (*ChatResponse, error) {
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

	var totalInput, totalOutput int
	var finalText string
	maxRounds := 5

	for round := 0; round < maxRounds; round++ {
		isLastRound := round == maxRounds-1

		if round > 0 || len(messages) > 1 {
			// After tool execution, use non-streaming for tool rounds
			resp, err := s.provider.Generate(ctx, provider.Request{
				System:    systemPrompt,
				Messages:  messages,
				Tools:     s.tools.Definitions(),
				MaxTokens: 4096,
			})
			if err != nil {
				return nil, fmt.Errorf("llm generate round %d: %w", round, err)
			}
			totalInput += resp.Usage.InputTokens
			totalOutput += resp.Usage.OutputTokens

			if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 && !isLastRound {
				messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: resp.Content})
				for _, tc := range resp.ToolCalls {
					result, err := s.tools.Execute(ctx, tc.Name, companyID, userID, tc.Input)
					if err != nil {
						result = fmt.Sprintf("Error: %s", err.Error())
					}
					messages = append(messages, provider.Message{
						Role: provider.RoleUser,
						Tool: &provider.ToolResult{ToolUseID: tc.ID, Content: result, IsError: err != nil},
					})
				}
				continue
			}

			// Stream the final response
			finalText = resp.Content
			onChunk(provider.StreamChunk{Type: "text_delta", Text: resp.Content})
			onChunk(provider.StreamChunk{Type: "message_stop", Usage: &resp.Usage})
			break
		}

		// First round: stream directly
		resp, err := s.provider.Stream(ctx, provider.Request{
			System:    systemPrompt,
			Messages:  messages,
			Tools:     s.tools.Definitions(),
			MaxTokens: 4096,
		}, onChunk)
		if err != nil {
			return nil, fmt.Errorf("llm stream: %w", err)
		}
		totalInput += resp.Usage.InputTokens
		totalOutput += resp.Usage.OutputTokens

		if resp.StopReason == provider.StopToolUse && len(resp.ToolCalls) > 0 {
			messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: resp.Content})
			for _, tc := range resp.ToolCalls {
				result, err := s.tools.Execute(ctx, tc.Name, companyID, userID, tc.Input)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				}
				messages = append(messages, provider.Message{
					Role: provider.RoleUser,
					Tool: &provider.ToolResult{ToolUseID: tc.ID, Content: result, IsError: err != nil},
				})
			}
			continue
		}

		finalText = resp.Content
		break
	}

	latency := time.Since(start)
	redactedOutput := redact.RedactText(finalText)
	promptHash := fmt.Sprintf("%x", sha256.Sum256([]byte(redactedInput)))

	_ = s.writeAuditLog(ctx, companyID, userID, requestID, sessionID,
		"chat.stream", promptHash, redactedInput, redactedOutput,
		totalInput, totalOutput, int(latency.Milliseconds()))

	return &ChatResponse{
		RequestID: requestID,
		Message:   finalText,
		SessionID: sessionID,
	}, nil
}

func (s *Service) writeAuditLog(ctx context.Context, companyID, userID int64,
	requestID, sessionID, intent, promptHash, redactedInput, redactedOutput string,
	inputTokens, outputTokens, latencyMs int) error {

	_, err := s.queries.InsertAIAuditLog(ctx, store.InsertAIAuditLogParams{
		CompanyID:      companyID,
		UserID:         userID,
		RequestID:      requestID,
		SessionID:      &sessionID,
		Intent:         intent,
		Model:          s.provider.Name(),
		PromptHash:     &promptHash,
		RiskLevel:      "low",
		InputTokens:    int32(inputTokens),
		OutputTokens:   int32(outputTokens),
		LatencyMs:      int32(latencyMs),
		RedactedInput:  &redactedInput,
		RedactedOutput: &redactedOutput,
	})
	if err != nil {
		s.logger.Error("failed to write AI audit log", "error", err)
	}
	return err
}

// ToolCallSummary returns a JSON summary of tool calls for audit.
func ToolCallSummary(calls []provider.ToolCall) json.RawMessage {
	if len(calls) == 0 {
		return nil
	}
	type summary struct {
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	}
	summaries := make([]summary, len(calls))
	for i, c := range calls {
		summaries[i] = summary{Name: c.Name, Input: c.Input}
	}
	b, _ := json.Marshal(summaries)
	return b
}
