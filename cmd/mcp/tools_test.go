package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates an httptest server that responds to any request with the given JSON body.
func newTestServer(t *testing.T, wantPath, wantMethod string, response any) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wantPath != "" {
			assert.Equal(t, "/api/v1"+wantPath, r.URL.Path)
		}
		if wantMethod != "" {
			assert.Equal(t, wantMethod, r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	client := NewClient(ts.URL, "halaos_test123")
	return ts, client
}

func callTool(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestListEmployees(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"id": 1, "first_name": "Juan", "last_name": "Dela Cruz"}},
		"meta":    map[string]any{"total": 1, "page": 1, "limit": 10},
	}
	ts, client := newTestServer(t, "/employees", "GET", apiResp)
	defer ts.Close()

	handler := makeListEmployees(client)
	result := callTool(t, handler, map[string]any{"status": "active", "page": "1"})

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Juan")
	assert.Contains(t, text, "Dela Cruz")
}

func TestGetEmployee(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"id": 42, "first_name": "Maria", "email": "maria@test.com"},
	}
	ts, client := newTestServer(t, "/employees/42", "GET", apiResp)
	defer ts.Close()

	handler := makeGetEmployee(client)
	result := callTool(t, handler, map[string]any{"employee_id": "42"})

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Maria")
}

func TestGetEmployee_MissingID(t *testing.T) {
	ts, client := newTestServer(t, "", "", nil)
	defer ts.Close()

	handler := makeGetEmployee(client)
	result := callTool(t, handler, map[string]any{})

	assert.True(t, result.IsError)
}

func TestGetDirectory(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"id": 1, "name": "HR Department"}},
	}
	ts, client := newTestServer(t, "/directory", "GET", apiResp)
	defer ts.Close()

	handler := makeGetDirectory(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "HR Department")
}

func TestGetOrgChart(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"root": map[string]any{"name": "CEO"}},
	}
	ts, client := newTestServer(t, "/directory/org-chart", "GET", apiResp)
	defer ts.Close()

	handler := makeGetOrgChart(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "CEO")
}

func TestGetAttendanceRecords(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"clock_in": "09:00", "clock_out": "18:00"}},
	}
	ts, client := newTestServer(t, "/attendance/records", "GET", apiResp)
	defer ts.Close()

	handler := makeGetAttendanceRecords(client)
	result := callTool(t, handler, map[string]any{"from": "2026-03-01", "to": "2026-03-24"})

	assert.False(t, result.IsError)
}

func TestClockIn(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"clock_in": "2026-03-24T09:00:00Z"},
	}
	ts, client := newTestServer(t, "/attendance/clock-in", "POST", apiResp)
	defer ts.Close()

	handler := makeClockIn(client)
	result := callTool(t, handler, map[string]any{"notes": "WFH"})

	assert.False(t, result.IsError)
}

func TestClockOut(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"clock_out": "2026-03-24T18:00:00Z"},
	}
	ts, client := newTestServer(t, "/attendance/clock-out", "POST", apiResp)
	defer ts.Close()

	handler := makeClockOut(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
}

func TestGetLeaveBalances(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"type": "vacation", "balance": 10}},
	}
	ts, client := newTestServer(t, "/leaves/balances", "GET", apiResp)
	defer ts.Close()

	handler := makeGetLeaveBalances(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "vacation")
}

func TestListLeaveRequests(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"id": 1, "status": "pending"}},
	}
	ts, client := newTestServer(t, "/leaves/requests", "GET", apiResp)
	defer ts.Close()

	handler := makeListLeaveRequests(client)
	result := callTool(t, handler, map[string]any{"status": "pending"})

	assert.False(t, result.IsError)
}

func TestCreateLeaveRequest(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"id": 5, "status": "pending"},
	}
	ts, client := newTestServer(t, "/leaves/requests", "POST", apiResp)
	defer ts.Close()

	handler := makeCreateLeaveRequest(client)
	result := callTool(t, handler, map[string]any{
		"leave_type_id": "1",
		"start_date":    "2026-04-01",
		"end_date":      "2026-04-03",
		"reason":        "Family vacation",
	})

	assert.False(t, result.IsError)
}

func TestCreateLeaveRequest_MissingFields(t *testing.T) {
	ts, client := newTestServer(t, "", "", nil)
	defer ts.Close()

	handler := makeCreateLeaveRequest(client)
	result := callTool(t, handler, map[string]any{})

	assert.True(t, result.IsError)
}

