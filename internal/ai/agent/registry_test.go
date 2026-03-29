package agent

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

// agentRow returns a []interface{} that matches the Agent model scan order (16 columns).
func agentRow(slug, name string, companyID *int64, tools []string, costMultiplier pgtype.Numeric, maxRounds, maxTokens int32) []interface{} {
	now := time.Now()
	return []interface{}{
		int64(1),       // ID
		slug,           // Slug
		name,           // Name
		"desc-" + slug, // Description
		"prompt-" + slug, // SystemPrompt
		tools,            // Tools
		costMultiplier,   // CostMultiplier
		true,             // IsActive
		false,            // IsAutonomous
		maxRounds,        // MaxRounds
		maxTokens,        // MaxTokens
		"robot",          // Icon
		now,              // CreatedAt
		now,              // UpdatedAt
		"",               // Model
		companyID,        // CompanyID
	}
}

func validNumeric(val float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.1f", val))
	return n
}

// newTestRegistry creates a registry with pre-populated cache (no DB call needed).
func newTestRegistry(agents map[string]AgentConfig) *Registry {
	return &Registry{
		logger:   slog.Default(),
		agents:   agents,
		lastLoad: time.Now(),
		cacheTTL: 5 * time.Minute,
	}
}

func TestNumericToFloat(t *testing.T) {
	tests := []struct {
		name   string
		input  pgtype.Numeric
		expect float64
	}{
		{"valid_1.5", validNumeric(1.5), 1.5},
		{"valid_2.0", validNumeric(2.0), 2.0},
		{"zero_value_defaults_to_1", pgtype.Numeric{}, 1.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := numericToFloat(tc.input)
			if got != tc.expect {
				t.Errorf("numericToFloat() = %f, want %f", got, tc.expect)
			}
		})
	}
}

func TestRegistryRefresh(t *testing.T) {
	mock := testutil.NewMockDBTX()
	companyID := int64(42)
	mock.OnQuery(testutil.NewRows([][]interface{}{
		agentRow("general", "General", nil, []string{"tool1"}, validNumeric(1.0), 5, 4096),
		agentRow("payroll", "Payroll", &companyID, []string{"calc_pay"}, validNumeric(1.5), 3, 2048),
	}), nil)

	queries := store.New(mock)
	r := &Registry{
		queries:  queries,
		logger:   slog.Default(),
		agents:   make(map[string]AgentConfig),
		cacheTTL: 5 * time.Minute,
	}

	if err := r.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if len(r.agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(r.agents))
	}

	general := r.agents["general"]
	if general.Slug != "general" || general.CompanyID != 0 {
		t.Errorf("general agent: slug=%s companyID=%d", general.Slug, general.CompanyID)
	}
	if general.CostMultiplier != 1.0 {
		t.Errorf("general cost multiplier = %f, want 1.0", general.CostMultiplier)
	}

	payroll := r.agents["payroll"]
	if payroll.CompanyID != 42 {
		t.Errorf("payroll companyID = %d, want 42", payroll.CompanyID)
	}
	if payroll.CostMultiplier != 1.5 {
		t.Errorf("payroll cost multiplier = %f, want 1.5", payroll.CostMultiplier)
	}
	if len(payroll.Tools) != 1 || payroll.Tools[0] != "calc_pay" {
		t.Errorf("payroll tools = %v, want [calc_pay]", payroll.Tools)
	}
}

func TestRegistryRefreshError(t *testing.T) {
	mock := testutil.NewMockDBTX()
	mock.OnQuery(nil, fmt.Errorf("db down"))

	queries := store.New(mock)
	r := &Registry{
		queries:  queries,
		logger:   slog.Default(),
		agents:   make(map[string]AgentConfig),
		cacheTTL: 5 * time.Minute,
	}

	err := r.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error from Refresh")
	}
}

func TestRegistryGet(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", Name: "General"},
		"payroll": {Slug: "payroll", Name: "Payroll"},
	}
	r := newTestRegistry(agents)

	t.Run("found", func(t *testing.T) {
		cfg, ok := r.Get(context.Background(), "general")
		if !ok {
			t.Fatal("expected to find 'general'")
		}
		if cfg.Name != "General" {
			t.Errorf("name = %s, want General", cfg.Name)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, ok := r.Get(context.Background(), "nonexistent")
		if ok {
			t.Error("expected not found")
		}
	})
}

