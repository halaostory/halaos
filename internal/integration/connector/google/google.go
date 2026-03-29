package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/halaostory/halaos/internal/integration/connector"
)

const adminAPIBase = "https://admin.googleapis.com/admin/directory/v1"

// Connector implements the Google Workspace integration.
type Connector struct {
	token  string
	domain string
	client *http.Client
}

// New creates a Google Workspace connector from credentials.
func New(creds connector.Credentials) (connector.Connector, error) {
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("google: access_token required")
	}
	domain := creds.Extra["domain"]
	return &Connector{
		token:  creds.AccessToken,
		domain: domain,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Connector) Name() string { return "google" }

func (c *Connector) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adminAPIBase+"/users?maxResults=1&domain="+c.domain, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google health check failed: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Connector) Provision(ctx context.Context, preq connector.ProvisionRequest) (*connector.ProvisionResult, error) {
	if preq.Email == "" {
		return nil, fmt.Errorf("google: email is required")
	}

	body := map[string]any{
		"primaryEmail": preq.Email,
		"name": map[string]string{
			"givenName":  preq.FirstName,
			"familyName": preq.LastName,
		},
		"password":            generateTempPassword(),
		"changePasswordAtNextLogin": true,
		"orgUnitPath": "/",
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, adminAPIBase+"/users", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google create user: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict {
		return &connector.ProvisionResult{
			ExternalEmail: preq.Email,
			Message:       "User already exists in Google Workspace",
		}, nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("google create user failed: %s", string(respBody))
	}

	var result map[string]any
	json.Unmarshal(respBody, &result)

	externalID, _ := result["id"].(string)

	return &connector.ProvisionResult{
		ExternalID:    externalID,
		ExternalEmail: preq.Email,
		Message:       "Google Workspace account created",
	}, nil
}

func (c *Connector) Deprovision(ctx context.Context, req connector.DeprovisionRequest) (*connector.DeprovisionResult, error) {
	if req.ExternalID == "" {
		return nil, fmt.Errorf("google: external_id required")
	}

	switch req.DeprovisionMode {
	case "disable":
		body := map[string]any{"suspended": true}
		jsonBody, _ := json.Marshal(body)
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPut,
			adminAPIBase+"/users/"+req.ExternalID, bytes.NewReader(jsonBody))
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("google suspend: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("google suspend failed: %s", string(respBody))
		}

		return &connector.DeprovisionResult{
			ExternalID: req.ExternalID,
			Status:     "disabled",
			Message:    "Google Workspace account suspended",
		}, nil

	case "delete":
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
			adminAPIBase+"/users/"+req.ExternalID, nil)
		httpReq.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("google delete: %w", err)
		}
		defer resp.Body.Close()

		return &connector.DeprovisionResult{
			ExternalID: req.ExternalID,
			Status:     "deleted",
			Message:    "Google Workspace account deleted",
		}, nil

	default:
		return &connector.DeprovisionResult{
			ExternalID: req.ExternalID,
			Status:     "none",
			Message:    "No action taken",
		}, nil
	}
}

func (c *Connector) SyncAccount(ctx context.Context, req connector.SyncRequest) (*connector.SyncResult, error) {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		adminAPIBase+"/users/"+req.ExternalID, nil)
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("google get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &connector.SyncResult{
			ExternalID:    req.ExternalID,
			AccountStatus: "deleted",
		}, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	var user map[string]any
	json.Unmarshal(respBody, &user)

	status := "active"
	if suspended, _ := user["suspended"].(bool); suspended {
		status = "disabled"
	}

	email, _ := user["primaryEmail"].(string)
	name, _ := user["name"].(map[string]any)
	fullName := ""
	if name != nil {
		fn, _ := name["fullName"].(string)
		fullName = fn
	}

	return &connector.SyncResult{
		ExternalID:       req.ExternalID,
		ExternalEmail:    email,
		ExternalUsername: fullName,
		AccountStatus:    status,
	}, nil
}

func generateTempPassword() string {
	return "ChangeMe123!" // In production, use crypto/rand
}
