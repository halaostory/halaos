package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func getAttendanceRecordsTool() mcp.Tool {
	return mcp.NewTool("get_attendance_records",
		mcp.WithDescription("Get attendance records with optional date range and employee filter. Returns clock-in/out times, hours worked, and status."),
		mcp.WithString("from", mcp.Description("Start date (YYYY-MM-DD). Default: 30 days ago")),
		mcp.WithString("to", mcp.Description("End date (YYYY-MM-DD). Default: tomorrow")),
		mcp.WithString("employee_id", mcp.Description("Filter by employee ID")),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),
		mcp.WithString("limit", mcp.Description("Items per page (default: 10)")),
	)
}

func makeGetAttendanceRecords(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"from":        req.GetString("from", ""),
			"to":          req.GetString("to", ""),
			"employee_id": req.GetString("employee_id", ""),
			"page":        req.GetString("page", "1"),
			"limit":       req.GetString("limit", "10"),
		}
		data, err := c.Get("/attendance/records", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getAttendanceSummaryTool() mcp.Tool {
	return mcp.NewTool("get_attendance_summary",
		mcp.WithDescription("Get attendance summary statistics including present, absent, late counts for the current user."),
	)
}

func makeGetAttendanceSummary(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/attendance/summary", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func clockInTool() mcp.Tool {
	return mcp.NewTool("clock_in",
		mcp.WithDescription("Record clock-in for the current user. Optionally provide location coordinates."),
		mcp.WithNumber("latitude", mcp.Description("Latitude coordinate")),
		mcp.WithNumber("longitude", mcp.Description("Longitude coordinate")),
		mcp.WithString("notes", mcp.Description("Optional clock-in notes")),
		mcp.WithDestructiveHintAnnotation(true),
	)
}

func makeClockIn(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body := map[string]any{}
		if lat := req.GetFloat("latitude", 0); lat != 0 {
			body["latitude"] = lat
		}
		if lng := req.GetFloat("longitude", 0); lng != 0 {
			body["longitude"] = lng
		}
		if notes := req.GetString("notes", ""); notes != "" {
			body["notes"] = notes
		}
		data, err := c.Post("/attendance/clock-in", body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func clockOutTool() mcp.Tool {
	return mcp.NewTool("clock_out",
		mcp.WithDescription("Record clock-out for the current user. Optionally provide location coordinates."),
		mcp.WithNumber("latitude", mcp.Description("Latitude coordinate")),
		mcp.WithNumber("longitude", mcp.Description("Longitude coordinate")),
		mcp.WithString("notes", mcp.Description("Optional clock-out notes")),
		mcp.WithDestructiveHintAnnotation(true),
	)
}

func makeClockOut(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body := map[string]any{}
		if lat := req.GetFloat("latitude", 0); lat != 0 {
			body["latitude"] = lat
		}
		if lng := req.GetFloat("longitude", 0); lng != 0 {
			body["longitude"] = lng
		}
		if notes := req.GetString("notes", ""); notes != "" {
			body["notes"] = notes
		}
		data, err := c.Post("/attendance/clock-out", body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}
