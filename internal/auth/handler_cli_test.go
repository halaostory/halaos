package auth

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

// cliTestCompany returns scan values for the Company struct in exact Scan order (31 fields).
// Field order matches CreateCompanyWithCountry RETURNING clause as scanned in company.sql.go.
func cliTestCompany() []interface{} {
	now := time.Now()
	return []interface{}{
		int64(1),       // ID
		"example.com",  // Name
		(*string)(nil), // LegalName
		(*string)(nil), // Tin
		(*string)(nil), // BirRdo
		(*string)(nil), // Address
		(*string)(nil), // City
		(*string)(nil), // Province
		(*string)(nil), // ZipCode
		"PHL",          // Country
		"Asia/Manila",  // Timezone
		"PHP",          // Currency
		"semi_monthly", // PayFrequency
		"active",       // Status
		(*string)(nil), // LogoUrl
		now,            // CreatedAt
		now,            // UpdatedAt
		false,          // GeofenceEnabled
		(*string)(nil), // SssErNo
		(*string)(nil), // PhilhealthErNo
		(*string)(nil), // PagibigErNo
		(*string)(nil), // BankName
		(*string)(nil), // BankBranch
		(*string)(nil), // BankAccountNo
		(*string)(nil), // BankAccountName
		(*string)(nil), // ContactPerson
		(*string)(nil), // ContactEmail
		(*string)(nil), // ContactPhone
		(*string)(nil), // ReferralCode
		(*string)(nil), // ReferredByCode
		false,          // ReferralRewardClaimed
	}
}

// cliTestAPIKeyRow returns scan values for the CreateAPIKey RETURNING clause:
// id, prefix, name, is_active, last_used_at, created_at
func cliTestAPIKeyRow() []interface{} {
	return []interface{}{
		int64(1),              // id
		"halaos_a1b2c3",      // prefix
		"cli-default",        // name
		true,                 // is_active
		pgtype.Timestamptz{}, // last_used_at
		time.Now(),           // created_at
	}
}

func TestCLIRegister_ExistingEmail(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByEmail returns a user (email already exists)
	u := activeUser("existing@example.com", "password123")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...))

	c, w := testutil.NewGinContext("POST", "/auth/cli-register", map[string]interface{}{
		"email":    "existing@example.com",
		"password": "password123",
	}, testutil.AuthContext{})

	h.CLIRegister(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLIRegister_PasswordTooShort(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	// GetUserByEmail returns no rows (email is free), but password validation fails
	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows))

	c, w := testutil.NewGinContext("POST", "/auth/cli-register", map[string]interface{}{
		"email":    "newuser@example.com",
		"password": "abc",
	}, testutil.AuthContext{})

	h.CLIRegister(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// Compile-time check: ensure store.Company has the expected field set used in cliTestCompany.
var _ = store.Company{}

// --- CLILogin Tests ---

func TestCLILogin_Success(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("cli@example.com", "password123")
	u.EmailVerified = true
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny
	mockDB.OnExecSuccess()                                   // UpdateLastLogin
	mockDB.OnExecSuccess()                                   // RevokeAPIKeyByName
	mockDB.OnQueryRow(testutil.NewRow(cliTestAPIKeyRow()...)) // CreateAPIKey
	mockDB.OnQueryRow(testutil.NewRow(cliTestCompany()...))   // GetCompanyByID

	c, w := testutil.NewGinContext("POST", "/auth/cli-login", map[string]interface{}{
		"email":    "cli@example.com",
		"password": "password123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["token"] == nil {
		t.Fatal("expected token in response")
	}
	apiKey, ok := data["api_key"].(string)
	if !ok || !strings.HasPrefix(apiKey, "halaos_") {
		t.Fatalf("expected api_key starting with halaos_, got %v", data["api_key"])
	}
}

func TestCLILogin_InvalidCredentials(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	mockDB.OnQueryRow(testutil.NewErrorRow(pgx.ErrNoRows)) // GetUserByEmailAny returns no rows

	c, w := testutil.NewGinContext("POST", "/auth/cli-login", map[string]interface{}{
		"email":    "nobody@example.com",
		"password": "password123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLILogin_WrongPassword(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("cli@example.com", "correctpassword")
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny

	c, w := testutil.NewGinContext("POST", "/auth/cli-login", map[string]interface{}{
		"email":    "cli@example.com",
		"password": "wrongpassword",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLILogin_InactiveAccount(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("cli@example.com", "password123")
	u.Status = "inactive"
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny

	c, w := testutil.NewGinContext("POST", "/auth/cli-login", map[string]interface{}{
		"email":    "cli@example.com",
		"password": "password123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCLILogin_AutoVerifiesUnverifiedEmail(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	h := newTestHandler(mockDB)

	u := activeUser("cli@example.com", "password123")
	u.EmailVerified = false
	mockDB.OnQueryRow(testutil.NewRow(userScanValues(u)...)) // GetUserByEmailAny
	mockDB.OnExecSuccess()                                   // MarkEmailVerified
	mockDB.OnExecSuccess()                                   // UpdateLastLogin
	mockDB.OnExecSuccess()                                   // RevokeAPIKeyByName
	mockDB.OnQueryRow(testutil.NewRow(cliTestAPIKeyRow()...)) // CreateAPIKey
	mockDB.OnQueryRow(testutil.NewRow(cliTestCompany()...))   // GetCompanyByID

	c, w := testutil.NewGinContext("POST", "/auth/cli-login", map[string]interface{}{
		"email":    "cli@example.com",
		"password": "password123",
	}, testutil.AuthContext{})

	h.CLILogin(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := extractData(w)
	if data["api_key"] == nil {
		t.Fatal("expected api_key in response")
	}
}
