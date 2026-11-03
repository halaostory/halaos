package pdf

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

// Party is a payee or payor on BIR Form 2307.
type Party struct {
	TIN     string
	Name    string
	Address string
}

// Row2307 is one ATC line: income payments per month of the quarter and the tax withheld.
type Row2307 struct {
	Description string
	ATC         string
	Months      [3]float64
	Withheld    float64
}

// Certificate2307 is the data of a Certificate of Creditable Tax Withheld at Source.
type Certificate2307 struct {
	PeriodFrom string // MM/DD/YYYY
	PeriodTo   string
	Payee      Party
	Payor      Party
	Rows       []Row2307
	Generator  string
}

func money(f float64) string {
	if f == 0 {
		return ""
	}
	s := fmt.Sprintf("%.2f", f)
	in, dec := s[:len(s)-3], s[len(s)-2:]
	out := ""
	for i, r := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out += ","
		}
		out += string(r)
	}
	return out + "." + dec
}

// Generate2307 renders BIR Form 2307 (structure of the January 2018 ENCS version).
// The payor must sign it before issuing it to the payee.
func Generate2307(c Certificate2307) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFont("Arial", "", 8)
	pdf.CellFormat(0, 4, tr("Republic of the Philippines · Department of Finance · Bureau of Internal Revenue"), "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(0, 7, "BIR Form No. 2307", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "Certificate of Creditable Tax Withheld at Source", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("1  For the Period   From: %s     To: %s", c.PeriodFrom, c.PeriodTo)), "1", 1, "L", false, 0, "")

	part := func(title string, p Party, n int) {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(230, 230, 230)
		pdf.CellFormat(0, 6, tr(title), "1", 1, "L", true, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(55, 6, fmt.Sprintf("%d  Taxpayer Identification Number", n), "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, tr(p.TIN), "1", 1, "L", false, 0, "")
		pdf.CellFormat(55, 6, fmt.Sprintf("%d  Name", n+1), "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, tr(p.Name), "1", 1, "L", false, 0, "")
		pdf.CellFormat(55, 6, fmt.Sprintf("%d  Registered Address", n+2), "1", 0, "L", false, 0, "")
		pdf.MultiCell(0, 6, tr(p.Address), "1", "L", false)
	}
	part("Part I – Payee Information", c.Payee, 2)
	part("Part II – Payor Information", c.Payor, 6)

	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(0, 6, tr("Part III – Details of Monthly Income Payments and Taxes Withheld"), "1", 1, "L", true, 0, "")
	w := []float64{52, 16, 24, 24, 24, 24, 22}
	hdr := []string{"Income Payments Subject to EWT", "ATC", "1st Month", "2nd Month", "3rd Month", "Total", "Tax Withheld"}
	pdf.SetFont("Arial", "B", 7.5)
	for i, h := range hdr {
		pdf.CellFormat(w[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 8)
	var tm [3]float64
	var tt, tw float64
	for _, r := range c.Rows {
		total := r.Months[0] + r.Months[1] + r.Months[2]
		cells := []string{r.Description, r.ATC, money(r.Months[0]), money(r.Months[1]), money(r.Months[2]), money(total), money(r.Withheld)}
		for i, v := range cells {
			align := "R"
			if i < 2 {
				align = "L"
			}
			txt := tr(v)
			if i == 0 && pdf.GetStringWidth(txt) > w[0]-2 {
				for pdf.GetStringWidth(txt+"...") > w[0]-2 && len(txt) > 0 {
					txt = txt[:len(txt)-1]
				}
				txt += "..."
			}
			pdf.CellFormat(w[i], 6, txt, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
		for i := range tm {
			tm[i] += r.Months[i]
		}
		tt += total
		tw += r.Withheld
	}
	pdf.SetFont("Arial", "B", 8)
	for i, v := range []string{"Total", "", money(tm[0]), money(tm[1]), money(tm[2]), money(tt), money(tw)} {
		align := "R"
		if i < 2 {
			align = "L"
		}
		pdf.CellFormat(w[i], 6, v, "1", 0, align, false, 0, "")
	}
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 8)
	pdf.MultiCell(0, 4, "We declare under the penalties of perjury that this certificate has been made in good faith, verified by us, and to the best of our knowledge and belief, is true and correct, pursuant to the provisions of the National Internal Revenue Code, as amended, and the regulations issued under authority thereof.", "", "J", false)
	pdf.Ln(14)
	pdf.Line(12, pdf.GetY(), 100, pdf.GetY())
	pdf.Line(110, pdf.GetY(), 198, pdf.GetY())
	pdf.Ln(1)
	pdf.CellFormat(98, 4, tr("Payor / Authorized Representative — Signature over Printed Name"), "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 4, tr("Payee / Authorized Representative — Signature over Printed Name"), "", 1, "L", false, 0, "")
	if c.Generator != "" {
		pdf.Ln(6)
		pdf.SetFont("Arial", "I", 7)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 4, tr(c.Generator), "", 1, "L", false, 0, "")
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
