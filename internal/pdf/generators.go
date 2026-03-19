package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/tonypk/aigonhr/internal/store"
)

// GenerateCOE creates a Certificate of Employment PDF.
func GenerateCOE(comp store.Company, emp store.GetEmployeeForCOERow) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(25, 25, 25)

	companyName := comp.Name
	if comp.LegalName != nil && *comp.LegalName != "" {
		companyName = *comp.LegalName
	}

	writeCompanyHeader(pdf, comp, companyName)
	pdf.Ln(15)

	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "CERTIFICATE OF EMPLOYMENT", "", 1, "C", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, time.Now().Format("January 02, 2006"), "", 1, "R", false, 0, "")
	pdf.Ln(5)

	pdf.CellFormat(160, 7, "TO WHOM IT MAY CONCERN:", "", 1, "L", false, 0, "")
	pdf.Ln(5)

	fullName := buildFullName(emp.FirstName, emp.MiddleName, emp.LastName)

	body := fmt.Sprintf(
		"This is to certify that %s has been employed with %s since %s",
		fullName, companyName, emp.HireDate.Format("January 02, 2006"),
	)
	if emp.Status == "active" {
		body += " up to the present."
	} else {
		body += "."
	}
	pdf.MultiCell(160, 6, body, "", "L", false)
	pdf.Ln(3)

	if emp.PositionTitle != "" || emp.DepartmentName != "" {
		var detail string
		if emp.PositionTitle != "" && emp.DepartmentName != "" {
			detail = fmt.Sprintf(
				"During the period of employment, %s held the position of %s under the %s department.",
				fullName, emp.PositionTitle, emp.DepartmentName,
			)
		} else if emp.PositionTitle != "" {
			detail = fmt.Sprintf("During the period of employment, %s held the position of %s.", fullName, emp.PositionTitle)
		} else {
			detail = fmt.Sprintf("During the period of employment, %s was assigned to the %s department.", fullName, emp.DepartmentName)
		}
		pdf.MultiCell(160, 6, detail, "", "L", false)
		pdf.Ln(3)
	}

	empType := emp.EmploymentType
	if len(empType) > 0 {
		empType = strings.ToUpper(empType[:1]) + empType[1:]
	}
	pdf.MultiCell(160, 6, fmt.Sprintf("Employment type: %s.", empType), "", "L", false)
	pdf.Ln(3)

	pdf.MultiCell(160, 6, "This certificate is issued upon request for whatever legal purpose it may serve.", "", "L", false)
	pdf.Ln(25)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Authorized Signatory", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")

	pdf.Ln(15)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(160, 5, "This is a system-generated document.", "", 1, "C", false, 0, "")

	addBrandingFooter(pdf)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateLetter creates a letter PDF (NTE, COEC, clearance, memo).
func GenerateLetter(comp store.Company, emp store.GetEmployeeForCOERow, letterType, subject, body, violations, deadline string, salary float64) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(25, 25, 25)

	companyName := comp.Name
	if comp.LegalName != nil && *comp.LegalName != "" {
		companyName = *comp.LegalName
	}

	fullName := buildFullName(emp.FirstName, emp.MiddleName, emp.LastName)

	writeCompanyHeader(pdf, comp, companyName)
	pdf.Ln(10)

	switch letterType {
	case "nte":
		generateNTE(pdf, companyName, fullName, emp, subject, violations, deadline)
	case "coec":
		generateCOEC(pdf, companyName, fullName, emp, salary)
	case "clearance":
		generateClearanceLetter(pdf, companyName, fullName, emp)
	case "memo":
		generateMemo(pdf, companyName, fullName, emp, subject, body)
	default:
		return nil, fmt.Errorf("unsupported letter type: %s", letterType)
	}

	pdf.Ln(15)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(160, 5, "This is a system-generated document.", "", 1, "C", false, 0, "")

	addBrandingFooter(pdf)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateDOLERegister creates a DOLE Employee Register PDF.
