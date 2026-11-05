package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// SignedDocument is everything needed to render an e-signed document with
// its audit trail.
type SignedDocument struct {
	CompanyName    string
	CompanyAddress string
	Title          string
	Body           string
	SignerName     string
	SignerEmail    string
	SignedName     string // what the signer typed
	SignaturePNG   []byte // optional drawn signature
	SignedAt       time.Time
	SignedIP       string
	UserAgent      string
	ViewedAt       *time.Time
	SentAt         time.Time
	RequestID      int64
	BodySHA256     string
}

// GenerateSignedDocument renders the document, the signature block and an
// audit-trail page (document hash, timestamps, IP, user agent).
func GenerateSignedDocument(d SignedDocument) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(22, 22, 22)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 15)
	pdf.CellFormat(0, 8, tr(d.CompanyName), "", 1, "C", false, 0, "")
	if d.CompanyAddress != "" {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(0, 5, tr(d.CompanyAddress), "", 1, "C", false, 0, "")
	}
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 13)
	pdf.MultiCell(0, 7, tr(d.Title), "", "C", false)
	pdf.Ln(4)

	pdf.SetFont("Arial", "", 10.5)
	for _, para := range strings.Split(strings.ReplaceAll(d.Body, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(para) == "" {
			pdf.Ln(3)
			continue
		}
		if strings.HasPrefix(para, "# ") || strings.HasPrefix(para, "## ") {
			pdf.SetFont("Arial", "B", 11)
			pdf.MultiCell(0, 6, tr(strings.TrimLeft(para, "# ")), "", "L", false)
			pdf.SetFont("Arial", "", 10.5)
			continue
		}
		pdf.MultiCell(0, 5.5, tr(para), "", "J", false)
	}

	// Signature block
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "Signed electronically by:", "", 1, "L", false, 0, "")
	y := pdf.GetY()
	if len(d.SignaturePNG) > 0 {
		opt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
		pdf.RegisterImageOptionsReader("sig", opt, bytes.NewReader(d.SignaturePNG))
		if pdf.Err() {
			return nil, pdf.Error()
		}
		pdf.ImageOptions("sig", 22, y, 60, 0, false, opt, 0, "")
		pdf.SetY(y + 24)
	} else {
		pdf.SetFont("Times", "I", 20)
		pdf.CellFormat(0, 12, tr(d.SignedName), "", 1, "L", false, 0, "")
	}
	pdf.SetFont("Arial", "", 10)
	pdf.Line(22, pdf.GetY(), 100, pdf.GetY())
	pdf.Ln(1)
	pdf.CellFormat(0, 5, tr(d.SignerName), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 5, "Date: "+d.SignedAt.UTC().Format("January 2, 2006 15:04 MST"), "", 1, "L", false, 0, "")

	// Audit trail
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(0, 8, "Electronic Signature Audit Trail", "", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont("Arial", "", 9.5)
	row := func(k, v string) {
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(48, 6, k, "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9.5)
		pdf.MultiCell(0, 6, tr(v), "", "L", false)
	}
	row("Document", d.Title)
	row("Request ID", fmt.Sprintf("SIG-%06d", d.RequestID))
	row("Document SHA-256", d.BodySHA256)
	row("Signer", d.SignerName)
	if d.SignerEmail != "" {
		row("Signer e-mail", d.SignerEmail)
	}
	row("Typed signature", d.SignedName)
	row("Sent", d.SentAt.UTC().Format(time.RFC3339))
	if d.ViewedAt != nil {
		row("First viewed", d.ViewedAt.UTC().Format(time.RFC3339))
	}
	row("Signed", d.SignedAt.UTC().Format(time.RFC3339))
	row("IP address", d.SignedIP)
	row("User agent", d.UserAgent)
	pdf.Ln(4)
	pdf.SetFont("Arial", "I", 8.5)
	pdf.MultiCell(0, 4.5, "The signer opened a unique, single-use link, reviewed the document above and confirmed their intent to sign by "+
		"typing their name and accepting the electronic-signature consent. The SHA-256 hash identifies the exact text that was signed. "+
		"Electronic signatures are recognised under the Philippine E-Commerce Act (RA 8792), Singapore's Electronic Transactions Act, "+
		"and the US ESIGN Act.", "", "L", false)
	addBrandingFooter(pdf)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
