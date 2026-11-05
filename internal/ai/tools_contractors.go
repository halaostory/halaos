package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/contractors"
	"github.com/halaostory/halaos/internal/store"
)

func (r *ToolRegistry) toolListContractors(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	rows, err := r.queries.ListContractors(ctx, companyID)
	if err != nil {
		return "", err
	}
	type c struct {
		ID      int64   `json:"id"`
		Name    string  `json:"name"`
		Kind    string  `json:"kind"`
		ATC     string  `json:"atc_code"`
		Rate    float64 `json:"ewt_rate"`
		VAT     bool    `json:"vat_registered"`
		Status  string  `json:"status"`
		YTDPaid float64 `json:"ytd_paid"`
	}
	out := []c{}
	for _, x := range rows {
		f, _ := x.EwtRate.Float64Value()
		out = append(out, c{x.ID, x.Name, x.Kind, x.AtcCode, f.Float64, x.IsVatRegistered, x.Status, x.YtdPaid})
	}
	return toJSON(map[string]any{"contractors": out})
}

func (r *ToolRegistry) toolCreateContractorPayment(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}
	cid := idIn(input, "contractor_id")
	gross, _ := input["gross_amount"].(float64)
	con, err := r.queries.GetContractor(ctx, store.GetContractorParams{ID: cid, CompanyID: companyID})
	if err != nil || con.Status != "active" {
		return "", fmt.Errorf("contractor not found or inactive")
	}
	f, _ := con.EwtRate.Float64Value()
	b, err := contractors.Compute(gross, f.Float64, con.IsVatRegistered)
	if err != nil {
		return "", err
	}
	sd := time.Now()
	if s := strIn(input, "service_date"); s != "" {
		if sd, err = time.Parse("2006-01-02", s); err != nil {
			return "", fmt.Errorf("service_date must be YYYY-MM-DD")
		}
	}
	n := func(v float64) pgtype.Numeric {
		var x pgtype.Numeric
		_ = x.Scan(fmt.Sprintf("%.4f", v))
		return x
	}
	p, err := r.queries.CreateContractorPayment(ctx, store.CreateContractorPaymentParams{
		CompanyID: companyID, ContractorID: con.ID, InvoiceNo: strIn(input, "invoice_no"), Description: strings.TrimSpace(strIn(input, "description")),
		ServiceDate: sd, GrossAmount: n(b.Gross), VatAmount: n(b.VAT), EwtBase: n(b.Base), EwtRate: n(b.Rate), EwtAmount: n(b.EWT),
		NetAmount: n(b.Net), AtcCode: con.AtcCode, CreatedBy: &userID,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "payment_id": p.ID, "status": "draft", "breakdown": b,
		"next_step": "Approve it on the Contractors page, then pay it (bank file) and mark it paid."})
}

func contractorDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{Name: "list_contractors", Description: "List contractors/freelancers with their withholding (ATC, EWT rate, VAT status) and year-to-date paid amounts. Admin only.",
			Parameters: jsonSchema(map[string]any{"type": "object", "properties": map[string]any{}})},
		{Name: "create_contractor_payment", Description: "Record a contractor invoice as a draft payment. Expanded withholding tax (EWT) is computed from the contractor's ATC; VAT-registered payees are withheld on the amount net of 12% VAT. Requires user confirmation.",
			Parameters: jsonSchema(map[string]any{"type": "object", "properties": map[string]any{
				"contractor_id": map[string]any{"type": "integer"},
				"gross_amount":  map[string]any{"type": "number", "description": "Invoice amount (VAT-inclusive if the payee is VAT-registered)"},
				"invoice_no":    map[string]any{"type": "string"},
				"description":   map[string]any{"type": "string"},
				"service_date":  map[string]any{"type": "string", "description": "YYYY-MM-DD"},
			}, "required": []string{"contractor_id", "gross_amount"}})},
	}
}

func (r *ToolRegistry) registerContractorTools() {
	r.tools["list_contractors"] = r.toolListContractors
	r.tools["create_contractor_payment"] = r.toolCreateContractorPayment
}
