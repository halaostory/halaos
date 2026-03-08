package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	openaiAPIURL        = "https://api.openai.com/v1/chat/completions"
	defaultOpenAIModel  = "gpt-4o"
)

// OpenAI implements Provider for the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAI creates an OpenAI provider.
func NewOpenAI(apiKey, model string) *OpenAI {
	if model == "" {
		model = defaultOpenAIModel
	}
	return &OpenAI{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAI) Name() string { return "openai" }

// --- OpenAI API types ---

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMsg     `json:"messages"`
	Tools    []openaiTool    `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens,omitempty"`
	Stream   bool            `json:"stream,omitempty"`
	StreamOptions *openaiStreamOpts `json:"stream_options,omitempty"`
}

type openaiStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiMsg struct {
	Role       string          `json:"role"`
	Content    any             `json:"content"`           // string or null
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function openaiToolCallFn `json:"function"`
}

type openaiToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiToolDef  `json:"function"`
}

type openaiToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiResponse struct {
	ID      string          `json:"id"`
	Choices []openaiChoice  `json:"choices"`
	Usage   openaiUsage     `json:"usage"`
	Error   *openaiError    `json:"error,omitempty"`
}

type openaiChoice struct {
	Message      openaiMsg `json:"message"`
	Delta        openaiMsg `json:"delta"`
	FinishReason string    `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// --- Implementation ---

func (o *OpenAI) buildMessages(req Request) []openaiMsg {
	msgs := make([]openaiMsg, 0, len(req.Messages)+1)

	// System prompt as first message
	if req.System != "" {
		msgs = append(msgs, openaiMsg{Role: "system", Content: req.System})
	}

	for _, m := range req.Messages {
		if m.Tool != nil {
			// Tool result → role: "tool"
			msgs = append(msgs, openaiMsg{
				Role:       "tool",
				Content:    m.Tool.Content,
				ToolCallID: m.Tool.ToolUseID,
			})
		} else if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			// Assistant with tool calls
			tcs := make([]openaiToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Input)
				tcs[i] = openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openaiToolCallFn{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				}
			}
			var content any
			if m.Content != "" {
				content = m.Content
			}
			msgs = append(msgs, openaiMsg{
				Role:      "assistant",
				Content:   content,
				ToolCalls: tcs,
			})
		} else {
			msgs = append(msgs, openaiMsg{Role: m.Role, Content: m.Content})
		}
	}

	return msgs
}

func (o *OpenAI) buildTools(tools []ToolDefinition) []openaiTool {
	result := make([]openaiTool, len(tools))
	for i, t := range tools {
		result[i] = openaiTool{
			Type: "function",
			Function: openaiToolDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return result
}

func (o *OpenAI) doRequest(ctx context.Context, req Request, stream bool) (*http.Request, error) {
	model := o.model
	if req.Model != "" {
		model = req.Model
	}
	maxTokens := defaultMaxTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	body := openaiRequest{
		Model:     model,
		Messages:  o.buildMessages(req),
		MaxTokens: maxTokens,
		Stream:    stream,
	}
	if len(req.Tools) > 0 {
		body.Tools = o.buildTools(req.Tools)
	}
	if stream {
		body.StreamOptions = &openaiStreamOpts{IncludeUsage: true}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	return httpReq, nil
}

// Generate performs a synchronous OpenAI chat completion.
func (o *OpenAI) Generate(ctx context.Context, req Request) (*Response, error) {
	httpReq, err := o.doRequest(ctx, req, false)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("openai error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	return o.parseResponse(&apiResp), nil
}

// Stream performs a streaming OpenAI chat completion.
func (o *OpenAI) Stream(ctx context.Context, req Request, onChunk func(StreamChunk)) (*Response, error) {
	httpReq, err := o.doRequest(ctx, req, true)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var fullText strings.Builder
	var toolCalls []ToolCall
	var usage Usage
	var stopReason StopReason
	// Track partial tool calls being accumulated across chunks
	toolCallMap := make(map[int]*ToolCall)
	toolCallArgs := make(map[int]*strings.Builder)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openaiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Usage comes in the final chunk with stream_options
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			usage.InputTokens = chunk.Usage.PromptTokens
			usage.OutputTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Text content
		if text, ok := delta.Content.(string); ok && text != "" {
			fullText.WriteString(text)
			onChunk(StreamChunk{Type: "text_delta", Text: text})
		}

		// Tool calls (streamed incrementally)
		for _, tc := range delta.ToolCalls {
			idx := 0 // Use index from the slice position
			for i, existingTC := range delta.ToolCalls {
				if existingTC.ID == tc.ID {
					idx = i
					break
				}
			}
			// If we see a new ID, it's a new tool call
			if tc.ID != "" {
				toolCallMap[idx] = &ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolCallArgs[idx] = &strings.Builder{}
			}
			// Accumulate arguments
			if tc.Function.Arguments != "" {
				if builder, ok := toolCallArgs[idx]; ok {
					builder.WriteString(tc.Function.Arguments)
				}
			}
		}

		// Check finish reason
		if choice.FinishReason != "" {
			stopReason = mapOpenAIStopReason(choice.FinishReason)
		}
	}

	// Finalize tool calls
	for idx, tc := range toolCallMap {
		if builder, ok := toolCallArgs[idx]; ok {
			var input map[string]any
			if err := json.Unmarshal([]byte(builder.String()), &input); err != nil {
				slog.Warn("openai: failed to parse tool arguments", "tool", tc.Name, "error", err)
			}
			tc.Input = input
		}
		toolCalls = append(toolCalls, *tc)
		onChunk(StreamChunk{Type: "tool_use", ToolCall: tc})
	}

	onChunk(StreamChunk{Type: "message_stop", Usage: &usage})

	return &Response{
		Content:    fullText.String(),
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage:      usage,
	}, nil
}

// Health checks API availability.
func (o *OpenAI) Health(ctx context.Context) error {
	req := Request{
		Messages:  []Message{{Role: RoleUser, Content: "ping"}},
		MaxTokens: 5,
	}
	_, err := o.Generate(ctx, req)
	return err
}

func (o *OpenAI) parseResponse(resp *openaiResponse) *Response {
	if len(resp.Choices) == 0 {
		return &Response{
			Usage: Usage{
				InputTokens:  resp.Usage.PromptTokens,
				OutputTokens: resp.Usage.CompletionTokens,
			},
		}
	}

	choice := resp.Choices[0]
	content, _ := choice.Message.Content.(string)

	var toolCalls []ToolCall
	for _, tc := range choice.Message.ToolCalls {
		var input map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			slog.Warn("openai: failed to parse tool arguments", "tool", tc.Function.Name, "error", err)
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return &Response{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: mapOpenAIStopReason(choice.FinishReason),
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

func mapOpenAIStopReason(reason string) StopReason {
	switch reason {
	case "stop":
		return StopEndTurn
	case "tool_calls":
		return StopToolUse
	case "length":
		return StopMaxToken
	default:
		return StopEndTurn
	}
}
