package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Single sign-on with OpenID Connect (Google Workspace, Okta, Microsoft
// Entra ID, Keycloak…): authorization-code flow with PKCE and a nonce. The
// ID token is verified against the provider's JWKS (RS256; issuer,
// audience, expiry, nonce). The callback never puts tokens in a URL: it
// redirects with a single-use code the web app exchanges for a session.

// OIDCConfig wires public URLs.
type OIDCConfig struct {
	// AppURL is where the web app lives (redirect target after sign-in).
	AppURL string
	// RedirectURL is the callback registered with the provider. Defaults to
	// AppURL + "/api/v1/auth/oidc/callback".
	RedirectURL string
	// HTTPClient talks to providers (tests inject a fake).
	HTTPClient *http.Client
}

type oidcState struct {
	cfg   OIDCConfig
	mu    sync.Mutex
	disco map[string]cachedDiscovery
	keys  map[string]cachedKeys
}

type discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type cachedDiscovery struct {
	d  discovery
	at time.Time
}

type cachedKeys struct {
	keys map[string]*rsa.PublicKey
	at   time.Time
}

// SetOIDC enables OpenID Connect sign-in.
func (h *Handler) SetOIDC(cfg OIDCConfig) {
	cfg.AppURL = strings.TrimRight(cfg.AppURL, "/")
	if cfg.RedirectURL == "" {
		cfg.RedirectURL = cfg.AppURL + "/api/v1/auth/oidc/callback"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	h.oidc = &oidcState{cfg: cfg, disco: map[string]cachedDiscovery{}, keys: map[string]cachedKeys{}}
}

func (o *oidcState) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := o.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

func (o *oidcState) discover(ctx context.Context, issuer string) (discovery, error) {
	issuer = strings.TrimRight(issuer, "/")
	o.mu.Lock()
	c, ok := o.disco[issuer]
	o.mu.Unlock()
	if ok && time.Since(c.at) < time.Hour {
		return c.d, nil
	}
	var d discovery
	if err := o.getJSON(ctx, issuer+"/.well-known/openid-configuration", &d); err != nil {
		return d, err
	}
	if strings.TrimRight(d.Issuer, "/") != issuer || d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return d, errors.New("provider discovery document is incomplete or names another issuer")
	}
	o.mu.Lock()
	o.disco[issuer] = cachedDiscovery{d, time.Now()}
	o.mu.Unlock()
	return d, nil
}

func (o *oidcState) key(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	o.mu.Lock()
	c, ok := o.keys[jwksURI]
	o.mu.Unlock()
	if !ok || time.Since(c.at) > time.Hour || c.keys[kid] == nil {
		var set struct {
			Keys []struct {
				Kty string `json:"kty"`
				Kid string `json:"kid"`
				N   string `json:"n"`
				E   string `json:"e"`
			} `json:"keys"`
		}
		if err := o.getJSON(ctx, jwksURI, &set); err != nil {
			return nil, err
		}
		c = cachedKeys{keys: map[string]*rsa.PublicKey{}, at: time.Now()}
		for _, k := range set.Keys {
			if k.Kty != "RSA" {
				continue
			}
			n, err1 := base64.RawURLEncoding.DecodeString(k.N)
			e, err2 := base64.RawURLEncoding.DecodeString(k.E)
			if err1 != nil || err2 != nil {
				continue
			}
			c.keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		}
		o.mu.Lock()
		o.keys[jwksURI] = c
		o.mu.Unlock()
	}
	if k := c.keys[kid]; k != nil {
		return k, nil
	}
	return nil, errors.New("ID token signed with an unknown key")
}

type idClaims struct {
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // bool, or "true" for some providers
	Nonce         string `json:"nonce"`
	jwt.RegisteredClaims
}

func (c idClaims) verified() bool {
	switch v := c.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

// verifyIDToken checks signature, issuer, audience, expiry and nonce.
func (o *oidcState) verifyIDToken(ctx context.Context, d discovery, clientID, raw, nonce string) (*idClaims, error) {
	var claims idClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return o.key(ctx, d.JWKSURI, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(d.Issuer),
		jwt.WithAudience(clientID),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid ID token: %w", err)
	}
	if claims.Nonce == "" || claims.Nonce != nonce {
		return nil, errors.New("ID token nonce mismatch")
	}
	return &claims, nil
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:]))
}

// OIDCCheck handles GET /auth/oidc/check?email= — whether to offer SSO.
func (h *Handler) OIDCCheck(c *gin.Context) {
	if h.oidc == nil {
		response.OK(c, gin.H{"sso": false})
		return
	}
	_, err := h.queries.FindOIDCProviderByDomain(c.Request.Context(), emailDomain(c.Query("email")))
	response.OK(c, gin.H{"sso": err == nil})
}

func (h *Handler) oidcFail(c *gin.Context, msg string) {
	target := "/login"
	if h.oidc != nil {
		target = h.oidc.cfg.AppURL + "/login"
	}
	c.Redirect(http.StatusFound, target+"?sso_error="+strings.ReplaceAll(url.QueryEscape(msg), "+", "%20"))
}

