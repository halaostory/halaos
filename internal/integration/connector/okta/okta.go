// Package okta provisions Okta accounts (Warp: "Okta: Create, Update,
// Deactivate… at hire and exit"). It uses the Okta Users API with an API
// token: credentials {"api_key": "<SSWS token>", "extra": {"domain":
// "acme.okta.com"}}.
package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/halaostory/halaos/internal/integration/connector"
)

// Connector implements the Okta integration.
type Connector struct {
	base   string
	token  string
	client *http.Client
}

// New creates an Okta connector from credentials.
func New(creds connector.Credentials) (connector.Connector, error) {
	token := creds.APIKey
	if token == "" {
		token = creds.AccessToken
	}
	domain := strings.TrimSpace(creds.Extra["domain"])
	if token == "" || domain == "" {
		return nil, errors.New("okta: api_key and extra.domain (e.g. acme.okta.com) are required")
	}
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
	if strings.ContainsAny(domain, "/?#@ ") {
		return nil, errors.New("okta: domain must be a host name such as acme.okta.com")
	}
	return &Connector{base: "https://" + domain, token: token, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

// NewWithBase is used by tests to point at a fake Okta.
func NewWithBase(base, token string, client *http.Client) *Connector {
	return &Connector{base: strings.TrimRight(base, "/"), token: token, client: client}
}

// Name returns the provider key.
func (c *Connector) Name() string { return "okta" }

type oktaUser struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Profile struct {
		Login string `json:"login"`
		Email string `json:"email"`
	} `json:"profile"`
}

type apiError struct {
	Code    string `json:"errorCode"`
	Summary string `json:"errorSummary"`
}

func (c *Connector) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "SSWS "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("okta: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		var e apiError
		_ = json.Unmarshal(raw, &e)
		if e.Summary == "" {
			e.Summary = resp.Status
		}
		return resp.StatusCode, fmt.Errorf("okta %s %s: %s", method, path, e.Summary)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("okta: decode: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// Health checks the token by listing one user.
func (c *Connector) Health(ctx context.Context) error {
	var users []oktaUser
	_, err := c.do(ctx, http.MethodGet, "/api/v1/users?limit=1", nil, &users)
	return err
}

// Provision creates and activates the user, or returns the existing one
// (re-activating a deactivated account for a rehire).
func (c *Connector) Provision(ctx context.Context, req connector.ProvisionRequest) (*connector.ProvisionResult, error) {
	if req.Email == "" {
		return nil, errors.New("okta: email is required")
	}
	var existing oktaUser
	code, err := c.do(ctx, http.MethodGet, "/api/v1/users/"+url.PathEscape(req.Email), nil, &existing)
	if err == nil {
		msg := "Okta account already exists"
		if existing.Status == "DEPROVISIONED" {
			if _, err := c.do(ctx, http.MethodPost, "/api/v1/users/"+existing.ID+"/lifecycle/activate?sendEmail=true", nil, nil); err != nil {
				return nil, err
			}
			msg = "Re-activated the existing Okta account"
		}
		return &connector.ProvisionResult{ExternalID: existing.ID, ExternalEmail: req.Email, ExternalUsername: existing.Profile.Login, Message: msg}, nil
	}
	if code != http.StatusNotFound {
		return nil, err
	}
	profile := map[string]string{"firstName": req.FirstName, "lastName": req.LastName, "email": req.Email, "login": req.Email}
	if req.DisplayName != "" {
		profile["displayName"] = req.DisplayName
	}
	if req.Department != "" {
		profile["department"] = req.Department
	}
	if req.Position != "" {
		profile["title"] = req.Position
	}
	var created oktaUser
	if _, err := c.do(ctx, http.MethodPost, "/api/v1/users?activate=true", map[string]any{"profile": profile}, &created); err != nil {
		return nil, err
	}
	return &connector.ProvisionResult{ExternalID: created.ID, ExternalEmail: req.Email, ExternalUsername: req.Email, Message: "Okta account created and activation e-mail sent"}, nil
}

// Deprovision deactivates the user (mode "delete" also deletes it).
func (c *Connector) Deprovision(ctx context.Context, req connector.DeprovisionRequest) (*connector.DeprovisionResult, error) {
	if req.ExternalID == "" {
		return nil, errors.New("okta: external id is required")
	}
	path := "/api/v1/users/" + url.PathEscape(req.ExternalID)
	if code, err := c.do(ctx, http.MethodPost, path+"/lifecycle/deactivate", nil, nil); err != nil && code != http.StatusNotFound {
		return nil, err
	}
	if req.DeprovisionMode == "delete" {
		if code, err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil && code != http.StatusNotFound {
			return nil, err
		}
		return &connector.DeprovisionResult{ExternalID: req.ExternalID, Status: "deleted", Message: "Okta account deactivated and deleted"}, nil
	}
	return &connector.DeprovisionResult{ExternalID: req.ExternalID, Status: "disabled", Message: "Okta account deactivated; sessions ended"}, nil
}

// SyncAccount reads the user's status.
func (c *Connector) SyncAccount(ctx context.Context, req connector.SyncRequest) (*connector.SyncResult, error) {
	var u oktaUser
	code, err := c.do(ctx, http.MethodGet, "/api/v1/users/"+url.PathEscape(req.ExternalID), nil, &u)
	if code == http.StatusNotFound {
		return &connector.SyncResult{ExternalID: req.ExternalID, AccountStatus: "deleted"}, nil
	}
	if err != nil {
		return nil, err
	}
	status := "active"
	switch u.Status {
	case "DEPROVISIONED", "SUSPENDED", "LOCKED_OUT":
		status = "disabled"
	}
	return &connector.SyncResult{ExternalID: u.ID, ExternalEmail: u.Profile.Email, ExternalUsername: u.Profile.Login, AccountStatus: status}, nil
}
