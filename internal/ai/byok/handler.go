package byok

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/integration/crypto"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Handler handles BYOK key management HTTP requests.
type Handler struct {
	queries   *store.Queries
	encryptor *crypto.CredentialEncryptor
}

// NewHandler creates a BYOK handler.
func NewHandler(queries *store.Queries, encryptor *crypto.CredentialEncryptor) *Handler {
	return &Handler{queries: queries, encryptor: encryptor}
}

// RegisterRoutes registers BYOK routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/byok")
	g.GET("/keys", h.ListKeys)
	g.POST("/keys", h.CreateKey)
	g.PUT("/keys/:id", h.UpdateKey)
	g.DELETE("/keys/:id", h.DeleteKey)
	g.POST("/keys/validate", h.ValidateKey)
}

type createKeyRequest struct {
	Provider      string `json:"provider" binding:"required,oneof=anthropic openai gemini"`
	APIKey        string `json:"api_key" binding:"required,min=10"`
	ModelOverride string `json:"model_override"`
	Label         string `json:"label"`
	UserID        *int64 `json:"user_id"` // nil = company-wide
}

// CreateKey stores a new BYOK API key.
func (h *Handler) CreateKey(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	actorID := auth.GetUserID(c)

	encrypted, err := h.encryptor.Encrypt([]byte(req.APIKey))
	if err != nil {
		response.InternalError(c, "Failed to encrypt key")
		return
	}

	hint := MakeKeyHint(req.APIKey)

	key, err := h.queries.CreateByokKey(c.Request.Context(), store.CreateByokKeyParams{
		CompanyID:     companyID,
		UserID:        req.UserID,
		Provider:      req.Provider,
		EncryptedKey:  encrypted,
		KeyHint:       hint,
		ModelOverride: req.ModelOverride,
		Label:         req.Label,
		IsActive:      true,
		CreatedBy:     &actorID,
	})
	if err != nil {
		response.InternalError(c, "Failed to save key")
		return
	}

	response.Created(c, keyToResponse(key))
}

// ListKeys returns all active BYOK keys for the company (without secrets).
func (h *Handler) ListKeys(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	keys, err := h.queries.ListByokKeys(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list keys")
		return
	}

	result := make([]keyResponse, len(keys))
	for i, k := range keys {
		result[i] = keyResponse{
			ID:            k.ID,
			CompanyID:     k.CompanyID,
			UserID:        k.UserID,
			Provider:      k.Provider,
			KeyHint:       k.KeyHint,
			ModelOverride: k.ModelOverride,
			Label:         k.Label,
			IsActive:      k.IsActive,
			CreatedBy:     k.CreatedBy,
			CreatedAt:     k.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:     k.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	response.OK(c, result)
}

type updateKeyRequest struct {
	APIKey        *string `json:"api_key"`        // optional: only update if provided
	ModelOverride string  `json:"model_override"`
	Label         string  `json:"label"`
	IsActive      bool    `json:"is_active"`
}

// UpdateKey updates an existing BYOK key.
func (h *Handler) UpdateKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	var req updateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	var encryptedKey []byte
	var hint string

	if req.APIKey != nil && *req.APIKey != "" {
		encryptedKey, err = h.encryptor.Encrypt([]byte(*req.APIKey))
		if err != nil {
			response.InternalError(c, "Failed to encrypt key")
			return
		}
		hint = MakeKeyHint(*req.APIKey)
	}

	key, err := h.queries.UpdateByokKey(c.Request.Context(), store.UpdateByokKeyParams{
		ID:            id,
		CompanyID:     companyID,
		EncryptedKey:  encryptedKey,
		KeyHint:       hint,
		ModelOverride: req.ModelOverride,
		Label:         req.Label,
		IsActive:      req.IsActive,
	})
	if err != nil {
		response.InternalError(c, "Failed to update key")
		return
	}

	response.OK(c, keyToResponse(key))
}

// DeleteKey removes a BYOK key.
func (h *Handler) DeleteKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	companyID := auth.GetCompanyID(c)

	if err := h.queries.DeleteByokKey(c.Request.Context(), store.DeleteByokKeyParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete key")
		return
	}

	response.OK(c, gin.H{"deleted": true})
}

type validateKeyRequest struct {
	Provider string `json:"provider" binding:"required,oneof=anthropic openai gemini"`
	APIKey   string `json:"api_key" binding:"required,min=10"`
}

// ValidateKey tests if an API key is valid by making a health check call.
func (h *Handler) ValidateKey(c *gin.Context) {
	var req validateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var p provider.Provider
	switch req.Provider {
	case "anthropic":
		p = provider.NewAnthropic(req.APIKey, "")
	case "openai":
		p = provider.NewOpenAI(req.APIKey, "")
	default:
		response.BadRequest(c, "Unsupported provider for validation")
		return
	}

	if err := p.Health(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"valid": false, "error": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"valid": true}})
}

type keyResponse struct {
	ID            uuid.UUID `json:"id"`
	CompanyID     int64     `json:"company_id"`
	UserID        *int64    `json:"user_id"`
	Provider      string    `json:"provider"`
	KeyHint       string    `json:"key_hint"`
	ModelOverride string    `json:"model_override"`
	Label         string    `json:"label"`
	IsActive      bool      `json:"is_active"`
	CreatedBy     *int64    `json:"created_by"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
}

func keyToResponse(k store.ByokKey) keyResponse {
	return keyResponse{
		ID:            k.ID,
		CompanyID:     k.CompanyID,
		UserID:        k.UserID,
		Provider:      k.Provider,
		KeyHint:       k.KeyHint,
		ModelOverride: k.ModelOverride,
		Label:         k.Label,
		IsActive:      k.IsActive,
		CreatedBy:     k.CreatedBy,
		CreatedAt:     k.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     k.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
