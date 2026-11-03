package contractors

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Contractor tax information form (Warp: contractors complete their own tax
// forms, e.g. W-8BEN). HR sends a one-time link; the contractor states tax
// residency, TIN, address, VAT status and payout details and signs a
// declaration. The contractor record is updated and the signed snapshot
// kept. Withholding (ATC / EWT rate) stays HR's decision.

const taxFormTTL = 14 * 24 * time.Hour

// Declaration is the text the contractor signs (stored with each form).
func Declaration(company, country string) string {
	return fmt.Sprintf("I certify that the information in this form is true, correct and complete; "+
		"that I am a tax resident of %s; and that I will tell %s promptly if any of it changes. "+
		"I understand %s relies on it to withhold and report taxes on payments to me.", country, company, company)
}

func tokenHash(t string) string {
	s := sha256.Sum256([]byte(strings.TrimSpace(t)))
	return hex.EncodeToString(s[:])
}

// SetAppURL sets the public base URL used in tax-form links.
func (h *Handler) SetAppURL(u string) { h.appURL = strings.TrimRight(u, "/") }

// RegisterTaxFormRoutes: admin (protected) and the contractor's public link.
func (h *Handler) RegisterTaxFormRoutes(protected, public *gin.RouterGroup, read, write gin.HandlerFunc) {
	g := protected.Group("/contractors", auth.AdminOnly())
	g.POST("/:id/tax-form", h.createTaxForm)
	g.GET("/:id/tax-form", h.getTaxForm)
	p := public.Group("/public")
	p.GET("/contractor-form/:token", read, h.publicTaxForm)
	p.POST("/contractor-form/:token", write, h.submitTaxForm)
}

func (h *Handler) createTaxForm(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	if _, err := h.q.GetContractor(ctx, store.GetContractorParams{ID: id, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Contractor not found")
		return
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	uid := auth.GetUserID(c)
	var by *int64
	if uid > 0 {
		by = &uid
	}
	_ = h.q.RevokePendingContractorTaxForms(ctx, store.RevokePendingContractorTaxFormsParams{ContractorID: id, CompanyID: companyID})
	f, err := h.q.CreateContractorTaxForm(ctx, store.CreateContractorTaxFormParams{CompanyID: companyID, ContractorID: id, TokenHash: tokenHash(token),
		ExpiresAt: time.Now().Add(taxFormTTL), CreatedBy: by})
	if err != nil {
		response.InternalError(c, "Could not create the form link")
		return
	}
	response.OK(c, gin.H{"url": h.appURL + "/contractor-form/" + token, "expires_at": f.ExpiresAt})
}

func (h *Handler) getTaxForm(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	f, err := h.q.LatestContractorTaxForm(c.Request.Context(), store.LatestContractorTaxFormParams{ContractorID: id, CompanyID: auth.GetCompanyID(c)})
	if errors.Is(err, pgx.ErrNoRows) {
		response.OK(c, nil)
		return
	} else if err != nil {
		response.InternalError(c, "Could not load the form")
		return
	}
	f.TokenHash = ""
	response.OK(c, f)
}

func (h *Handler) formByToken(c *gin.Context) (store.GetContractorTaxFormByTokenRow, bool) {
	f, err := h.q.GetContractorTaxFormByToken(c.Request.Context(), tokenHash(c.Param("token")))
	if err != nil || f.Status != "pending" || time.Now().After(f.ExpiresAt) {
		response.Error(c, 410, "gone", "This link has expired or was already used. Please ask for a new one.")
		return f, false
	}
	return f, true
}

// TaxCountries are the residence choices on the form (ISO 3166-1 alpha-2)
// with the wording used in the signed declaration.
var TaxCountries = []struct {
	Code, Label, InDeclaration string
}{
	{"PH", "Philippines", "the Philippines"}, {"SG", "Singapore", "Singapore"}, {"ID", "Indonesia", "Indonesia"},
	{"MY", "Malaysia", "Malaysia"}, {"VN", "Vietnam", "Vietnam"}, {"TH", "Thailand", "Thailand"}, {"IN", "India", "India"},
	{"LK", "Sri Lanka", "Sri Lanka"}, {"US", "United States", "the United States"}, {"GB", "United Kingdom", "the United Kingdom"},
	{"AU", "Australia", "Australia"}, {"XX", "Other", "my country of residence"},
}

func countryName(code string) (string, bool) {
	for _, c := range TaxCountries {
		if c.Code == strings.ToUpper(code) {
			return c.InDeclaration, true
		}
	}
	return "", false
}

func (h *Handler) publicTaxForm(c *gin.Context) {
	f, ok := h.formByToken(c)
	if !ok {
		return
	}
	var countries []gin.H
	for _, x := range TaxCountries {
		countries = append(countries, gin.H{"code": x.Code, "label": x.Label, "declaration": Declaration(f.CompanyName, x.InDeclaration)})
	}
	response.OK(c, gin.H{
		"countries": countries,
		"company":   f.CompanyName, "company_country": f.CompanyCountry, "contractor": f.ContractorName, "kind": f.ContractorKind,
		"tin": f.CurrentTin, "address": f.CurrentAddress, "is_vat_registered": f.CurrentVat, "expires_at": f.ExpiresAt,
	})
}

func (h *Handler) submitTaxForm(c *gin.Context) {
	f, ok := h.formByToken(c)
	if !ok {
		return
	}
	var req struct {
		TaxCountry      string `json:"tax_country" binding:"required"`
		EntityType      string `json:"entity_type" binding:"required"`
		TIN             string `json:"tin"`
		Address         string `json:"address" binding:"required"`
		IsVATRegistered bool   `json:"is_vat_registered"`
		BankName        string `json:"bank_name"`
		BankAccountNo   string `json:"bank_account_no"`
		BankAccountName string `json:"bank_account_name"`
		SignerName      string `json:"signer_name" binding:"required"`
		Agree           bool   `json:"agree"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Please complete the required fields")
		return
	}
	req.TaxCountry = strings.ToUpper(strings.TrimSpace(req.TaxCountry))
	req.TIN = strings.TrimSpace(req.TIN)
	req.SignerName = strings.TrimSpace(req.SignerName)
	switch {
	case len(req.TaxCountry) != 2:
		response.BadRequest(c, "Choose your country of tax residence")
		return
	case req.EntityType != "individual" && req.EntityType != "company":
		response.BadRequest(c, "Choose individual or company")
		return
	case !req.Agree || len(req.SignerName) < 2 || len(req.SignerName) > 160:
		response.BadRequest(c, "Type your full name and accept the declaration to sign")
		return
	case len(req.TIN) > 40 || len(req.Address) > 1000:
		response.BadRequest(c, "A field is too long")
		return
	}
	// A resident of the company's own country must give a local TIN.
	if f.CompanyCountry != "" && strings.HasPrefix(f.CompanyCountry, req.TaxCountry[:2]) && req.TIN == "" {
		response.BadRequest(c, "Your TIN is required")
		return
	}
	country, known := countryName(req.TaxCountry)
	if !known {
		response.BadRequest(c, "Choose your country of tax residence")
		return
	}
	decl := Declaration(f.CompanyName, country) // exactly the text shown for this country
	ua := c.GetHeader("User-Agent")
	if len(ua) > 300 {
		ua = ua[:300]
	}
	ctx := c.Request.Context()
	form, err := h.q.SubmitContractorTaxForm(ctx, store.SubmitContractorTaxFormParams{ID: f.ID, TaxCountry: req.TaxCountry[:2],
		EntityType: req.EntityType, Tin: req.TIN, Address: strings.TrimSpace(req.Address), IsVatRegistered: req.IsVATRegistered,
		Declaration: decl, SignerName: req.SignerName, SignerIp: c.ClientIP(), SignerAgent: ua})
	if err != nil {
		response.Error(c, 410, "gone", "This link has expired or was already used. Please ask for a new one.")
		return
	}
	_ = h.q.ApplyContractorTaxInfo(ctx, store.ApplyContractorTaxInfoParams{ID: f.ContractorID, CompanyID: f.CompanyID, Tin: req.TIN, Address: strings.TrimSpace(req.Address),
		IsVatRegistered: req.IsVATRegistered, BankName: optional(req.BankName), BankAccountNo: optional(req.BankAccountNo), BankAccountName: optional(req.BankAccountName)})
	response.OK(c, gin.H{"submitted": true, "submitted_at": form.SubmittedAt})
}

func optional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
