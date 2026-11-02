package store

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestAllQueriesPrepare parses every query sqlc generated and asks a real
// PostgreSQL to PREPARE it against the migrated schema. This catches errors
// that compile fine in Go but fail on every call at runtime, e.g.
// "inconsistent types deduced for parameter" or unknown columns.
//
// Opt-in: set HALAOS_TEST_DATABASE_URL to a database migrated to the latest
// version (CI does this in the "sql" job).
func TestAllQueriesPrepare(t *testing.T) {
	dsn := os.Getenv("HALAOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HALAOS_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	files, _ := filepath.Glob("*.sql.go")
	re := regexp.MustCompile("(?s)const \\w+ = `-- name: (\\w+) :\\w+\\n(.*?)`\\n")
	n := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			name, sql := m[1], m[2]
			n++
			if _, err := conn.Prepare(ctx, "", sql); err != nil {
				t.Errorf("%s (%s): %v", name, f, err)
			}
		}
	}
	if n < 100 {
		t.Fatalf("only found %d queries — parser broken?", n)
	}
	t.Logf("prepared %d queries", n)
}
