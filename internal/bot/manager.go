package bot

import (
	"context"
	"log/slog"
	"sync"

	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/store"
)

// BotFactory creates and runs a bot for the given token. It blocks until ctx is cancelled.
type BotFactory func(ctx context.Context, token string)

// BotManager manages the lifecycle of telegram bot instances with hot reload.
// companyID=0 is reserved for the shared fallback bot.
type BotManager struct {
	mu      sync.Mutex
	bots    map[int64]runningBot
	factory BotFactory
	queries *store.Queries
	cfg     *config.BotConfig
	logger  *slog.Logger
}

type runningBot struct {
	cancel context.CancelFunc
	token  string
}

// NewBotManager creates a BotManager. The factory function is called in a goroutine
// for each bot that needs to be started.
func NewBotManager(factory BotFactory, queries *store.Queries, cfg *config.BotConfig, logger *slog.Logger) *BotManager {
	return &BotManager{
		bots:    make(map[int64]runningBot),
		factory: factory,
		queries: queries,
		cfg:     cfg,
		logger:  logger,
	}
}

// StartBot launches a telegram bot for the given companyID.
// If a bot is already running for this company, it is stopped first.
func (m *BotManager) StartBot(companyID int64, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.bots[companyID]; ok {
		existing.cancel()
		delete(m.bots, companyID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go m.factory(ctx, token)

	m.bots[companyID] = runningBot{cancel: cancel, token: token}
	m.logger.Info("bot started", "company_id", companyID)
}

// StopBot stops the bot for the given companyID.
func (m *BotManager) StopBot(companyID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.bots[companyID]; ok {
		existing.cancel()
		delete(m.bots, companyID)
		m.logger.Info("bot stopped", "company_id", companyID)
	}
}

// Reload stops any existing bot for companyID, queries the DB for current config,
// and starts a new bot if the config is active with a valid token.
func (m *BotManager) Reload(companyID int64) {
	cfg, err := m.queries.GetBotConfig(context.Background(), store.GetBotConfigParams{
		CompanyID: companyID,
		Platform:  "telegram",
	})
	if err != nil {
		m.logger.Warn("reload: failed to get bot config", "company_id", companyID, "error", err)
		m.StopBot(companyID)
		return
	}

	if !cfg.IsActive || cfg.BotToken == "" {
		m.StopBot(companyID)
		return
	}

	m.StartBot(companyID, cfg.BotToken)
}

// StartAll starts the shared fallback bot (companyID=0) and all active DB configs.
func (m *BotManager) StartAll() {
	// Shared fallback bot from env var
	if m.cfg.Enabled && m.cfg.TelegramBotToken != "" {
		m.StartBot(0, m.cfg.TelegramBotToken)
		m.logger.Info("shared bot started", "username", m.cfg.TelegramBotUsername)
	}

	// Per-company bots from database
	activeConfigs, err := m.queries.ListActiveBotConfigs(context.Background(), "telegram")
	if err != nil {
		m.logger.Warn("failed to load bot configs from database", "error", err)
		return
	}
	for _, cfg := range activeConfigs {
		if cfg.BotToken == "" {
			continue
		}
		m.StartBot(cfg.CompanyID, cfg.BotToken)
		m.logger.Info("company bot started from db", "company_id", cfg.CompanyID, "username", cfg.BotUsername)
	}
}

// StopAll stops all running bots.
func (m *BotManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, b := range m.bots {
		b.cancel()
		delete(m.bots, id)
	}
	m.logger.Info("all bots stopped")
}

// IsRunning returns whether a bot is running for the given companyID.
func (m *BotManager) IsRunning(companyID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.bots[companyID]
	return ok
}
