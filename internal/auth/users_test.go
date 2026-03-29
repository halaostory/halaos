package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

// --- ListUsers Tests ---

func TestListUsers_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u1 := activeUser("user1@test.com", "pw")
	u1.ID = 1
	u2 := activeUser("user2@test.com", "pw")
	u2.ID = 2

	mockDB.OnQuery(testutil.NewRows(testutil.UserRowsData([]store.User{u1, u2})), nil)
	mockDB.OnQueryRow(testutil.NewRow(int64(2))) // CountUsersByCompany

	c, w := testutil.NewGinContextWithQuery("GET", "/users", url.Values{
		"page": {"1"}, "page_size": {"50"},
	}, adminAuth)

	h.ListUsers(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListUsers_Empty(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQuery(testutil.NewEmptyRows(), nil)
	mockDB.OnQueryRow(testutil.NewRow(int64(0))) // CountUsersByCompany

	c, w := testutil.NewGinContextWithQuery("GET", "/users", nil, adminAuth)
	h.ListUsers(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateUserRole Tests ---

func TestUpdateUserRole_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnExecSuccess()

	c, w := testutil.NewGinContextWithParams("PUT", "/users/5/role",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"role": "manager"}, adminAuth)

	h.UpdateUserRole(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("PUT", "/users/5/role",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"role": "superuser"}, adminAuth)

	h.UpdateUserRole(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateUserRole_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnExec(testutil.ZeroCommandTag(), fmt.Errorf("db error"))

	c, w := testutil.NewGinContextWithParams("PUT", "/users/5/role",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"role": "admin"}, adminAuth)

	h.UpdateUserRole(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateUserStatus Tests ---

func TestUpdateUserStatus_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnExecSuccess()

	c, w := testutil.NewGinContextWithParams("PUT", "/users/5/status",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"status": "inactive"}, adminAuth)

	h.UpdateUserStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateUserStatus_InvalidStatus(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("PUT", "/users/5/status",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"status": "banned"}, adminAuth)

	h.UpdateUserStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- AdminResetPassword Tests ---

func TestAdminResetPassword_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnExecSuccess()

	c, w := testutil.NewGinContextWithParams("POST", "/users/5/reset-password",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"password": "newpassword123"}, adminAuth)

	h.AdminResetPassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminResetPassword_ShortPassword(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContextWithParams("POST", "/users/5/reset-password",
		gin.Params{{Key: "id", Value: "5"}},
		gin.H{"password": "short"}, adminAuth)

	h.AdminResetPassword(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
