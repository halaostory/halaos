package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func listLeaveTypesTool() mcp.Tool {
	return mcp.NewTool("list_leave_types",
		mcp.WithDescription("List all available leave types (vacation, sick, maternity, etc.) with their IDs and default entitlements. Use this to find leave_type_id values for creating leave requests."),
	)
}

func makeListLeaveTypes(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/leaves/types", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func listAllLeaveBalancesTool() mcp.Tool {
	return mcp.NewTool("list_all_leave_balances",
		mcp.WithDescription("List leave balances for ALL employees in the company. Admin only. Shows earned, used, and remaining days per employee per leave type."),
		mcp.WithString("year", mcp.Description("Year to query (default: current year)")),
	)
}

func makeListAllLeaveBalances(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"year": req.GetString("year", ""),
		}
		data, err := c.Get("/leaves/balances/all", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getLeaveBalancesTool() mcp.Tool {
	return mcp.NewTool("get_leave_balances",
		mcp.WithDescription("Get leave balances for the current user, showing available days for each leave type (vacation, sick, etc.)."),
	)
}

func makeGetLeaveBalances(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/leaves/balances", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func listLeaveRequestsTool() mcp.Tool {
	return mcp.NewTool("list_leave_requests",
		mcp.WithDescription("List leave requests with optional filters. Managers can filter by employee. Shows status, dates, and type."),
		mcp.WithString("employee_id", mcp.Description("Filter by employee ID (manager/admin only)")),
		mcp.WithString("status", mcp.Description("Filter by status: pending, approved, rejected, cancelled")),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),
		mcp.WithString("limit", mcp.Description("Items per page (default: 10)")),
	)
}

func makeListLeaveRequests(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"employee_id": req.GetString("employee_id", ""),
			"status":      req.GetString("status", ""),
			"page":        req.GetString("page", "1"),
			"limit":       req.GetString("limit", "10"),
		}
		data, err := c.Get("/leaves/requests", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func createLeaveRequestTool() mcp.Tool {
	return mcp.NewTool("create_leave_request",
		mcp.WithDescription("Submit a new leave request. Specify the leave type, date range, and reason."),
		mcp.WithString("leave_type_id", mcp.Description("Leave type ID (get from leave types)"), mcp.Required()),
		mcp.WithString("start_date", mcp.Description("Start date (YYYY-MM-DD)"), mcp.Required()),
		mcp.WithString("end_date", mcp.Description("End date (YYYY-MM-DD)"), mcp.Required()),
		mcp.WithString("reason", mcp.Description("Reason for leave request"), mcp.Required()),
		mcp.WithBoolean("half_day", mcp.Description("Whether this is a half-day leave")),
		mcp.WithDestructiveHintAnnotation(true),
	)
}

func makeCreateLeaveRequest(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		leaveTypeID, err := req.RequireString("leave_type_id")
		if err != nil {
			return mcp.NewToolResultError("leave_type_id is required"), nil
		}
		startDate, err := req.RequireString("start_date")
		if err != nil {
			return mcp.NewToolResultError("start_date is required"), nil
		}
		endDate, err := req.RequireString("end_date")
		if err != nil {
			return mcp.NewToolResultError("end_date is required"), nil
		}
		reason, err := req.RequireString("reason")
		if err != nil {
			return mcp.NewToolResultError("reason is required"), nil
		}

		body := map[string]any{
			"leave_type_id": leaveTypeID,
			"start_date":    startDate,
			"end_date":      endDate,
			"reason":        reason,
		}
		if halfDay := req.GetBool("half_day", false); halfDay {
			body["half_day"] = true
		}

		data, apiErr := c.Post("/leaves/requests", body)
		if apiErr != nil {
			return mcp.NewToolResultError(apiErr.Error()), nil
		}
		return formatJSON(data)
	}
}
