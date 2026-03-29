package breaks

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/testutil"
)

// breakPolicyScanValues returns mock scan values matching the BreakPolicy scan order (7 fields).
func breakPolicyScanValues(breakType string, maxMinutes int32) []interface{} {
	return []interface{}{
		int64(1),    // ID
		int64(1),    // CompanyID
		breakType,   // BreakType
		maxMinutes,  // MaxMinutes
		true,        // IsActive
		time.Now(),  // CreatedAt
		time.Now(),  // UpdatedAt
	}
}

// --- ListPolicies ---

func TestListPolicies_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// ListBreakPolicies returns empty list
	mockDB.OnQuery(testutil.NewEmptyRows(), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/break-policies", nil, adminAuth)

	h.ListPolicies(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListPolicies_WithData(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// ListBreakPolicies returns two policies
	rows := [][]interface{}{
		breakPolicyScanValues("meal", 30),
		breakPolicyScanValues("bathroom", 10),
	}
	mockDB.OnQuery(testutil.NewRows(rows), nil)

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/break-policies", nil, adminAuth)

	h.ListPolicies(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListPolicies_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// ListBreakPolicies returns error
	mockDB.OnQuery(nil, fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithQuery("GET", "/attendance/break-policies", nil, adminAuth)

	h.ListPolicies(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpsertPolicies ---

func TestUpsertPolicies_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Each UpsertBreakPolicy call returns a policy
	mockDB.OnQueryRow(testutil.NewRow(breakPolicyScanValues("meal", 30)...))
	mockDB.OnQueryRow(testutil.NewRow(breakPolicyScanValues("bathroom", 10)...))

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", gin.H{
		"policies": []gin.H{
			{"break_type": "meal", "max_minutes": 30},
			{"break_type": "bathroom", "max_minutes": 10},
		},
	}, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpsertPolicies_InvalidType(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", gin.H{
		"policies": []gin.H{
			{"break_type": "smoking", "max_minutes": 15},
		},
	}, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpsertPolicies_EmptyPolicies(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", gin.H{
		"policies": []gin.H{},
	}, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpsertPolicies_MissingBody(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", gin.H{}, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpsertPolicies_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// UpsertBreakPolicy fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("db error")))

	c, w := testutil.NewGinContext("PUT", "/attendance/break-policies", gin.H{
		"policies": []gin.H{
			{"break_type": "meal", "max_minutes": 30},
		},
	}, adminAuth)

	h.UpsertPolicies(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
