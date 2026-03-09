package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tonypk/aigonhr/internal/integration/connector"
)

const apiBase = "https://api.github.com"

// Connector implements the GitHub organization integration.
type Connector struct {
	token string
	org   string
	client *http.Client
}

// New creates a GitHub connector from credentials.
func New(creds connector.Credentials) (connector.Connector, error) {
	token := creds.AccessToken
	if token == "" {
		token = creds.APIKey
	}
	if token == "" {
		return nil, fmt.Errorf("github: access_token or api_key required")
	}
	org := creds.Extra["org"]
	if org == "" {
		return nil, fmt.Errorf("github: org name required in extra.org")
	}
	return &Connector{
		token:  token,
		org:    org,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Connector) Name() string { return "github" }

func (c *Connector) Health(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/orgs/%s", apiBase, c.org), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github health check failed: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Connector) Provision(ctx context.Context, preq connector.ProvisionRequest) (*connector.ProvisionResult, error) {
	username := preq.Params["github_username"]
	if username == "" {
		return nil, fmt.Errorf("github: github_username param required")
	}

	// Invite user to org
	body := map[string]any{
		"role": "direct_member",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/orgs/%s/memberships/%s", apiBase, c.org, username),
		bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github invite: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("github invite failed: %s", string(respBody))
	}

	// Add to team if specified
	if team := preq.Params["team_slug"]; team != "" {
		c.addToTeam(ctx, team, username)
	}

	return &connector.ProvisionResult{
		ExternalID:       username,
		ExternalUsername: username,
		Message:          fmt.Sprintf("GitHub org membership created for %s", username),
	}, nil
}

func (c *Connector) Deprovision(ctx context.Context, req connector.DeprovisionRequest) (*connector.DeprovisionResult, error) {
	username := req.ExternalID
	if username == "" {
		return nil, fmt.Errorf("github: external_id (username) required")
	}

	switch req.DeprovisionMode {
	case "disable", "delete":
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
			fmt.Sprintf("%s/orgs/%s/members/%s", apiBase, c.org, username), nil)
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
		httpReq.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("github remove: %w", err)
		}
		defer resp.Body.Close()

		return &connector.DeprovisionResult{
			ExternalID: username,
			Status:     "deleted",
			Message:    "Removed from GitHub organization",
		}, nil

	default:
		return &connector.DeprovisionResult{
			ExternalID: username,
			Status:     "none",
			Message:    "No action taken",
		}, nil
	}
}

func (c *Connector) SyncAccount(ctx context.Context, req connector.SyncRequest) (*connector.SyncResult, error) {
	username := req.ExternalID
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/orgs/%s/members/%s", apiBase, c.org, username), nil)
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("github check membership: %w", err)
	}
	defer resp.Body.Close()

	status := "active"
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusFound {
		status = "deleted"
	}

	return &connector.SyncResult{
		ExternalID:       username,
		ExternalUsername: username,
		AccountStatus:    status,
	}, nil
}

func (c *Connector) addToTeam(ctx context.Context, teamSlug, username string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/orgs/%s/teams/%s/memberships/%s", apiBase, c.org, teamSlug, username), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
