package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func getSSSTableTool() mcp.Tool {
	return mcp.NewTool("get_sss_table",
		mcp.WithDescription("Get the SSS (Social Security System) contribution table showing salary brackets and contribution amounts. Admin only."),
		mcp.WithString("as_of", mcp.Description("Effective date (YYYY-MM-DD). Default: today")),
	)
}

func makeGetSSSTable(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"as_of": req.GetString("as_of", ""),
		}
		data, err := c.Get("/compliance/sss-table", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getPhilHealthTableTool() mcp.Tool {
	return mcp.NewTool("get_philhealth_table",
		mcp.WithDescription("Get the PhilHealth contribution table showing premium rates and salary brackets. Admin only."),
		mcp.WithString("as_of", mcp.Description("Effective date (YYYY-MM-DD). Default: today")),
	)
}

func makeGetPhilHealthTable(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"as_of": req.GetString("as_of", ""),
		}
		data, err := c.Get("/compliance/philhealth-table", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}

func getBIRTaxTableTool() mcp.Tool {
	return mcp.NewTool("get_bir_tax_table",
		mcp.WithDescription("Get the BIR (Bureau of Internal Revenue) withholding tax table showing tax brackets and rates. Admin only."),
		mcp.WithString("as_of", mcp.Description("Effective date (YYYY-MM-DD). Default: today")),
		mcp.WithString("frequency", mcp.Description("Pay frequency: semi_monthly, monthly, weekly, daily. Default: semi_monthly")),
	)
}

func makeGetBIRTaxTable(c *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := map[string]string{
			"as_of":     req.GetString("as_of", ""),
			"frequency": req.GetString("frequency", ""),
		}
		data, err := c.Get("/compliance/bir-tax-table", query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return formatJSON(data)
	}
}
