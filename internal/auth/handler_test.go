package auth

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
)

// mockSSOValidator is a test implementation of FinanceSSOValidator.
type mockSSOValidator struct {
	email string
	err   error
}

func (m *mockSSOValidator) ValidateFinanceEmail(tokenStr string) (string, error) {
	return m.email, m.err
}

// adminAuth returns an AuthContext with proper auth.Role type for GetRole() assertion.
var adminAuth = testutil.AuthContext{
	UserID: 1, Email: "admin@test.com", Role: RoleAdmin, CompanyID: 1,
}

func newTestHandler(mockDB *testutil.MockDBTX) *Handler {
	queries := store.New(mockDB)
	jwt := NewJWTService("test-secret-key-for-unit-tests", time.Hour, 24*time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(queries, nil, jwt, nil, logger, nil, nil)
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

// --- Logout Tests ---

func TestLogout_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// With nil Redis, logout should still return 204 (graceful degradation)
	ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
	c, w := testutil.NewGinContext("POST", "/api/v1/auth/logout", gin.H{
		"refresh_token": "some-token",
	}, ac)
	h.Logout(c)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
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

// --- SSOLogin Tests ---

func TestSSOLogin_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	h.sso = &mockSSOValidator{email: "admin@test.com"}

	// Mock: GetUserByEmail returns active, verified user
	u := activeUser("admin@test.com", "password123")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))
	// Mock: UpdateLastLogin
	mockDB.OnExecSuccess()

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{
		"sso_token": "valid-sso-token",
	}, testutil.AuthContext{})
	h.SSOLogin(c)

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

func TestSSOLogin_MissingToken(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	h.sso = &mockSSOValidator{email: "admin@test.com"}

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{}, testutil.AuthContext{})
	h.SSOLogin(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOLogin_SSONotConfigured(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	// h.sso is nil

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{
		"sso_token": "some-token",
	}, testutil.AuthContext{})
	h.SSOLogin(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOLogin_InvalidToken(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	h.sso = &mockSSOValidator{err: fmt.Errorf("invalid token")}

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{
		"sso_token": "invalid-token-string",
	}, testutil.AuthContext{})
	h.SSOLogin(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOLogin_UserNotFound(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	h.sso = &mockSSOValidator{email: "nobody@test.com"}

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{
		"sso_token": "valid-sso-token",
	}, testutil.AuthContext{})
	h.SSOLogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOLogin_InactiveUser(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	h.sso = &mockSSOValidator{email: "admin@test.com"}

	u := activeUser("admin@test.com", "password123")
	u.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{
		"sso_token": "valid-sso-token",
	}, testutil.AuthContext{})
	h.SSOLogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSSOLogin_UnverifiedEmail(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)
	h.sso = &mockSSOValidator{email: "admin@test.com"}

	u := activeUser("admin@test.com", "password123")
	u.EmailVerified = false
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/api/v1/auth/sso", gin.H{
		"sso_token": "valid-sso-token",
	}, testutil.AuthContext{})
	h.SSOLogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// --- SwitchCompany Tests ---

// companyScanValues returns mock scan values matching the Company struct (30 fields).
func companyScanValues(id int64, name, country, timezone, currency string) []interface{} {
	now := time.Now()
	return []interface{}{
		id,              // ID
		name,            // Name
		(*string)(nil),  // LegalName
		(*string)(nil),  // Tin
		(*string)(nil),  // BirRdo
		(*string)(nil),  // Address
		(*string)(nil),  // City
		(*string)(nil),  // Province
		(*string)(nil),  // ZipCode
		country,         // Country
		timezone,        // Timezone
		currency,        // Currency
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
		(*string)(nil),  // ReferralCode
		(*string)(nil),  // ReferredByCode
		false,           // ReferralRewardClaimed
	}
}

func TestSwitchCompany_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock: GetUserCompanyMembership — user belongs to target company
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1), int64(2), "admin", time.Now(),
	))
	// Mock: UpdateUserActiveCompany
	mockDB.OnExecSuccess()
	// Mock: GetCompanyByID for response enrichment
	mockDB.OnQueryRow(testutil.NewRow(companyScanValues(2, "Other Co", "SGP", "Asia/Singapore", "SGD")...))

	ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
	c, w := testutil.NewGinContext("POST", "/api/v1/auth/switch-company", gin.H{"company_id": 2}, ac)
	h.SwitchCompany(c)

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
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user object in response")
	}
	if user["company_id"] != float64(2) {
		t.Fatalf("expected company_id=2, got %v", user["company_id"])
	}
	if user["company_country"] != "SGP" {
		t.Fatalf("expected company_country=SGP, got %v", user["company_country"])
	}
}

func TestSwitchCompany_NotMember(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock: GetUserCompanyMembership — no rows (not a member)
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
	c, w := testutil.NewGinContext("POST", "/api/v1/auth/switch-company", gin.H{"company_id": 999}, ac)
	h.SwitchCompany(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSwitchCompany_MissingCompanyID(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
	c, w := testutil.NewGinContext("POST", "/api/v1/auth/switch-company", gin.H{}, ac)
	h.SwitchCompany(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSwitchCompany_UpdateFails(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// Mock: GetUserCompanyMembership — user is a member
	mockDB.OnQueryRow(testutil.NewRow(
		int64(1), int64(2), "admin", time.Now(),
	))
	// Mock: UpdateUserActiveCompany — fails
	mockDB.OnExec(pgconn.NewCommandTag("UPDATE 0"), fmt.Errorf("db error"))

	ac := testutil.AuthContext{UserID: 1, Email: "user@test.com", Role: RoleAdmin, CompanyID: 1}
	c, w := testutil.NewGinContext("POST", "/api/v1/auth/switch-company", gin.H{"company_id": 2}, ac)
	h.SwitchCompany(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
