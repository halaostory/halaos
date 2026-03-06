package company

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: auth.RoleAdmin, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, logger)
}

// companyScanValues returns values matching the Company scan order (28 fields).
func companyScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{
		int64(1),        // ID
		"Test Corp",     // Name
		(*string)(nil),  // LegalName
		(*string)(nil),  // Tin
		(*string)(nil),  // BirRdo
		(*string)(nil),  // Address
		(*string)(nil),  // City
		(*string)(nil),  // Province
		(*string)(nil),  // ZipCode
		"PH",            // Country
		"Asia/Manila",   // Timezone
		"PHP",           // Currency
		"semi_monthly",  // PayFrequency
		"active",        // Status
		(*string)(nil),  // LogoUrl
		now,             // CreatedAt
		now,             // UpdatedAt
		false,           // GeofenceEnabled
		(*string)(nil),  // SssErNo
		(*string)(nil),  // PhilhealthErNo
		(*string)(nil),  // PagibigErNo
		(*string)(nil),  // BankName
		(*string)(nil),  // BankBranch
		(*string)(nil),  // BankAccountNo
		(*string)(nil),  // BankAccountName
		(*string)(nil),  // ContactPerson
		(*string)(nil),  // ContactEmail
		(*string)(nil),  // ContactPhone
	}
}

// departmentScanValues returns values matching the Department scan order (9 fields).
func departmentScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{
		int64(1),       // ID
		int64(1),       // CompanyID
		"ENG",          // Code
		"Engineering",  // Name
		(*int64)(nil),  // ParentID
		(*int64)(nil),  // HeadEmployeeID
		true,           // IsActive
		now,            // CreatedAt
		now,            // UpdatedAt
	}
}

// positionScanValues returns values matching the Position scan order (9 fields).
func positionScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{
		int64(1),        // ID
		int64(1),        // CompanyID
		"SE",            // Code
		"Software Eng",  // Title
		(*int64)(nil),   // DepartmentID
		(*string)(nil),  // Grade
		true,            // IsActive
		now,             // CreatedAt
		now,             // UpdatedAt
	}
}

// --- GetCompany ---

func TestGetCompany_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(companyScanValues()...))
	c, w := testutil.NewGinContextWithQuery("GET", "/company", nil, adminAuth)
	h.GetCompany(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetCompany_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithQuery("GET", "/company", nil, adminAuth)
	h.GetCompany(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateCompany ---

func TestUpdateCompany_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(companyScanValues()...))
	c, w := testutil.NewGinContext("PUT", "/company", gin.H{"name": "NewCo"}, adminAuth)
	h.UpdateCompany(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateCompany_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("PUT", "/company", "invalid", adminAuth)
	h.UpdateCompany(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateCompany_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))
	c, w := testutil.NewGinContext("PUT", "/company", gin.H{"name": "NewCo"}, adminAuth)
	h.UpdateCompany(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListDepartments ---

func TestListDepartments_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{departmentScanValues()}), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/company/departments", nil, adminAuth)
	h.ListDepartments(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListDepartments_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/company/departments", nil, adminAuth)
	h.ListDepartments(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateDepartment ---

func TestCreateDepartment_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(departmentScanValues()...))
	c, w := testutil.NewGinContext("POST", "/company/departments", gin.H{
		"code": "ENG", "name": "Engineering",
	}, adminAuth)
	h.CreateDepartment(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDepartment_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/company/departments", gin.H{}, adminAuth)
	h.CreateDepartment(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDepartment_Conflict(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("unique violation")))
	c, w := testutil.NewGinContext("POST", "/company/departments", gin.H{
		"code": "ENG", "name": "Engineering",
	}, adminAuth)
	h.CreateDepartment(c)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateDepartment ---

func TestUpdateDepartment_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(departmentScanValues()...))
	c, w := testutil.NewGinContextWithParams("PUT", "/company/departments/1",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"name": "Eng Updated"}, adminAuth)
	h.UpdateDepartment(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateDepartment_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithParams("PUT", "/company/departments/999",
		gin.Params{{Key: "id", Value: "999"}},
		gin.H{"name": "Nope"}, adminAuth)
	h.UpdateDepartment(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateDepartment_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("PUT", "/company/departments/abc",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.UpdateDepartment(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListPositions ---

func TestListPositions_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(testutil.NewRows([][]interface{}{positionScanValues()}), nil)
	c, w := testutil.NewGinContextWithQuery("GET", "/company/positions", nil, adminAuth)
	h.ListPositions(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListPositions_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQuery(nil, fmt.Errorf("db error"))
	c, w := testutil.NewGinContextWithQuery("GET", "/company/positions", nil, adminAuth)
	h.ListPositions(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreatePosition ---

func TestCreatePosition_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(positionScanValues()...))
	c, w := testutil.NewGinContext("POST", "/company/positions", gin.H{
		"code": "SE", "title": "Software Eng",
	}, adminAuth)
	h.CreatePosition(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePosition_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContext("POST", "/company/positions", gin.H{}, adminAuth)
	h.CreatePosition(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePosition_Conflict(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("unique violation")))
	c, w := testutil.NewGinContext("POST", "/company/positions", gin.H{
		"code": "SE", "title": "Software Eng",
	}, adminAuth)
	h.CreatePosition(c)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdatePosition ---

func TestUpdatePosition_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewRow(positionScanValues()...))
	c, w := testutil.NewGinContextWithParams("PUT", "/company/positions/1",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"title": "Senior Engineer"}, adminAuth)
	h.UpdatePosition(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePosition_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))
	c, w := testutil.NewGinContextWithParams("PUT", "/company/positions/999",
		gin.Params{{Key: "id", Value: "999"}},
		gin.H{"title": "Nope"}, adminAuth)
	h.UpdatePosition(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePosition_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	c, w := testutil.NewGinContextWithParams("PUT", "/company/positions/abc",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)
	h.UpdatePosition(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