func GenerateDOLERegister(comp store.Company, emps []store.ListEmployeesForDOLERegisterRow) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "Legal", "")
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()

	companyName := comp.Name

	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, companyName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, "DOLE Employee Register", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("As of %s", time.Now().Format("January 2, 2006")), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	colWidths := []float64{15, 25, 40, 15, 20, 18, 18, 20, 30, 30, 22, 22, 22, 22}
	headers := []string{"No.", "Emp No", "Name", "Sex", "Birth Date", "Civil St.", "Nationality", "Hire Date", "Department", "Position", "TIN", "SSS", "PhilHealth", "Pag-IBIG"}

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 6.5)
	pdf.SetFillColor(245, 245, 245)
	for idx, emp := range emps {
		fill := idx%2 == 1

		name := emp.LastName + ", " + emp.FirstName
		if emp.MiddleName != nil && *emp.MiddleName != "" {
			name += " " + string((*emp.MiddleName)[0]) + "."
		}

		gender := ""
		if emp.Gender != nil {
			g := strings.ToUpper(*emp.Gender)
			if len(g) > 0 {
				gender = string(g[0])
			}
		}

		birthDate := ""
		if emp.BirthDate.Valid {
			birthDate = emp.BirthDate.Time.Format("01/02/2006")
		}

		civilStatus := ""
		if emp.CivilStatus != nil {
			civilStatus = *emp.CivilStatus
		}

		nationality := ""
		if emp.Nationality != nil {
			nationality = *emp.Nationality
		}

		row := []string{
			fmt.Sprintf("%d", idx+1),
			emp.EmployeeNo,
			name,
			gender,
			birthDate,
			civilStatus,
			nationality,
			emp.HireDate.Format("01/02/2006"),
			emp.DepartmentName,
			emp.PositionTitle,
			emp.Tin,
			emp.SssNo,
			emp.PhilhealthNo,
			emp.PagibigNo,
		}

		for i, val := range row {
			pdf.CellFormat(colWidths[i], 5, val, "1", 0, "L", fill, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Employees: %d", len(emps)), "", 1, "L", false, 0, "")

	pdf.Ln(15)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(100, 6, "Prepared by: ________________________________", "", 0, "L", false, 0, "")
	pdf.CellFormat(100, 6, "Noted by: ________________________________", "", 1, "L", false, 0, "")

	addBrandingFooter(pdf)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- internal helpers ---

// addBrandingFooter adds a "Powered by HalaOS" footer to every page.
func addBrandingFooter(pdf *fpdf.Fpdf) {
	totalPages := pdf.PageCount()
	for i := 1; i <= totalPages; i++ {
		pdf.SetPage(i)
		_, pageH := pdf.GetPageSize()
		pdf.SetY(pageH - 10)
		pdf.SetFont("Arial", "", 7)
		pdf.SetTextColor(160, 160, 160)
		pdf.CellFormat(0, 4, "Powered by HalaOS | halaos.com", "", 0, "C", false, 0, "https://halaos.com")
		pdf.SetTextColor(0, 0, 0)
	}
}

func writeCompanyHeader(pdf *fpdf.Fpdf, comp store.Company, companyName string) {
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(160, 10, companyName, "", 1, "C", false, 0, "")

	if comp.Address != nil {
		pdf.SetFont("Arial", "", 10)
		addr := *comp.Address
		if comp.City != nil {
			addr += ", " + *comp.City
		}
		if comp.Province != nil {
			addr += ", " + *comp.Province
		}
		pdf.CellFormat(160, 5, addr, "", 1, "C", false, 0, "")
	}
	if comp.Tin != nil {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(160, 5, "TIN: "+*comp.Tin, "", 1, "C", false, 0, "")
	}
}

func buildFullName(firstName string, middleName *string, lastName string) string {
	fullName := strings.TrimSpace(firstName + " " + lastName)
	if middleName != nil && *middleName != "" {
		fullName = strings.TrimSpace(firstName + " " + *middleName + " " + lastName)
	}
	return fullName
}

func generateNTE(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow, subject, violations, deadline string) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "NOTICE TO EXPLAIN", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, "Date: "+time.Now().Format("January 02, 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "To:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, fullName, "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "Dept:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, emp.DepartmentName, "", 1, "L", false, 0, "")
	pdf.Ln(5)

	if subject != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(160, 7, "RE: "+subject, "", 1, "L", false, 0, "")
		pdf.Ln(3)
	}

	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(160, 6, "Dear "+emp.FirstName+",", "", "L", false)
	pdf.Ln(3)

	intro := "This is to formally notify you that you are being required to explain the following matter(s) which may constitute a violation of company policy:"
	pdf.MultiCell(160, 6, intro, "", "L", false)
	pdf.Ln(3)

	if violations != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(160, 6, "Alleged Violation(s):", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.MultiCell(160, 6, violations, "", "L", false)
		pdf.Ln(3)
	}

	pdf.SetFont("Arial", "", 11)
	responseText := "You are hereby given the opportunity to explain your side in writing."
	if deadline != "" {
		responseText += " Please submit your written explanation on or before " + deadline + "."
	} else {
		responseText += " Please submit your written explanation within five (5) calendar days from receipt of this notice."
	}
	pdf.MultiCell(160, 6, responseText, "", "L", false)
	pdf.Ln(3)

	pdf.MultiCell(160, 6, "Failure to respond within the given period shall be construed as a waiver of your right to be heard, and management will proceed to resolve the matter based on available evidence.", "", "L", false)

	pdf.Ln(20)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Human Resources Department", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")
}

