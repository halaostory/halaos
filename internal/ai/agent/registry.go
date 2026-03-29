package agent

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

// AgentConfig is a resolved agent configuration.
type AgentConfig struct {
	Slug           string
	Name           string
	Description    string
	SystemPrompt   string
	Tools          []string
	CostMultiplier float64
	IsAutonomous   bool
	MaxRounds      int
	MaxTokens      int
	Icon           string
	Model          string // LLM model override (empty = provider default)
	CompanyID      int64  // 0 = system agent, >0 = company-specific
}

// Registry loads agent definitions from the database and caches them in memory.
type Registry struct {
	queries  *store.Queries
	logger   *slog.Logger
	mu       sync.RWMutex
	agents   map[string]AgentConfig
	lastLoad time.Time
	cacheTTL time.Duration
}

// NewRegistry creates a registry and performs an initial load.
func NewRegistry(queries *store.Queries, logger *slog.Logger) *Registry {
	r := &Registry{
		queries:  queries,
		logger:   logger,
		agents:   make(map[string]AgentConfig),
		cacheTTL: 5 * time.Minute,
	}
	if err := r.Refresh(context.Background()); err != nil {
		logger.Error("initial agent registry load failed", "error", err)
	}
	return r
}

// Get returns the agent config for the given slug.
// It refreshes the cache if the TTL has expired.
func (r *Registry) Get(ctx context.Context, slug string) (AgentConfig, bool) {
	if r.cacheExpired() {
		if err := r.Refresh(ctx); err != nil {
			r.logger.Warn("agent registry refresh failed, using stale cache", "error", err)
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, ok := r.agents[slug]
	return cfg, ok
}

// GetForCompany returns the agent config for the given slug, only if
// it's a system agent or belongs to the specified company.
func (r *Registry) GetForCompany(ctx context.Context, slug string, companyID int64) (AgentConfig, bool) {
	cfg, ok := r.Get(ctx, slug)
	if !ok {
		return cfg, false
	}
	if cfg.CompanyID != 0 && cfg.CompanyID != companyID {
		return AgentConfig{}, false
	}
	return cfg, true
}

// List returns all agent configs sorted by slug.
// It refreshes the cache if the TTL has expired.
func (r *Registry) List(ctx context.Context) []AgentConfig {
	if r.cacheExpired() {
		if err := r.Refresh(ctx); err != nil {
			r.logger.Warn("agent registry refresh failed, using stale cache", "error", err)
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]AgentConfig, 0, len(r.agents))
	for _, cfg := range r.agents {
		configs = append(configs, cfg)
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Slug < configs[j].Slug
	})
	return configs
}

// ListForCompany returns system agents + company-specific agents, sorted.
func (r *Registry) ListForCompany(ctx context.Context, companyID int64) []AgentConfig {
	all := r.List(ctx)
	filtered := make([]AgentConfig, 0, len(all))
	for _, cfg := range all {
		if cfg.CompanyID == 0 || cfg.CompanyID == companyID {
			filtered = append(filtered, cfg)
		}
	}
	return filtered
}

// Refresh reloads all agent definitions from the database.
func (r *Registry) Refresh(ctx context.Context) error {
	rows, err := r.queries.ListAgents(ctx)
	if err != nil {
		return err
	}

	agents := make(map[string]AgentConfig, len(rows))
	for _, row := range rows {
		var companyID int64
		if row.CompanyID != nil {
			companyID = *row.CompanyID
		}
		agents[row.Slug] = AgentConfig{
			Slug:           row.Slug,
			Name:           row.Name,
			Description:    row.Description,
			SystemPrompt:   row.SystemPrompt,
			Tools:          row.Tools,
			CostMultiplier: numericToFloat(row.CostMultiplier),
			IsAutonomous:   row.IsAutonomous,
			MaxRounds:      int(row.MaxRounds),
			MaxTokens:      int(row.MaxTokens),
			Icon:           row.Icon,
			Model:          row.Model,
			CompanyID:      companyID,
		}
	}

	r.mu.Lock()
	r.agents = agents
	r.lastLoad = time.Now()
	r.mu.Unlock()

	r.logger.Info("agent registry refreshed", "count", len(agents))
	return nil
}

// InvalidateCache forces the next Get/List call to reload from DB.
func (r *Registry) InvalidateCache() {
	r.mu.Lock()
	r.lastLoad = time.Time{}
	r.mu.Unlock()
}

// cacheExpired returns true if the cache TTL has elapsed.
func (r *Registry) cacheExpired() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.lastLoad) > r.cacheTTL
}

// numericToFloat converts a pgtype.Numeric to float64, defaulting to 1.0.
func numericToFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 1.0
	}
	return f.Float64
}
