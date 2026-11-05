package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/halaostory/halaos/internal/store"
)

// One person, accounts at two companies: the shared employee app must
// let them choose, and never pick one at random.
func TestPickLoginAccountAcrossCompanies(t *testing.T) {
	dsn := os.Getenv("HALAOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HALAOS_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h := &Handler{queries: store.New(pool)}
	sfx := fmt.Sprint(time.Now().UnixNano())
	email := "multi" + sfx + "@t.test"
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw-one"), bcrypt.MinCost)
	other, _ := bcrypt.GenerateFromPassword([]byte("pw-two"), bcrypt.MinCost)
	var a, b, c int64
	for _, x := range []*int64{&a, &b, &c} {
		if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "MC "+sfx+fmt.Sprint(x)).Scan(x); err != nil {
			t.Fatal(err)
		}
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email)               //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id IN ($1, $2, $3)`, a, b, c) //nolint:errcheck
	for _, row := range []struct {
		cid  int64
		hash []byte
	}{{a, hash}, {b, hash}, {c, other}} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (company_id, email, password_hash, first_name, last_name, role, status, email_verified) VALUES ($1, $2, $3, 'M', 'C', 'employee', 'active', true)`, row.cid, email, string(row.hash)); err != nil {
			t.Fatal(err)
		}
	}
	// pw-one matches companies a and b → choose.
	_, choice, err := h.pickLoginAccount(ctx, loginRequest{Email: email, Password: "pw-one"})
	if err != nil || len(choice) != 2 {
		t.Fatalf("want a choice of 2 companies, got %v %v", choice, err)
	}
	u, choice, err := h.pickLoginAccount(ctx, loginRequest{Email: email, Password: "pw-one", CompanyID: b})
	if err != nil || choice != nil || u.CompanyID != b {
		t.Fatalf("chosen company b: %+v %v %v", u.CompanyID, choice, err)
	}
	// pw-two matches only company c → straight in.
	u, choice, err = h.pickLoginAccount(ctx, loginRequest{Email: email, Password: "pw-two"})
	if err != nil || choice != nil || u.CompanyID != c {
		t.Fatalf("single match: %v %v %v", u.CompanyID, choice, err)
	}
	// Wrong password, or a company where the password doesn't match.
	if _, _, err := h.pickLoginAccount(ctx, loginRequest{Email: email, Password: "nope"}); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, _, err := h.pickLoginAccount(ctx, loginRequest{Email: email, Password: "pw-two", CompanyID: a}); err == nil {
		t.Fatal("password of another company's account accepted")
	}
	// Email is matched case-insensitively.
	if _, _, err := h.pickLoginAccount(ctx, loginRequest{Email: "MULTI" + sfx + "@T.TEST", Password: "pw-two"}); err != nil {
		t.Fatalf("case-insensitive email: %v", err)
	}
}
