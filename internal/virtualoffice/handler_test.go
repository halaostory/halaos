package virtualoffice

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

var empAuth = testutil.AuthContext{
	UserID: 10, Email: "emp@test.com", Role: auth.RoleEmployee, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger, nil)
}

// voConfigScanValues returns scan values for VirtualOfficeConfig in sqlc scan order:
// company_id, template, created_at, updated_at
func voConfigScanValues(companyID int64, template string) []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{companyID, template, now, now}
}

func TestUpdateConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// UpsertVirtualOfficeConfig scans: company_id, template, created_at, updated_at
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "medium")...))
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{"template": "medium"}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfig_InvalidTemplate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{"template": "huge"}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfig_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/config", nil, adminAuth)
	h.GetConfig(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/config", nil, adminAuth)
	h.GetConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfig_MissingTemplate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/config",
		gin.H{}, adminAuth)
	h.UpdateConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignSeat_EmployeeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetEmployeeByID returns error
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(99),
		"zone":        "desk-a",
		"seat_x":      int32(2),
		"seat_y":      int32(2),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignSeat_InactiveEmployee(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// Return an inactive employee
	emp := testutil.FixtureEmployee()
	emp.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(1),
		"zone":        "desk-a",
		"seat_x":      int32(2),
		"seat_y":      int32(2),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAutoAssign_NotConfigured(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetVirtualOfficeConfig returns no rows
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/auto", nil, adminAuth)
	h.AutoAssign(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAutoAssign_NoUnassigned(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// GetVirtualOfficeConfig returns config
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	// ListUnassignedActiveEmployees returns empty
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/auto", nil, adminAuth)
	h.AutoAssign(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSnapshot_NotConfigured(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/snapshot", nil, empAuth)
	h.GetSnapshot(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyAvatar_InvalidType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-avatar",
		gin.H{"avatar_type": "unicorn", "avatar_color": "#FF0000"}, empAuth)
	h.UpdateMyAvatar(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyAvatar_InvalidColor(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-avatar",
		gin.H{"avatar_type": "person_1", "avatar_color": "red"}, empAuth)
	h.UpdateMyAvatar(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// voSeatScanValues returns scan values for VirtualOfficeSeat in sqlc scan order (15 fields):
// id, company_id, employee_id, floor, zone, seat_x, seat_y, avatar_type, avatar_color,
// custom_status, custom_emoji, manual_status, meeting_room_zone, created_at, updated_at
func voSeatScanValues(id, companyID, employeeID int64, floor int32, zone string, x, y int32) []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{
		id, companyID, employeeID, floor, zone, x, y,
		"person_1", "#4A90D9",
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		now, now,
	}
}

func TestAssignSeat_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetEmployeeByID → active employee (27 fields)
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	// 2. GetVirtualOfficeConfig → config exists (4 fields)
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	// 3. AssignSeat → seat created (15 fields)
	mockDB.OnQueryRow(testutil.NewRow(voSeatScanValues(1, 1, 1, 1, "desk-a", 2, 2)...))

	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(1),
		"zone":        "desk-a",
		"seat_x":      int32(2),
		"seat_y":      int32(2),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignSeat_NoConfig(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetEmployeeByID → active employee
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	// 2. GetVirtualOfficeConfig → no rows
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(1),
		"zone":        "desk-a",
		"seat_x":      int32(2),
		"seat_y":      int32(2),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignSeat_OutOfBounds(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetEmployeeByID → active employee
	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	// 2. GetVirtualOfficeConfig → config with "small" template (Width: 20, Height: 16)
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	// No more mocks needed; returns 400 before DB call

	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/assign", gin.H{
		"employee_id": int64(1),
		"zone":        "desk-a",
		"seat_x":      int32(999),
		"seat_y":      int32(999),
	}, adminAuth)
	h.AssignSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveSeat_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// RemoveSeat → exec success
	mockDB.OnExecSuccess()

	c, w := testutil.NewGinContextWithParams("DELETE", "/virtual-office/seats/1",
		gin.Params{{Key: "employee_id", Value: "1"}}, nil, adminAuth)
	h.RemoveSeat(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveSeat_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// No DB mock needed, returns 400 immediately

	c, w := testutil.NewGinContextWithParams("DELETE", "/virtual-office/seats/abc",
		gin.Params{{Key: "employee_id", Value: "abc"}}, nil, adminAuth)
	h.RemoveSeat(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAutoAssign_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetVirtualOfficeConfig → config "small"
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	// 2. ListUnassignedActiveEmployees → one employee (4 fields: id, first_name, last_name, department_id)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{
		{int64(1), "John", "Doe", (*int64)(nil)},
	}), nil)
	// 3. ListOccupiedPositions → empty (no occupied seats)
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	// 4. AssignSeat → seat created (15 fields)
	mockDB.OnQueryRow(testutil.NewRow(voSeatScanValues(1, 1, 1, 1, "desk-a", 2, 2)...))

	c, w := testutil.NewGinContext("POST", "/virtual-office/seats/auto", nil, adminAuth)
	h.AutoAssign(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSnapshot_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetVirtualOfficeConfig → config "small"
	mockDB.OnQueryRow(testutil.NewRow(voConfigScanValues(1, "small")...))
	// 2. GetSnapshotSeats → one row (19 fields)
	clockIn := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	clockOut := pgtype.Timestamptz{} // null
	mockDB.OnQuery(testutil.NewRows([][]interface{}{
		{
			int64(1),          // seat_id
			int64(1),          // employee_id
			"John Doe",        // name
			"Developer",       // position
			"Engineering",     // department
			int32(1),          // floor
			"desk-a",          // zone
			int32(2),          // seat_x
			int32(2),          // seat_y
			"person_1",        // avatar_type
			"#4A90D9",         // avatar_color
			(*string)(nil),    // custom_status
			(*string)(nil),    // custom_emoji
			(*string)(nil),    // manual_status
			(*string)(nil),    // meeting_room_zone
			(*string)(nil),    // leave_type
			clockIn,           // clock_in_at
			clockOut,          // clock_out_at
			int32(0),          // late_minutes
		},
	}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/snapshot", nil, empAuth)
	h.GetSnapshot(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyStatus_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetEmployeeByUserID → employee (27 fields)
	emp := testutil.FixtureEmployee()
	emp.UserID = &empAuth.UserID
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	// 2. GetSeatByEmployee → seat exists (15 fields)
	mockDB.OnQueryRow(testutil.NewRow(voSeatScanValues(1, 1, 1, 1, "desk-a", 2, 2)...))
	// 3. UpdateSeatStatus → exec success
	mockDB.OnExecSuccess()

	status := "focused"
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-status",
		gin.H{"manual_status": status}, empAuth)
	h.UpdateMyStatus(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyStatus_InvalidStatus(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// No DB mock needed, returns 400 immediately

	status := "dancing"
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-status",
		gin.H{"manual_status": status}, empAuth)
	h.UpdateMyStatus(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyStatus_NoSeat(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetEmployeeByUserID → employee (27 fields)
	emp := testutil.FixtureEmployee()
	emp.UserID = &empAuth.UserID
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	// 2. GetSeatByEmployee → no rows
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	status := "focused"
	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-status",
		gin.H{"manual_status": status}, empAuth)
	h.UpdateMyStatus(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyAvatar_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. GetEmployeeByUserID → employee (27 fields)
	emp := testutil.FixtureEmployee()
	emp.UserID = &empAuth.UserID
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))
	// 2. UpdateSeatAvatar → exec success
	mockDB.OnExecSuccess()

	c, w := testutil.NewGinContext("PUT", "/virtual-office/my-avatar",
		gin.H{"avatar_type": "person_1", "avatar_color": "#FF0000"}, empAuth)
	h.UpdateMyAvatar(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSeats_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	// 1. ListSeats → one row (18 fields)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{
		{
			int64(1),       // id
			int64(1),       // company_id
			int64(1),       // employee_id
			int32(1),       // floor
			"desk-a",       // zone
			int32(2),       // seat_x
			int32(2),       // seat_y
			"person_1",     // avatar_type
			"#4A90D9",      // avatar_color
			(*string)(nil), // custom_status
			(*string)(nil), // custom_emoji
			(*string)(nil), // manual_status
			(*string)(nil), // meeting_room_zone
			now,            // created_at
			now,            // updated_at
			"John",         // first_name
			"Doe",          // last_name
			"Engineering",  // department_name
		},
	}), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/seats", nil, adminAuth)
	h.ListSeats(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSeats_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// 1. ListSeats → empty rows
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/virtual-office/seats", nil, adminAuth)
	h.ListSeats(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
