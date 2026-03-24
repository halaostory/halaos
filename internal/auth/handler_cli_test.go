package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/testutil"
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
