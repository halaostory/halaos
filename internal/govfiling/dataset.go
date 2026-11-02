package govfiling

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/halaostory/halaos/internal/compliance"
	"github.com/halaostory/halaos/internal/store"
)

// Line is one employee row of a return.
type Line struct {
	EmployeeNo string             `json:"employee_no"`
	Name       string             `json:"name"`
	MemberID   string             `json:"member_id"`
	Amounts    map[string]float64 `json:"amounts"`
}

// Dataset is the normalized content of a return, independent of file layout.
type Dataset struct {
	FormCode      string             `json:"form_code"`
	Period        string             `json:"period"`
	CompanyName   string             `json:"company_name"`
	EmployerIDs   map[string]string  `json:"employer_ids"`
	MemberIDLabel string             `json:"member_id_label"`
	Columns       []string           `json:"columns"`
	Lines         []Line             `json:"lines"`
	Totals        map[string]float64 `json:"totals"`
}

// ErrNoGenerator means HalaOS cannot build this return's data from payroll yet.
var ErrNoGenerator = errors.New("HalaOS cannot generate this return's data yet")

// Period is a parsed filing period.
type Period struct {
	Year    int
	Month   int    // 1-12 for monthly
	Quarter int    // 1-4 for quarterly
	State   string // two-letter US state for per-state registrations
}

var (
	reMonth   = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
	reQuarter = regexp.MustCompile(`^(\d{4})-Q([1-4])$`)
	reYear    = regexp.MustCompile(`^(\d{4})$`)
	reState   = regexp.MustCompile(`^[A-Z]{2}$`)
)

// ParsePeriod parses "2026-10", "2026-Q3" or "2026" according to frequency.
func ParsePeriod(frequency, s string) (Period, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch frequency {
	case "monthly":
		m := reMonth.FindStringSubmatch(s)
		if m == nil {
			return Period{}, fmt.Errorf("period must be YYYY-MM")
		}
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		if mo < 1 || mo > 12 {
			return Period{}, fmt.Errorf("invalid month")
		}
		return Period{Year: y, Month: mo}, nil
	case "per_state":
		if !reState.MatchString(s) {
			return Period{}, fmt.Errorf("period must be a two-letter state code")
		}
		return Period{State: s}, nil
	case "quarterly":
		m := reQuarter.FindStringSubmatch(s)
		if m == nil {
			return Period{}, fmt.Errorf("period must be YYYY-Qn")
		}
		y, _ := strconv.Atoi(m[1])
		q, _ := strconv.Atoi(m[2])
		return Period{Year: y, Quarter: q}, nil
	default: // annual, once
		m := reYear.FindStringSubmatch(s)
		if m == nil {
			return Period{}, fmt.Errorf("period must be YYYY")
		}
		y, _ := strconv.Atoi(m[1])
		return Period{Year: y}, nil
	}
}

