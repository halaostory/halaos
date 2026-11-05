package contractors

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // only used to derive a stable idempotency UUID
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Cross-border contractor payouts through Wise (Warp: international
// contractors paid in local currency). The company pays from its own Wise
// Business balance; HalaPeople never holds funds. A contractor is linked
// to a recipient created in Wise; an approved payment is quoted, and after
// the admin confirms the quote, a transfer is created (idempotent per
// payment) and funded from the balance.

// Wise is a minimal Wise Platform API client.
type Wise struct {
	Base      string
	Token     string
	ProfileID int64
	Source    string // source currency override (default: company currency)
	HTTP      *http.Client
}

// WiseFromEnv reads WISE_API_TOKEN, WISE_PROFILE_ID, WISE_API_URL and WISE_SOURCE_CURRENCY.
func WiseFromEnv() *Wise {
	base := os.Getenv("WISE_API_URL")
	if base == "" {
		base = "https://api.transferwise.com"
	}
	pid, _ := strconv.ParseInt(os.Getenv("WISE_PROFILE_ID"), 10, 64)
	return &Wise{Base: strings.TrimRight(base, "/"), Token: os.Getenv("WISE_API_TOKEN"), ProfileID: pid,
		Source: strings.ToUpper(os.Getenv("WISE_SOURCE_CURRENCY")), HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Available reports whether Wise is configured.
func (w *Wise) Available() bool { return w != nil && w.Token != "" && w.ProfileID != 0 }

func (w *Wise) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, w.Base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("wise: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return fmt.Errorf("wise %s %s: %s %s", method, path, resp.Status, msg)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// WiseQuote is a priced transfer.
type WiseQuote struct {
	ID             string  `json:"id"`
	SourceCurrency string  `json:"sourceCurrency"`
	TargetCurrency string  `json:"targetCurrency"`
	SourceAmount   float64 `json:"sourceAmount"`
	TargetAmount   float64 `json:"targetAmount"`
	Rate           float64 `json:"rate"`
	Fee            float64 `json:"fee"`
}

// Quote prices sending sourceAmount to a recipient.
func (w *Wise) Quote(ctx context.Context, source, target string, sourceAmount float64, recipient int64) (*WiseQuote, error) {
	var out struct {
		ID             string  `json:"id"`
		SourceCurrency string  `json:"sourceCurrency"`
		TargetCurrency string  `json:"targetCurrency"`
		SourceAmount   float64 `json:"sourceAmount"`
		TargetAmount   float64 `json:"targetAmount"`
		Rate           float64 `json:"rate"`
		PaymentOptions []struct {
			PayIn        string  `json:"payIn"`
			TargetAmount float64 `json:"targetAmount"`
			Fee          struct {
				Total float64 `json:"total"`
			} `json:"fee"`
		} `json:"paymentOptions"`
	}
	body := map[string]any{"sourceCurrency": source, "targetCurrency": target, "sourceAmount": sourceAmount, "targetAccount": recipient, "payOut": "BANK_TRANSFER"}
	if err := w.do(ctx, http.MethodPost, fmt.Sprintf("/v3/profiles/%d/quotes", w.ProfileID), body, &out); err != nil {
		return nil, err
	}
	q := &WiseQuote{ID: out.ID, SourceCurrency: out.SourceCurrency, TargetCurrency: out.TargetCurrency, SourceAmount: out.SourceAmount, TargetAmount: out.TargetAmount, Rate: out.Rate}
	for _, o := range out.PaymentOptions {
		if o.PayIn == "BALANCE" {
			q.Fee = o.Fee.Total
			if o.TargetAmount > 0 {
				q.TargetAmount = o.TargetAmount
			}
		}
	}
	return q, nil
}

// idempotencyUUID derives a stable UUID so retries never pay twice.
func idempotencyUUID(paymentID int64) string {
	h := sha1.Sum([]byte(fmt.Sprintf("halapeople-contractor-payment-%d", paymentID))) //nolint:gosec
	h[6] = (h[6] & 0x0f) | 0x50
	h[8] = (h[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// TransferAndFund creates the transfer for a quote and pays it from the balance.
func (w *Wise) TransferAndFund(ctx context.Context, quoteID string, recipient, paymentID int64, reference string) (int64, string, error) {
	var tr struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if len(reference) > 35 {
		reference = reference[:35]
	}
	body := map[string]any{"targetAccount": recipient, "quoteUuid": quoteID, "customerTransactionId": idempotencyUUID(paymentID),
		"details": map[string]string{"reference": reference}}
	if err := w.do(ctx, http.MethodPost, "/v1/transfers", body, &tr); err != nil {
		return 0, "", err
	}
	var fund struct {
		Status string `json:"status"`
	}
	if err := w.do(ctx, http.MethodPost, fmt.Sprintf("/v3/profiles/%d/transfers/%d/payments", w.ProfileID, tr.ID), map[string]string{"type": "BALANCE"}, &fund); err != nil {
		return tr.ID, tr.Status, fmt.Errorf("transfer %d created but not funded: %w", tr.ID, err)
	}
	if fund.Status != "COMPLETED" {
		return tr.ID, tr.Status, fmt.Errorf("transfer %d created but Wise did not fund it (%s) — check the balance", tr.ID, fund.Status)
	}
	return tr.ID, "funded", nil
}

// TransferStatus reads a transfer's status.
func (w *Wise) TransferStatus(ctx context.Context, id int64) (string, error) {
	var tr struct {
		Status string `json:"status"`
	}
	err := w.do(ctx, http.MethodGet, fmt.Sprintf("/v1/transfers/%d", id), nil, &tr)
	return tr.Status, err
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// SetWise enables Wise payouts.
func (h *Handler) SetWise(w *Wise) { h.wise = w }

func (h *Handler) registerWise(g *gin.RouterGroup) {
	g.GET("/:id/payout-account", h.getPayoutAccount)
	g.PUT("/:id/payout-account", h.setPayoutAccount)
	g.GET("/payments/:id/wise", h.wiseQuote)
	g.POST("/payments/:id/wise", h.wisePay)
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}

func (h *Handler) getPayoutAccount(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.q.GetContractorPayoutAccount(c.Request.Context(), store.GetContractorPayoutAccountParams{ContractorID: id, CompanyID: auth.GetCompanyID(c)})
	if errors.Is(err, pgx.ErrNoRows) {
		response.OK(c, gin.H{"account": nil, "available": h.wise.Available()})
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to load payout account")
		return
	}
	response.OK(c, gin.H{"account": a, "available": h.wise.Available()})
}

func (h *Handler) setPayoutAccount(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		RecipientID int64  `json:"recipient_id" binding:"required"`
		Currency    string `json:"currency" binding:"required"`
		Label       string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "recipient_id and currency are required")
		return
	}
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	if len(req.Currency) != 3 {
		response.BadRequest(c, "currency must be a 3-letter code such as USD")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	if _, err := h.q.GetContractor(ctx, store.GetContractorParams{ID: id, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Contractor not found")
		return
	}
	a, err := h.q.UpsertContractorPayoutAccount(ctx, store.UpsertContractorPayoutAccountParams{ContractorID: id, CompanyID: companyID,
		RecipientID: req.RecipientID, Currency: req.Currency, Label: strings.TrimSpace(req.Label)})
	if err != nil {
		response.InternalError(c, "Failed to save")
		return
	}
	response.OK(c, a)
}

func (h *Handler) sourceCurrency(ctx context.Context, companyID int64) string {
	if h.wise.Source != "" {
		return h.wise.Source
	}
	if comp, err := h.q.GetCompanyByID(ctx, companyID); err == nil && comp.Currency != "" {
		return comp.Currency
	}
	return "PHP"
}

// wiseQuote handles GET /contractors/payments/:id/wise — a live quote.
func (h *Handler) wiseQuote(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	p, err := h.q.GetContractorPaymentForPayout(ctx, store.GetContractorPaymentForPayoutParams{ID: id, CompanyID: companyID})
	if err != nil {
		response.NotFound(c, "Payment not found")
		return
	}
	out := gin.H{"payment": p, "available": h.wise.Available()}
	if t, err := h.q.GetWiseTransfer(ctx, store.GetWiseTransferParams{PaymentID: id, CompanyID: companyID}); err == nil {
		if st, err := h.wise.TransferStatus(ctx, t.TransferID); err == nil && st != "" && st != t.Status {
			_ = h.q.SetWiseTransferStatus(ctx, store.SetWiseTransferStatusParams{PaymentID: id, CompanyID: companyID, Status: st})
			t.Status = st
		}
		out["transfer"] = t
		response.OK(c, out)
		return
	}
	acct, err := h.q.GetContractorPayoutAccount(ctx, store.GetContractorPayoutAccountParams{ContractorID: p.ContractorID, CompanyID: companyID})
	switch {
	case !h.wise.Available():
		out["blocker"] = "Wise is not configured on this server"
	case err != nil:
		out["blocker"] = "link this contractor to a Wise recipient first"
	case p.Status != "approved":
		out["blocker"] = "approve the payment first"
	default:
		q, err := h.wise.Quote(ctx, h.sourceCurrency(ctx, companyID), acct.Currency, math.Round(p.NetAmount*100)/100, acct.RecipientID)
		if err != nil {
			out["blocker"] = err.Error()
		} else {
			out["quote"] = q
		}
	}
	response.OK(c, out)
}

// wisePay handles POST /contractors/payments/:id/wise {quote_id, confirm}.
func (h *Handler) wisePay(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		QuoteID      string  `json:"quote_id" binding:"required"`
		TargetAmount float64 `json:"target_amount"`
		Rate         float64 `json:"rate"`
		Confirm      bool    `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		response.BadRequest(c, "quote_id and confirm: true are required")
		return
	}
	if !h.wise.Available() {
		response.BadRequest(c, "Wise is not configured on this server")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	p, err := h.q.GetContractorPaymentForPayout(ctx, store.GetContractorPaymentForPayoutParams{ID: id, CompanyID: companyID})
	if err != nil || p.Status != "approved" {
		response.BadRequest(c, "Only approved payments can be paid")
		return
	}
	if _, err := h.q.GetWiseTransfer(ctx, store.GetWiseTransferParams{PaymentID: id, CompanyID: companyID}); err == nil {
		response.BadRequest(c, "This payment was already sent through Wise")
		return
	}
	acct, err := h.q.GetContractorPayoutAccount(ctx, store.GetContractorPayoutAccountParams{ContractorID: p.ContractorID, CompanyID: companyID})
	if err != nil {
		response.BadRequest(c, "Link this contractor to a Wise recipient first")
		return
	}
	src := h.sourceCurrency(ctx, companyID)
	ref := strings.TrimSpace("Invoice " + p.InvoiceNo)
	trID, status, err := h.wise.TransferAndFund(ctx, req.QuoteID, acct.RecipientID, id, ref)
	if err != nil && trID == 0 {
		response.BadRequest(c, err.Error())
		return
	}
	var by *int64
	if uid := auth.GetUserID(c); uid > 0 {
		by = &uid
	}
	rec, recErr := h.q.CreateWiseTransfer(ctx, store.CreateWiseTransferParams{PaymentID: id, CompanyID: companyID, QuoteID: req.QuoteID, TransferID: trID,
		SourceAmount: num(p.NetAmount), SourceCurrency: src, TargetAmount: num(req.TargetAmount), TargetCurrency: acct.Currency, Rate: num(req.Rate), Status: status, CreatedBy: by})
	if recErr != nil {
		h.logger.Error("contractors: record wise transfer", "payment_id", id, "transfer_id", trID, "error", recErr)
	}
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if _, err := h.q.MarkContractorPaymentPaid(ctx, store.MarkContractorPaymentPaidParams{ID: id, CompanyID: companyID,
		PaidAt: pgtype.Date{Time: time.Now(), Valid: true}, PaidReference: strPtr(fmt.Sprintf("wise:%d", trID))}); err != nil {
		h.logger.Warn("contractors: mark paid after wise", "payment_id", id, "error", err)
	}
	response.OK(c, rec)
}

func strPtr(s string) *string { return &s }
