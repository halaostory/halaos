package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

const apiKeyPrefixStr = "halaos_"

// generateAPIKey returns "halaos_" + 40 random hex chars (20 bytes).
func generateAPIKey() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return apiKeyPrefixStr + hex.EncodeToString(b)
}

// apiKeyPrefix returns the first 13 chars for display (e.g. "halaos_a1b2c3").
func apiKeyPrefix(key string) string {
	if len(key) <= 13 {
		return key
	}
	return key[:13]
}

// hashAPIKey returns the SHA-256 hex digest of the key.
func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// APIKeyMiddleware checks for "halaos_" prefixed Bearer tokens.
// If found and valid, sets user context. Otherwise falls through to next middleware (JWT).
func APIKeyMiddleware(queries *store.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" || !strings.HasPrefix(token, apiKeyPrefixStr) {
			c.Next()
			return
		}

		hash := hashAPIKey(token)
		ak, err := queries.GetAPIKeyByHash(c.Request.Context(), hash)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "invalid_api_key", "message": "Invalid or revoked API key"},
			})
			return
		}

		c.Set(ContextKeyUserID, ak.UserID)
		c.Set(ContextKeyEmail, ak.Email)
		c.Set(ContextKeyRole, Role(ak.Role))
		c.Set(ContextKeyCompanyID, ak.CompanyID)

		// Async update last_used_at (best-effort)
		go func() {
			_ = queries.TouchAPIKeyLastUsed(c.Request.Context(), ak.ID)
		}()

		c.Next()
	}
}

type createAPIKeyRequest struct {
	Name string `json:"name"`
}

// CreateAPIKey handles POST /api-keys
func (h *Handler) CreateAPIKey(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body — use default name
	}
	if req.Name == "" {
		req.Name = "default"
	}

	key := generateAPIKey()
	prefix := apiKeyPrefix(key)
	hash := hashAPIKey(key)

	userID := GetUserID(c)
	companyID := GetCompanyID(c)

	row, err := h.queries.CreateAPIKey(c.Request.Context(), store.CreateAPIKeyParams{
		UserID:    userID,
		CompanyID: companyID,
		Prefix:    prefix,
		KeyHash:   hash,
		Name:      req.Name,
	})
	if err != nil {
		h.logger.Error("failed to create API key", "error", err)
		response.InternalError(c, "Failed to create API key")
		return
	}

	response.Created(c, gin.H{
		"id":         row.ID,
		"prefix":     row.Prefix,
		"name":       row.Name,
		"key":        key, // Raw key — shown only once
		"created_at": row.CreatedAt,
	})
}

// ListAPIKeys handles GET /api-keys
func (h *Handler) ListAPIKeys(c *gin.Context) {
	userID := GetUserID(c)

	keys, err := h.queries.ListAPIKeysByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list API keys", "error", err)
		response.InternalError(c, "Failed to list API keys")
		return
	}

	result := make([]gin.H, len(keys))
	for i, k := range keys {
		result[i] = gin.H{
			"id":           k.ID,
			"prefix":       k.Prefix,
			"name":         k.Name,
			"created_at":   k.CreatedAt,
			"last_used_at": nil,
		}
		if k.LastUsedAt.Valid {
			result[i]["last_used_at"] = k.LastUsedAt.Time
		}
	}

	response.OK(c, result)
}

// RevokeAPIKey handles DELETE /api-keys/:id
func (h *Handler) RevokeAPIKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	userID := GetUserID(c)

	if err := h.queries.RevokeAPIKey(c.Request.Context(), store.RevokeAPIKeyParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		h.logger.Error("failed to revoke API key", "error", err)
		response.InternalError(c, "Failed to revoke API key")
		return
	}

	response.OK(c, gin.H{"revoked": true})
}
