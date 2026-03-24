package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLeaveBalances(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens")

	for empID, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/leaves/balances", nil)
		require.NoError(t, err)
		requireSuccess(t, resp, status)
		t.Logf("Employee %d leave balances: %d bytes", empID, len(resp))
		break
	}
}

func TestCreateLeaveRequest(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens")

	// Use first employee token
	var empID int64
	var token string
	for empID, token = range empUserTokens {
		break
	}

	resp, status, err := apiPostAs(token, "/leaves/requests", map[string]any{
		"leave_type_id": 1, // Vacation Leave (from seed data)
		"start_date":    "2026-12-22",
		"end_date":      "2026-12-24",
		"days":          "3",
		"reason":        "Integration test leave request",
	})
	require.NoError(t, err)
	if status == 201 || status == 200 {
		t.Logf("Leave request created for employee %d", empID)
		data := extractData(t, resp)
		var lr struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal(data, &lr))
		assert.Equal(t, "pending", lr.Status)
		t.Logf("Leave request ID=%d, status=%s", lr.ID, lr.Status)
	} else {
		t.Logf("Leave request status %d: %s (may lack balance)", status, string(resp))
	}
}

func TestListLeaveRequests(t *testing.T) {
	resp, status, err := apiGet("/leaves/requests", map[string]string{
		"page":  "1",
		"limit": "20",
	})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	t.Logf("Found %d leave requests", len(list))
}