func TestListPayrollCycles(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"id": 1, "period": "2026-03"}},
	}
	ts, client := newTestServer(t, "/payroll/cycles", "GET", apiResp)
	defer ts.Close()

	handler := makeListPayrollCycles(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
}

func TestGetPayslip(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"id": 7, "gross": 50000, "net": 42000},
	}
	ts, client := newTestServer(t, "/payroll/payslips/7", "GET", apiResp)
	defer ts.Close()

	handler := makeGetPayslip(client)
	result := callTool(t, handler, map[string]any{"payslip_id": "7"})

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "50000")
}

func TestGetPayslip_MissingID(t *testing.T) {
	ts, client := newTestServer(t, "", "", nil)
	defer ts.Close()

	handler := makeGetPayslip(client)
	result := callTool(t, handler, map[string]any{})

	assert.True(t, result.IsError)
}

func TestGetSSSTable(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"bracket": "1000-1999", "contribution": 180}},
	}
	ts, client := newTestServer(t, "/compliance/sss-table", "GET", apiResp)
	defer ts.Close()

	handler := makeGetSSSTable(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
}

func TestGetPhilHealthTable(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"rate": 5.0}},
	}
	ts, client := newTestServer(t, "/compliance/philhealth-table", "GET", apiResp)
	defer ts.Close()

	handler := makeGetPhilHealthTable(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
}

func TestGetBIRTaxTable(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"bracket": "0-20833", "rate": 0}},
	}
	ts, client := newTestServer(t, "/compliance/bir-tax-table", "GET", apiResp)
	defer ts.Close()

	handler := makeGetBIRTaxTable(client)
	result := callTool(t, handler, map[string]any{"frequency": "monthly"})

	assert.False(t, result.IsError)
}

func TestGetDashboardStats(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"total_employees": 150, "departments": 8},
	}
	ts, client := newTestServer(t, "/dashboard/stats", "GET", apiResp)
	defer ts.Close()

	handler := makeGetDashboardStats(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "150")
}

func TestGetFlightRisk(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"employee": "John", "risk": "high"}},
	}
	ts, client := newTestServer(t, "/dashboard/flight-risk", "GET", apiResp)
	defer ts.Close()

	handler := makeGetFlightRisk(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
}

func TestGetComplianceAlerts(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    []map[string]any{{"alert": "SSS filing overdue"}},
	}
	ts, client := newTestServer(t, "/dashboard/compliance-alerts", "GET", apiResp)
	defer ts.Close()

	handler := makeGetComplianceAlerts(client)
	result := callTool(t, handler, nil)

	assert.False(t, result.IsError)
}

func TestAIChat(t *testing.T) {
	apiResp := map[string]any{
		"success": true,
		"data":    map[string]any{"response": "There are 150 active employees."},
	}
	ts, client := newTestServer(t, "/ai/chat", "POST", apiResp)
	defer ts.Close()

	handler := makeAIChat(client)
	result := callTool(t, handler, map[string]any{"message": "How many employees?"})

	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "150 active employees")
}

func TestAIChat_MissingMessage(t *testing.T) {
	ts, client := newTestServer(t, "", "", nil)
	defer ts.Close()

	handler := makeAIChat(client)
	result := callTool(t, handler, map[string]any{})

	assert.True(t, result.IsError)
}

func TestAPIError_ReturnsToolError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"success":false,"error":{"message":"insufficient permissions"}}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "halaos_test123")
	handler := makeGetDashboardStats(client)
	result := callTool(t, handler, nil)

	assert.True(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "403")
}

func TestFormatJSON(t *testing.T) {
	data := json.RawMessage(`{"name":"test","value":42}`)
	result, err := formatJSON(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "  \"name\": \"test\"")
}

func TestToolDefinitions(t *testing.T) {
	tools := []mcp.Tool{
		listEmployeesTool(),
		getEmployeeTool(),
		getDirectoryTool(),
		getOrgChartTool(),
		getAttendanceRecordsTool(),
		getAttendanceSummaryTool(),
		clockInTool(),
		clockOutTool(),
		getLeaveBalancesTool(),
		listLeaveRequestsTool(),
		createLeaveRequestTool(),
		listPayrollCyclesTool(),
		getPayslipTool(),
		calculate13thMonthTool(),
		getSSSTableTool(),
		getPhilHealthTableTool(),
		getBIRTaxTableTool(),
		getDashboardStatsTool(),
		getFlightRiskTool(),
		getComplianceAlertsTool(),
		aiChatTool(),
	}

	assert.Len(t, tools, 21, "expected 21 tools")

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool should have a name")
		assert.NotEmpty(t, tool.Description, "tool %s should have a description", tool.Name)
	}
}
