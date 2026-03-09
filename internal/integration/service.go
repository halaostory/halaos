package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/integration/connector"
	"github.com/tonypk/aigonhr/internal/integration/crypto"
	"github.com/tonypk/aigonhr/internal/store"
)

// Service handles CRUD for integration connections and templates.
type Service struct {
	queries   *store.Queries
	registry  *connector.Registry
	encryptor *crypto.CredentialEncryptor
	logger    *slog.Logger
}

// NewService creates an integration service.
func NewService(
	queries *store.Queries,
	registry *connector.Registry,
	encryptor *crypto.CredentialEncryptor,
	logger *slog.Logger,
) *Service {
	return &Service{
		queries:   queries,
		registry:  registry,
		encryptor: encryptor,
		logger:    logger,
	}
}

// CreateConnection creates a new SaaS connection for a company.
func (s *Service) CreateConnection(ctx context.Context, companyID int64, userID int64, req CreateConnectionRequest) (*store.IntegrationConnection, error) {
	if !s.registry.Has(req.Provider) {
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}

	var encryptedCreds []byte
	if req.Credentials != nil {
		credsJSON, err := json.Marshal(req.Credentials)
		if err != nil {
			return nil, fmt.Errorf("marshal credentials: %w", err)
		}
		encrypted, err := s.encryptor.Encrypt(credsJSON)
		if err != nil {
			return nil, fmt.Errorf("encrypt credentials: %w", err)
		}
		encryptedCreds = encrypted
	}

	configJSON, _ := json.Marshal(req.Config)
	if configJSON == nil {
		configJSON = []byte("{}")
	}

	var tokenExpiry pgtype.Timestamptz
	if req.OAuthTokenExpiry != nil {
		tokenExpiry = pgtype.Timestamptz{Time: *req.OAuthTokenExpiry, Valid: true}
	}

	conn, err := s.queries.CreateIntegrationConnection(ctx, store.CreateIntegrationConnectionParams{
		CompanyID:        companyID,
		Provider:         req.Provider,
		DisplayName:      req.DisplayName,
		Status:           "active",
		AuthType:         req.AuthType,
		EncryptedCreds:   encryptedCreds,
		OauthTokenExpiry: tokenExpiry,
		OauthScope:       req.OAuthScope,
		Config:           configJSON,
		CreatedBy:        &userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}

	s.logger.Info("integration connection created",
		"id", conn.ID,
		"provider", req.Provider,
		"company_id", companyID,
	)

	return &conn, nil
}

// TestConnection tests a connection's health.
func (s *Service) TestConnection(ctx context.Context, companyID int64, connectionID uuid.UUID) error {
	conn, err := s.queries.GetIntegrationConnection(ctx, store.GetIntegrationConnectionParams{
		ID:        connectionID,
		CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("connection not found: %w", err)
	}

	creds, err := s.decryptCredentials(conn.EncryptedCreds)
	if err != nil {
		return fmt.Errorf("decrypt credentials: %w", err)
	}

	c, err := s.registry.Create(conn.Provider, creds)
	if err != nil {
		return fmt.Errorf("create connector: %w", err)
	}

	if err := c.Health(ctx); err != nil {
		errMsg := err.Error()
		s.queries.UpdateConnectionError(ctx, store.UpdateConnectionErrorParams{
			ID:        connectionID,
			LastError: &errMsg,
		})
		return fmt.Errorf("health check failed: %w", err)
	}

	s.queries.UpdateConnectionLastUsed(ctx, connectionID)
	return nil
}

// GetConnector returns a live connector instance for a connection.
func (s *Service) GetConnector(ctx context.Context, companyID int64, connectionID uuid.UUID) (connector.Connector, error) {
	conn, err := s.queries.GetConnectionCredentials(ctx, store.GetConnectionCredentialsParams{
		ID:        connectionID,
		CompanyID: companyID,
	})
	if err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}

	creds, err := s.decryptCredentials(conn.EncryptedCreds)
	if err != nil {
		return nil, err
	}

	return s.registry.Create(conn.Provider, creds)
}

// GetConnectorByRow returns a live connector for an already-fetched connection row.
func (s *Service) GetConnectorByRow(encryptedCreds []byte, provider string) (connector.Connector, error) {
	creds, err := s.decryptCredentials(encryptedCreds)
	if err != nil {
		return nil, err
	}
	return s.registry.Create(provider, creds)
}

func (s *Service) decryptCredentials(encrypted []byte) (connector.Credentials, error) {
	if encrypted == nil {
		return connector.Credentials{}, nil
	}
	plaintext, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return connector.Credentials{}, fmt.Errorf("decrypt: %w", err)
	}
	var creds connector.Credentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return connector.Credentials{}, fmt.Errorf("unmarshal credentials: %w", err)
	}
	return creds, nil
}

// CreateConnectionRequest is the input for creating a connection.
type CreateConnectionRequest struct {
	Provider         string                  `json:"provider" binding:"required"`
	DisplayName      string                  `json:"display_name"`
	AuthType         string                  `json:"auth_type" binding:"required"`
	Credentials      *connector.Credentials  `json:"credentials"`
	OAuthScope       string                  `json:"oauth_scope"`
	OAuthTokenExpiry *time.Time              `json:"oauth_token_expiry"`
	Config           map[string]any          `json:"config"`
}

// CreateTemplateRequest is the input for creating a provisioning template.
type CreateTemplateRequest struct {
	ConnectionID         uuid.UUID `json:"connection_id" binding:"required"`
	Provider             string    `json:"provider" binding:"required"`
	EventTrigger         string    `json:"event_trigger" binding:"required"`
	ActionType           string    `json:"action_type" binding:"required"`
	FilterDepartmentID   *int64    `json:"filter_department_id"`
	FilterEmploymentType *string   `json:"filter_employment_type"`
	Params               map[string]any `json:"params"`
	DeprovisionMode      string    `json:"deprovision_mode"`
	RequiresApproval     bool      `json:"requires_approval"`
	IsActive             bool      `json:"is_active"`
}

// UpdateTemplateRequest is the input for updating a provisioning template.
type UpdateTemplateRequest struct {
	EventTrigger         string         `json:"event_trigger"`
	ActionType           string         `json:"action_type"`
	FilterDepartmentID   *int64         `json:"filter_department_id"`
	FilterEmploymentType *string        `json:"filter_employment_type"`
	Params               map[string]any `json:"params"`
	DeprovisionMode      string         `json:"deprovision_mode"`
	RequiresApproval     *bool          `json:"requires_approval"`
	IsActive             *bool          `json:"is_active"`
}
