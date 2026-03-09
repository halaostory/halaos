package connector

import "context"

// Connector is the interface for external SaaS integrations.
type Connector interface {
	// Name returns the provider identifier (e.g., "slack", "google", "github").
	Name() string

	// Health checks if the connection is active and credentials are valid.
	Health(ctx context.Context) error

	// Provision creates an external account for an employee.
	Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error)

	// Deprovision disables or removes an external account.
	Deprovision(ctx context.Context, req DeprovisionRequest) (*DeprovisionResult, error)

	// SyncAccount synchronizes account status from the external service.
	SyncAccount(ctx context.Context, req SyncRequest) (*SyncResult, error)
}

// ProvisionRequest contains the data needed to create an external account.
type ProvisionRequest struct {
	EmployeeID   int64             `json:"employee_id"`
	Email        string            `json:"email"`
	FirstName    string            `json:"first_name"`
	LastName     string            `json:"last_name"`
	DisplayName  string            `json:"display_name"`
	Department   string            `json:"department"`
	Position     string            `json:"position"`
	Params       map[string]string `json:"params"`
}

// ProvisionResult is the outcome of a provisioning operation.
type ProvisionResult struct {
	ExternalID       string `json:"external_id"`
	ExternalEmail    string `json:"external_email"`
	ExternalUsername string `json:"external_username"`
	Message          string `json:"message"`
}

// DeprovisionRequest contains the data needed to disable/remove an external account.
type DeprovisionRequest struct {
	ExternalID      string            `json:"external_id"`
	DeprovisionMode string            `json:"deprovision_mode"` // disable, delete, transfer
	Params          map[string]string `json:"params"`
}

// DeprovisionResult is the outcome of a deprovisioning operation.
type DeprovisionResult struct {
	ExternalID string `json:"external_id"`
	Status     string `json:"status"` // disabled, deleted, transferred
	Message    string `json:"message"`
}

// SyncRequest contains the data needed to check an external account's status.
type SyncRequest struct {
	ExternalID string `json:"external_id"`
}

// SyncResult is the outcome of an account sync check.
type SyncResult struct {
	ExternalID       string `json:"external_id"`
	ExternalEmail    string `json:"external_email"`
	ExternalUsername string `json:"external_username"`
	AccountStatus    string `json:"account_status"` // active, disabled, deleted
}

// Credentials holds decrypted credentials for a connector.
type Credentials struct {
	AuthType     string            `json:"auth_type"`
	AccessToken  string            `json:"access_token,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	APIKey       string            `json:"api_key,omitempty"`
	BotToken     string            `json:"bot_token,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}