// End returns the last day covered by the period.
func (p Period) End() time.Time {
	switch {
	case p.Month > 0:
		return time.Date(p.Year, time.Month(p.Month)+1, 0, 0, 0, 0, 0, time.UTC)
	case p.Quarter > 0:
		return time.Date(p.Year, time.Month(p.Quarter*3)+1, 0, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(p.Year, 12, 31, 0, 0, 0, 0, time.UTC)
	}
}

func r2(f float64) float64 { return math.Round(f*100) / 100 }

func sortLines(ls []Line) {
	sort.Slice(ls, func(i, j int) bool {
		if ls[i].Name != ls[j].Name {
			return ls[i].Name < ls[j].Name
		}
		return ls[i].EmployeeNo < ls[j].EmployeeNo
	})
}

func companyIDs(c store.Company) map[string]string {
	v := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	return map[string]string{
		"tin": v(c.Tin), "bir_rdo": v(c.BirRdo), "sss_er_no": v(c.SssErNo),
		"philhealth_er_no": v(c.PhilhealthErNo), "pagibig_er_no": v(c.PagibigErNo),
	}
}

// BuildDataset loads the return's data from payroll.
func BuildDataset(ctx context.Context, q *store.Queries, companyID int64, spec FormSpec, p Period) (*Dataset, error) {
	company, err := q.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("company: %w", err)
	}
	fg := compliance.NewFormGenerator(q)
	ds := &Dataset{FormCode: spec.Code, CompanyName: company.Name, EmployerIDs: companyIDs(company), Totals: map[string]float64{}}
	switch spec.Code {
	case "bir_1601c":
		ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
		f, err := fg.GenerateBIR1601C(ctx, companyID, int32(p.Year), p.Month)
		if err != nil {
			return nil, err
		}
		ds.MemberIDLabel, ds.Columns = "TIN", []string{"gross_pay", "taxable_income", "tax_withheld"}
		for _, e := range f.Employees {
			ds.Lines = append(ds.Lines, Line{EmployeeNo: e.EmployeeNo, Name: e.FullName, MemberID: e.TIN,
				Amounts: map[string]float64{"gross_pay": e.GrossPay, "taxable_income": e.TaxableIncome, "tax_withheld": e.TaxWithheld}})
		}
	case "bir_1604c":
		ds.Period = strconv.Itoa(p.Year)
		f, err := fg.GenerateBIR2316(ctx, companyID, int32(p.Year))
		if err != nil {
			return nil, err
		}
		ds.MemberIDLabel = "TIN"
		ds.Columns = []string{"gross_pay", "thirteenth_month", "sss", "philhealth", "pagibig", "tax_withheld"}
		for _, e := range f.Employees {
			ds.Lines = append(ds.Lines, Line{EmployeeNo: e.EmployeeNo, Name: e.FullName, MemberID: e.TIN, Amounts: map[string]float64{
				"gross_pay": e.TotalGrossPay, "thirteenth_month": e.ThirteenthMonth, "sss": e.TotalSSS,
				"philhealth": e.TotalPhilHealth, "pagibig": e.TotalPagIBIG, "tax_withheld": e.TotalTaxWithheld}})
		}
	case "sss_r3":
		ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
		f, err := fg.GenerateSSSR3(ctx, companyID, int32(p.Year), p.Month)
		if err != nil {
			return nil, err
		}
		ds.MemberIDLabel, ds.Columns = "SSS number", []string{"ee", "er", "ec"}
		for _, e := range f.Employees {
			ds.Lines = append(ds.Lines, Line{Name: e.FullName, MemberID: e.SSSNo, Amounts: map[string]float64{"ee": e.EEShare, "er": e.ERShare, "ec": e.EC}})
		}
	case "philhealth_rf1":
		ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
		f, err := fg.GeneratePhilHealthRF1(ctx, companyID, int32(p.Year), p.Month)
		if err != nil {
			return nil, err
		}
		ds.MemberIDLabel, ds.Columns = "PhilHealth number", []string{"ee", "er"}
		for _, e := range f.Employees {
			ds.Lines = append(ds.Lines, Line{Name: e.FullName, MemberID: e.PhilHealthNo, Amounts: map[string]float64{"ee": e.EEShare, "er": e.ERShare}})
		}
	case "pagibig_mcrf":
		ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
		f, err := fg.GeneratePagibigMCRF(ctx, companyID, int32(p.Year), p.Month)
		if err != nil {
			return nil, err
		}
		ds.MemberIDLabel, ds.Columns = "Pag-IBIG MID", []string{"ee", "er"}
		for _, e := range f.Employees {
			ds.Lines = append(ds.Lines, Line{EmployeeNo: e.EmployeeNo, Name: e.LastName + ", " + e.FirstName, MemberID: e.PagibigNo,
				Amounts: map[string]float64{"ee": e.EEShare, "er": e.ERShare}})
		}
	case "irs_941":
		ds.Period = fmt.Sprintf("%04d-Q%d", p.Year, p.Quarter)
		f, err := fg.Generate941(ctx, companyID, int32(p.Year), p.Quarter)
		if err != nil {
			return nil, err
		}
		ds.Totals = map[string]float64{
			"employee_count": float64(f.EmployeeCount), "line2_wages": f.TotalWages, "line3_federal_withheld": f.FederalWithheld,
			"line5a_ss_wages": f.SSWages, "line5c_medicare_wages": f.MedicareWages, "line10_total_taxes": f.TotalTaxes,
		}
		return ds, nil
	case "mock_return":
		ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
		ds.MemberIDLabel, ds.Columns = "Employee no.", []string{"amount"}
		ds.Lines = []Line{{EmployeeNo: "DEMO-1", Name: "Demo Employee", MemberID: "DEMO-1", Amounts: map[string]float64{"amount": 100}}}
	default:
		return nil, fmt.Errorf("%w: %s", ErrNoGenerator, spec.Code)
	}
	if ds.Lines == nil {
		ds.Lines = []Line{}
	}
	sortLines(ds.Lines)
	for _, l := range ds.Lines {
		for k, v := range l.Amounts {
			ds.Totals[k] = r2(ds.Totals[k] + v)
		}
	}
	return ds, nil
}

