package provider

import (
	"context"
)

// Role constants for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Message represents a single chat message.
type Message struct {
	Role    string      `json:"role"`
	Content string      `json:"content,omitempty"`
	Tool    *ToolResult `json:"tool_result,omitempty"`
}

// ToolDefinition describes a tool the LLM can invoke.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"input_schema"`
}

// ToolCall is a tool invocation requested by the LLM.
type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResult is the outcome of executing a tool.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Request holds the parameters for an LLM generation call.
type Request struct {
	System    string           `json:"system,omitempty"`
	Messages  []Message        `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	MaxTokens int              `json:"max_tokens,omitempty"`
	Model     string           `json:"model,omitempty"`
}

// StopReason indicates why the LLM stopped generating.
type StopReason string

const (
	StopEndTurn  StopReason = "end_turn"
	StopToolUse  StopReason = "tool_use"
	StopMaxToken StopReason = "max_tokens"
)

// Response is the result of an LLM generation.
type Response struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason StopReason `json:"stop_reason"`
	Usage      Usage      `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamChunk is a piece of a streaming response.
type StreamChunk struct {
	Type      string    `json:"type"` // text_delta, tool_use, message_stop
	Text      string    `json:"text,omitempty"`
	ToolCall  *ToolCall `json:"tool_call,omitempty"`
	Usage     *Usage    `json:"usage,omitempty"`
	Error     error     `json:"-"`
}

// Provider is the interface for LLM backends.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Generate performs a synchronous LLM call.
	Generate(ctx context.Context, req Request) (*Response, error)

	// Stream performs a streaming LLM call, sending chunks to the callback.
	Stream(ctx context.Context, req Request, onChunk func(StreamChunk)) (*Response, error)

	// Health checks if the provider is available.
	Health(ctx context.Context) error
}
