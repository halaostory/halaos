package govfiling

import (
	"context"
	"fmt"

	"github.com/halaostory/halaos/internal/store"
)

// AutopilotResult summarises one autopilot pass.
type AutopilotResult struct {
	Prepared      int `json:"prepared"`
	Failed        int `json:"failed"`
	Registrations int `json:"registrations"`
}

// Autopilot keeps every company's statutory calendar moving, Warp-style:
//  1. ensures this year's Philippine filing calendar exists;
//  2. prepares (file + pre-flight review) every return due within 10 days so
//     HR only has to review and approve it from the inbox;
//  3. opens a state-registration task for US work states without one.
//
// Nothing is submitted here — submission always waits for approval.
func (s *Service) Autopilot(ctx context.Context) AutopilotResult {
	var res AutopilotResult
	year := int32(s.now().Year())
	companies, err := s.q.ListCompaniesForAutopilot(ctx)
	if err != nil {
		s.logger.Error("govfiling autopilot: companies", "error", err)
		return res
	}
	for _, c := range companies {
		switch c.Country {
		case "PHL":
			if err := s.q.GenerateAnnualFilings(ctx, store.GenerateAnnualFilingsParams{CompanyID: c.ID, PeriodYear: year}); err != nil {
				s.logger.Warn("govfiling autopilot: annual calendar", "company_id", c.ID, "error", err)
			}
		case "USA":
			states, err := s.q.ListUnregisteredWorkStates(ctx, c.ID)
			if err != nil {
				continue
			}
			for _, st := range states {
				if _, err := s.Prepare(ctx, c.ID, nil, PrepareInput{FormCode: "us_state_registration", Period: st, Channel: string(ChannelManual)}); err == nil {
					res.Registrations++
				}
			}
		}
	}

	due, err := s.q.ListFilingsToPrepare(ctx, 10)
	if err != nil {
		s.logger.Error("govfiling autopilot: due filings", "error", err)
		return res
	}
	for _, tf := range due {
		spec, ok := Lookup(tf.FilingType)
		if !ok {
			continue
		}
		period := periodOf(tf)
		if pp, err := ParsePeriod(spec.Frequency, period); err != nil || !dateOnly(pp.End()).Before(dateOnly(s.now())) {
			continue // period not closed yet — prepare once it ends
		}
		id := tf.ID
		if _, err := s.Prepare(ctx, tf.CompanyID, nil, PrepareInput{FormCode: spec.Code, Period: period, TaxFilingID: &id}); err != nil {
			res.Failed++
			s.logger.Warn("govfiling autopilot: prepare", "tax_filing_id", tf.ID, "form", spec.Code, "period", period, "error", err)
			continue
		}
		res.Prepared++
	}
	return res
}

func periodOf(tf store.TaxFiling) string {
	switch {
	case tf.PeriodMonth != nil:
		return fmt.Sprintf("%04d-%02d", tf.PeriodYear, *tf.PeriodMonth)
	case tf.PeriodQuarter != nil:
		return fmt.Sprintf("%04d-Q%d", tf.PeriodYear, *tf.PeriodQuarter)
	default:
		return fmt.Sprintf("%04d", tf.PeriodYear)
	}
}
