package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func aiChatTool() mcp.Tool {
	return mcp.NewTool("ai_chat",
		mcp.WithDescription("Chat with the HalaOS AI assistant about HR topics. The AI can answer questions about policies, employees, payroll, compliance, and more using the company's HR data."),
		mcp.WithString("message", mcp.Description("The message or question to send to the AI assistant"), mcp.Required()),
		mcp.WithString("session_id", mcp.Description("Optional session ID to continue a previous conversation")),
	)
}

func makeAIChat(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message, err := req.RequireString("message")
		if err != nil {
			return mcp.NewToolResultError("message is required"), nil
		}

		body := map[string]any{
			"message": message,
		}
		if sessionID := req.GetString("session_id", ""); sessionID != "" {
			body["session_id"] = sessionID
		}

		data, apiErr := c.Post("/ai/chat", body)
		if apiErr != nil {
			return mcp.NewToolResultError(apiErr.Error()), nil
		}
		return formatJSON(data)
	}
}
