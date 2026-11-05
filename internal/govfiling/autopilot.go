package govfiling

import (
	"context"
	"fmt"
	"time"

	"github.com/halaostory/halaos/internal/store"
)

// AutopilotResult summarises one autopilot pass.
type AutopilotResult struct {
	Prepared      int `json:"prepared"`
	Failed        int `json:"failed"`
	Registrations int `json:"registrations"`
	NewHireForms  int `json:"new_hire_forms"`
	// Companies holds what the pass did (or, at "recommend" autonomy, would
	// have done) per company, for the agent's routine run log.
	Companies map[int64]*CompanyActivity `json:"companies,omitempty"`
}

// CompanyActivity is one company's share of an autopilot pass.
type CompanyActivity struct {
	Prepared []AutopilotItem `json:"prepared"`
	Proposed []AutopilotItem `json:"proposed"`
	Failed   []AutopilotItem `json:"failed"`
}

// AutopilotItem names one return.
type AutopilotItem struct {
	FormCode     string `json:"form_code"`
	Name         string `json:"name"`
	Agency       string `json:"agency"`
	Period       string `json:"period"`
	SubmissionID int64  `json:"submission_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Autonomy levels the autopilot understands (see internal/routines).
const (
	AutonomyOff       = "off"       // routine disabled: do nothing
	AutonomyRecommend = "recommend" // list what is due, prepare nothing
)

// SetPolicy lets the agent routines decide, per company, whether the
// autopilot prepares returns ("approve"/"autonomous"), only recommends them
// ("recommend") or stays out ("off"). Submission always needs approval.
func (s *Service) SetPolicy(fn func(ctx context.Context, companyID int64) string) { s.policy = fn }

func (s *Service) autonomy(ctx context.Context, companyID int64) string {
	if s.policy == nil {
		return "approve"
	}
	return s.policy(ctx, companyID)
}

// autoPrepare prepares one return on the autopilot's behalf, honouring the
// company's autonomy and recording the outcome.
func (s *Service) autoPrepare(ctx context.Context, res *AutopilotResult, companyID int64, in PrepareInput) bool {
	spec, _ := Lookup(in.FormCode)
	item := AutopilotItem{FormCode: spec.Code, Name: spec.Name, Agency: spec.Agency, Period: in.Period}
	if res.Companies == nil {
		res.Companies = map[int64]*CompanyActivity{}
	}
	act := res.Companies[companyID]
	if act == nil {
		act = &CompanyActivity{}
		res.Companies[companyID] = act
	}
	if s.autonomy(ctx, companyID) == AutonomyRecommend {
		act.Proposed = append(act.Proposed, item)
		return false
	}
	p, err := s.Prepare(ctx, companyID, nil, in)
	if err != nil {
		item.Error = err.Error()
		act.Failed = append(act.Failed, item)
		return false
	}
	item.SubmissionID = p.Submission.ID
	act.Prepared = append(act.Prepared, item)
	return true
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
		if s.autonomy(ctx, c.ID) == AutonomyOff {
			continue
		}
		switch c.Country {
		case "PHL":
			if err := s.q.GenerateAnnualFilings(ctx, store.GenerateAnnualFilingsParams{CompanyID: c.ID, PeriodYear: year}); err != nil {
				s.logger.Warn("govfiling autopilot: annual calendar", "company_id", c.ID, "error", err)
			}
			res.NewHireForms += s.prepareNewHireRegistrations(ctx, &res, c.ID)
			res.Prepared += s.prepareExpandedWithholding(ctx, &res, c.ID)
		case "USA":
			states, err := s.q.ListUnregisteredWorkStates(ctx, c.ID)
			if err != nil {
				continue
			}
			for _, st := range states {
				if s.autoPrepare(ctx, &res, c.ID, PrepareInput{FormCode: "us_state_registration", Period: st, Channel: string(ChannelManual)}) {
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
		if s.autonomy(ctx, tf.CompanyID) == AutonomyOff {
			continue
		}
		id := tf.ID
		if !s.autoPrepare(ctx, &res, tf.CompanyID, PrepareInput{FormCode: spec.Code, Period: period, TaxFilingID: &id}) {
			if a := res.Companies[tf.CompanyID]; a != nil && len(a.Failed) > 0 && a.Failed[len(a.Failed)-1].FormCode == spec.Code {
				res.Failed++
				s.logger.Warn("govfiling autopilot: prepare", "tax_filing_id", tf.ID, "form", spec.Code, "period", period, "error", a.Failed[len(a.Failed)-1].Error)
			}
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

// registrationForms are prepared automatically for new hires (Philippines).
var registrationForms = []string{"sss_r1a", "philhealth_er2", "pagibig_registration", "bir_1902"}

// prepareNewHireRegistrations prepares agency registrations for employees
// hired this month and last month. Months without new hires are skipped; a
// draft blocked by missing IDs is refreshed at most once a day so IDs that
// new hires enter through the onboarding portal are picked up.
func (s *Service) prepareNewHireRegistrations(ctx context.Context, res *AutopilotResult, companyID int64) int {
	now := s.now()
	months := []Period{
		{Year: now.Year(), Month: int(now.Month())},
	}
	prev := now.AddDate(0, -1, 0)
	months = append(months, Period{Year: prev.Year(), Month: int(prev.Month())})
	n := 0
	for _, code := range registrationForms {
		spec, _ := Lookup(code)
		for _, p := range months {
			period := fmt.Sprintf("%04d-%02d", p.Year, p.Month)
			live, err := s.q.GetLiveGovSubmission(ctx, store.GetLiveGovSubmissionParams{CompanyID: companyID, Agency: spec.Agency, FormCode: code, Period: period})
			if err == nil && !(live.Status == "draft" && now.Sub(live.CreatedAt) > 24*time.Hour) {
				continue // already in progress (or a fresh draft)
			}
			ds, err := BuildDataset(ctx, s.q, companyID, spec, p)
			if err != nil || len(ds.Lines) == 0 {
				continue
			}
			if !s.autoPrepare(ctx, res, companyID, PrepareInput{FormCode: code, Period: period}) {
				continue
			}
			n++
		}
	}
	return n
}

// prepareExpandedWithholding prepares 0619-E for the month that just ended
// (1st/2nd month of a quarter) and 1601-EQ + QAP for the quarter that just
// ended, when the company paid contractors with withholding in that period.
func (s *Service) prepareExpandedWithholding(ctx context.Context, res *AutopilotResult, companyID int64) int {
	now := s.now()
	prev := now.AddDate(0, -1, 0)
	type job struct {
		code   string
		period string
		p      Period
	}
	var jobs []job
	if m := int(prev.Month()); (m-1)%3 != 2 { // not the quarter's last month
		jobs = append(jobs, job{"bir_0619e", fmt.Sprintf("%04d-%02d", prev.Year(), m), Period{Year: prev.Year(), Month: m}})
	}
	if int(now.Month())%3 == 1 { // first month after a quarter
		q := (int(prev.Month())-1)/3 + 1
		jobs = append(jobs, job{"bir_1601eq", fmt.Sprintf("%04d-Q%d", prev.Year(), q), Period{Year: prev.Year(), Quarter: q}})
	}
	n := 0
	for _, j := range jobs {
		spec, _ := Lookup(j.code)
		if _, err := s.q.GetLiveGovSubmission(ctx, store.GetLiveGovSubmissionParams{CompanyID: companyID, Agency: spec.Agency, FormCode: j.code, Period: j.period}); err == nil {
			continue
		}
		ds, err := BuildDataset(ctx, s.q, companyID, spec, j.p)
		if err != nil || len(ds.Lines) == 0 {
			continue
		}
		if !s.autoPrepare(ctx, res, companyID, PrepareInput{FormCode: j.code, Period: j.period}) {
			continue
		}
		n++
	}
	return n
}

// PrepareNewHireRegistrations runs only the new-hire registration step of
// the autopilot for one company (used by the "New hire" routine right after
// a hire, instead of waiting for the next hourly pass).
func (s *Service) PrepareNewHireRegistrations(ctx context.Context, companyID int64) *CompanyActivity {
	var res AutopilotResult
	s.prepareNewHireRegistrations(ctx, &res, companyID)
	if a := res.Companies[companyID]; a != nil {
		return a
	}
	return &CompanyActivity{}
}
