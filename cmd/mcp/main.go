package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

const version = "1.0.0"

func main() {
	baseURL := os.Getenv("HALAOS_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	apiKey := os.Getenv("HALAOS_API_KEY")

	var client *Client
	if apiKey != "" {
		client = NewClient(baseURL, apiKey)
	}

	s := server.NewMCPServer("halaos-hr", version,
		server.WithToolCapabilities(true),
	)

	// Always register setup tool (works without API key)
	s.AddTool(setupAccountTool(), makeSetupAccount(baseURL))

	if client != nil {
		// Employees
		s.AddTool(listEmployeesTool(), makeListEmployees(client))
		s.AddTool(getEmployeeTool(), makeGetEmployee(client))
		s.AddTool(getDirectoryTool(), makeGetDirectory(client))
		s.AddTool(getOrgChartTool(), makeGetOrgChart(client))

		// Attendance
		s.AddTool(getAttendanceRecordsTool(), makeGetAttendanceRecords(client))
		s.AddTool(getAttendanceSummaryTool(), makeGetAttendanceSummary(client))
		s.AddTool(clockInTool(), makeClockIn(client))
		s.AddTool(clockOutTool(), makeClockOut(client))

		// Leave
		s.AddTool(listLeaveTypesTool(), makeListLeaveTypes(client))
		s.AddTool(getLeaveBalancesTool(), makeGetLeaveBalances(client))
		s.AddTool(listAllLeaveBalancesTool(), makeListAllLeaveBalances(client))
		s.AddTool(listLeaveRequestsTool(), makeListLeaveRequests(client))
		s.AddTool(createLeaveRequestTool(), makeCreateLeaveRequest(client))

		// Payroll
		s.AddTool(listPayrollCyclesTool(), makeListPayrollCycles(client))
		s.AddTool(listPayslipsTool(), makeListPayslips(client))
		s.AddTool(getPayslipTool(), makeGetPayslip(client))
		s.AddTool(calculate13thMonthTool(), makeCalculate13thMonth(client))

		// Compliance
		s.AddTool(getSSSTableTool(), makeGetSSSTable(client))
		s.AddTool(getPhilHealthTableTool(), makeGetPhilHealthTable(client))
		s.AddTool(getBIRTaxTableTool(), makeGetBIRTaxTable(client))

		// Dashboard
		s.AddTool(getDashboardStatsTool(), makeGetDashboardStats(client))
		s.AddTool(getFlightRiskTool(), makeGetFlightRisk(client))
		s.AddTool(getComplianceAlertsTool(), makeGetComplianceAlerts(client))

		// AI
		s.AddTool(aiChatTool(), makeAIChat(client))
	}

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
