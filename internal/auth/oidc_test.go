package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/halaostory/halaos/internal/store"
)

// fakeIdP is a minimal OpenID provider: discovery, JWKS, authorize (not
// used — tests jump straight to the callback) and token endpoints.
type fakeIdP struct {
	srv   *httptest.Server
	key   *rsa.PrivateKey
	email string
	nonce string // nonce to put in the next ID token ("" = echo none)
	aud   string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeIdP{key: key, aud: "client-123"}
	mux := http.NewServeMux()
	f.srv = httptest.NewTLSServer(mux)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/authorize",
			"token_endpoint": f.srv.URL + "/token", "jwks_uri": f.srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "good-code" || r.Form.Get("client_secret") != "s3cret" || r.Form.Get("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": f.idToken(f.nonce, f.aud, time.Now().Add(time.Hour))})
	})
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) idToken(nonce, aud string, exp time.Time) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": f.srv.URL, "aud": aud, "sub": "u1", "email": f.email, "email_verified": true,
		"nonce": nonce, "exp": exp.Unix(), "iat": time.Now().Unix(),
	})
	tok.Header["kid"] = "k1"
	s, _ := tok.SignedString(f.key)
	return s
}

func TestVerifyIDToken(t *testing.T) {
	f := newFakeIdP(t)
	h := &Handler{jwt: NewJWTService("x-secret-0123456789", time.Hour, time.Hour), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h.SetOIDC(OIDCConfig{AppURL: "https://people.example.com", HTTPClient: f.srv.Client()})
	ctx := context.Background()
	d, err := h.oidc.discover(ctx, f.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	f.email = "ana@acme.test"
	if c, err := h.oidc.verifyIDToken(ctx, d, "client-123", f.idToken("n1", "client-123", time.Now().Add(time.Hour)), "n1"); err != nil || c.Email != "ana@acme.test" || !c.verified() {
		t.Fatalf("valid token: %v %+v", err, c)
	}
	for name, raw := range map[string]string{
		"wrong nonce":    f.idToken("other", "client-123", time.Now().Add(time.Hour)),
		"wrong audience": f.idToken("n1", "someone-else", time.Now().Add(time.Hour)),
		"expired":        f.idToken("n1", "client-123", time.Now().Add(-time.Hour)),
		"tampered":       strings.Replace(f.idToken("n1", "client-123", time.Now().Add(time.Hour)), ".", ".x", 1),
	} {
		if _, err := h.oidc.verifyIDToken(ctx, d, "client-123", raw, "n1"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// HS256 with the public key as secret (alg confusion) must be refused.
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": f.srv.URL, "aud": "client-123", "nonce": "n1", "exp": time.Now().Add(time.Hour).Unix()})
	hs.Header["kid"] = "k1"
	raw, _ := hs.SignedString(f.key.N.Bytes())
	if _, err := h.oidc.verifyIDToken(ctx, d, "client-123", raw, "n1"); err == nil {
		t.Error("HS256 token accepted")
	}
}

// TestOIDCFlow runs start → callback → exchange against a real database
// (HALAOS_TEST_DATABASE_URL) and the fake provider.
func TestOIDCFlow(t *testing.T) {
	dsn := os.Getenv("HALAOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HALAOS_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := store.New(pool)
	f := newFakeIdP(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	domain := "sso" + sfx + ".test"
	var companyID, userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "SSO Co "+sfx).Scan(&companyID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, companyID) //nolint:errcheck
	pw, _ := bcrypt.GenerateFromPassword([]byte("x"), bcrypt.MinCost)
	if err := pool.QueryRow(ctx, `INSERT INTO users (company_id, email, password_hash, first_name, last_name, role) VALUES ($1, $2, $3, 'Ana', 'Cruz', 'employee') RETURNING id`,
		companyID, "ana@"+domain, string(pw)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck

	gin.SetMode(gin.TestMode)
	h := NewHandler(q, pool, NewJWTService("x-secret-0123456789", time.Hour, time.Hour), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	h.SetOIDC(OIDCConfig{AppURL: "https://people.example.com", HTTPClient: f.srv.Client()})
	sealed, _ := h.sealSecret([]byte("s3cret"))
	if _, err := q.UpsertOIDCProvider(ctx, store.UpsertOIDCProviderParams{CompanyID: companyID, Issuer: f.srv.URL, ClientID: "client-123", ClientSecretEnc: sealed, EmailDomains: []string{domain}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/start", h.OIDCStart)
	r.GET("/callback", h.OIDCCallback)
	r.POST("/exchange", h.OIDCExchange)
	do := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	// 1. start → redirect to the provider with PKCE, state and nonce
	w := do("GET", "/start?email=ana@"+domain, "")
	loc, _ := url.Parse(w.Header().Get("Location"))
	if w.Code != http.StatusFound || !strings.HasPrefix(loc.String(), f.srv.URL+"/authorize") || loc.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("start: %d %s", w.Code, loc)
	}
	state := loc.Query().Get("state")
	var nonce string
	_ = pool.QueryRow(ctx, `SELECT nonce FROM oidc_logins WHERE state = $1`, state).Scan(&nonce)
	f.email, f.nonce = "ana@"+domain, nonce

	// 2. callback → single-use code in the redirect, never tokens
	w = do("GET", "/callback?code=good-code&state="+url.QueryEscape(state), "")
	loc, _ = url.Parse(w.Header().Get("Location"))
	code := loc.Query().Get("oidc")
	if w.Code != http.StatusFound || loc.Path != "/sso" || code == "" || strings.Contains(loc.String(), "token") {
		t.Fatalf("callback: %d %s", w.Code, loc)
	}
	// replaying the state fails
	if w2 := do("GET", "/callback?code=good-code&state="+url.QueryEscape(state), ""); !strings.Contains(w2.Header().Get("Location"), "sso_error") {
		t.Error("state replay accepted")
	}

	// 3. exchange → session, once
	w = do("POST", "/exchange", `{"code":"`+code+`"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"token"`) {
		t.Fatalf("exchange: %d %s", w.Code, w.Body.String())
	}
	if w = do("POST", "/exchange", `{"code":"`+code+`"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("second exchange: %d", w.Code)
	}

	// someone from another domain cannot ride the provider
	w = do("GET", "/start?email=eve@evil.test", "")
	if !strings.Contains(w.Header().Get("Location"), "sso_error") {
		t.Errorf("unknown domain: %s", w.Header().Get("Location"))
	}
}
