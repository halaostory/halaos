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
		TotalEmployees   int `json:"total_employees"`
		ActiveEmployees  int `json:"active_employees"`
		PresentToday     int `json:"present_today"`
		PendingLeaves    int `json:"pending_leaves"`
	}
	require.NoError(t, json.Unmarshal(data, &stats))
	assert.Greater(t, stats.TotalEmployees, 0, "should have employees")
	assert.GreaterOrEqual(t, stats.ActiveEmployees, 0, "active employees should be non-negative")
	t.Logf("Dashboard: total=%d, active=%d, present=%d, pending_leaves=%d",
		stats.TotalEmployees, stats.ActiveEmployees, stats.PresentToday, stats.PendingLeaves)
}

func TestGetDashboardAttendance(t *testing.T) {
	resp, status, err := apiGet("/dashboard/attendance", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "attendance dashboard data should not be nil")
	t.Logf("Dashboard attendance: %d bytes", len(data))
}

func TestGetDepartmentDistribution(t *testing.T) {
	resp, status, err := apiGet("/dashboard/department-distribution", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	// Should return a list of departments with counts
	var depts []map[string]any
	if err := json.Unmarshal(data, &depts); err == nil {
		assert.NotEmpty(t, depts, "should have department distribution data")
		for _, dept := range depts {
			t.Logf("  Dept: %v, count: %v", dept["name"], dept["count"])
		}
	} else {
		t.Logf("Department distribution: %d bytes (non-list format)", len(data))
	}
}

func TestGetPayrollTrend(t *testing.T) {
	resp, status, err := apiGet("/dashboard/payroll-trend", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "payroll trend data should not be nil")
	t.Logf("Payroll trend: %d bytes", len(data))
}

func TestGetLeaveSummary(t *testing.T) {
	resp, status, err := apiGet("/dashboard/leave-summary", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "leave summary data should not be nil")
	t.Logf("Leave summary: %d bytes", len(data))
}

func TestGetActionItems(t *testing.T) {
	resp, status, err := apiGet("/dashboard/action-items", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "action items data should not be nil")
	t.Logf("Action items: %d bytes", len(data))
}

func TestGetCelebrations(t *testing.T) {
	resp, status, err := apiGet("/dashboard/celebrations", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "celebrations data should not be nil")
	t.Logf("Celebrations: %d bytes", len(data))
}

func TestGetSuggestions(t *testing.T) {
	resp, status, err := apiGet("/dashboard/suggestions", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "suggestions data should not be nil")
	t.Logf("Suggestions: %d bytes", len(data))
}

func TestGetFlightRisk(t *testing.T) {
	resp, status, err := apiGet("/dashboard/flight-risk", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "flight risk data should not be nil")
	t.Logf("Flight risk: %d bytes", len(data))
}

func TestGetTeamHealth(t *testing.T) {
	resp, status, err := apiGet("/dashboard/team-health", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "team health data should not be nil")
	t.Logf("Team health: %d bytes", len(data))
}

func TestGetBurnoutRisk(t *testing.T) {
	resp, status, err := apiGet("/dashboard/burnout-risk", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "burnout risk data should not be nil")
	t.Logf("Burnout risk: %d bytes", len(data))
}

func TestGetComplianceAlerts(t *testing.T) {
	resp, status, err := apiGet("/dashboard/compliance-alerts", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	assert.NotNil(t, data, "compliance alerts data should not be nil")
	t.Logf("Compliance alerts: %d bytes", len(data))
}
