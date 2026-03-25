package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func listPayrollCyclesTool() mcp.Tool {
	return mcp.NewTool("list_payroll_cycles",
		mcp.WithDescription("List payroll cycles showing period, status, and totals. Admin only."),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),
		mcp.WithString("limit", mcp.Description("Items per page (default: 10)")),
	)
}

func makeListPayrollCycles(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"page":  req.GetString("page", "1"),
			"limit": req.GetString("limit", "10"),
		}
		data, err := c.Get("/payroll/cycles", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func listPayslipsTool() mcp.Tool {
	return mcp.NewTool("list_payslips",
		mcp.WithDescription("List payslips for the current user. Shows gross pay, deductions, and net pay for each payroll period."),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),
		mcp.WithString("limit", mcp.Description("Items per page (default: 10)")),
	)
}

func makeListPayslips(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"page":  req.GetString("page", "1"),
			"limit": req.GetString("limit", "10"),
		}
		data, err := c.Get("/payroll/payslips", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getPayslipTool() mcp.Tool {
	return mcp.NewTool("get_payslip",
		mcp.WithDescription("Get a specific payslip by ID. Shows gross pay, deductions, net pay, and breakdown."),
		mcp.WithString("payslip_id", mcp.Description("The payslip ID"), mcp.Required()),
	)
}

func makeGetPayslip(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("payslip_id")
		if err != nil {
			return mcp.NewToolResultError("payslip_id is required"), nil
		}
		data, apiErr := c.Get("/payroll/payslips/"+id, nil)
		if apiErr != nil {
			return mcp.NewToolResultError(apiErr.Error()), nil
		}
		return formatJSON(data)
	}
}

func calculate13thMonthTool() mcp.Tool {
	return mcp.NewTool("calculate_13th_month",
		mcp.WithDescription("Calculate 13th month pay for employees. Philippine labor law mandatory benefit. Admin only."),
		mcp.WithString("year", mcp.Description("Year to calculate for (e.g., 2026)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
}

func makeCalculate13thMonth(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body := map[string]any{}
		if year := req.GetString("year", ""); year != "" {
			body["year"] = year
		}
		data, err := c.Post("/payroll/13th-month/calculate", body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}
