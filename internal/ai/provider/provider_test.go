package provider

import (
	"testing"
)

func TestMapOpenAIStopReason(t *testing.T) {
	tests := []struct {
		input string
		want  StopReason
	}{
		{"stop", StopEndTurn},
		{"tool_calls", StopToolUse},
		{"length", StopMaxToken},
		{"unknown", StopEndTurn},
		{"", StopEndTurn},
	}
	for _, tc := range tests {
		got := mapOpenAIStopReason(tc.input)
		if got != tc.want {
			t.Errorf("mapOpenAIStopReason(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestNewOpenAI_DefaultModel(t *testing.T) {
	o := NewOpenAI("test-key", "")
	if o.model != defaultOpenAIModel {
		t.Errorf("model = %q, want %q", o.model, defaultOpenAIModel)
	}
	if o.Name() != "openai" {
		t.Errorf("Name() = %q, want openai", o.Name())
	}
}

func TestNewOpenAI_CustomModel(t *testing.T) {
	o := NewOpenAI("test-key", "gpt-4")
	if o.model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", o.model)
	}
}

func TestNewAnthropic_DefaultModel(t *testing.T) {
	a := NewAnthropic("test-key", "")
	if a.model != defaultModel {
		t.Errorf("model = %q, want %q", a.model, defaultModel)
	}
	if a.Name() != "anthropic" {
		t.Errorf("Name() = %q, want anthropic", a.Name())
	}
}

func TestNewAnthropic_CustomModel(t *testing.T) {
	a := NewAnthropic("test-key", "claude-sonnet-4-5-20250514")
	if a.model != "claude-sonnet-4-5-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-5-20250514", a.model)
	}
}

func TestOpenAI_BuildTools(t *testing.T) {
	o := NewOpenAI("test-key", "")
	tools := []ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get weather for a city",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
			},
		},
	}

	result := o.buildTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Type != "function" {
		t.Errorf("type = %q, want function", result[0].Type)
	}
	if result[0].Function.Name != "get_weather" {
		t.Errorf("name = %q, want get_weather", result[0].Function.Name)
	}
}

func TestOpenAI_BuildMessages(t *testing.T) {
	o := NewOpenAI("test-key", "")
	req := Request{
		System: "You are helpful",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
			{Role: RoleAssistant, Content: "Hi there!"},
		},
	}

	msgs := o.buildMessages(req)

	// System message + 2 user messages = 3
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first msg role = %q, want system", msgs[0].Role)
	}
	if msgs[1].Role != "user" {
		t.Errorf("second msg role = %q, want user", msgs[1].Role)
	}
}

func TestOpenAI_BuildMessages_NoSystem(t *testing.T) {
	o := NewOpenAI("test-key", "")
	req := Request{
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
	}

	msgs := o.buildMessages(req)

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (no system), got %d", len(msgs))
	}
}

func TestOpenAI_BuildMessages_WithToolResult(t *testing.T) {
	o := NewOpenAI("test-key", "")
	req := Request{
		Messages: []Message{
			{Role: RoleUser, Content: "What's the weather?"},
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "get_weather", Input: map[string]any{"city": "Manila"}},
				},
			},
			{
				Role: RoleUser,
				Tool: &ToolResult{ToolUseID: "call_1", Content: "Sunny, 32C"},
			},
		},
	}

	msgs := o.buildMessages(req)

	// Should have 3 messages: user, assistant with tool_calls, tool result
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[2].Role != "tool" {
		t.Errorf("tool result role = %q, want tool", msgs[2].Role)
	}
}

func TestConstants(t *testing.T) {
	// Verify role constants
	if RoleUser != "user" {
		t.Errorf("RoleUser = %q", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q", RoleAssistant)
	}
	if RoleSystem != "system" {
		t.Errorf("RoleSystem = %q", RoleSystem)
	}

	// Verify stop reasons
	if StopEndTurn != "end_turn" {
		t.Errorf("StopEndTurn = %q", StopEndTurn)
	}
	if StopToolUse != "tool_use" {
		t.Errorf("StopToolUse = %q", StopToolUse)
	}
	if StopMaxToken != "max_tokens" {
		t.Errorf("StopMaxToken = %q", StopMaxToken)
	}
}
