// Package push sends Web Push notifications to employees' phones (the
// installed mobile app / PWA). Every in-app notification is also pushed to
// the person's subscribed devices; nothing else is sent.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Service holds the VAPID keys and sends pushes.
type Service struct {
	q       *store.Queries
	logger  *slog.Logger
	public  string
	private string
	subject string
	HTTP    *http.Client
}

// New loads VAPID keys from VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY, or from
// the database, generating and saving a pair the first time.
func New(ctx context.Context, q *store.Queries, logger *slog.Logger, appURL string) (*Service, error) {
	s := &Service{q: q, logger: logger, HTTP: &http.Client{Timeout: 15 * time.Second},
		public: os.Getenv("VAPID_PUBLIC_KEY"), private: os.Getenv("VAPID_PRIVATE_KEY"), subject: os.Getenv("VAPID_SUBJECT")}
	if s.subject == "" {
		s.subject = strings.TrimRight(appURL, "/")
		if s.subject == "" {
			s.subject = "mailto:admin@localhost"
		}
	}
	if s.public != "" && s.private != "" {
		return s, nil
	}
	k, err := q.GetVapidKeys(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		priv, pub, gerr := webpush.GenerateVAPIDKeys()
		if gerr != nil {
			return nil, gerr
		}
		if err := q.InsertVapidKeys(ctx, store.InsertVapidKeysParams{PublicKey: pub, PrivateKey: priv}); err != nil {
			return nil, err
		}
		k, err = q.GetVapidKeys(ctx) // another instance may have won the race
	}
	if err != nil {
		return nil, err
	}
	s.public, s.private = k.PublicKey, k.PrivateKey
	return s, nil
}

// RegisterRoutes: the public key, and subscribe/unsubscribe for the caller.
func (s *Service) RegisterRoutes(protected *gin.RouterGroup) {
	g := protected.Group("/push")
	g.GET("/key", func(c *gin.Context) {
		n, _ := s.q.CountPushSubscriptions(c.Request.Context(), auth.GetUserID(c))
		response.OK(c, gin.H{"public_key": s.public, "devices": n})
	})
	g.POST("/subscribe", s.subscribe)
	g.POST("/unsubscribe", s.unsubscribe)
}

type subscription struct {
	Endpoint string `json:"endpoint" binding:"required"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *Service) subscribe(c *gin.Context) {
	var req subscription
	if err := c.ShouldBindJSON(&req); err != nil || !strings.HasPrefix(req.Endpoint, "https://") || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		response.BadRequest(c, "Invalid push subscription")
		return
	}
	ua := c.GetHeader("User-Agent")
	if len(ua) > 300 {
		ua = ua[:300]
	}
	if err := s.q.UpsertPushSubscription(c.Request.Context(), store.UpsertPushSubscriptionParams{CompanyID: auth.GetCompanyID(c), UserID: auth.GetUserID(c),
		Endpoint: req.Endpoint, P256dh: req.Keys.P256dh, Auth: req.Keys.Auth, UserAgent: ua}); err != nil {
		response.InternalError(c, "Could not save the subscription")
		return
	}
	response.OK(c, gin.H{"subscribed": true})
}

func (s *Service) unsubscribe(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "endpoint is required")
		return
	}
	_ = s.q.DeletePushSubscription(c.Request.Context(), store.DeletePushSubscriptionParams{Endpoint: req.Endpoint, UserID: auth.GetUserID(c)})
	response.OK(c, gin.H{"subscribed": false})
}

// Message is what the service worker shows.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag,omitempty"`
}

// LinkFor maps a notification category to the app page that opens.
func LinkFor(category string) string {
	switch category {
	case "leave":
		return "/m/leave"
	case "payroll":
		return "/m/payslips"
	case "expense":
		return "/m/expenses"
	default:
		return "/m/notifications"
	}
}

// Send pushes m to every device of the user; expired subscriptions are
// removed. It returns how many devices accepted it.
func (s *Service) Send(ctx context.Context, companyID, userID int64, m Message) int {
	subs, err := s.q.ListPushSubscriptions(ctx, store.ListPushSubscriptionsParams{UserID: userID, CompanyID: companyID})
	if err != nil || len(subs) == 0 {
		return 0
	}
	body, _ := json.Marshal(m)
	sent := 0
	for _, sub := range subs {
		res, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth}}, &webpush.Options{
			HTTPClient: s.HTTP, Subscriber: s.subject, VAPIDPublicKey: s.public, VAPIDPrivateKey: s.private, TTL: 86400, Urgency: webpush.UrgencyNormal, Topic: m.Tag,
		})
		if err != nil {
			s.logger.Warn("push: send failed", "user_id", userID, "error", err)
			continue
		}
		res.Body.Close()
		switch {
		case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
			_ = s.q.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint) // device unsubscribed or app removed
		case res.StatusCode < 300:
			sent++
			_ = s.q.TouchPushSubscription(ctx, sub.ID)
		default:
			s.logger.Warn("push: rejected", "user_id", userID, "status", res.StatusCode)
		}
	}
	return sent
}
