// Package alerts posts HR alerts to a company's team chat (Warp: "payroll
// notifications, tax alerts and deadline reminders in your Slack channels").
// Besides Slack it supports Microsoft Teams, Google Chat, Lark/Feishu,
// Telegram groups and LINE groups, which most Southeast Asian teams use.
// Each company stores one channel (webhook URL or bot token, encrypted) and
// picks which events to receive.
package alerts

import (
	"context"
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
			s.logger.Warn("alerts: post failed", "company_id", companyID, "event", event, "error", err)
		}
	}()
}

var errNotConfigured = errors.New("team chat alerts not configured")

// ErrNotConnected means the company has no enabled team chat channel.
var ErrNotConnected = errNotConfigured

// ErrSlackNotConnected is the old name of ErrNotConnected.
var ErrSlackNotConnected = ErrNotConnected

// PostMessage posts text to the company's team chat now, whatever the event
// subscriptions (a routine step asked for it explicitly).
func (s *Service) PostMessage(ctx context.Context, companyID int64, text, link string) error {
	return s.send(ctx, companyID, "", text, link)
}

// ChannelName is the display name of the company's connected channel ("" if none).
func (s *Service) ChannelName(ctx context.Context, companyID int64) string {
	if s == nil || s.q == nil {
		return ""
	}
	if ch, err := s.q.GetAlertChannel(ctx, companyID); err == nil && ch.Enabled && len(ch.SecretEnc) > 0 {
		return KindNames[ch.Kind]
	}
	return ""
}

// KindNames are the platforms' display names.
var KindNames = map[string]string{
	KindSlack: "Slack", KindTeams: "Microsoft Teams", KindGoogleChat: "Google Chat",
	KindLark: "Lark", KindTelegram: "Telegram", KindLINE: "LINE",
}

// send posts to the stored channel; event "" skips the subscription check.
func (s *Service) send(ctx context.Context, companyID int64, event, text, link string) error {
	if s == nil || s.q == nil {
		return errNotConfigured
	}
	ch, err := s.q.GetAlertChannel(ctx, companyID)
	if err != nil || !ch.Enabled || len(ch.SecretEnc) == 0 || s.enc == nil || (event != "" && !slices.Contains(ch.Events, event)) {
		return errNotConfigured
	}
	t, err := s.resolve(ch)
	if err != nil {
		return err
	}
	return s.post(ctx, t, s.msg(text, link))
}

func (s *Service) resolve(ch store.AlertChannel) (target, error) {
	secret, err := s.enc.Decrypt(ch.SecretEnc)
	if err != nil {
		return target{}, err
	}
	return target{kind: ch.Kind, secret: string(secret), chat: ch.Target}, nil
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
		b.WriteString("🗓️ *Filings due in the next 3 days*")
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
	g.GET("/channel", s.get)
	g.PUT("/channel", s.put)
	g.DELETE("/channel", s.disconnect)
	g.POST("/channel/test", s.test)
	g.POST("/channel/telegram/groups", s.findTelegramGroups)
}

