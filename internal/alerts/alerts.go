// Package alerts posts HR alerts to a company's Slack channel (Warp:
// "payroll notifications, tax alerts and deadline reminders in your Slack
// channels"). Each company stores one Slack incoming-webhook URL,
// encrypted, and picks which events to receive.
package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Events a company can subscribe to.
const (
	EventAgentAttention = "agent_attention" // a routine run left something for a person, or failed
	EventNotice         = "notice"          // a government notice was added
	EventFilingDue      = "filing_due"      // daily digest of returns due within 3 days
	EventPayrollReady   = "payroll_ready"   // pre-payroll check finished; ready for approval
)

// AllEvents lists the events in display order.
var AllEvents = []string{EventAgentAttention, EventNotice, EventFilingDue, EventPayrollReady}

// webhookPrefix restricts destinations to Slack incoming webhooks (no SSRF).
var webhookPrefix = "https://hooks.slack.com/"

// Service sends alerts.
type Service struct {
	q       *store.Queries
	enc     *crypto.CredentialEncryptor
	logger  *slog.Logger
	baseURL string
	client  *http.Client
}

// NewService builds the service. enc may be nil (then Slack can't be configured).
func NewService(q *store.Queries, enc *crypto.CredentialEncryptor, logger *slog.Logger, baseURL string) *Service {
	return &Service{q: q, enc: enc, logger: logger, baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

// NewServiceFromKey builds the service from a base64 encryption key.
func NewServiceFromKey(q *store.Queries, key string, logger *slog.Logger, baseURL string) *Service {
	var enc *crypto.CredentialEncryptor
	if key != "" {
		if e, err := crypto.NewCredentialEncryptor(key); err == nil {
			enc = e
		}
	}
	return NewService(q, enc, logger, baseURL)
}

// Notify posts an alert in the background if the company subscribed to the
// event. link is an app path such as "/routines?run=12".
func (s *Service) Notify(companyID int64, event, text, link string) {
	if s == nil || s.q == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.send(ctx, companyID, event, text, link); err != nil && !errors.Is(err, errNotConfigured) {
			s.logger.Warn("alerts: slack post failed", "company_id", companyID, "event", event, "error", err)
		}
	}()
}

var errNotConfigured = errors.New("slack alerts not configured")

func (s *Service) send(ctx context.Context, companyID int64, event, text, link string) error {
	ch, err := s.q.GetAlertChannel(ctx, companyID)
	if err != nil || !ch.Enabled || len(ch.SlackWebhookEnc) == 0 || !slices.Contains(ch.Events, event) || s.enc == nil {
		return errNotConfigured
	}
	url, err := s.enc.Decrypt(ch.SlackWebhookEnc)
	if err != nil {
		return err
	}
	return s.post(ctx, string(url), s.message(text, link))
}

func (s *Service) message(text, link string) string {
	if link != "" && s.baseURL != "" {
		return fmt.Sprintf("%s\n<%s%s|Open in HalaPeople>", text, s.baseURL, link)
	}
	return text
}

func (s *Service) post(ctx context.Context, url, text string) error {
	if !strings.HasPrefix(url, webhookPrefix) {
		return errors.New("not a Slack incoming webhook URL")
	}
	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned %s", resp.Status)
	}
	return nil
}

