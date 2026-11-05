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
	Fields     map[string]string  `json:"fields,omitempty"` // text columns (hire date, birth date …)
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
	FieldColumns  []string           `json:"field_columns,omitempty"`
	// DueDate is set by registration forms (earliest hire + N days).
	DueDate *time.Time `json:"due_date,omitempty"`
}

// ErrNoGenerator means HalaPeople cannot build this return's data from payroll yet.
var ErrNoGenerator = errors.New("HalaPeople cannot generate this return's data yet")

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
	case "monthly", "hire_month":
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
	case "bir_0619e", "bir_1601eq":
		var start, end time.Time
		if spec.Code == "bir_0619e" {
			ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
			start = time.Date(p.Year, time.Month(p.Month), 1, 0, 0, 0, 0, time.UTC)
			end = start.AddDate(0, 1, -1)
		} else {
			ds.Period = fmt.Sprintf("%04d-Q%d", p.Year, p.Quarter)
			start = time.Date(p.Year, time.Month((p.Quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
			end = start.AddDate(0, 3, -1)
		}
		rows, err := q.ContractorWithholdingByPayee(ctx, store.ContractorWithholdingByPayeeParams{CompanyID: companyID, StartDate: start, EndDate: end})
		if err != nil {
			return nil, err
		}
		ds.MemberIDLabel, ds.Columns, ds.FieldColumns = "TIN", []string{"income_payment", "tax_withheld"}, []string{"atc"}
		agg := map[string]*Line{}
		var order []string
		for _, r := range rows {
			k := fmt.Sprintf("%d|%s", r.ContractorID, r.AtcCode)
			l, ok := agg[k]
			if !ok {
				l = &Line{Name: r.Name, MemberID: r.Tin, Amounts: map[string]float64{}, Fields: map[string]string{"atc": r.AtcCode}}
				agg[k] = l
				order = append(order, k)
			}
			l.Amounts["income_payment"] = r2(l.Amounts["income_payment"] + r.Base)
			l.Amounts["tax_withheld"] = r2(l.Amounts["tax_withheld"] + r.Ewt)
		}
		for _, k := range order {
			ds.Lines = append(ds.Lines, *agg[k])
		}
		// Due: 0619-E on the 10th of the next month; 1601-EQ on the last day of the month after the quarter.
		due := end.AddDate(0, 0, 10)
		if spec.Code == "bir_1601eq" {
			due = end.AddDate(0, 0, 1).AddDate(0, 1, -1)
		}
		ds.DueDate = &due
	case "sss_r1a", "philhealth_er2", "pagibig_registration", "bir_1902":
		ds.Period = fmt.Sprintf("%04d-%02d", p.Year, p.Month)
		if err := buildRegistration(ctx, q, companyID, spec, p, ds); err != nil {
			return nil, err
		}
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

func buildRegistration(ctx context.Context, q *store.Queries, companyID int64, spec FormSpec, p Period, ds *Dataset) error {
	start := time.Date(p.Year, time.Month(p.Month), 1, 0, 0, 0, 0, time.UTC)
	rows, err := q.ListNewHiresForRegistration(ctx, store.ListNewHiresForRegistrationParams{
		CompanyID: companyID, StartDate: start, EndDate: start.AddDate(0, 1, -1),
	})
	if err != nil {
		return err
	}
	ds.Columns = []string{"monthly_salary"}
	ds.FieldColumns = []string{"hire_date", "birth_date"}
	switch spec.Code {
	case "sss_r1a":
		ds.MemberIDLabel = "SSS number"
	case "philhealth_er2":
		ds.MemberIDLabel = "PhilHealth number"
	case "pagibig_registration":
		ds.MemberIDLabel = "Pag-IBIG MID"
	case "bir_1902":
		ds.MemberIDLabel = "" // only employees without a TIN are listed
	}
	var earliest time.Time
	for _, r := range rows {
		id := map[string]string{"sss_r1a": r.SssNo, "philhealth_er2": r.PhilhealthNo, "pagibig_registration": r.PagibigNo, "bir_1902": ""}[spec.Code]
		if spec.Code == "bir_1902" && strings.TrimSpace(r.Tin) != "" {
			continue
		}
		name := r.LastName + ", " + r.FirstName
		if r.MiddleName != nil && *r.MiddleName != "" {
			name += " " + *r.MiddleName
		}
		bd := ""
		if r.BirthDate.Valid {
			bd = r.BirthDate.Time.Format("2006-01-02")
		}
		ds.Lines = append(ds.Lines, Line{EmployeeNo: r.EmployeeNo, Name: name, MemberID: strings.TrimSpace(id),
			Amounts: map[string]float64{"monthly_salary": r2(r.MonthlySalary)},
			Fields:  map[string]string{"hire_date": r.HireDate.Format("2006-01-02"), "birth_date": bd}})
		if earliest.IsZero() || r.HireDate.Before(earliest) {
			earliest = r.HireDate
		}
	}
	if !earliest.IsZero() && spec.DueDaysAfterHire > 0 {
		d := earliest.AddDate(0, 0, spec.DueDaysAfterHire)
		ds.DueDate = &d
	}
	return nil
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
	case "sss_r1a":
		// R-1A style text: header, one line per new employee, trailer.
		var b bytes.Buffer
		fmt.Fprintf(&b, "H|%s|%s|%s\n", digits(ds.EmployerIDs["sss_er_no"]), strings.ToUpper(ds.CompanyName), safe)
		for _, l := range ds.Lines {
			fmt.Fprintf(&b, "D|%s|%s|%s|%s|%.2f\n", digits(l.MemberID), strings.ToUpper(l.Name), l.Fields["birth_date"], l.Fields["hire_date"], l.Amounts["monthly_salary"])
		}
		fmt.Fprintf(&b, "T|%d\n", len(ds.Lines))
		return fmt.Sprintf("R1A_%s_%s.txt", digits(ds.EmployerIDs["sss_er_no"]), safe), "text/plain", b.Bytes(), nil
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
		idCol := strings.ToLower(strings.ReplaceAll(ds.MemberIDLabel, " ", "_"))
		if idCol == "" {
			idCol = "member_id"
		}
		hdr := append(append([]string{"seq", "employee_no", "name", idCol}, ds.FieldColumns...), ds.Columns...)
		_ = w.Write(hdr)
		for i, l := range ds.Lines {
			row := []string{strconv.Itoa(i + 1), l.EmployeeNo, l.Name, l.MemberID}
			for _, f := range ds.FieldColumns {
				row = append(row, l.Fields[f])
			}
			for _, c := range ds.Columns {
				row = append(row, fmt.Sprintf("%.2f", l.Amounts[c]))
			}
			_ = w.Write(row)
		}
		if !spec.Registration {
			tot := []string{"", "", "TOTAL", ""}
			for _, c := range ds.Columns {
				tot = append(tot, fmt.Sprintf("%.2f", ds.Totals[c]))
			}
			_ = w.Write(tot)
		}
		w.Flush()
		prefix := map[string]string{"bir_1601c": "1601C", "bir_1604c": "1604C_ALPHALIST", "philhealth_rf1": "RF1", "mock_return": "MOCK",
			"philhealth_er2": "ER2", "pagibig_registration": "PAGIBIG_NEW_MEMBERS", "bir_1902": "BIR1902_NO_TIN",
			"bir_0619e": "0619E", "bir_1601eq": "1601EQ_QAP"}[spec.Code]
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
