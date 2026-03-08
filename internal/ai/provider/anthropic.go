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
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	defaultModel        = "claude-sonnet-4-5-20250514"
	defaultMaxTokens    = 4096
)

// Anthropic implements Provider for the Claude API.
type Anthropic struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropic creates an Anthropic provider.
func NewAnthropic(apiKey, model string) *Anthropic {
	if model == "" {
		model = defaultModel
	}
	return &Anthropic{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

// anthropicRequest is the API request body.
type anthropicRequest struct {
	Model     string            `json:"model"`
	System    string            `json:"system,omitempty"`
	Messages  []anthropicMsg    `json:"messages"`
	Tools     []anthropicTool   `json:"tools,omitempty"`
	MaxTokens int               `json:"max_tokens"`
	Stream    bool              `json:"stream,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicResponse struct {
	ID         string         `json:"id"`
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) buildRequest(req Request, stream bool) (*http.Request, error) {
	model := a.model
	if req.Model != "" {
		model = req.Model
	}
	maxTokens := defaultMaxTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	msgs := make([]anthropicMsg, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Tool != nil {
			// Merge consecutive tool_result messages into a single user message
			block := contentBlock{
				Type:      "tool_result",
				ToolUseID: m.Tool.ToolUseID,
				Content:   m.Tool.Content,
				IsError:   m.Tool.IsError,
			}
			if len(msgs) > 0 && msgs[len(msgs)-1].Role == "user" {
				// Check if last message is already a tool_result array
				if blocks, ok := msgs[len(msgs)-1].Content.([]contentBlock); ok {
					msgs[len(msgs)-1].Content = append(blocks, block)
					continue
				}
			}
			msgs = append(msgs, anthropicMsg{
				Role:    "user",
				Content: []contentBlock{block},
			})
		} else if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			// Assistant message with tool_use: build full content blocks
			blocks := make([]contentBlock, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				blocks = append(blocks, contentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			msgs = append(msgs, anthropicMsg{Role: "assistant", Content: blocks})
		} else {
			msgs = append(msgs, anthropicMsg{Role: m.Role, Content: m.Content})
		}
	}

	tools := make([]anthropicTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	body := anthropicRequest{
		Model:     model,
		System:    req.System,
		Messages:  msgs,
		Tools:     tools,
		MaxTokens: maxTokens,
		Stream:    stream,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	return httpReq, nil
}

// Generate performs a synchronous Claude API call.
func (a *Anthropic) Generate(ctx context.Context, req Request) (*Response, error) {
	httpReq, err := a.buildRequest(req, false)
	if err != nil {
		return nil, err
	}
	httpReq = httpReq.WithContext(ctx)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("anthropic error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	return parseResponse(&apiResp), nil
}

// Stream performs a streaming Claude API call.
func (a *Anthropic) Stream(ctx context.Context, req Request, onChunk func(StreamChunk)) (*Response, error) {
	httpReq, err := a.buildRequest(req, true)
	if err != nil {
		return nil, err
	}
	httpReq = httpReq.WithContext(ctx)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var fullText strings.Builder
	var toolCalls []ToolCall
	var usage Usage
	var stopReason StopReason
	var currentToolCall *ToolCall
	var currentToolInput strings.Builder

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

		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)
		switch eventType {
		case "content_block_start":
			cb, _ := event["content_block"].(map[string]any)
			if cbType, _ := cb["type"].(string); cbType == "tool_use" {
				currentToolCall = &ToolCall{
					ID:   cb["id"].(string),
					Name: cb["name"].(string),
				}
				currentToolInput.Reset()
			}

		case "content_block_delta":
			delta, _ := event["delta"].(map[string]any)
			deltaType, _ := delta["type"].(string)
			switch deltaType {
			case "text_delta":
				text, _ := delta["text"].(string)
				fullText.WriteString(text)
				onChunk(StreamChunk{Type: "text_delta", Text: text})
			case "input_json_delta":
				partial, _ := delta["partial_json"].(string)
				currentToolInput.WriteString(partial)
			}

		case "content_block_stop":
			if currentToolCall != nil {
				var input map[string]any
				if err := json.Unmarshal([]byte(currentToolInput.String()), &input); err != nil {
					slog.Warn("anthropic: failed to parse tool arguments", "tool", currentToolCall.Name, "error", err)
				}
				currentToolCall.Input = input
				toolCalls = append(toolCalls, *currentToolCall)
				onChunk(StreamChunk{Type: "tool_use", ToolCall: currentToolCall})
				currentToolCall = nil
			}

		case "message_delta":
			delta, _ := event["delta"].(map[string]any)
			if sr, ok := delta["stop_reason"].(string); ok {
				stopReason = StopReason(sr)
			}
			if u, ok := event["usage"].(map[string]any); ok {
				if ot, ok := u["output_tokens"].(float64); ok {
					usage.OutputTokens = int(ot)
				}
			}

		case "message_start":
			if msg, ok := event["message"].(map[string]any); ok {
				if u, ok := msg["usage"].(map[string]any); ok {
					if it, ok := u["input_tokens"].(float64); ok {
						usage.InputTokens = int(it)
					}
				}
			}
		}
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
func (a *Anthropic) Health(ctx context.Context) error {
	req := Request{
		Messages:  []Message{{Role: RoleUser, Content: "ping"}},
		MaxTokens: 5,
	}
	_, err := a.Generate(ctx, req)
	return err
}

func parseResponse(resp *anthropicResponse) *Response {
	var text strings.Builder
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return &Response{
		Content:    text.String(),
		ToolCalls:  toolCalls,
		StopReason: StopReason(resp.StopReason),
		Usage: Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}
}
