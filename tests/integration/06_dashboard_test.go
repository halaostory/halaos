package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDashboardStats(t *testing.T) {
	resp, status, err := apiGet("/dashboard/stats", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	var stats struct {
		TotalEmployees int `json:"total_employees"`
	}
	require.NoError(t, json.Unmarshal(data, &stats))
	assert.Greater(t, stats.TotalEmployees, 0, "should have employees")
	t.Logf("Dashboard stats: %d total employees", stats.TotalEmployees)
}

func TestGetDashboardAttendance(t *testing.T) {
	resp, status, err := apiGet("/dashboard/attendance", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Dashboard attendance: %d bytes", len(resp))
}

func TestGetDepartmentDistribution(t *testing.T) {
	resp, status, err := apiGet("/dashboard/department-distribution", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Department distribution: %d bytes", len(resp))
}

func TestGetPayrollTrend(t *testing.T) {
	resp, status, err := apiGet("/dashboard/payroll-trend", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Payroll trend: %d bytes", len(resp))
}

func TestGetLeaveSummary(t *testing.T) {
	resp, status, err := apiGet("/dashboard/leave-summary", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Leave summary: %d bytes", len(resp))
}

func TestGetActionItems(t *testing.T) {
	resp, status, err := apiGet("/dashboard/action-items", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Action items: %d bytes", len(resp))
}

func TestGetCelebrations(t *testing.T) {
	resp, status, err := apiGet("/dashboard/celebrations", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Celebrations: %d bytes", len(resp))
}

func TestGetSuggestions(t *testing.T) {
	resp, status, err := apiGet("/dashboard/suggestions", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Suggestions: %d bytes", len(resp))
}

func TestGetFlightRisk(t *testing.T) {
	resp, status, err := apiGet("/dashboard/flight-risk", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Flight risk: %d bytes", len(resp))
}

func TestGetTeamHealth(t *testing.T) {
	resp, status, err := apiGet("/dashboard/team-health", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Team health: %d bytes", len(resp))
}

func TestGetBurnoutRisk(t *testing.T) {
	resp, status, err := apiGet("/dashboard/burnout-risk", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Burnout risk: %d bytes", len(resp))
}

func TestGetComplianceAlerts(t *testing.T) {
	resp, status, err := apiGet("/dashboard/compliance-alerts", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Compliance alerts: %d bytes", len(resp))
}
