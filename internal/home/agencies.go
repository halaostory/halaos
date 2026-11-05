package home

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Warp's Tax & Compliance dashboard: one row per agency (jurisdiction)
// with its account number, status, last filing, next due date, open
// notices and the last thing the agent did there.

// AgencyRow is one row of the compliance overview.
type AgencyRow struct {
	Agency
	LastFiled    *LastFiled `json:"last_filed,omitempty"`
	NextDue      *NextDue   `json:"next_due,omitempty"`
	OpenNotices  int        `json:"open_notices"`
	LastActivity *Activity  `json:"last_activity,omitempty"`
}

// LastFiled is the agency's latest submitted return.
type LastFiled struct {
	Form        string    `json:"form"`
	Period      string    `json:"period"`
	At          time.Time `json:"at"`
	ReferenceNo string    `json:"reference_no,omitempty"`
}

// NextDue is the agency's earliest open return.
type NextDue struct {
	Filing  string    `json:"filing"`
	Date    time.Time `json:"date"`
	Overdue bool      `json:"overdue"`
}

// Activity is the latest filing or notice event.
type Activity struct {
	Kind   string    `json:"kind"` // filing, notice
	Title  string    `json:"title"`
	Status string    `json:"status"`
	At     time.Time `json:"at"`
	Link   string    `json:"link"`
}

var agencyNames = map[string]string{
	"BIR": "BIR", "SSS": "SSS", "PHILHEALTH": "PhilHealth", "PAGIBIG": "Pag-IBIG",
	"IRAS": "IRAS", "CPF": "CPF", "IRS": "IRS", "BPJS": "BPJS", "EPF": "EPF", "SOCSO": "SOCSO", "LHDN": "LHDN",
}

// Agencies handles GET /home/agencies.
func (h *Handler) Agencies(c *gin.Context) {
	ctx := c.Request.Context()
	comp, err := h.q.GetCompanyByID(ctx, auth.GetCompanyID(c))
	if err != nil {
		response.NotFound(c, "Company not found")
		return
	}
	response.OK(c, h.agencyRows(ctx, comp))
}

func (h *Handler) agencyRows(ctx context.Context, comp store.Company) []AgencyRow {
	byCode := map[string]*AgencyRow{}
	var order []string
	add := func(a Agency) *AgencyRow {
		code := strings.ToUpper(a.Code)
		if r := byCode[code]; r != nil {
			return r
		}
		byCode[code] = &AgencyRow{Agency: a}
		order = append(order, code)
		return byCode[code]
	}
	if reg := h.registrations(ctx, comp); reg != nil {
		for _, a := range reg.Agencies {
			add(a)
		}
	}
	// Agencies the company deals with but that have no registration row
	// (e.g. a notice from IRAS) still get a row.
	extra := func(code string) *AgencyRow {
		code = strings.ToUpper(code)
		if r := byCode[code]; r != nil {
			return r
		}
		name := agencyNames[code]
		if name == "" {
			name = code
		}
		return add(Agency{Code: code, Name: name, Status: "active", Detail: "Seen in filings or notices", Link: "/gov-filings"})
	}
	if rows, err := h.q.AgencyLastFiled(ctx, comp.ID); err == nil {
		for _, r := range rows {
			if r.SubmittedAt.Valid {
				extra(r.Agency).LastFiled = &LastFiled{Form: r.FormCode, Period: r.Period, At: r.SubmittedAt.Time, ReferenceNo: r.ReferenceNo}
			}
		}
	}
	if rows, err := h.q.AgencyNextDue(ctx, comp.ID); err == nil {
		for _, r := range rows {
			row := extra(r.Agency)
			row.NextDue = &NextDue{Filing: r.FilingType, Date: r.DueDate, Overdue: r.Overdue}
			if r.Overdue && row.Status == "active" {
				row.Status, row.Detail = "needs_info", "A return is overdue"
			}
		}
	}
	if rows, err := h.q.AgencyOpenNotices(ctx, comp.ID); err == nil {
		for _, r := range rows {
			row := extra(r.Agency)
			row.OpenNotices = int(r.N)
			if row.Status == "active" {
				row.Status, row.Detail = "needs_info", plural(int(r.N), "open notice", "open notices")
			}
		}
	}
	if rows, err := h.q.AgencyLatestActivity(ctx, comp.ID); err == nil {
		for _, r := range rows {
			link := "/gov-filings"
			if r.Kind == "notice" {
				link = "/notices"
			}
			extra(r.Agency).LastActivity = &Activity{Kind: r.Kind, Title: r.Title, Status: r.Status, At: r.At, Link: link}
		}
	}
	out := make([]AgencyRow, 0, len(order))
	for _, code := range order {
		out = append(out, *byCode[code])
	}
	// Agencies needing attention first, then registration order.
	rank := map[string]int{"needs_info": 0, "needs_approval": 1, "filing_started": 2, "awaiting_agency": 3, "active": 4}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Status] < rank[out[j].Status] })
	return out
}
