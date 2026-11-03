// Package payouts sends approved net pay to employees' bank accounts
// through a licensed payout partner (Xendit Batch Disbursements), so
// HalaPeople moves money without holding funds itself (Warp: "taxes and
// direct deposits are processed"). Hard limits: admins only, approved
// pay runs only, an explicit confirmation, at most once per run — the AI
// agent and routines never trigger it. The partner's callback reports
// each transfer's result.
package payouts

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// bankCodes maps common Philippine bank names to Xendit bank codes.
// Employees may also enter the provider's code directly in "bank name".
// Verify against your Xendit account's list of channel codes.
var bankCodes = map[string]string{
	"bdo": "PH_BDO", "banco de oro": "PH_BDO", "bpi": "PH_BPI", "bank of the philippine islands": "PH_BPI",
	"metrobank": "PH_MET", "metropolitan bank": "PH_MET", "unionbank": "PH_UBP", "union bank": "PH_UBP",
	"security bank": "PH_SEC", "rcbc": "PH_RCBC", "landbank": "PH_LBP", "land bank": "PH_LBP",
	"pnb": "PH_PNB", "philippine national bank": "PH_PNB", "chinabank": "PH_CBC", "china bank": "PH_CBC",
	"eastwest": "PH_EWB", "eastwest bank": "PH_EWB", "psbank": "PH_PSB", "gcash": "PH_GCASH", "maya": "PH_PAYMAYA", "paymaya": "PH_PAYMAYA",
}

// BankCode resolves a free-text bank name to a provider code ("" = unknown).
func BankCode(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(n), "PH_") {
		return strings.ToUpper(n)
	}
	if c, ok := bankCodes[n]; ok {
		return c
	}
	for k, c := range bankCodes {
		if strings.Contains(n, k) {
			return c
		}
	}
	return ""
}

// Line is one transfer in a payout.
type Line struct {
	EmployeeID  int64   `json:"employee_id"`
	EmployeeNo  string  `json:"employee_no"`
	Name        string  `json:"name"`
	Amount      float64 `json:"amount"`
	BankName    string  `json:"bank_name"`
	BankCode    string  `json:"bank_code"`
	AccountNo   string  `json:"account_no"`
	AccountName string  `json:"account_name"`
	Issue       string  `json:"issue,omitempty"`
}

// Plan is what a payout would send.
type Plan struct {
	CycleID int64              `json:"cycle_id"`
	RunID   int64              `json:"run_id"`
	Cycle   string             `json:"cycle"`
	Status  string             `json:"cycle_status"`
	Lines   []Line             `json:"lines"`
	Total   float64            `json:"total"`
	Issues  int                `json:"issues"`
	Blocker string             `json:"blocker,omitempty"`
	Sent    *store.PayoutBatch `json:"sent,omitempty"`
}

// Service plans and sends payouts.
type Service struct {
	q        *store.Queries
	pool     *pgxpool.Pool
	logger   *slog.Logger
	apiBase  string
	secret   string
	callback string // token Xendit sends in x-callback-token
	http     *http.Client
}