// OIDCStart handles GET /auth/oidc/start?email= — redirects to the provider.
func (h *Handler) OIDCStart(c *gin.Context) {
	if h.oidc == nil {
		h.oidcFail(c, "Single sign-on is not configured")
		return
	}
	ctx := c.Request.Context()
	email := strings.TrimSpace(c.Query("email"))
	p, err := h.queries.FindOIDCProviderByDomain(ctx, emailDomain(email))
	if err != nil {
		h.oidcFail(c, "Your company has not set up single sign-on")
		return
	}
	d, err := h.oidc.discover(ctx, p.Issuer)
	if err != nil {
		h.logger.Warn("oidc: discovery", "issuer", p.Issuer, "error", err)
		h.oidcFail(c, "Your identity provider could not be reached")
		return
	}
	state, nonce, verifier := randomToken(24), randomToken(24), randomToken(48)
	if err := h.queries.CreateOIDCLogin(ctx, store.CreateOIDCLoginParams{State: state, CompanyID: p.CompanyID, Nonce: nonce, Verifier: verifier}); err != nil {
		h.oidcFail(c, "Could not start sign-in")
		return
	}
	_ = h.queries.PurgeOIDCLogins(ctx)
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {p.ClientID},
		"redirect_uri":          {h.oidc.cfg.RedirectURL},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	if email != "" {
		q.Set("login_hint", email)
	}
	sep := "?"
	if strings.Contains(d.AuthorizationEndpoint, "?") {
		sep = "&"
	}
	c.Redirect(http.StatusFound, d.AuthorizationEndpoint+sep+q.Encode())
}

// OIDCCallback handles GET /auth/oidc/callback?code=&state=.
func (h *Handler) OIDCCallback(c *gin.Context) {
	if h.oidc == nil {
		h.oidcFail(c, "Single sign-on is not configured")
		return
	}
	ctx := c.Request.Context()
	if e := c.Query("error"); e != "" {
		h.oidcFail(c, "Sign-in was cancelled or denied ("+e+")")
		return
	}
	login, err := h.queries.TakeOIDCLogin(ctx, c.Query("state"))
	if err != nil {
		h.oidcFail(c, "Sign-in expired; please try again")
		return
	}
	p, err := h.queries.GetOIDCProvider(ctx, login.CompanyID)
	if err != nil || !p.Enabled {
		h.oidcFail(c, "Single sign-on is not enabled")
		return
	}
	d, err := h.oidc.discover(ctx, p.Issuer)
	if err != nil {
		h.oidcFail(c, "Your identity provider could not be reached")
		return
	}
	secret, err := h.openSecret(p.ClientSecretEnc)
	if err != nil {
		h.oidcFail(c, "Single sign-on is misconfigured")
		return
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {c.Query("code")},
		"redirect_uri":  {h.oidc.cfg.RedirectURL},
		"client_id":     {p.ClientID},
		"client_secret": {string(secret)},
		"code_verifier": {login.Verifier},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenEndpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := h.oidc.cfg.HTTPClient.Do(req)
	if err != nil {
		h.oidcFail(c, "Your identity provider could not be reached")
		return
	}
	defer resp.Body.Close()
	var tok struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tok)
	if resp.StatusCode != http.StatusOK || tok.IDToken == "" {
		h.logger.Warn("oidc: token exchange", "status", resp.Status, "error", tok.Error)
		h.oidcFail(c, "Your identity provider refused the sign-in")
		return
	}
	claims, err := h.oidc.verifyIDToken(ctx, d, p.ClientID, tok.IDToken, login.Nonce)
	if err != nil {
		h.logger.Warn("oidc: id token", "error", err)
		h.oidcFail(c, "Sign-in could not be verified")
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" || !claims.verified() || !slices.Contains(p.EmailDomains, emailDomain(email)) {
		h.oidcFail(c, "Your identity provider did not confirm a company e-mail address")
		return
	}
	// The account at the company whose identity provider signed them in
	// (the same email may have accounts at other companies too).
	var user store.User
	found := false
	if rows, err := h.queries.ListUsersByEmail(ctx, email); err == nil {
		for _, r := range rows {
			if r.CompanyID == login.CompanyID {
				if u, err := h.queries.GetUserByID(ctx, r.ID); err == nil {
					user, found = u, true
				}
				break
			}
		}
	}
	if !found {
		h.oidcFail(c, "No account for "+email+"; ask your HR admin to invite you")
		return
	}
	if user.Status != "active" {
		h.oidcFail(c, "Account has been disabled")
		return
	}
	code := randomToken(32)
	hash := sha256hex(code)
	if err := h.queries.SetOIDCLoginResult(ctx, store.SetOIDCLoginResultParams{State: login.State, UserID: &user.ID, ExchangeHash: &hash}); err != nil {
		h.oidcFail(c, "Could not complete sign-in")
		return
	}
	c.Redirect(http.StatusFound, h.oidc.cfg.AppURL+"/sso?oidc="+url.QueryEscape(code))
}