// RenderFile turns the dataset into the upload file. Layouts follow each
// agency's published column order; the agency's own validator remains the
// authority, which preflight reminds the reviewer about.
func RenderFile(spec FormSpec, ds *Dataset) (name, contentType string, data []byte, err error) {
	safe := strings.NewReplacer("-", "", " ", "").Replace(ds.Period)
	switch spec.Code {
	case "irs_941":
		data, err = json.MarshalIndent(ds, "", "  ")
		return fmt.Sprintf("941_%s.json", safe), "application/json", data, err
	case "sss_r3":
		// R-3 style text: header + one pipe-delimited line per member + trailer.
		var b bytes.Buffer
		fmt.Fprintf(&b, "H|%s|%s|%s\n", ds.EmployerIDs["sss_er_no"], strings.ToUpper(ds.CompanyName), safe)
		for _, l := range ds.Lines {
			fmt.Fprintf(&b, "D|%s|%s|%.2f|%.2f|%.2f\n", digits(l.MemberID), strings.ToUpper(l.Name), l.Amounts["ee"], l.Amounts["er"], l.Amounts["ec"])
		}
		fmt.Fprintf(&b, "T|%d|%.2f|%.2f|%.2f\n", len(ds.Lines), ds.Totals["ee"], ds.Totals["er"], ds.Totals["ec"])
		return fmt.Sprintf("R3_%s_%s.txt", digits(ds.EmployerIDs["sss_er_no"]), safe), "text/plain", b.Bytes(), nil
	case "pagibig_mcrf":
		var b bytes.Buffer
		fmt.Fprintf(&b, "%s|%s|%s\n", digits(ds.EmployerIDs["pagibig_er_no"]), strings.ToUpper(ds.CompanyName), safe)
		for _, l := range ds.Lines {
			fmt.Fprintf(&b, "%s|%s|%s|%.2f|%.2f\n", digits(l.MemberID), l.EmployeeNo, strings.ToUpper(l.Name), l.Amounts["ee"], l.Amounts["er"])
		}
		return fmt.Sprintf("MCRF_%s.txt", safe), "text/plain", b.Bytes(), nil
	default:
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		hdr := append([]string{"seq", "employee_no", "name", strings.ToLower(strings.ReplaceAll(ds.MemberIDLabel, " ", "_"))}, ds.Columns...)
		_ = w.Write(hdr)
		for i, l := range ds.Lines {
			row := []string{strconv.Itoa(i + 1), l.EmployeeNo, l.Name, l.MemberID}
			for _, c := range ds.Columns {
				row = append(row, fmt.Sprintf("%.2f", l.Amounts[c]))
			}
			_ = w.Write(row)
		}
		tot := []string{"", "", "TOTAL", ""}
		for _, c := range ds.Columns {
			tot = append(tot, fmt.Sprintf("%.2f", ds.Totals[c]))
		}
		_ = w.Write(tot)
		w.Flush()
		prefix := map[string]string{"bir_1601c": "1601C", "bir_1604c": "1604C_ALPHALIST", "philhealth_rf1": "RF1", "mock_return": "MOCK"}[spec.Code]
		if prefix == "" {
			prefix = strings.ToUpper(spec.Code)
		}
		return fmt.Sprintf("%s_%s.csv", prefix, safe), "text/csv", b.Bytes(), w.Error()
	}
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
