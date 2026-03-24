package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPayrollCycles(t *testing.T) {
	resp, status, err := apiGet("/payroll/cycles", map[string]string{"page": "1", "limit": "10"})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	t.Logf("Found %d payroll cycles", len(list))
}

func TestCreatePayrollCycle(t *testing.T) {
	resp, status, err := apiPost("/payroll/cycles", map[string]any{
		"name":         "Integration Test - Feb 2026 P1",
		"period_start": "2026-02-01",
		"period_end":   "2026-02-15",
		"pay_date":     "2026-02-20",
		"cycle_type":   "regular",
	})
	require.NoError(t, err)
	requireCreated(t, resp, status)

	data := extractData(t, resp)
	var cycle struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(data, &cycle))
	assert.Equal(t, "draft", cycle.Status)
	testCycleID = cycle.ID
	t.Logf("Created payroll cycle ID=%d, status=%s", cycle.ID, cycle.Status)
}

func TestRunPayroll(t *testing.T) {
	require.NotZero(t, testCycleID, "need payroll cycle from TestCreatePayrollCycle")

	resp, status, err := apiPost("/payroll/runs", map[string]any{
		"cycle_id": testCycleID,
		"run_type": "regular",
	})
	require.NoError(t, err)
	requireCreated(t, resp, status)

	data := extractData(t, resp)
	var run struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(data, &run))
	t.Logf("Payroll run ID=%d, status=%s", run.ID, run.Status)
}

func TestGetPayslips(t *testing.T) {
	// Check existing payslips (from seed data or previous runs)
	for _, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/payroll/payslips", nil)
		require.NoError(t, err)
		requireSuccess(t, resp, status)
		t.Logf("Payslips response: %d bytes", len(resp))
		break
	}
}