// NewFromEnv reads XENDIT_SECRET_KEY, XENDIT_CALLBACK_TOKEN and XENDIT_API_URL.
func NewFromEnv(q *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Service {
	base := os.Getenv("XENDIT_API_URL")
	if base == "" {
		base = "https://api.xendit.co"
	}
	return &Service{q: q, pool: pool, logger: logger, apiBase: strings.TrimRight(base, "/"),
		secret: os.Getenv("XENDIT_SECRET_KEY"), callback: os.Getenv("XENDIT_CALLBACK_TOKEN"), http: &http.Client{Timeout: 30 * time.Second}}
}

// Available reports whether a payout partner is configured.
func (s *Service) Available() bool { return s.secret != "" && s.callback != "" }

// Preview builds the payout for a cycle's latest calculated run.
func (s *Service) Preview(ctx context.Context, companyID, cycleID int64) (*Plan, error) {
	cyc, err := s.q.GetCycleForPayout(ctx, store.GetCycleForPayoutParams{ID: cycleID, CompanyID: companyID})
	if err != nil {
		return nil, errors.New("pay run not found")
	}
	runID, err := s.q.LatestCompletedRunForCycle(ctx, store.LatestCompletedRunForCycleParams{CycleID: cycleID, CompanyID: companyID})
	if err != nil {
		return nil, errors.New("this cycle has no calculated payroll run yet")
	}
	rows, err := s.q.ListNetPayForRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	p := &Plan{CycleID: cycleID, RunID: runID, Cycle: cyc.Name, Status: cyc.Status, Lines: []Line{}}
	for _, r := range rows {
		if r.NetPay <= 0 {
			continue
		}
		l := Line{EmployeeID: r.EmployeeID, EmployeeNo: r.EmployeeNo, Name: strings.TrimSpace(r.FirstName + " " + r.LastName),
			Amount: math.Round(r.NetPay*100) / 100, BankName: r.BankName, BankCode: BankCode(r.BankName),
			AccountNo: strings.ReplaceAll(r.BankAccountNo, " ", ""), AccountName: strings.TrimSpace(r.BankAccountName)}
		if l.AccountName == "" {
			l.AccountName = l.Name
		}
		switch {
		case l.AccountNo == "":
			l.Issue = "no bank account number"
		case l.BankCode == "":
			l.Issue = "bank not recognised — enter a bank such as BDO, BPI, Metrobank, or the provider's code"
		}
		if l.Issue != "" {
			p.Issues++
		}
		p.Total += l.Amount
		p.Lines = append(p.Lines, l)
	}
	p.Total = math.Round(p.Total*100) / 100
	if b, err := s.q.GetPayoutBatchForRun(ctx, store.GetPayoutBatchForRunParams{RunID: runID, Provider: "xendit"}); err == nil {
		p.Sent = &b
	}
	switch {
	case p.Sent != nil:
		p.Blocker = "already sent"
	case cyc.Status != "approved":
		p.Blocker = "approve the pay run first"
	case p.Issues > 0:
		p.Blocker = "fix the bank details listed"
	case len(p.Lines) == 0:
		p.Blocker = "nothing to pay"
	case !s.Available():
		p.Blocker = "no payout partner configured on this server"
	}
	return p, nil
}

func num(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', 2, 64))
	return n
}

// Send submits the payout. It refuses unless Preview has no blocker.
func (s *Service) Send(ctx context.Context, companyID, userID, cycleID int64) (*store.PayoutBatch, error) {
	p, err := s.Preview(ctx, companyID, cycleID)
	if err != nil {
		return nil, err
	}
	if p.Blocker != "" {
		return nil, errors.New("cannot send: " + p.Blocker)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qt := s.q.WithTx(tx)
	var by *int64
	if userID > 0 {
		by = &userID
	}
	batch, err := qt.CreatePayoutBatch(ctx, store.CreatePayoutBatchParams{CompanyID: companyID, CycleID: cycleID, RunID: p.RunID,
		Provider: "xendit", Total: num(p.Total), ItemCount: int32(len(p.Lines)), CreatedBy: by})
	if err != nil {
		return nil, errors.New("cannot send: already sent")
	}
	type disb struct {
		Amount            float64 `json:"amount"`
		BankCode          string  `json:"bank_code"`
		BankAccountName   string  `json:"bank_account_name"`
		BankAccountNumber string  `json:"bank_account_number"`
		Description       string  `json:"description"`
		ExternalID        string  `json:"external_id"`
	}
	var list []disb
	for _, l := range p.Lines {
		ext := fmt.Sprintf("hp-%d-%d", p.RunID, l.EmployeeID)
		if err := qt.CreatePayoutItem(ctx, store.CreatePayoutItemParams{BatchID: batch.ID, EmployeeID: l.EmployeeID, ExternalID: ext,
			Amount: num(l.Amount), BankCode: l.BankCode, AccountNo: l.AccountNo, AccountName: l.AccountName}); err != nil {
			return nil, err
		}
		list = append(list, disb{Amount: l.Amount, BankCode: l.BankCode, BankAccountName: l.AccountName, BankAccountNumber: l.AccountNo,
			Description: "Salary " + p.Cycle, ExternalID: ext})
	}
	body, _ := json.Marshal(map[string]any{"reference": fmt.Sprintf("hp-run-%d", p.RunID), "disbursements": list})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBase+"/batch_disbursements", bytes.NewReader(body))
	req.SetBasicAuth(s.secret, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-IDEMPOTENCY-KEY", fmt.Sprintf("hp-run-%d", p.RunID))
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("payout partner unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("payout partner refused the batch: %s %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(raw, &out)
	status := strings.ToLower(out.Status)
	if status == "" {
		status = "submitted"
	}
	if err := qt.SetPayoutBatchSubmitted(ctx, store.SetPayoutBatchSubmittedParams{ID: batch.ID, ExternalID: out.ID, Status: status}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	batch.ExternalID, batch.Status = out.ID, status
	return &batch, nil
}

// Callback applies a partner status update (one disbursement or a batch).
func (s *Service) Callback(ctx context.Context, body []byte) (int, error) {
	type item struct {
		ExternalID  string `json:"external_id"`
		Status      string `json:"status"`
		FailureCode string `json:"failure_code"`
	}
	var payload struct {
		item
		Disbursements []item `json:"disbursements"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	items := payload.Disbursements
	if payload.ExternalID != "" {
		items = append(items, payload.item)
	}
	n := 0
	for _, it := range items {
		st := "pending"
		switch strings.ToUpper(it.Status) {
		case "COMPLETED", "SUCCEEDED":
			st = "completed"
		case "FAILED":
			st = "failed"
		}
		if c, err := s.q.UpdatePayoutItemStatus(ctx, store.UpdatePayoutItemStatusParams{ExternalID: it.ExternalID, Status: st, FailureCode: it.FailureCode}); err == nil && c > 0 {
			_ = s.q.RefreshPayoutBatchStatus(ctx, it.ExternalID)
			n++
		}
	}
	return n, nil
}

// RegisterRoutes registers admin routes and the partner callback.
func (s *Service) RegisterRoutes(protected, public *gin.RouterGroup) {
	g := protected.Group("/payouts", auth.AdminOnly())
	g.GET("", s.list)
	g.GET("/preview", s.preview)
	g.POST("", s.send)
	g.GET("/:id/items", s.items)
	public.POST("/payouts/xendit/callback", s.callbackHandler)
}

func (s *Service) list(c *gin.Context) {
	rows, err := s.q.ListPayoutBatches(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to load payouts")
		return
	}
	if rows == nil {
		rows = []store.ListPayoutBatchesRow{}
	}
	response.OK(c, gin.H{"batches": rows, "available": s.Available()})
}

func (s *Service) preview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("cycle_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "cycle_id is required")
		return
	}
	p, err := s.Preview(c.Request.Context(), auth.GetCompanyID(c), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, p)
}

func (s *Service) send(c *gin.Context) {
	var req struct {
		CycleID int64   `json:"cycle_id" binding:"required"`
		Confirm bool    `json:"confirm"`
		Total   float64 `json:"total"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		response.BadRequest(c, "cycle_id and confirm: true are required")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	// The total the admin confirmed must still be the total being sent.
	if p, err := s.Preview(ctx, companyID, req.CycleID); err == nil && math.Abs(p.Total-req.Total) > 0.005 {
		response.BadRequest(c, "The total changed since you reviewed it; review again")
		return
	}
	b, err := s.Send(ctx, companyID, auth.GetUserID(c), req.CycleID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, b)
}

func (s *Service) items(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	// Scope to the company through the batch list.
	batches, _ := s.q.ListPayoutBatches(c.Request.Context(), auth.GetCompanyID(c))
	found := false
	for _, b := range batches {
		if b.ID == id {
			found = true
		}
	}
	if !found {
		response.NotFound(c, "Payout not found")
		return
	}
	rows, err := s.q.ListPayoutItems(c.Request.Context(), id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		response.InternalError(c, "Failed to load payout")
		return
	}
	response.OK(c, rows)
}

func (s *Service) callbackHandler(c *gin.Context) {
	if s.callback == "" || subtle.ConstantTimeCompare([]byte(c.GetHeader("x-callback-token")), []byte(s.callback)) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	n, err := s.Callback(c.Request.Context(), body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": n})
}
