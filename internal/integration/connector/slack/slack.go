package slack

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

const apiBase = "https://slack.com/api"

// Connector implements the Slack integration.
type Connector struct {
	token  string
	client *http.Client
}

// New creates a Slack connector from credentials.
func New(creds connector.Credentials) (connector.Connector, error) {
	token := creds.AccessToken
	if token == "" {
		token = creds.BotToken
	}
	if token == "" {
		return nil, fmt.Errorf("slack: access_token or bot_token required")
	}
	return &Connector{
		token: token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Connector) Name() string { return "slack" }

func (c *Connector) Health(ctx context.Context) error {
	resp, err := c.apiCall(ctx, "auth.test", nil)
	if err != nil {
		return err
	}
	if !resp["ok"].(bool) {
		return fmt.Errorf("slack auth.test failed: %v", resp["error"])
	}
	return nil
}

func (c *Connector) Provision(ctx context.Context, req connector.ProvisionRequest) (*connector.ProvisionResult, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("slack: email is required for provisioning")
	}

	// Invite user via admin.users.invite (requires admin scope) or conversations.invite
	// For MVP, use users.lookupByEmail first, if not found use admin invite
	resp, err := c.apiCall(ctx, "users.lookupByEmail", map[string]string{
		"email": req.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("slack lookup: %w", err)
	}

	if resp["ok"].(bool) {
		user := resp["user"].(map[string]any)
		return &connector.ProvisionResult{
			ExternalID:       user["id"].(string),
			ExternalEmail:    req.Email,
			ExternalUsername: user["name"].(string),
			Message:          "User already exists in Slack workspace",
		}, nil
	}

	// User not found — attempt invite
	inviteResp, err := c.apiCall(ctx, "admin.users.invite", map[string]string{
		"email":      req.Email,
		"channel_ids": "",
		"real_name":   fmt.Sprintf("%s %s", req.FirstName, req.LastName),
	})
	if err != nil {
		return nil, fmt.Errorf("slack invite: %w", err)
	}

	if ok, _ := inviteResp["ok"].(bool); !ok {
		errMsg, _ := inviteResp["error"].(string)
		return nil, fmt.Errorf("slack invite failed: %s", errMsg)
	}

	return &connector.ProvisionResult{
		ExternalEmail: req.Email,
		Message:       "Slack invitation sent",
	}, nil
}

func (c *Connector) Deprovision(ctx context.Context, req connector.DeprovisionRequest) (*connector.DeprovisionResult, error) {
	if req.ExternalID == "" {
		return nil, fmt.Errorf("slack: external_id required for deprovisioning")
	}

	switch req.DeprovisionMode {
	case "disable":
		resp, err := c.apiCall(ctx, "admin.users.setInactive", map[string]string{
			"user_id": req.ExternalID,
		})
		if err != nil {
			return nil, fmt.Errorf("slack deactivate: %w", err)
		}
		if ok, _ := resp["ok"].(bool); !ok {
			errMsg, _ := resp["error"].(string)
			return nil, fmt.Errorf("slack deactivate failed: %s", errMsg)
		}
		return &connector.DeprovisionResult{
			ExternalID: req.ExternalID,
			Status:     "disabled",
			Message:    "Slack account deactivated",
		}, nil

	case "delete":
		// Slack doesn't support account deletion via API, fall back to disable
		return c.Deprovision(ctx, connector.DeprovisionRequest{
			ExternalID:      req.ExternalID,
			DeprovisionMode: "disable",
		})

	default:
		return &connector.DeprovisionResult{
			ExternalID: req.ExternalID,
			Status:     "none",
			Message:    "No deprovisioning action taken",
		}, nil
	}
}

func (c *Connector) SyncAccount(ctx context.Context, req connector.SyncRequest) (*connector.SyncResult, error) {
	resp, err := c.apiCall(ctx, "users.info", map[string]string{
		"user": req.ExternalID,
	})
	if err != nil {
		return nil, fmt.Errorf("slack user info: %w", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return &connector.SyncResult{
			ExternalID:    req.ExternalID,
			AccountStatus: "deleted",
		}, nil
	}

	user := resp["user"].(map[string]any)
	profile, _ := user["profile"].(map[string]any)
	status := "active"
	if deleted, _ := user["deleted"].(bool); deleted {
		status = "disabled"
	}

	email, _ := profile["email"].(string)
	displayName, _ := profile["display_name"].(string)

	return &connector.SyncResult{
		ExternalID:       req.ExternalID,
		ExternalEmail:    email,
		ExternalUsername: displayName,
		AccountStatus:    status,
	}, nil
}

func (c *Connector) apiCall(ctx context.Context, method string, params map[string]string) (map[string]any, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/"+method, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("slack: invalid response: %w", err)
	}

	return result, nil
}
