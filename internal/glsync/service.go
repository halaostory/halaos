package glsync

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Service connects companies to Xero / QuickBooks and pushes journals.
type Service struct {
	q         *store.Queries
	enc       *crypto.CredentialEncryptor
	logger    *slog.Logger
	appURL    string
	providers map[string]*Provider
}

// NewFromEnv builds the service from XERO_CLIENT_ID/SECRET,
// QBO_CLIENT_ID/SECRET (QBO_API_URL for the sandbox) and APP_BASE_URL.
func NewFromEnv(q *store.Queries, encKey, appURL string, logger *slog.Logger) *Service {
	var enc *crypto.CredentialEncryptor
	if encKey != "" {
		enc, _ = crypto.NewCredentialEncryptor(encKey)
	}
	appURL = strings.TrimRight(appURL, "/")
	client := &http.Client{Timeout: 20 * time.Second}
	qbo := QuickBooksEndpoints
	if v := os.Getenv("QBO_API_URL"); v != "" {
		qbo.API = v
	}
	return New(q, enc, appURL, logger, map[string]*Provider{
		"xero": {Name: "xero", ClientID: os.Getenv("XERO_CLIENT_ID"), ClientSecret: os.Getenv("XERO_CLIENT_SECRET"),
			Scope: "openid profile email accounting.transactions offline_access", EP: XeroEndpoints, HTTP: client},
		"quickbooks": {Name: "quickbooks", ClientID: os.Getenv("QBO_CLIENT_ID"), ClientSecret: os.Getenv("QBO_CLIENT_SECRET"),
			Scope: "com.intuit.quickbooks.accounting", EP: qbo, HTTP: client},
	})
}

// New builds the service with explicit providers (tests).
func New(q *store.Queries, enc *crypto.CredentialEncryptor, appURL string, logger *slog.Logger, providers map[string]*Provider) *Service {
	for name, p := range providers {
		if p.RedirectURL == "" {
			p.RedirectURL = appURL + "/api/v1/gl/" + name + "/callback"
		}
	}
	return &Service{q: q, enc: enc, logger: logger, appURL: appURL, providers: providers}
}

// RegisterRoutes registers admin routes (protected) and the OAuth callback (public).
func (s *Service) RegisterRoutes(protected, public *gin.RouterGroup) {
	g := protected.Group("/gl", auth.AdminOnly())
	g.GET("/connections", s.list)
	g.POST("/:provider/connect", s.connect)
	g.DELETE("/:provider", s.disconnect)
	g.POST("/:provider/push", s.push)
	public.GET("/gl/:provider/callback", s.callback)
}

func (s *Service) provider(c *gin.Context) (*Provider, bool) {
	p := s.providers[c.Param("provider")]
	if p == nil {
		response.NotFound(c, "Unknown accounting system")
		return nil, false
	}
	return p, true
}

func (s *Service) list(c *gin.Context) {
	rows, _ := s.q.ListGLConnections(c.Request.Context(), auth.GetCompanyID(c))
	conn := map[string]store.ListGLConnectionsRow{}
	for _, r := range rows {
		conn[r.Provider] = r
	}
	out := []gin.H{}
	for _, name := range []string{"xero", "quickbooks"} {
		p := s.providers[name]
		item := gin.H{"provider": name, "available": p.Configured() && s.enc != nil, "connected": false}
		if r, ok := conn[name]; ok {
			item["connected"], item["tenant_name"], item["connected_at"] = true, r.TenantName, r.CreatedAt
		}
		out = append(out, item)
	}
	response.OK(c, out)
}

func randomState() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Service) connect(c *gin.Context) {
	p, ok := s.provider(c)
	if !ok {
		return
	}
	if !p.Configured() || s.enc == nil {
		response.BadRequest(c, "This server has no "+p.Name+" app credentials (set the client ID/secret and INTEGRATION_ENCRYPTION_KEY)")
		return
	}
	state := randomState()
	uid := auth.GetUserID(c)
	if err := s.q.CreateGLOAuthState(c.Request.Context(), store.CreateGLOAuthStateParams{State: state, CompanyID: auth.GetCompanyID(c), UserID: &uid, Provider: p.Name}); err != nil {
		response.InternalError(c, "Could not start the connection")
		return
	}
	response.OK(c, gin.H{"url": p.AuthURL(state)})
}

func (s *Service) back(c *gin.Context, msg string) {
	q := "gl=connected"
	if msg != "" {
		q = "gl_error=" + strings.ReplaceAll(url.QueryEscape(msg), "+", "%20")
	}
	c.Redirect(http.StatusFound, s.appURL+"/payroll?"+q)
}

