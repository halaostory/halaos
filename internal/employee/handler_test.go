package employee

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

// --- ListEmployees ---

func TestListEmployees_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	e1 := testutil.FixtureEmployee()
	e2 := testutil.FixtureEmployee()
	e2.ID = 2
	e2.EmployeeNo = "EMP-002"

	mockDB.OnQuery(testutil.NewRows(testutil.EmployeeRowsData([]store.Employee{e1, e2})), nil)
	mockDB.OnQueryRow(testutil.NewRow(int64(2))) // CountEmployees

	c, w := testutil.NewGinContextWithQuery("GET", "/employees", nil, adminAuth)
	h.ListEmployees(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListEmployees_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQueryRow(testutil.NewRow(int64(0)))

	c, w := testutil.NewGinContextWithQuery("GET", "/employees", nil, adminAuth)
	h.ListEmployees(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListEmployees_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/employees", nil, adminAuth)
	h.ListEmployees(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- CreateEmployee ---

func TestCreateEmployee_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	c, w := testutil.NewGinContext("POST", "/employees", gin.H{
		"employee_no": "EMP-001",
		"first_name":  "John",
		"last_name":   "Doe",
		"hire_date":   "2025-01-15",
	}, adminAuth)

	h.CreateEmployee(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateEmployee_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/employees", gin.H{
		"first_name": "John",
		// missing required fields
	}, adminAuth)

	h.CreateEmployee(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateEmployee_InvalidHireDate(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/employees", gin.H{
		"employee_no": "EMP-001",
		"first_name":  "John",
		"last_name":   "Doe",
		"hire_date":   "invalid-date",
	}, adminAuth)

	h.CreateEmployee(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateEmployee_Conflict(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("unique violation")))

	c, w := testutil.NewGinContext("POST", "/employees", gin.H{
		"employee_no": "EMP-001",
		"first_name":  "John",
		"last_name":   "Doe",
		"hire_date":   "2025-01-15",
	}, adminAuth)

	h.CreateEmployee(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetEmployee ---

func TestGetEmployee_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	emp := testutil.FixtureEmployee()
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	c, w := testutil.NewGinContextWithParams("GET", "/employees/1",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.GetEmployee(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetEmployee_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("GET", "/employees/999",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)

	h.GetEmployee(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetEmployee_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/employees/abc",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)

	h.GetEmployee(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateEmployee ---

func TestUpdateEmployee_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	emp := testutil.FixtureEmployee()
	emp.FirstName = "Jane"
	mockDB.OnQueryRow(testutil.NewRow(testutil.EmployeeScanValues(emp)...))

	firstName := "Jane"
	c, w := testutil.NewGinContextWithParams("PUT", "/employees/1",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"first_name": firstName}, adminAuth)

	h.UpdateEmployee(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEmployee_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("PUT", "/employees/999",
		gin.Params{{Key: "id", Value: "999"}},
		gin.H{"first_name": "Jane"}, adminAuth)

	h.UpdateEmployee(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEmployee_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("PUT", "/employees/abc",
		gin.Params{{Key: "id", Value: "abc"}},
		gin.H{"first_name": "Jane"}, adminAuth)

	h.UpdateEmployee(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEmployee_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContextWithParams("PUT", "/employees/1",
		gin.Params{{Key: "id", Value: "1"}},
		gin.H{"first_name": "Jane"}, adminAuth)

	h.UpdateEmployee(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GetProfile ---

func profileScanValues() []interface{} {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []interface{}{
		int64(1),        // EmployeeID
		(*string)(nil),  // AddressLine1
		(*string)(nil),  // AddressLine2
		(*string)(nil),  // City
		(*string)(nil),  // Province
		(*string)(nil),  // ZipCode
		(*string)(nil),  // EmergencyName
		(*string)(nil),  // EmergencyPhone
		(*string)(nil),  // EmergencyRelation
		(*string)(nil),  // BankName
		(*string)(nil),  // BankAccountNo
		(*string)(nil),  // BankAccountName
		(*string)(nil),  // Tin
		(*string)(nil),  // SssNo
		(*string)(nil),  // PhilhealthNo
		(*string)(nil),  // PagibigNo
		(*string)(nil),  // BloodType
		(*string)(nil),  // Religion
		now,             // UpdatedAt
	}
}

func TestGetProfile_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewRow(profileScanValues()...))

	c, w := testutil.NewGinContextWithParams("GET", "/employees/1/profile",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.GetProfile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("GET", "/employees/999/profile",
		gin.Params{{Key: "id", Value: "999"}}, nil, adminAuth)

	h.GetProfile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetProfile_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/employees/abc/profile",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)

	h.GetProfile(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListDocuments ---

func TestListDocuments_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithParams("GET", "/employees/1/documents",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.ListDocuments(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListDocuments_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithParams("GET", "/employees/1/documents",
		gin.Params{{Key: "id", Value: "1"}}, nil, adminAuth)

	h.ListDocuments(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListDocuments_InvalidID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/employees/abc/documents",
		gin.Params{{Key: "id", Value: "abc"}}, nil, adminAuth)

	h.ListDocuments(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DownloadDocument ---

func TestDownloadDocument_InvalidDocID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("GET", "/employees/1/documents/invalid",
		gin.Params{{Key: "id", Value: "1"}, {Key: "doc_id", Value: "invalid"}}, nil, adminAuth)

	h.DownloadDocument(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDownloadDocument_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("GET", "/employees/1/documents/550e8400-e29b-41d4-a716-446655440000",
		gin.Params{{Key: "id", Value: "1"}, {Key: "doc_id", Value: "550e8400-e29b-41d4-a716-446655440000"}}, nil, adminAuth)

	h.DownloadDocument(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DeleteDocument ---

func TestDeleteDocument_InvalidDocID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("DELETE", "/employees/1/documents/invalid",
		gin.Params{{Key: "id", Value: "1"}, {Key: "doc_id", Value: "invalid"}}, nil, adminAuth)

	h.DeleteDocument(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContextWithParams("DELETE", "/employees/1/documents/550e8400-e29b-41d4-a716-446655440000",
		gin.Params{{Key: "id", Value: "1"}, {Key: "doc_id", Value: "550e8400-e29b-41d4-a716-446655440000"}}, nil, adminAuth)

	h.DeleteDocument(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
