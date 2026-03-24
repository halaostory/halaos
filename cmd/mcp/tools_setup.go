package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// setupAccountTool returns the MCP tool definition for setup_account.
func setupAccountTool() mcp.Tool {
	return mcp.NewTool("setup_account",
		mcp.WithDescription("Register a new HalaOS account or login to an existing one. Use action=register for new accounts, action=login for existing accounts. Returns an API key for subsequent tool calls."),
		mcp.WithString("action",
			mcp.Description("Action to perform: 'register' for new accounts, 'login' for existing accounts"),
			mcp.Required(),
			mcp.Enum("register", "login"),
		),
		mcp.WithString("email",
			mcp.Description("Your email address"),
			mcp.Required(),
		),
		mcp.WithString("password",
			mcp.Description("Your password (min 8 chars, must contain uppercase, lowercase, and number)"),
			mcp.Required(),
		),
		mcp.WithString("company_name",
			mcp.Description("Your company name (required for register)"),
		),
		mcp.WithString("country",
			mcp.Description("Country code, e.g. PH, SG, LK (required for register)"),
		),
		mcp.WithString("referral_code",
			mcp.Description("Optional referral code"),
		),
	)
}

// callCLIEndpoint makes an unauthenticated POST to baseURL + "/api/v1/auth/" + endpoint.
// It returns the parsed JSON response map, or an error with the message extracted from the response.
func callCLIEndpoint(baseURL, endpoint string, body interface{}) (map[string]interface{}, error) {
	url := strings.TrimRight(baseURL, "/") + "/api/v1/auth/" + endpoint

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// No Authorization header — this is a public endpoint.

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response (HTTP %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode >= 400 {
		msg := extractErrorMessage(result, resp.StatusCode)
		return nil, fmt.Errorf("%s (HTTP %d)", msg, resp.StatusCode)
	}

	return result, nil
}

// extractErrorMessage pulls the human-readable message from a standard error envelope.
func extractErrorMessage(result map[string]interface{}, statusCode int) string {
	if errField, ok := result["error"]; ok {
		switch e := errField.(type) {
		case map[string]interface{}:
			if msg, ok := e["message"].(string); ok && msg != "" {
				return msg
			}
		case string:
			if e != "" {
				return e
			}
		}
	}
	return fmt.Sprintf("request failed with HTTP %d", statusCode)
}

// makeSetupAccount returns a ToolHandlerFunc for the setup_account tool.
func makeSetupAccount(baseURL string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		action, err := req.RequireString("action")
		if err != nil {
			return mcp.NewToolResultError("action is required (register or login)"), nil
		}

		email, err := req.RequireString("email")
		if err != nil {
			return mcp.NewToolResultError("email is required"), nil
		}

		password, err := req.RequireString("password")
		if err != nil {
			return mcp.NewToolResultError("password is required"), nil
		}

		var endpoint string
		requestBody := map[string]interface{}{
			"email":    email,
			"password": password,
		}

		switch action {
		case "register":
			endpoint = "cli-register"
			if companyName := req.GetString("company_name", ""); companyName != "" {
				requestBody["company_name"] = companyName
			}
			if country := req.GetString("country", ""); country != "" {
				requestBody["country"] = country
			}
			if referralCode := req.GetString("referral_code", ""); referralCode != "" {
				requestBody["referral_code"] = referralCode
			}
		case "login":
			endpoint = "cli-login"
		default:
			return mcp.NewToolResultError("action must be 'register' or 'login'"), nil
		}

		result, err := callCLIEndpoint(baseURL, endpoint, requestBody)
		if err != nil {
			errMsg := err.Error()
			var suggestion string
			if strings.Contains(errMsg, "email_exists") || strings.Contains(errMsg, "already registered") {
				suggestion = "\n\nSuggestion: This email is already registered. Try action=login instead."
			} else if strings.Contains(errMsg, "invalid_credentials") || strings.Contains(errMsg, "Invalid email or password") {
				suggestion = "\n\nSuggestion: Invalid credentials. If you don't have an account yet, try action=register instead."
			}
			return mcp.NewToolResultError(errMsg + suggestion), nil
		}

		// Extract the data payload — never echo the password back.
		data, _ := result["data"].(map[string]interface{})
		if data == nil {
			data = map[string]interface{}{}
		}

		apiKey, _ := data["api_key"].(string)
		apiKeyPrefix, _ := data["api_key_prefix"].(string)

		var message string
		if action == "register" {
			message = "Account created successfully."
		} else {
			message = "Login successful."
		}

		output := map[string]interface{}{
			"message":        message,
			"api_key":        apiKey,
			"api_key_prefix": apiKeyPrefix,
			"config_hint":    fmt.Sprintf("Set HALAOS_API_KEY=%s in your environment to use other HalaOS MCP tools.", apiKey),
		}

		pretty, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return mcp.NewToolResultError("failed to format response"), nil
		}
		return mcp.NewToolResultText(string(pretty)), nil
	}
}
