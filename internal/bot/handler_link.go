package bot

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// LinkHandler handles HTTP requests for bot link management.
type LinkHandler struct {
	linker  *Linker
	queries *store.Queries
	logger  *slog.Logger
}

// NewLinkHandler creates a bot link HTTP handler.
func NewLinkHandler(linker *Linker, queries *store.Queries, logger *slog.Logger) *LinkHandler {
	return &LinkHandler{linker: linker, queries: queries, logger: logger}
}

// GetLinkCode generates a link code for the current user.
func (h *LinkHandler) GetLinkCode(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	code, err := h.linker.GenerateLinkCode(c.Request.Context(), userID, companyID)
	if err != nil {
		response.InternalError(c, "Failed to generate link code")
		return
	}

	response.OK(c, gin.H{
		"code":     code,
		"platform": "telegram",
		"ttl":      "15 minutes",
	})
}

// GetLinkStatus checks if the current user has an active bot link.
func (h *LinkHandler) GetLinkStatus(c *gin.Context) {
	userID := auth.GetUserID(c)

	link, err := h.queries.GetBotUserLinkByUserID(c.Request.Context(), store.GetBotUserLinkByUserIDParams{
		UserID:   userID,
		Platform: "telegram",
	})
	if err != nil {
		response.OK(c, gin.H{"linked": false})
		return
	}

	response.OK(c, gin.H{
		"linked":      link.VerifiedAt.Valid,
		"platform":    link.Platform,
		"verified_at": link.VerifiedAt,
	})
}

// UnlinkBot removes the bot link for the current user.
func (h *LinkHandler) UnlinkBot(c *gin.Context) {
	userID := auth.GetUserID(c)
	platform := c.Param("platform")
	if platform == "" {
		platform = "telegram"
	}

	if err := h.queries.UnlinkBotUser(c.Request.Context(), store.UnlinkBotUserParams{
		UserID:   userID,
		Platform: platform,
	}); err != nil {
		response.InternalError(c, "Failed to unlink")
		return
	}

	response.OK(c, gin.H{"unlinked": true})
}

// ListBotConfigs returns bot configurations (admin only).
func (h *LinkHandler) ListBotConfigs(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	configs, err := h.queries.ListBotConfigs(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list bot configs")
		return
	}

	// Mask bot tokens
	type safeConfig struct {
		store.BotConfig
		BotToken string `json:"bot_token"`
	}
	safe := make([]safeConfig, len(configs))
	for i, cfg := range configs {
		safe[i] = safeConfig{BotConfig: cfg}
		if cfg.BotToken != "" {
			safe[i].BotToken = cfg.BotToken[:8] + "..."
		}
	}

	response.OK(c, safe)
}

// UpsertBotConfig creates or updates a bot configuration.
func (h *LinkHandler) UpsertBotConfig(c *gin.Context) {
	var req struct {
		Platform    string `json:"platform" binding:"required"`
		BotToken    string `json:"bot_token"`
		BotUsername string `json:"bot_username"`
		IsActive    bool   `json:"is_active"`
		WebhookURL  string `json:"webhook_url"`
		Mode        string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	mode := req.Mode
	if mode == "" {
		mode = "polling"
	}

	cfg, err := h.queries.UpsertBotConfig(c.Request.Context(), store.UpsertBotConfigParams{
		CompanyID:   companyID,
		Platform:    req.Platform,
		BotToken:    req.BotToken,
		BotUsername: req.BotUsername,
		IsActive:    req.IsActive,
		WebhookUrl:  req.WebhookURL,
		Mode:        mode,
	})
	if err != nil {
		response.InternalError(c, "Failed to save bot config")
		return
	}

	response.OK(c, cfg)
}

// RegisterRoutes registers bot HTTP routes.
func (h *LinkHandler) RegisterRoutes(protected *gin.RouterGroup) {
	// User-facing endpoints
	protected.GET("/bot/link-code", h.GetLinkCode)
	protected.GET("/bot/link-status", h.GetLinkStatus)
	protected.DELETE("/bot/link/:platform", h.UnlinkBot)

	// Admin endpoints
	protected.GET("/admin/bot/configs", h.ListBotConfigs)
	protected.POST("/admin/bot/configs", h.UpsertBotConfig)
}
