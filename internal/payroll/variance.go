package payroll

import (
	"context"
	"math"

	"github.com/halaostory/halaos/internal/store"
)

// ItemWithPrevious is a payroll line plus the employee's previous regular
// pay, so reviewers see what changed before approving (Warp checks every
// payroll for variances before it runs).
type ItemWithPrevious struct {
	store.ListPayrollItemsRow
	PrevNet        *float64 `json:"prev_net_pay"`
	PrevGross      *float64 `json:"prev_gross_pay"`
	NetChangePct   *float64 `json:"net_change_pct"`
	FirstPayroll   bool     `json:"first_payroll"`
	NeedsAttention bool     `json:"needs_attention"` // net pay moved ≥ 20%
}

// VarianceThreshold is the net-pay change that flags a line for review.
const VarianceThreshold = 20.0

// WithPrevious attaches each employee's previous regular pay.
func WithPrevious(ctx context.Context, q *store.Queries, companyID, runID int64, items []store.ListPayrollItemsRow) []ItemWithPrevious {
	prev := map[int64]store.PreviousPayForRunRow{}
	if rows, err := q.PreviousPayForRun(ctx, store.PreviousPayForRunParams{CompanyID: companyID, RunID: runID}); err == nil {
		for _, r := range rows {
			prev[r.EmployeeID] = r
		}
	}
	out := make([]ItemWithPrevious, 0, len(items))
	for _, it := range items {
		row := ItemWithPrevious{ListPayrollItemsRow: it}
		p, ok := prev[it.EmployeeID]
		if !ok {
			row.FirstPayroll = true
			out = append(out, row)
			continue
		}
		net, gross := p.NetPay, p.GrossPay
		row.PrevNet, row.PrevGross = &net, &gross
		if net != 0 {
			pct := math.Round((numToF64(it.NetPay)-net)/math.Abs(net)*1000) / 10
			row.NetChangePct = &pct
			row.NeedsAttention = math.Abs(pct) >= VarianceThreshold
		}
		out = append(out, row)
	}
	return out
}
