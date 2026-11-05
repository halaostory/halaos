// Package contractors handles freelancers and contractors paid against
// invoices: Philippine expanded withholding tax (EWT), approval and payment,
// bank files, BIR 2307 certificates and data for 0619-E / 1601-EQ.
package contractors

import (
	"fmt"
	"math"
	"strings"
)

// ATC is a BIR alphanumeric tax code for expanded withholding.
type ATC struct {
	Code        string  `json:"code"`
	Description string  `json:"description"`
	Rate        float64 `json:"rate"`
	Kind        string  `json:"kind"` // individual, company
}

// ATCs lists the codes HalaPeople supports (RR 2-98 as amended, TRAIN).
var ATCs = []ATC{
	{"WI010", "Professional fees — individual, gross income ≤ ₱3M", 0.05, "individual"},
	{"WI011", "Professional fees — individual, gross income > ₱3M or no sworn declaration", 0.10, "individual"},
	{"WC010", "Professional fees — juridical person, gross income ≤ ₱720K", 0.10, "company"},
	{"WC011", "Professional fees — juridical person, gross income > ₱720K", 0.15, "company"},
	{"WI120", "Income payments to contractors (services) — individual", 0.02, "individual"},
	{"WC120", "Income payments to contractors (services) — juridical person", 0.02, "company"},
	{"WI100", "Rentals — individual", 0.05, "individual"},
	{"WC100", "Rentals — juridical person", 0.05, "company"},
}

// LookupATC finds an ATC by code.
func LookupATC(code string) (ATC, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, a := range ATCs {
		if a.Code == code {
			return a, true
		}
	}
	return ATC{}, false
}

// DefaultATC is the usual code for a payee kind (professional fees).
func DefaultATC(kind string) ATC {
	if kind == "company" {
		a, _ := LookupATC("WC010")
		return a
	}
	a, _ := LookupATC("WI010")
	return a
}

// Breakdown is the computed withholding for one payment.
type Breakdown struct {
	Gross float64 `json:"gross_amount"`
	VAT   float64 `json:"vat_amount"`
	Base  float64 `json:"ewt_base"`
	Rate  float64 `json:"ewt_rate"`
	EWT   float64 `json:"ewt_amount"`
	Net   float64 `json:"net_amount"`
}

func r2(f float64) float64 { return math.Round(f*100) / 100 }

// Compute applies EWT to an invoice amount. For VAT-registered payees the
// tax base excludes the 12% VAT (gross / 1.12); otherwise it is the gross.
func Compute(gross, rate float64, vatRegistered bool) (Breakdown, error) {
	if gross <= 0 {
		return Breakdown{}, fmt.Errorf("amount must be positive")
	}
	if rate < 0 || rate > 0.5 {
		return Breakdown{}, fmt.Errorf("invalid withholding rate")
	}
	b := Breakdown{Gross: r2(gross), Rate: rate}
	if vatRegistered {
		b.Base = r2(gross / 1.12)
		b.VAT = r2(b.Gross - b.Base)
	} else {
		b.Base = b.Gross
	}
	b.EWT = r2(b.Base * rate)
	b.Net = r2(b.Gross - b.EWT)
	return b, nil
}

// ValidTIN reports whether a PH TIN has 9–14 digits.
func ValidTIN(tin string) bool {
	n := 0
	for _, r := range tin {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n >= 9 && n <= 14
}
