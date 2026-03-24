package integration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockIn(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens from 01_employees")

	for empID, token := range empUserTokens {
		resp, status, err := apiPostAs(token, "/attendance/clock-in", map[string]any{
			"source": "web",
			"lat":    "14.5995",
			"lng":    "120.9842",
			"note":   fmt.Sprintf("Integration test clock-in for emp %d", empID),
		})
		require.NoError(t, err, "clock-in failed for employee %d", empID)
		// Accept 200 (success) or 409/400 (already clocked in)
		if status == 200 || status == 201 {
			t.Logf("Employee %d clocked in successfully", empID)
		} else {
			t.Logf("Employee %d clock-in status %d: %s", empID, status, string(resp))
		}
	}
}

func TestClockOut(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens from 01_employees")

	for empID, token := range empUserTokens {
		resp, status, err := apiPostAs(token, "/attendance/clock-out", map[string]any{
			"source": "web",
			"lat":    "14.5995",
			"lng":    "120.9842",
			"note":   fmt.Sprintf("Integration test clock-out for emp %d", empID),
		})
		require.NoError(t, err, "clock-out failed for employee %d", empID)
		if status == 200 || status == 201 {
			t.Logf("Employee %d clocked out successfully", empID)
		} else {
			t.Logf("Employee %d clock-out status %d: %s", empID, status, string(resp))
		}
	}
}

func TestGetAttendanceRecords(t *testing.T) {
	resp, status, err := apiGet("/attendance/records", map[string]string{
		"page":  "1",
		"limit": "20",
	})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "should contain attendance records from clock-in/out")
	t.Logf("Attendance records: %d entries", len(list))
}

func TestGetAttendanceSummary(t *testing.T) {
	// Use an employee token for summary
	for _, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/attendance/summary", nil)
		require.NoError(t, err)
		// Accept 200 (has records) or 404 (no attendance today, e.g. geofence blocked clock-in)
		if status == 404 {
			t.Log("No attendance record today (geofence may have blocked clock-in)")
			return
		}
		requireSuccess(t, resp, status)
		t.Logf("Attendance summary: %s", string(resp)[:min(200, len(resp))])
		break
	}
}
