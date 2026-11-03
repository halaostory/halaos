package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// FCM sends native-app pushes through Firebase Cloud Messaging (HTTP v1),
// which delivers to Android directly and to iOS through APNs. Configure
// FCM_SERVICE_ACCOUNT_FILE (or FCM_SERVICE_ACCOUNT_JSON) with the Firebase
// project's service-account key.
type FCM struct {
	Email     string
	Key       any // *rsa.PrivateKey
	ProjectID string
	TokenURI  string
	SendURL   string // overridable for tests
	HTTP      *http.Client

	mu     sync.Mutex
	access string
	expiry time.Time
}

type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// FCMFromEnv returns nil when no service account is configured.
func FCMFromEnv() (*FCM, error) {
	raw := []byte(os.Getenv("FCM_SERVICE_ACCOUNT_JSON"))
	if f := os.Getenv("FCM_SERVICE_ACCOUNT_FILE"); len(raw) == 0 && f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	return NewFCM(raw)
}

// NewFCM parses a Firebase service-account key.
func NewFCM(raw []byte) (*FCM, error) {
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, fmt.Errorf("fcm: service account: %w", err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("fcm: service account is missing project_id, client_email or private_key")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("fcm: private key: %w", err)
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return &FCM{Email: sa.ClientEmail, Key: key, ProjectID: sa.ProjectID, TokenURI: sa.TokenURI,
		SendURL: "https://fcm.googleapis.com/v1/projects/" + sa.ProjectID + "/messages:send", HTTP: &http.Client{Timeout: 15 * time.Second}}, nil
}

// token returns a cached OAuth access token (JWT bearer grant).
func (f *FCM) token(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.access != "" && time.Now().Before(f.expiry) {
		return f.access, nil
	}
	now := time.Now()
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": f.Email, "scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud": f.TokenURI, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}).SignedString(f.Key)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.TokenURI, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := f.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil || res.StatusCode != http.StatusOK || out.AccessToken == "" {
		return "", fmt.Errorf("fcm: token exchange failed (%d)", res.StatusCode)
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 3600
	}
	f.access, f.expiry = out.AccessToken, now.Add(time.Duration(out.ExpiresIn-60)*time.Second)
	return f.access, nil
}

// ErrUnregistered means the device token is gone (app removed / token rotated).
var ErrUnregistered = errors.New("fcm: token unregistered")

// Send delivers one message to one device token.
func (f *FCM) Send(ctx context.Context, deviceToken string, m Message) error {
	access, err := f.token(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"message": map[string]any{
		"token":        deviceToken,
		"notification": map[string]string{"title": m.Title, "body": m.Body},
		"data":         map[string]string{"url": m.URL, "tag": m.Tag},
		"android":      map[string]any{"collapse_key": m.Tag, "notification": map[string]string{"tag": m.Tag}},
		"apns":         map[string]any{"headers": map[string]string{"apns-collapse-id": m.Tag}},
	}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.SendURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	res, err := f.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	switch {
	case res.StatusCode == http.StatusOK:
		return nil
	case res.StatusCode == http.StatusNotFound || strings.Contains(string(raw), "UNREGISTERED"):
		return ErrUnregistered
	default:
		return fmt.Errorf("fcm: send failed (%d)", res.StatusCode)
	}
}
