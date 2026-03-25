package integration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Makati office coordinates (inside seed geofence: 14.5547, 121.0244, 200m radius)
const (
	testLat = "14.5548"
	testLng = "121.0245"
)

func TestClockIn(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens from 01_employees")

	for empID, token := range empUserTokens {
		resp, status, err := apiPostAs(token, "/attendance/clock-in", map[string]any{
			"source": "web",
			"lat":    testLat,
			"lng":    testLng,
			"note":   fmt.Sprintf("Integration test clock-in for emp %d", empID),
		})
		require.NoError(t, err, "clock-in failed for employee %d", empID)
		// Accept 200/201 (success) or 409 (already clocked in today)
		if status == 409 {
			t.Logf("Employee %d already clocked in today", empID)
		} else {
			assert.True(t, status == 200 || status == 201,
				"employee %d clock-in expected 200/201/409, got %d: %s", empID, status, string(resp))
			t.Logf("Employee %d clocked in successfully", empID)
		}
	}
}

func TestClockOut(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens from 01_employees")

	for empID, token := range empUserTokens {
		resp, status, err := apiPostAs(token, "/attendance/clock-out", map[string]any{
			"source": "web",
			"lat":    testLat,
			"lng":    testLng,
			"note":   fmt.Sprintf("Integration test clock-out for emp %d", empID),
		})
		require.NoError(t, err, "clock-out failed for employee %d", empID)
		// Accept 200/201 (success) or 404 (already clocked out / no open record)
		if status == 404 {
			t.Logf("Employee %d has no open record (may have already clocked out)", empID)
		} else {
			assert.True(t, status == 200 || status == 201,
				"employee %d clock-out expected 200/201/404, got %d: %s", empID, status, string(resp))
			t.Logf("Employee %d clocked out successfully", empID)
		}
	}
}

func TestGetAttendanceRecords(t *testing.T) {
	resp, status, err := apiGet("/attendance/records", map[string]string{
		"page":  "1",
		"limit": "50",
	})
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "should contain attendance records")
	t.Logf("Attendance records: %d entries", len(list))
}

func TestGetAttendanceSummary(t *testing.T) {
	require.NotEmpty(t, empUserTokens, "need employee user tokens")

	for empID, token := range empUserTokens {
		resp, status, err := apiGetAs(token, "/attendance/summary", nil)
		require.NoError(t, err)
		// 200 = has records, 404 = no attendance today (valid if clock-in was rejected)
		assert.True(t, status == 200 || status == 404,
			"expected 200 or 404, got %d: %s", status, string(resp))
		t.Logf("Employee %d attendance summary: status=%d, %d bytes", empID, status, len(resp))
		break
	}
}