func (s *Service) callback(c *gin.Context) {
	p, ok := s.provider(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	st, err := s.q.TakeGLOAuthState(ctx, c.Query("state"))
	if err != nil || st.Provider != p.Name {
		s.back(c, "The connection expired; try again")
		return
	}
	if e := c.Query("error"); e != "" {
		s.back(c, p.Name+" declined the connection ("+e+")")
		return
	}
	tok, err := p.Exchange(ctx, c.Query("code"))
	if err != nil {
		s.logger.Warn("glsync: exchange", "provider", p.Name, "error", err)
		s.back(c, "Could not connect to "+p.Name)
		return
	}
	tenant, name := c.Query("realmId"), "QuickBooks company "+c.Query("realmId")
	if p.Name == "xero" {
		if tenant, name, err = p.XeroTenant(ctx, tok.Access); err != nil {
			s.back(c, err.Error())
			return
		}
	}
	if tenant == "" {
		s.back(c, "No company was selected")
		return
	}
	if err := s.save(ctx, st.CompanyID, p.Name, tenant, name, tok, st.UserID); err != nil {
		s.back(c, "Could not store the connection")
		return
	}
	s.back(c, "")
}

func (s *Service) save(ctx context.Context, companyID int64, provider, tenant, name string, t Tokens, by *int64) error {
	a, err := s.enc.Encrypt([]byte(t.Access))
	if err != nil {
		return err
	}
	r, err := s.enc.Encrypt([]byte(t.Refresh))
	if err != nil {
		return err
	}
	return s.q.UpsertGLConnection(ctx, store.UpsertGLConnectionParams{CompanyID: companyID, Provider: provider, TenantID: tenant, TenantName: name,
		AccessTokenEnc: a, RefreshTokenEnc: r, ExpiresAt: t.ExpiresAt, ConnectedBy: by})
}

// accessToken returns a valid access token, refreshing it when expired.
func (s *Service) accessToken(ctx context.Context, p *Provider, conn store.GlConnection) (string, error) {
	access, err := s.enc.Decrypt(conn.AccessTokenEnc)
	if err != nil {
		return "", err
	}
	if time.Now().Before(conn.ExpiresAt) {
		return string(access), nil
	}
	refresh, err := s.enc.Decrypt(conn.RefreshTokenEnc)
	if err != nil {
		return "", err
	}
	t, err := p.Refresh(ctx, string(refresh))
	if err != nil {
		return "", errors.New(p.Name + " connection expired; reconnect it")
	}
	a, _ := s.enc.Encrypt([]byte(t.Access))
	r, _ := s.enc.Encrypt([]byte(t.Refresh))
	_ = s.q.UpdateGLTokens(ctx, store.UpdateGLTokensParams{CompanyID: conn.CompanyID, Provider: conn.Provider, AccessTokenEnc: a, RefreshTokenEnc: r, ExpiresAt: t.ExpiresAt})
	return t.Access, nil
}

// Push sends a cycle's journal; a run is pushed at most once per system.
func (s *Service) Push(ctx context.Context, companyID, userID, cycleID int64, provider string) (*store.GlPush, bool, error) {
	p := s.providers[provider]
	if p == nil || s.enc == nil {
		return nil, false, errors.New("unknown accounting system")
	}
	conn, err := s.q.GetGLConnection(ctx, store.GetGLConnectionParams{CompanyID: companyID, Provider: provider})
	if err != nil {
		return nil, false, errors.New("connect " + provider + " first")
	}
	j, err := payroll.CycleJournal(ctx, s.q, companyID, cycleID)
	if err != nil {
		return nil, false, errors.New("this cycle has no calculated payroll run yet")
	}
	if prev, err := s.q.GetGLPush(ctx, store.GetGLPushParams{Provider: provider, RunID: j.RunID}); err == nil {
		return &prev, true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	access, err := s.accessToken(ctx, p, conn)
	if err != nil {
		return nil, false, err
	}
	var ext string
	if provider == "xero" {
		ext, err = p.PushXero(ctx, access, conn.TenantID, *j)
	} else {
		ext, err = p.PushQuickBooks(ctx, access, conn.TenantID, *j)
	}
	if err != nil {
		return nil, false, err
	}
	var by *int64
	if userID > 0 {
		by = &userID
	}
	rec, err := s.q.CreateGLPush(ctx, store.CreateGLPushParams{CompanyID: companyID, Provider: provider, RunID: j.RunID, ExternalID: ext, PushedBy: by})
	if err != nil {
		return nil, false, err
	}
	return &rec, false, nil
}

func (s *Service) push(c *gin.Context) {
	var req struct {
		CycleID int64 `json:"cycle_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "cycle_id is required")
		return
	}
	rec, already, err := s.Push(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), req.CycleID, c.Param("provider"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"push": rec, "already_pushed": already})
}

func (s *Service) disconnect(c *gin.Context) {
	p, ok := s.provider(c)
	if !ok {
		return
	}
	_ = s.q.DeleteGLConnection(c.Request.Context(), store.DeleteGLConnectionParams{CompanyID: auth.GetCompanyID(c), Provider: p.Name})
	response.OK(c, gin.H{"disconnected": true, "provider": p.Name, "at": strconv.FormatInt(time.Now().Unix(), 10)})
}
