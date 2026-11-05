package payroll

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

// Payslips are visible once the run is approved, only from the latest
// calculation; a new run supersedes the previous one.
func TestPayslipVisibilityAndSupersede(t *testing.T) {
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
	var cid, eid, cyc int64
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, fmt.Sprint("Vis Co ", time.Now().UnixNano())).Scan(&cid))
	defer func() {
		for _, s := range []string{`DELETE FROM payslips WHERE company_id = $1`, `DELETE FROM payroll_runs WHERE company_id = $1`, `DELETE FROM payroll_cycles WHERE company_id = $1`, `DELETE FROM employees WHERE company_id = $1`, `DELETE FROM companies WHERE id = $1`} {
			_, _ = pool.Exec(ctx, s, cid)
		}
	}()
	must(pool.QueryRow(ctx, `INSERT INTO employees (company_id, employee_no, first_name, last_name, hire_date) VALUES ($1, 'V1', 'V', 'One', '2026-01-01') RETURNING id`, cid).Scan(&eid))
	must(pool.QueryRow(ctx, `INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date) VALUES ($1, 'Nov', '2026-11-01', '2026-11-15', '2026-11-20') RETURNING id`, cid).Scan(&cyc))
	run := func() int64 {
		r, err := q.CreatePayrollRun(ctx, store.CreatePayrollRunParams{CompanyID: cid, CycleID: cyc, RunType: "regular"})
		must(err)
		_, err = pool.Exec(ctx, `UPDATE payroll_runs SET status = 'ready_for_approval' WHERE id = $1`, r.ID)
		must(err)
		_, err = pool.Exec(ctx, `INSERT INTO payslips (company_id, run_id, employee_id, period_start, period_end, pay_date, payload) VALUES ($1, $2, $3, '2026-11-01', '2026-11-15', '2026-11-20', '{}')`, cid, r.ID, eid)
		must(err)
		return r.ID
	}
	visible := func() []int64 {
		ps, err := q.ListPayslips(ctx, store.ListPayslipsParams{CompanyID: cid, EmployeeID: eid, Limit: 10})
		must(err)
		var runs []int64
		for _, p := range ps {
			runs = append(runs, p.RunID)
		}
		return runs
	}
	first := run()
	if v := visible(); len(v) != 0 {
		t.Fatalf("payslip visible before approval: %v", v)
	}
	second := run()
	var st string
	must(pool.QueryRow(ctx, `SELECT status FROM payroll_runs WHERE id = $1`, first).Scan(&st))
	if st != "superseded" {
		t.Errorf("first run status %q, want superseded", st)
	}
	_, err = pool.Exec(ctx, `UPDATE payroll_cycles SET status = 'approved' WHERE id = $1`, cyc)
	must(err)
	if v := visible(); len(v) != 1 || v[0] != second {
		t.Fatalf("want only the latest run's payslip (%d), got %v", second, v)
	}
}
