package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

// adminAuth returns an AuthContext with proper auth.Role type for GetRole() assertion.
var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: RoleAdmin, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	jwt := NewJWTService("test-secret-key-for-unit-tests", time.Hour, 24*time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, jwt, nil, logger)
}

func hashedPassword(plain string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	return string(h)
}

func activeUser(email, password string) store.User {
	return store.User{
		ID:                         1,
		CompanyID:                  1,
		Email:                      email,
		PasswordHash:               hashedPassword(password),
		FirstName:                  "Test",
		LastName:                   "User",
		Role:                       "admin",
		Status:                     "active",
		Locale:                     "en",
		LastLoginAt:                pgtype.Timestamptz{},
		CreatedAt:                  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:                  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EmailVerified:              true,
		VerificationToken:          nil,
		VerificationTokenExpiresAt: pgtype.Timestamptz{},
	}
}

func userScanValues(u store.User) []interface{} {
	return testutil.UserScanValues(u)
}

// extractData extracts the "data" field from a wrapped response.
func extractData(w *httptest.ResponseRecorder) map[string]interface{} {
	body := testutil.ResponseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		return body // fallback to raw body
	}
	return data
}

// --- Login Tests ---

func TestLogin_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "password123")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))
	mockDB.OnExecSuccess() // UpdateLastLogin

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "admin@test.com", "password": "password123",
	}, adminAuth)

	h.Login(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["token"] == nil {
		t.Fatal("expected token in response")
	}
	if data["refresh_token"] == nil {
		t.Fatal("expected refresh_token in response")
	}
}

func TestLogin_WrongEmail(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "wrong@test.com", "password": "password123",
	}, adminAuth)

	h.Login(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "correctpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "admin@test.com", "password": "wrongpassword",
	}, adminAuth)

	h.Login(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_InactiveAccount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "password123")
	u.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/auth/login", gin.H{
		"email": "admin@test.com", "password": "password123",
	}, adminAuth)

	h.Login(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Me Tests ---

func TestMe_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "pw")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("GET", "/auth/me", nil, adminAuth)
	h.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["email"] != "admin@test.com" {
		t.Fatalf("expected email admin@test.com, got %v", data["email"])
	}
}

func TestMe_NotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("GET", "/auth/me", nil, adminAuth)
	h.Me(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Refresh Tests ---

func TestRefresh_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Generate a valid refresh token
	jwt := NewJWTService("test-secret-key-for-unit-tests", time.Hour, 24*time.Hour)
	refreshToken, _ := jwt.GenerateRefreshToken(1, "admin@test.com", RoleAdmin, 1)

	u := activeUser("admin@test.com", "pw")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/auth/refresh", gin.H{
		"refresh_token": refreshToken,
	}, adminAuth)

	h.Refresh(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["token"] == nil {
		t.Fatal("expected new token")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/auth/refresh", gin.H{
		"refresh_token": "invalid-token",
	}, adminAuth)

	h.Refresh(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRefresh_UserNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	jwt := NewJWTService("test-secret-key-for-unit-tests", time.Hour, 24*time.Hour)
	refreshToken, _ := jwt.GenerateRefreshToken(999, "gone@test.com", RoleAdmin, 1)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/auth/refresh", gin.H{
		"refresh_token": refreshToken,
	}, adminAuth)

	h.Refresh(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ChangePassword Tests ---

func TestChangePassword_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "oldpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))
	mockDB.OnExecSuccess() // UpdateUserPassword

	c, w := testutil.NewGinContext("POST", "/auth/change-password", gin.H{
		"current_password": "oldpassword",
		"new_password":     "newpassword123",
	}, adminAuth)

	h.ChangePassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "realpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/auth/change-password", gin.H{
		"current_password": "wrongpassword",
		"new_password":     "newpassword123",
	}, adminAuth)

	h.ChangePassword(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_ValidationError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	c, w := testutil.NewGinContext("POST", "/auth/change-password", gin.H{
		"current_password": "old",
		"new_password":     "short", // less than 8 chars
	}, adminAuth)

	h.ChangePassword(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- UpdateProfile Tests ---

func TestUpdateProfile_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("admin@test.com", "pw")
	u.FirstName = "Updated"
	u.LastName = "Name"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("PUT", "/auth/profile", gin.H{
		"first_name": "Updated",
		"last_name":  "Name",
		"locale":     "en",
	}, adminAuth)

	h.UpdateProfile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["first_name"] != "Updated" {
		t.Fatalf("expected first_name=Updated, got %v", data["first_name"])
	}
}

func TestUpdateProfile_DBError(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("PUT", "/auth/profile", gin.H{
		"first_name": "Test",
		"last_name":  "User",
	}, adminAuth)

	h.UpdateProfile(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