// OIDCExchange handles POST /auth/oidc/exchange {code} → session tokens.
func (h *Handler) OIDCExchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "code is required")
		return
	}
	hash := sha256hex(req.Code)
	uid, err := h.queries.RedeemOIDCExchange(c.Request.Context(), &hash)
	if err != nil || uid == nil {
		response.Error(c, http.StatusUnauthorized, "sso_expired", "Sign-in expired; please try again")
		return
	}
	user, err := h.queries.GetUserByID(c.Request.Context(), *uid)
	if err != nil || user.Status != "active" {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Account not available")
		return
	}
	_ = h.queries.MarkEmailVerified(c.Request.Context(), user.ID)
	h.completeLogin(c, user)
}

// ---------------------------------------------------------------------------
// Admin configuration
// ---------------------------------------------------------------------------

type oidcView struct {
	Configured   bool     `json:"configured"`
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	EmailDomains []string `json:"email_domains"`
	Enabled      bool     `json:"enabled"`
	RedirectURL  string   `json:"redirect_url"`
	Available    bool     `json:"available"`
}

func (h *Handler) oidcView(ctx context.Context, companyID int64) oidcView {
	v := oidcView{EmailDomains: []string{}, Available: h.oidc != nil}
	if h.oidc != nil {
		v.RedirectURL = h.oidc.cfg.RedirectURL
	}
	if p, err := h.queries.GetOIDCProvider(ctx, companyID); err == nil {
		v.Configured, v.Issuer, v.ClientID, v.EmailDomains, v.Enabled = true, p.Issuer, p.ClientID, p.EmailDomains, p.Enabled
	}
	return v
}

// GetOIDCProvider handles GET /auth/oidc/provider (admin).
func (h *Handler) GetOIDCProvider(c *gin.Context) {
	response.OK(c, h.oidcView(c.Request.Context(), GetCompanyID(c)))
}

// SaveOIDCProvider handles PUT /auth/oidc/provider (admin).
func (h *Handler) SaveOIDCProvider(c *gin.Context) {
	if h.oidc == nil {
		response.BadRequest(c, "Single sign-on is not available on this server")
		return
	}
	var req struct {
		Issuer       string   `json:"issuer" binding:"required"`
		ClientID     string   `json:"client_id" binding:"required"`
		ClientSecret string   `json:"client_secret"`
		EmailDomains []string `json:"email_domains"`
		Enabled      *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "issuer and client_id are required")
		return
	}
	ctx := c.Request.Context()
	companyID := GetCompanyID(c)
	issuer := strings.TrimRight(strings.TrimSpace(req.Issuer), "/")
	if u, err := url.Parse(issuer); err != nil || u.Scheme != "https" || u.Host == "" {
		response.BadRequest(c, "Issuer must be an https URL, e.g. https://accounts.google.com")
		return
	}
	domains := []string{}
	for _, d := range req.EmailDomains {
		d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(d, "@")))
		if d != "" && strings.Contains(d, ".") && !slices.Contains(domains, d) {
			domains = append(domains, d)
		}
	}
	if len(domains) == 0 {
		response.BadRequest(c, "Add at least one e-mail domain (e.g. acme.com)")
		return
	}
	if taken, err := h.queries.DomainTakenByOtherCompany(ctx, store.DomainTakenByOtherCompanyParams{CompanyID: companyID, Domains: domains}); err == nil && taken {
		response.BadRequest(c, "One of these domains already uses single sign-on with another company")
		return
	}
	if _, err := h.oidc.discover(ctx, issuer); err != nil {
		response.BadRequest(c, "Could not read the provider's discovery document: "+err.Error())
		return
	}
	var sealed []byte
	if req.ClientSecret != "" {
		s, err := h.sealSecret([]byte(req.ClientSecret))
		if err != nil {
			response.InternalError(c, "Failed to store the client secret")
			return
		}
		sealed = s
	} else if p, err := h.queries.GetOIDCProvider(ctx, companyID); err == nil {
		sealed = p.ClientSecretEnc
	} else {
		response.BadRequest(c, "client_secret is required")
		return
	}
	enabled := req.Enabled == nil || *req.Enabled
	uid := GetUserID(c)
	if _, err := h.queries.UpsertOIDCProvider(ctx, store.UpsertOIDCProviderParams{
		CompanyID: companyID, Issuer: issuer, ClientID: strings.TrimSpace(req.ClientID), ClientSecretEnc: sealed,
		EmailDomains: domains, Enabled: enabled, UpdatedBy: &uid,
	}); err != nil {
		response.InternalError(c, "Failed to save")
		return
	}
	response.OK(c, h.oidcView(ctx, companyID))
}

// DeleteOIDCProvider handles DELETE /auth/oidc/provider (admin).
func (h *Handler) DeleteOIDCProvider(c *gin.Context) {
	companyID := GetCompanyID(c)
	_ = h.queries.DeleteOIDCProvider(c.Request.Context(), companyID)
	response.OK(c, h.oidcView(c.Request.Context(), companyID))
}
