package home

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

func TestAgencyRows(t *testing.T) {
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
	q := store.New(pool)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var cid int64
	must(pool.QueryRow(ctx, `INSERT INTO companies (name, country, tin, sss_er_no, philhealth_er_no, pagibig_er_no)
		VALUES ($1, 'PHL', '123-456-789-000', '03-1234567-8', '', '1234-5678-9012') RETURNING id`, fmt.Sprint("Agency Co ", time.Now().UnixNano())).Scan(&cid))
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid)               //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM gov_submissions WHERE company_id = $1`, cid) //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM gov_notices WHERE company_id = $1`, cid)     //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM tax_filings WHERE company_id = $1`, cid)     //nolint:errcheck
	_, err = pool.Exec(ctx, `INSERT INTO gov_submissions (company_id, agency, form_code, period, channel, status, submitted_at, reference_no)
		VALUES ($1, 'sss', 'R3', '2026-09', 'browser', 'submitted', now() - interval '20 days', 'SSS-REF-1')`, cid)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO tax_filings (company_id, filing_type, period_type, period_year, period_month, due_date, status)
		VALUES ($1, 'bir_1601c', 'monthly', 2026, 9, CURRENT_DATE - 5, 'pending'),
		       ($1, 'bir_1601c', 'monthly', 2026, 10, CURRENT_DATE + 10, 'pending')`, cid)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO gov_notices (company_id, agency, title, status) VALUES ($1, 'IRAS', 'Late CPF', 'open')`, cid)
	must(err)

	comp, err := q.GetCompanyByID(ctx, cid)
	must(err)
	h := NewHandler(q, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	rows := h.agencyRows(ctx, comp)
	by := map[string]AgencyRow{}
	for _, r := range rows {
		by[r.Code] = r
	}
	if len(rows) != 5 {
		t.Fatalf("want BIR, SSS, PHILHEALTH, PAGIBIG, IRAS; got %+v", rows)
	}
	if r := by["SSS"]; r.LastFiled == nil || r.LastFiled.Form != "R3" || r.LastFiled.ReferenceNo != "SSS-REF-1" || r.LastActivity == nil || r.LastActivity.Kind != "filing" {
		t.Errorf("SSS: %+v", r)
	}
	if r := by["BIR"]; r.NextDue == nil || !r.NextDue.Overdue || r.Status != "needs_info" {
		t.Errorf("BIR must show the overdue return: %+v %+v", r, r.NextDue)
	}
	if r := by["PHILHEALTH"]; r.Status != "needs_info" || r.EmployerNo != "" {
		t.Errorf("PhilHealth has no employer number: %+v", r)
	}
	if r := by["IRAS"]; r.OpenNotices != 1 || r.Status != "needs_info" || r.LastActivity == nil || r.LastActivity.Link != "/notices" {
		t.Errorf("IRAS: %+v", r)
	}
	if rows[len(rows)-1].Status != "active" || rows[0].Status != "needs_info" {
		t.Errorf("attention first: %v … %v", rows[0].Status, rows[len(rows)-1].Status)
	}
}