func generateCOEC(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow, salary float64) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "CERTIFICATE OF EMPLOYMENT", "", 1, "C", false, 0, "")
	pdf.CellFormat(160, 8, "WITH COMPENSATION", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, time.Now().Format("January 02, 2006"), "", 1, "R", false, 0, "")
	pdf.Ln(3)

	pdf.CellFormat(160, 7, "TO WHOM IT MAY CONCERN:", "", 1, "L", false, 0, "")
	pdf.Ln(5)

	body := fmt.Sprintf(
		"This is to certify that %s has been employed with %s since %s",
		fullName, companyName, emp.HireDate.Format("January 02, 2006"),
	)
	if emp.Status == "active" {
		body += " up to the present."
	} else {
		body += "."
	}
	pdf.MultiCell(160, 6, body, "", "L", false)
	pdf.Ln(3)

	if emp.PositionTitle != "" {
		pdf.MultiCell(160, 6, fmt.Sprintf("Position: %s", emp.PositionTitle), "", "L", false)
	}
	if emp.DepartmentName != "" {
		pdf.MultiCell(160, 6, fmt.Sprintf("Department: %s", emp.DepartmentName), "", "L", false)
	}
	pdf.Ln(3)

	if salary > 0 {
		salaryStr := fmt.Sprintf("PHP %.2f", salary)
		pdf.MultiCell(160, 6, fmt.Sprintf("Current monthly compensation: %s", salaryStr), "", "L", false)
		pdf.Ln(3)
	}

	pdf.MultiCell(160, 6, "This certificate is issued upon request for whatever legal purpose it may serve.", "", "L", false)

	pdf.Ln(20)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Authorized Signatory", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")
}

func generateClearanceLetter(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "EMPLOYEE CLEARANCE CERTIFICATE", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, "Date: "+time.Now().Format("January 02, 2006"), "", 1, "R", false, 0, "")
	pdf.Ln(3)

	pdf.MultiCell(160, 6, fmt.Sprintf(
		"This is to certify that %s (Employee No. %s) has been cleared of all accountabilities and obligations with %s.",
		fullName, emp.EmployeeNo, companyName,
	), "", "L", false)
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 8, "Clearance Checklist:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)

	items := []string{
		"Company Property (ID, equipment, keys)",
		"Outstanding Cash Advances / Loans",
		"Pending Work Assignments",
		"IT Accounts & Access Deactivation",
		"Final Pay Computation",
	}
	for _, item := range items {
		pdf.CellFormat(8, 6, "", "1", 0, "C", false, 0, "")
		pdf.CellFormat(152, 6, "  "+item, "", 1, "L", false, 0, "")
	}

	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(80, 6, "________________________________", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "________________________________", "", 1, "C", false, 0, "")
	pdf.CellFormat(80, 6, "Department Head", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "HR Department", "", 1, "C", false, 0, "")

	pdf.Ln(10)
	pdf.CellFormat(80, 6, "________________________________", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "________________________________", "", 1, "C", false, 0, "")
	pdf.CellFormat(80, 6, "Finance / Accounting", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "IT Department", "", 1, "C", false, 0, "")

	addBrandingFooter(pdf)
}

func generateMemo(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow, subject, body string) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "MEMORANDUM", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, "Date: "+time.Now().Format("January 02, 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "To:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, fullName+" ("+emp.EmployeeNo+")", "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "From:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, "Human Resources Department", "", 1, "L", false, 0, "")

	if subject != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(20, 6, "RE:", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.CellFormat(140, 6, subject, "", 1, "L", false, 0, "")
	}

	pdf.Ln(5)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(25, pdf.GetY(), 185, pdf.GetY())
	pdf.Ln(5)

	if body != "" {
		pdf.SetFont("Arial", "", 11)
		pdf.MultiCell(160, 6, body, "", "L", false)
	}

	pdf.Ln(20)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Human Resources Department", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")

	pdf.Ln(15)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 6, "Acknowledged by:", "", 1, "L", false, 0, "")
	pdf.Ln(10)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, fullName, "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 5, "Date: _______________", "", 1, "L", false, 0, "")
}