type view struct {
	Kind      string   `json:"kind"`
	Connected bool     `json:"connected"`
	Hint      string   `json:"hint"`
	Target    string   `json:"target"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	All       []string `json:"all_events"`
	Kinds     []string `json:"kinds"`
	CanStore  bool     `json:"can_store"`
}

func (s *Service) current(ctx context.Context, companyID int64) view {
	v := view{Kind: KindSlack, Events: AllEvents, Enabled: true, All: AllEvents, Kinds: AllKinds, CanStore: s.enc != nil}
	if ch, err := s.q.GetAlertChannel(ctx, companyID); err == nil {
		v.Connected = len(ch.SecretEnc) > 0
		v.Kind, v.Hint, v.Target, v.Events, v.Enabled = ch.Kind, ch.Hint, ch.Target, ch.Events, ch.Enabled
	}
	return v
}

func (s *Service) get(c *gin.Context) {
	response.OK(c, s.current(c.Request.Context(), auth.GetCompanyID(c)))
}

// put connects a channel (kind + webhook_url, or kind + token + target) and/or
// changes the events and the on/off switch. A target alone may be changed
// for a connected Telegram/LINE channel without re-entering the token.
func (s *Service) put(c *gin.Context) {
	var req struct {
		Kind       string   `json:"kind"`
		WebhookURL string   `json:"webhook_url"`
		Token      string   `json:"token"`
		Target     *string  `json:"target"`
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
	kind, tgt := cur.Kind, cur.Target
	var enc []byte
	hint := ""
	secret := ""
	if req.Kind != "" && req.Kind != cur.Kind {
		if !slices.Contains(AllKinds, req.Kind) {
			response.BadRequest(c, "Unknown channel: "+req.Kind)
			return
		}
		kind, tgt = req.Kind, ""
	}
	if req.Target != nil {
		tgt = strings.TrimSpace(*req.Target)
	}
	if usesWebhook(kind) {
		tgt = ""
		if u := strings.TrimSpace(req.WebhookURL); u != "" {
			if !validWebhook(kind, u) {
				response.BadRequest(c, webhookHelp[kind])
				return
			}
			secret, hint = u, "…"+u[max(0, len(u)-6):]
		}
	} else if tok := strings.TrimSpace(req.Token); tok != "" || req.Target != nil {
		if err := validateBot(kind, tok, tgt); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		if tok != "" {
			secret, hint = tok, "…"+tok[max(0, len(tok)-4):]
		}
	}
	if kind != cur.Kind && secret == "" {
		response.BadRequest(c, "Enter the "+KindNames[kind]+" details to switch channels")
		return
	}
	if secret != "" {
		if s.enc == nil {
			response.BadRequest(c, "INTEGRATION_ENCRYPTION_KEY is not configured")
			return
		}
		var err error
		if enc, err = s.enc.Encrypt([]byte(secret)); err != nil {
			response.InternalError(c, "Failed to store the channel")
			return
		}
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
		CompanyID: companyID, Kind: kind, SecretEnc: enc, Hint: hint, Target: tgt, Events: events, Enabled: enabled, UpdatedBy: &uid,
	}); err != nil {
		s.logger.Error("alerts: save", "error", err)
		response.InternalError(c, "Failed to save")
		return
	}
	response.OK(c, s.current(ctx, companyID))
}

var webhookHelp = map[string]string{
	KindSlack:      "Paste a Slack incoming webhook URL (https://hooks.slack.com/services/…)",
	KindTeams:      "Paste the Teams workflow URL (Workflows → “Post to a channel when a webhook request is received”)",
	KindGoogleChat: "Paste a Google Chat webhook URL (https://chat.googleapis.com/v1/spaces/…)",
	KindLark:       "Paste a Lark/Feishu custom bot webhook URL (https://open.larksuite.com/open-apis/bot/v2/hook/…)",
}

func (s *Service) disconnect(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	if err := s.q.ClearAlertChannel(c.Request.Context(), companyID); err != nil {
		response.InternalError(c, "Failed to disconnect")
		return
	}
	response.OK(c, s.current(c.Request.Context(), companyID))
}

func (s *Service) test(c *gin.Context) {
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	ch, err := s.q.GetAlertChannel(ctx, companyID)
	if err != nil || len(ch.SecretEnc) == 0 || s.enc == nil {
		response.BadRequest(c, "Connect a team chat first")
		return
	}
	t, err := s.resolve(ch)
	if err != nil {
		response.InternalError(c, "Could not read the stored channel")
		return
	}
	if err := s.post(ctx, t, s.msg("👋 HalaPeople is connected. Payroll, filing and agent alerts will appear here.", "/")); err != nil {
		response.BadRequest(c, KindNames[ch.Kind]+" rejected the test message: "+err.Error())
		return
	}
	response.OK(c, gin.H{"sent": true})
}

// findTelegramGroups lists groups the bot was added to, for picking the chat
// ID. Uses the token in the request, or the stored one.
func (s *Service) findTelegramGroups(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	_ = c.ShouldBindJSON(&req)
	ctx := c.Request.Context()
	tok := strings.TrimSpace(req.Token)
	if tok == "" {
		if ch, err := s.q.GetAlertChannel(ctx, auth.GetCompanyID(c)); err == nil && ch.Kind == KindTelegram && len(ch.SecretEnc) > 0 && s.enc != nil {
			if b, err := s.enc.Decrypt(ch.SecretEnc); err == nil {
				tok = string(b)
			}
		}
	}
	if !reTelegramToken.MatchString(tok) {
		response.BadRequest(c, "Paste the bot token from @BotFather first")
		return
	}
	groups, err := s.telegramGroups(ctx, tok)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"groups": groups})
}