// FilingDigest posts, once a day per company, the returns due within three days.
func (s *Service) FilingDigest(ctx context.Context) int {
	ids, err := s.q.ListCompaniesDueForDigest(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, id := range ids {
		rows, err := s.q.ListFilingsDueSoon(ctx, id)
		if err != nil {
			continue
		}
		_ = s.q.MarkDigestSent(ctx, id)
		if len(rows) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString(":spiral_calendar_pad: *Filings due in the next 3 days*")
		for _, r := range rows {
			period := fmt.Sprint(r.PeriodYear)
			if r.PeriodMonth != nil {
				period = time.Date(int(r.PeriodYear), time.Month(*r.PeriodMonth), 1, 0, 0, 0, 0, time.UTC).Format("Jan 2006")
			} else if r.PeriodQuarter != nil {
				period = fmt.Sprintf("Q%d %d", *r.PeriodQuarter, r.PeriodYear)
			}
			fmt.Fprintf(&b, "\n• %s %s — due %s", strings.ToUpper(strings.ReplaceAll(r.FilingType, "_", " ")), period, r.DueDate.Format("Mon, Jan 2"))
		}
		if err := s.send(ctx, id, EventFilingDue, b.String(), "/gov-filings"); err == nil {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

// RegisterRoutes registers /alerts routes (admins).
func (s *Service) RegisterRoutes(protected *gin.RouterGroup) {
	g := protected.Group("/alerts", auth.AdminOnly())
	g.GET("/slack", s.get)
	g.PUT("/slack", s.put)
	g.DELETE("/slack", s.disconnect)
	g.POST("/slack/test", s.test)
}

type view struct {
	Connected bool     `json:"connected"`
	Hint      string   `json:"hint"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	All       []string `json:"all_events"`
	CanStore  bool     `json:"can_store"`
}

func (s *Service) current(ctx context.Context, companyID int64) view {
	v := view{Events: AllEvents, Enabled: true, All: AllEvents, CanStore: s.enc != nil}
	if ch, err := s.q.GetAlertChannel(ctx, companyID); err == nil {
		v.Connected = len(ch.SlackWebhookEnc) > 0
		v.Hint, v.Events, v.Enabled = ch.SlackHint, ch.Events, ch.Enabled
	}
	return v
}

func (s *Service) get(c *gin.Context) {
	response.OK(c, s.current(c.Request.Context(), auth.GetCompanyID(c)))
}

func (s *Service) put(c *gin.Context) {
	var req struct {
		WebhookURL string   `json:"webhook_url"`
		Events     []string `json:"events"`
		Enabled    *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	cur := s.current(ctx, companyID)
	var enc []byte
	hint := ""
	if u := strings.TrimSpace(req.WebhookURL); u != "" {
		if !strings.HasPrefix(u, webhookPrefix) {
			response.BadRequest(c, "Paste a Slack incoming webhook URL (https://hooks.slack.com/services/…)")
			return
		}
		if s.enc == nil {
			response.BadRequest(c, "INTEGRATION_ENCRYPTION_KEY is not configured")
			return
		}
		var err error
		if enc, err = s.enc.Encrypt([]byte(u)); err != nil {
			response.InternalError(c, "Failed to store webhook")
			return
		}
		hint = "…" + u[max(0, len(u)-6):]
	}
	events := cur.Events
	if req.Events != nil {
		events = []string{}
		for _, e := range req.Events {
			if !slices.Contains(AllEvents, e) {
				response.BadRequest(c, "Unknown event: "+e)
				return
			}
			events = append(events, e)
		}
	}
	enabled := cur.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	uid := auth.GetUserID(c)
	if _, err := s.q.UpsertAlertChannel(ctx, store.UpsertAlertChannelParams{
		CompanyID: companyID, SlackWebhookEnc: enc, SlackHint: hint, Events: events, Enabled: enabled, UpdatedBy: &uid,
	}); err != nil {
		s.logger.Error("alerts: save", "error", err)
		response.InternalError(c, "Failed to save")
		return
	}
	response.OK(c, s.current(ctx, companyID))
}

func (s *Service) disconnect(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	if err := s.q.ClearAlertSlack(c.Request.Context(), companyID); err != nil {
		response.InternalError(c, "Failed to disconnect")
		return
	}
	response.OK(c, s.current(c.Request.Context(), companyID))
}

func (s *Service) test(c *gin.Context) {
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	ch, err := s.q.GetAlertChannel(ctx, companyID)
	if err != nil || len(ch.SlackWebhookEnc) == 0 || s.enc == nil {
		response.BadRequest(c, "Connect Slack first")
		return
	}
	url, err := s.enc.Decrypt(ch.SlackWebhookEnc)
	if err != nil {
		response.InternalError(c, "Could not read the stored webhook")
		return
	}
	if err := s.post(ctx, string(url), s.message(":wave: HalaPeople is connected. Payroll, filing and agent alerts will appear here.", "/")); err != nil {
		response.BadRequest(c, "Slack rejected the test message: "+err.Error())
		return
	}
	response.OK(c, gin.H{"sent": true})
}
