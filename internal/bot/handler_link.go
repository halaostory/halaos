package bot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// LinkHandler handles HTTP requests for bot link management.
type LinkHandler struct {
	linker     *Linker
	queries    *store.Queries
	logger     *slog.Logger
	httpClient *http.Client // for testing; nil uses http.DefaultClient
	botManager *BotManager  // nil if AI not available
	cfg        *config.BotConfig
}

// NewLinkHandler creates a bot link HTTP handler.
func NewLinkHandler(linker *Linker, queries *store.Queries, logger *slog.Logger, botManager *BotManager, cfg *config.BotConfig) *LinkHandler {
	return &LinkHandler{linker: linker, queries: queries, logger: logger, botManager: botManager, cfg: cfg}
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

	// Hot reload: restart the bot for this company
	if h.botManager != nil && req.Platform == "telegram" {
		go h.botManager.Reload(companyID)
	}

	response.OK(c, cfg)
}

// TestBotToken validates a Telegram bot token by calling the Telegram getMe API.
func (h *LinkHandler) TestBotToken(c *gin.Context) {
	var req struct {
		BotToken string `json:"bot_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "bot_token is required")
		return
	}

	token := strings.TrimSpace(req.BotToken)
	if token == "" {
		response.BadRequest(c, "bot_token is required")
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	client := h.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(apiURL) //nolint:gosec // token is user-provided, URL is fixed Telegram API
	if err != nil {
		response.BadRequest(c, "Failed to reach Telegram API")
		return
	}
	defer resp.Body.Close()

	var tgResp struct {
		OK     bool `json:"ok"`
		Result struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		response.BadRequest(c, "Invalid response from Telegram")
		return
	}

	if !tgResp.OK {
		msg := tgResp.Description
		if msg == "" {
			msg = "Invalid bot token"
		}
		response.BadRequest(c, msg)
		return
	}

	response.OK(c, gin.H{
		"ok":           true,
		"bot_username": tgResp.Result.Username,
		"bot_name":     tgResp.Result.FirstName,
	})
}

// GetBotInfo returns the bot username and active status for the current company.
// Falls back to the shared bot username if no company-specific config exists.
// Does not expose the bot token.
func (h *LinkHandler) GetBotInfo(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	cfg, err := h.queries.GetBotConfig(c.Request.Context(), store.GetBotConfigParams{
		CompanyID: companyID,
		Platform:  "telegram",
	})
	if err != nil {
		// No company config — fall back to shared bot
		sharedUsername := ""
		sharedActive := false
		if h.cfg != nil && h.cfg.TelegramBotUsername != "" {
			sharedUsername = "@" + h.cfg.TelegramBotUsername
			sharedActive = h.cfg.Enabled && h.cfg.TelegramBotToken != ""
		}
		response.OK(c, gin.H{
			"bot_username": sharedUsername,
			"is_active":    sharedActive,
			"is_shared":    true,
		})
		return
	}

	username := cfg.BotUsername
	if username != "" && !strings.HasPrefix(username, "@") {
		username = "@" + username
	}

	response.OK(c, gin.H{
		"bot_username": username,
		"is_active":    cfg.IsActive,
		"is_shared":    false,
	})
}

// RegisterRoutes registers bot HTTP routes.
func (h *LinkHandler) RegisterRoutes(protected *gin.RouterGroup) {
	// User-facing endpoints
	protected.GET("/bot/link-code", h.GetLinkCode)
	protected.GET("/bot/link-status", h.GetLinkStatus)
	protected.DELETE("/bot/link/:platform", h.UnlinkBot)
	protected.GET("/bot/info", h.GetBotInfo)

	// Admin endpoints
	protected.GET("/admin/bot/configs", h.ListBotConfigs)
	protected.POST("/admin/bot/configs", h.UpsertBotConfig)
	protected.POST("/admin/bot/test-token", h.TestBotToken)
}
