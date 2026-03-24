package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func listEmployeesTool() mcp.Tool {
	return mcp.NewTool("list_employees",
		mcp.WithDescription("List employees with optional filters for status and department. Returns paginated results."),
		mcp.WithString("status", mcp.Description("Filter by status: active, inactive, terminated, on_leave")),
		mcp.WithString("department_id", mcp.Description("Filter by department ID")),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),
		mcp.WithString("limit", mcp.Description("Items per page (default: 10)")),
	)
}

func makeListEmployees(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"status":        req.GetString("status", ""),
			"department_id": req.GetString("department_id", ""),
			"page":          req.GetString("page", "1"),
			"limit":         req.GetString("limit", "10"),
		}
		data, err := c.Get("/employees", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getEmployeeTool() mcp.Tool {
	return mcp.NewTool("get_employee",
		mcp.WithDescription("Get detailed information about a specific employee by their ID."),
		mcp.WithString("employee_id", mcp.Description("The employee ID"), mcp.Required()),
	)
}

func makeGetEmployee(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("employee_id")
		if err != nil {
			return mcp.NewToolResultError("employee_id is required"), nil
		}
		data, apiErr := c.Get("/employees/"+id, nil)
		if apiErr != nil {
			return mcp.NewToolResultError(apiErr.Error()), nil
		}
		return formatJSON(data)
	}
}

func getDirectoryTool() mcp.Tool {
	return mcp.NewTool("get_directory",
		mcp.WithDescription("Get the employee directory with contact information and department details."),
	)
}

func makeGetDirectory(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/directory", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getOrgChartTool() mcp.Tool {
	return mcp.NewTool("get_org_chart",
		mcp.WithDescription("Get the organizational chart showing reporting structure and hierarchy."),
	)
}

func makeGetOrgChart(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/directory/org-chart", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

// formatJSON pretty-prints JSON for AI readability.
func formatJSON(data json.RawMessage) (*mcp.CallToolResult, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return mcp.NewToolResultText(string(data)), nil
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultText(string(data)), nil
	}
	text := string(pretty)
	if len(text) > 50000 {
		text = text[:50000] + fmt.Sprintf("\n... (truncated, %d bytes total)", len(text))
	}
	return mcp.NewToolResultText(text), nil
}