func TestRegistryGetForCompany(t *testing.T) {
	agents := map[string]AgentConfig{
		"general":    {Slug: "general", CompanyID: 0},  // system
		"custom-hr":  {Slug: "custom-hr", CompanyID: 1},
		"custom-fin": {Slug: "custom-fin", CompanyID: 2},
	}
	r := newTestRegistry(agents)

	t.Run("system_agent_accessible_by_any_company", func(t *testing.T) {
		cfg, ok := r.GetForCompany(context.Background(), "general", 99)
		if !ok {
			t.Fatal("system agent should be accessible by any company")
		}
		if cfg.Slug != "general" {
			t.Errorf("slug = %s, want general", cfg.Slug)
		}
	})

	t.Run("company_agent_accessible_by_owner", func(t *testing.T) {
		cfg, ok := r.GetForCompany(context.Background(), "custom-hr", 1)
		if !ok {
			t.Fatal("company agent should be accessible by owner")
		}
		if cfg.Slug != "custom-hr" {
			t.Errorf("slug = %s, want custom-hr", cfg.Slug)
		}
	})

	t.Run("company_agent_blocked_for_other_company", func(t *testing.T) {
		_, ok := r.GetForCompany(context.Background(), "custom-hr", 2)
		if ok {
			t.Error("company agent should NOT be accessible by other company")
		}
	})

	t.Run("nonexistent", func(t *testing.T) {
		_, ok := r.GetForCompany(context.Background(), "nope", 1)
		if ok {
			t.Error("expected not found")
		}
	})
}

func TestRegistryList(t *testing.T) {
	agents := map[string]AgentConfig{
		"z-agent": {Slug: "z-agent"},
		"a-agent": {Slug: "a-agent"},
		"m-agent": {Slug: "m-agent"},
	}
	r := newTestRegistry(agents)

	list := r.List(context.Background())
	if len(list) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(list))
	}
	if list[0].Slug != "a-agent" || list[1].Slug != "m-agent" || list[2].Slug != "z-agent" {
		t.Errorf("agents not sorted: %v", []string{list[0].Slug, list[1].Slug, list[2].Slug})
	}
}

func TestRegistryListForCompany(t *testing.T) {
	agents := map[string]AgentConfig{
		"general":    {Slug: "general", CompanyID: 0},
		"company1":   {Slug: "company1", CompanyID: 1},
		"company2":   {Slug: "company2", CompanyID: 2},
	}
	r := newTestRegistry(agents)

	list := r.ListForCompany(context.Background(), 1)
	if len(list) != 2 {
		t.Fatalf("expected 2 agents (system + company1), got %d", len(list))
	}
	slugs := make(map[string]bool)
	for _, cfg := range list {
		slugs[cfg.Slug] = true
	}
	if !slugs["general"] || !slugs["company1"] {
		t.Errorf("expected general + company1, got %v", slugs)
	}
	if slugs["company2"] {
		t.Error("company2 should not appear for company 1")
	}
}

func TestRegistryInvalidateCache(t *testing.T) {
	r := newTestRegistry(map[string]AgentConfig{
		"general": {Slug: "general"},
	})

	if r.cacheExpired() {
		t.Fatal("cache should NOT be expired right after creation")
	}

	r.InvalidateCache()

	if !r.cacheExpired() {
		t.Fatal("cache should be expired after InvalidateCache")
	}
}

func TestRegistryCacheExpired(t *testing.T) {
	r := &Registry{
		logger:   slog.Default(),
		agents:   make(map[string]AgentConfig),
		lastLoad: time.Now().Add(-10 * time.Minute), // 10 min ago
		cacheTTL: 5 * time.Minute,
	}

	if !r.cacheExpired() {
		t.Error("cache should be expired (10min > 5min TTL)")
	}

	r.lastLoad = time.Now()
	if r.cacheExpired() {
		t.Error("cache should NOT be expired (just loaded)")
	}
}
