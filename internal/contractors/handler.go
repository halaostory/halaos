package contractors

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/pdf"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler exposes contractor endpoints (admin).
type Handler struct {
	q       *store.Queries
	logger  *slog.Logger
	product string
}

// NewHandler creates the handler; product is the brand printed on generated documents.
func NewHandler(q *store.Queries, logger *slog.Logger, product string) *Handler {
	return &Handler{q: q, logger: logger, product: product}
}

// RegisterRoutes registers /contractors routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	g := protected.Group("/contractors", auth.AdminOnly())
	g.GET("/atcs", func(c *gin.Context) { response.OK(c, ATCs) })
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PUT("/:id", h.Update)
	g.GET("/:id/2307", h.Certificate2307)
	g.GET("/payments", h.ListPayments)
	g.POST("/payments", h.CreatePayment)
	g.POST("/payments/:id/approve", h.Approve)
	g.POST("/payments/:id/pay", h.MarkPaid)
	g.POST("/payments/:id/void", h.Void)
	g.GET("/bank-file", h.BankFile)
}

func num(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 4, 64))
	return n
}

func toF(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func pid(c *gin.Context) (int64, bool) {
	v, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || v <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return v, true
}

func opt(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

type contractorReq struct {
	Kind            string   `json:"kind"`
	Name            string   `json:"name" binding:"required"`
	Email           string   `json:"email"`
	TIN             string   `json:"tin"`
	Address         string   `json:"address"`
	IsVatRegistered bool     `json:"is_vat_registered"`
	ATC             string   `json:"atc_code"`
	EWTRate         *float64 `json:"ewt_rate"` // defaults to the ATC rate
	BankName        string   `json:"bank_name"`
	BankAccountNo   string   `json:"bank_account_no"`
	BankAccountName string   `json:"bank_account_name"`
	Notes           string   `json:"notes"`
	Status          string   `json:"status"`
}

func (r *contractorReq) normalize() (float64, error) {
	if r.Kind == "" {
		r.Kind = "individual"
	}
	if r.Kind != "individual" && r.Kind != "company" {
		return 0, fmt.Errorf("kind must be individual or company")
	}
	if r.TIN != "" && !ValidTIN(r.TIN) {
		return 0, fmt.Errorf("TIN must have 9–14 digits")
	}
	atc := DefaultATC(r.Kind)
	if r.ATC != "" {
		a, ok := LookupATC(r.ATC)
		if !ok {
			return 0, fmt.Errorf("unknown ATC %s", r.ATC)
		}
		if a.Kind != r.Kind {
			return 0, fmt.Errorf("ATC %s is for %s payees", a.Code, a.Kind)
		}
		atc = a
	}
	r.ATC = atc.Code
	rate := atc.Rate
	if r.EWTRate != nil {
		rate = *r.EWTRate
	}
	if rate < 0 || rate > 0.5 {
		return 0, fmt.Errorf("invalid withholding rate")
	}
	if r.Status == "" {
		r.Status = "active"
	}
	return rate, nil
}

func (h *Handler) List(c *gin.Context) {
	rows, err := h.q.ListContractors(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to list contractors")
		return
	}
	response.OK(c, rows)
}

func (h *Handler) Create(c *gin.Context) {
	var r contractorReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	rate, err := r.normalize()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	out, err := h.q.CreateContractor(c.Request.Context(), store.CreateContractorParams{
		CompanyID: auth.GetCompanyID(c), Kind: r.Kind, Name: strings.TrimSpace(r.Name), Email: opt(r.Email), Tin: strings.TrimSpace(r.TIN),
		Address: r.Address, IsVatRegistered: r.IsVatRegistered, EwtRate: num(rate), AtcCode: r.ATC,
		BankName: opt(r.BankName), BankAccountNo: opt(r.BankAccountNo), BankAccountName: opt(r.BankAccountName), Notes: opt(r.Notes),
	})
	if err != nil {
		h.logger.Error("create contractor", "error", err)
		response.InternalError(c, "Failed to create contractor")
		return
	}
	response.Created(c, out)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var r contractorReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	rate, err := r.normalize()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if r.Status != "active" && r.Status != "inactive" {
		response.BadRequest(c, "status must be active or inactive")
		return
	}
	out, err := h.q.UpdateContractor(c.Request.Context(), store.UpdateContractorParams{
		ID: id, CompanyID: auth.GetCompanyID(c), Kind: r.Kind, Name: strings.TrimSpace(r.Name), Email: opt(r.Email), Tin: strings.TrimSpace(r.TIN),
		Address: r.Address, IsVatRegistered: r.IsVatRegistered, EwtRate: num(rate), AtcCode: r.ATC,
		BankName: opt(r.BankName), BankAccountNo: opt(r.BankAccountNo), BankAccountName: opt(r.BankAccountName), Notes: opt(r.Notes), Status: r.Status,
	})
	if err != nil {
		response.NotFound(c, "Contractor not found")
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListPayments(c *gin.Context) {
	cid, _ := strconv.ParseInt(c.Query("contractor_id"), 10, 64)
	rows, err := h.q.ListContractorPayments(c.Request.Context(), store.ListContractorPaymentsParams{
		CompanyID: auth.GetCompanyID(c), Status: c.Query("status"), ContractorID: cid,
	})
	if err != nil {
		response.InternalError(c, "Failed to list payments")
		return
	}
	response.OK(c, rows)
}

func (h *Handler) CreatePayment(c *gin.Context) {
	var r struct {
		ContractorID int64   `json:"contractor_id" binding:"required"`
		InvoiceNo    string  `json:"invoice_no"`
		Description  string  `json:"description"`
		ServiceDate  string  `json:"service_date"`
		GrossAmount  float64 `json:"gross_amount" binding:"required"`
		ATC          string  `json:"atc_code"` // override, e.g. WI120 for services
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	con, err := h.q.GetContractor(c.Request.Context(), store.GetContractorParams{ID: r.ContractorID, CompanyID: companyID})
	if err != nil || con.Status != "active" {
		response.BadRequest(c, "Contractor not found or inactive")
		return
	}
	atc, rate := con.AtcCode, toF(con.EwtRate)
	if r.ATC != "" && !strings.EqualFold(r.ATC, con.AtcCode) {
		a, ok := LookupATC(r.ATC)
		if !ok || a.Kind != con.Kind {
			response.BadRequest(c, "ATC does not apply to this contractor")
			return
		}
		atc, rate = a.Code, a.Rate
	}
	b, err := Compute(r.GrossAmount, rate, con.IsVatRegistered)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	sd := time.Now()
	if r.ServiceDate != "" {
		if sd, err = time.Parse("2006-01-02", r.ServiceDate); err != nil {
			response.BadRequest(c, "service_date must be YYYY-MM-DD")
			return
		}
	}
	uid := auth.GetUserID(c)
	out, err := h.q.CreateContractorPayment(c.Request.Context(), store.CreateContractorPaymentParams{
		CompanyID: companyID, ContractorID: con.ID, InvoiceNo: strings.TrimSpace(r.InvoiceNo), Description: r.Description, ServiceDate: sd,
		GrossAmount: num(b.Gross), VatAmount: num(b.VAT), EwtBase: num(b.Base), EwtRate: num(b.Rate), EwtAmount: num(b.EWT),
		NetAmount: num(b.Net), AtcCode: atc, CreatedBy: &uid,
	})
	if err != nil {
		h.logger.Error("create contractor payment", "error", err)
		response.InternalError(c, "Failed to create payment")
		return
	}
	response.Created(c, gin.H{"payment": out, "breakdown": b})
}

func (h *Handler) Approve(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	uid := auth.GetUserID(c)
	out, err := h.q.ApproveContractorPayment(c.Request.Context(), store.ApproveContractorPaymentParams{ID: id, CompanyID: auth.GetCompanyID(c), ApprovedBy: &uid})
	if err != nil {
		response.Conflict(c, "Only draft payments can be approved")
		return
	}
	response.OK(c, out)
}

func (h *Handler) MarkPaid(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var r struct {
		PaidAt    string `json:"paid_at"`
		Reference string `json:"reference"`
	}
	_ = c.ShouldBindJSON(&r)
	paid := time.Now()
	if r.PaidAt != "" {
		var err error
		if paid, err = time.Parse("2006-01-02", r.PaidAt); err != nil {
			response.BadRequest(c, "paid_at must be YYYY-MM-DD")
			return
		}
	}
	out, err := h.q.MarkContractorPaymentPaid(c.Request.Context(), store.MarkContractorPaymentPaidParams{
		ID: id, CompanyID: auth.GetCompanyID(c), PaidAt: pgtype.Date{Time: paid, Valid: true}, PaidReference: opt(r.Reference),
	})
	if err != nil {
		response.Conflict(c, "Only approved payments can be marked paid")
		return
	}
	response.OK(c, out)
}

func (h *Handler) Void(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	out, err := h.q.VoidContractorPayment(c.Request.Context(), store.VoidContractorPaymentParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.Conflict(c, "Paid payments cannot be voided")
		return
	}
	response.OK(c, out)
}

// BankFile exports approved payments as a CSV for bulk bank upload.
func (h *Handler) BankFile(c *gin.Context) {
	rows, err := h.q.ListApprovedPaymentsForBankFile(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to build bank file")
		return
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"payment_id", "payee", "bank", "account_no", "account_name", "amount", "reference"})
	for _, r := range rows {
		name := r.Name
		if r.BankAccountName != nil && *r.BankAccountName != "" {
			name = *r.BankAccountName
		}
		_ = w.Write([]string{strconv.FormatInt(r.ID, 10), r.Name, deref(r.BankName), deref(r.BankAccountNo), name,
			fmt.Sprintf("%.2f", toF(r.NetAmount)), r.InvoiceNo})
	}
	w.Flush()
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="contractor_payments_%s.csv"`, time.Now().Format("20060102")))
	c.Data(200, "text/csv", buf.Bytes())
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// QuarterBounds parses "2026-Q3" into its first and last day.
func QuarterBounds(s string) (time.Time, time.Time, error) {
	var y, q int
	if _, err := fmt.Sscanf(strings.ToUpper(strings.TrimSpace(s)), "%d-Q%d", &y, &q); err != nil || q < 1 || q > 4 {
		return time.Time{}, time.Time{}, fmt.Errorf("quarter must be YYYY-Qn")
	}
	start := time.Date(y, time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 3, -1), nil
}

// Rows2307 groups paid withholding into 2307 rows (month index within quarter).
func Rows2307(rows []store.ContractorWithholdingByPayeeRow, quarterStart time.Time) []pdf.Row2307 {
	byATC := map[string]*pdf.Row2307{}
	var order []string
	for _, r := range rows {
		row, ok := byATC[r.AtcCode]
		if !ok {
			desc := r.AtcCode
			if a, ok := LookupATC(r.AtcCode); ok {
				desc = a.Description
			}
			row = &pdf.Row2307{ATC: r.AtcCode, Description: desc}
			byATC[r.AtcCode] = row
			order = append(order, r.AtcCode)
		}
		idx := int(r.Month) - int(quarterStart.Month())
		if idx >= 0 && idx < 3 {
			row.Months[idx] += r.Base
		}
		row.Withheld += r.Ewt
	}
	out := make([]pdf.Row2307, 0, len(order))
	for _, k := range order {
		out = append(out, *byATC[k])
	}
	return out
}

// Certificate2307 renders BIR 2307 for one contractor and quarter (?quarter=2026-Q3).
func (h *Handler) Certificate2307(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	start, end, err := QuarterBounds(c.Query("quarter"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	con, err := h.q.GetContractor(ctx, store.GetContractorParams{ID: id, CompanyID: companyID})
	if err != nil {
		response.NotFound(c, "Contractor not found")
		return
	}
	comp, err := h.q.GetCompanyByID(ctx, companyID)
	if err != nil {
		response.InternalError(c, "Company not found")
		return
	}
	rows, err := h.q.ContractorWithholdingByPayee(ctx, store.ContractorWithholdingByPayeeParams{CompanyID: companyID, StartDate: start, EndDate: end, ContractorID: id})
	if err != nil {
		response.InternalError(c, "Failed to load payments")
		return
	}
	if len(rows) == 0 {
		response.NotFound(c, "No paid amounts with withholding for this contractor in that quarter")
		return
	}
	payorName := comp.Name
	if comp.LegalName != nil && *comp.LegalName != "" {
		payorName = *comp.LegalName
	}
	out, err := pdf.Generate2307(pdf.Certificate2307{
		PeriodFrom: start.Format("01/02/2006"), PeriodTo: end.Format("01/02/2006"),
		Payee:     pdf.Party{TIN: con.Tin, Name: con.Name, Address: con.Address},
		Payor:     pdf.Party{TIN: deref(comp.Tin), Name: payorName, Address: deref(comp.Address)},
		Rows:      Rows2307(rows, start),
		Generator: "Generated by " + h.product + " — sign before issuing to the payee.",
	})
	if err != nil {
		response.InternalError(c, "Failed to render 2307")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="BIR2307_%s_%s.pdf"`, strings.ReplaceAll(con.Name, " ", "_"), c.Query("quarter")))
	c.Data(200, "application/pdf", out)
}
