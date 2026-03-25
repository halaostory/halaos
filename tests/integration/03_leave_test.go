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

		list := extractList(t, resp)
		assert.NotEmpty(t, list, "employee %d should have leave balance rows (initialized on creation)", empID)
		t.Logf("Employee %d leave balances: %d types", empID, len(list))
		break
	}
}

var createdLeaveRequestID int64

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
	require.True(t, status == 201 || status == 200,
		"leave request creation failed for employee %d: status=%d body=%s", empID, status, string(resp))

	data := extractData(t, resp)
	var lr struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(data, &lr))
	assert.Equal(t, "pending", lr.Status)
	createdLeaveRequestID = lr.ID
	t.Logf("Leave request ID=%d, status=%s", lr.ID, lr.Status)
}

func TestListLeaveRequests(t *testing.T) {
	resp, status, err := apiGet("/leaves/requests", map[string]string{
		"page":  "1",
		"limit": "50",
	})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "should have leave requests")

	// Verify our created request appears in the list
	if createdLeaveRequestID > 0 {
		found := false
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if id, ok := m["id"].(float64); ok && int64(id) == createdLeaveRequestID {
				found = true
				break
			}
		}
		assert.True(t, found, "created leave request ID=%d should appear in list", createdLeaveRequestID)
	}
	t.Logf("Found %d leave requests", len(list))
}
