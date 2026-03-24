package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func getDashboardStatsTool() mcp.Tool {
	return mcp.NewTool("get_dashboard_stats",
		mcp.WithDescription("Get dashboard statistics including total employees, departments, attendance rate, and key HR metrics."),
	)
}

func makeGetDashboardStats(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/dashboard/stats", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getFlightRiskTool() mcp.Tool {
	return mcp.NewTool("get_flight_risk",
		mcp.WithDescription("Get employees identified as flight risk based on engagement, attendance patterns, and other indicators. Manager only."),
	)
}

func makeGetFlightRisk(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/dashboard/flight-risk", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getComplianceAlertsTool() mcp.Tool {
	return mcp.NewTool("get_compliance_alerts",
		mcp.WithDescription("Get compliance alerts for expiring documents, missing government IDs, overdue filings, and regulatory issues. Manager only."),
	)
}

func makeGetComplianceAlerts(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := c.Get("/dashboard/compliance-alerts", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}
