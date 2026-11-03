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

func TestWithPrevious(t *testing.T) {
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
	must(pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, fmt.Sprint("Var Co ", time.Now().UnixNano())).Scan(&cid))
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid)                                                           //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM employees WHERE company_id = $1`, cid)                                                   //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM payroll_cycles WHERE company_id = $1`, cid)                                              //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM payroll_runs WHERE company_id = $1`, cid)                                                //nolint:errcheck
	defer pool.Exec(ctx, `DELETE FROM payroll_items WHERE run_id IN (SELECT id FROM payroll_runs WHERE company_id = $1)`, cid) //nolint:errcheck

	emp := func(no string) int64 {
		var id int64
		must(pool.QueryRow(ctx, `INSERT INTO employees (company_id, employee_no, first_name, last_name, hire_date) VALUES ($1, $2, 'A', $2, '2026-01-01') RETURNING id`, cid, no).Scan(&id))
		return id
	}
	run := func(pay, typ, cycleStatus, runStatus string) int64 {
		var cy, r int64
		must(pool.QueryRow(ctx, `INSERT INTO payroll_cycles (company_id, name, period_start, period_end, pay_date, cycle_type, status) VALUES ($1, $2, $3::date - 14, $3::date, $3::date, $4, $5) RETURNING id`, cid, pay+typ, pay, typ, cycleStatus).Scan(&cy))
		must(pool.QueryRow(ctx, `INSERT INTO payroll_runs (company_id, cycle_id, status, total_employees) VALUES ($1, $2, $3, 0) RETURNING id`, cid, cy, runStatus).Scan(&r))
		return r
	}
	item := func(r, e int64, net float64) {
		_, err := pool.Exec(ctx, `INSERT INTO payroll_items (run_id, employee_id, gross_pay, net_pay) VALUES ($1, $2, $3, $3)`, r, e, net)
		must(err)
	}
	steady, raise, newbie := emp("STEADY"), emp("RAISE"), emp("NEW")
	old := run("2026-09-30", "regular", "paid", "completed")
	item(old, steady, 20000)
	item(old, raise, 20000)
	last := run("2026-10-15", "imported", "paid", "completed") // history imported from the old provider
	item(last, steady, 20000)
	item(last, raise, 20000)
	// ignored: off-cycle bonus and a cancelled cycle between the runs
	item(run("2026-10-20", "off_cycle", "paid", "completed"), steady, 99999)
	item(run("2026-10-25", "regular", "cancelled", "completed"), raise, 1)

	cur := run("2026-10-31", "regular", "draft", "completed")
	item(cur, steady, 21000)
	item(cur, raise, 30000)
	item(cur, newbie, 15000)
	items, err := q.ListPayrollItems(ctx, cur)
	must(err)
	got := map[string]ItemWithPrevious{}
	for _, it := range WithPrevious(ctx, q, cid, cur, items) {
		got[it.EmployeeNo] = it
	}
	if s := got["STEADY"]; s.PrevNet == nil || *s.PrevNet != 20000 || *s.NetChangePct != 5 || s.NeedsAttention {
		t.Errorf("steady: %+v", s)
	}
	if r := got["RAISE"]; r.PrevNet == nil || *r.PrevNet != 20000 || *r.NetChangePct != 50 || !r.NeedsAttention {
		t.Errorf("raise must be flagged: %+v", r)
	}
	if n := got["NEW"]; !n.FirstPayroll || n.PrevNet != nil {
		t.Errorf("new hire: %+v", n)
	}
}
