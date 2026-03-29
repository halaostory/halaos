package agent

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

// --- Mock Provider ---

type mockProvider struct {
	name        string
	generateFn  func(ctx context.Context, req provider.Request) (*provider.Response, error)
	streamFn    func(ctx context.Context, req provider.Request, onChunk func(provider.StreamChunk)) (*provider.Response, error)
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Generate(ctx context.Context, req provider.Request) (*provider.Response, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, req)
	}
	return &provider.Response{Content: "mock response", StopReason: provider.StopEndTurn, Usage: provider.Usage{InputTokens: 100, OutputTokens: 50}}, nil
}
func (m *mockProvider) Stream(ctx context.Context, req provider.Request, onChunk func(provider.StreamChunk)) (*provider.Response, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, req, onChunk)
	}
	onChunk(provider.StreamChunk{Type: "text_delta", Text: "streamed"})
	onChunk(provider.StreamChunk{Type: "message_stop"})
	return &provider.Response{Content: "streamed", StopReason: provider.StopEndTurn, Usage: provider.Usage{InputTokens: 80, OutputTokens: 40}}, nil
}
func (m *mockProvider) Health(ctx context.Context) error { return nil }

// --- Mock ToolRegistry ---

type mockToolRegistry struct {
	defs      []provider.ToolDefinition
	executeFn func(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error)
}

func (m *mockToolRegistry) Definitions() []provider.ToolDefinition { return m.defs }
func (m *mockToolRegistry) DefinitionsForAgent(allowed []string) []provider.ToolDefinition {
	return m.defs
}
func (m *mockToolRegistry) Execute(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, name, companyID, userID, input)
	}
	return `{"ok": true}`, nil
}

// --- Mock BillingService ---

type mockBilling struct {
	balance    int64
	balanceErr error
	costFn     func(input, output int, mult float64) int64
	deductErr  error
}

func (m *mockBilling) CheckBalance(_ context.Context, _ int64) (int64, error) {
	return m.balance, m.balanceErr
}
func (m *mockBilling) CalculateTokenCost(input, output int, mult float64) int64 {
	if m.costFn != nil {
		return m.costFn(input, output, mult)
	}
	return int64(input+output) * int64(mult)
}
func (m *mockBilling) DeductTokens(_ context.Context, _, _, _ int64, _, _ string) error {
	return m.deductErr
}

// --- Helpers ---

// chatSessionRow returns values matching ChatSession scan order (7 cols).
func chatSessionRow(sessionID uuid.UUID) []interface{} {
	now := time.Now()
	return []interface{}{sessionID, int64(1), int64(1), "general", "", now, now}
}

// chatMessageRow returns values matching ChatMessage scan order (6 cols).
func chatMessageRow(id int64, sessionID uuid.UUID, role, content string) []interface{} {
	return []interface{}{id, sessionID, role, content, int32(0), time.Now()}
}

// auditLogRow returns values matching AiAuditLog scan order (17 cols).
func auditLogRow() []interface{} {
	now := time.Now()
	s := "test"
	return []interface{}{
		int64(1), int64(1), int64(1), "req-id", &s, "intent", "model",
		&s, []byte("[]"), (*string)(nil), "low",
		int32(100), int32(50), int32(10), &s, &s, now,
	}
}

// queueChatDBCalls queues all DB calls needed for a successful Chat flow (new session).
func queueChatDBCalls(mock *testutil.MockDBTX, sessionID uuid.UUID) {
	// 1. CreateChatSession → QueryRow
	mock.OnQueryRow(testutil.NewRow(chatSessionRow(sessionID)...))
	// 2. ListChatMessages → Query (empty history)
	mock.OnQuery(testutil.NewEmptyRows(), nil)
	// 3. InsertChatMessage (user) → QueryRow
	mock.OnQueryRow(testutil.NewRow(chatMessageRow(1, sessionID, "user", "hello")...))
	// 4. InsertChatMessage (assistant) → QueryRow
	mock.OnQueryRow(testutil.NewRow(chatMessageRow(2, sessionID, "assistant", "response")...))
	// 5. UpdateChatSessionTitle → Exec
	mock.OnExecSuccess()
	// 6. InsertAIAuditLog → QueryRow
	mock.OnQueryRow(testutil.NewRow(auditLogRow()...))
}

func newTestExecutor(p provider.Provider, tools ToolRegistry, billing BillingService, reg *Registry, mock *testutil.MockDBTX) *Executor {
	queries := store.New(mock)
	return NewExecutor(p, tools, billing, reg, queries, slog.Default(), nil, nil)
}

// --- Tests ---

func TestResolveAgent(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", Name: "General", MaxRounds: 5, MaxTokens: 4096},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	e := newTestExecutor(&mockProvider{name: "test"}, &mockToolRegistry{}, &mockBilling{balance: 1000}, reg, mock)

	t.Run("empty_slug_defaults_to_general", func(t *testing.T) {
		cfg, err := e.resolveAgent(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Slug != "general" {
			t.Errorf("slug = %s, want general", cfg.Slug)
		}
	})

	t.Run("found", func(t *testing.T) {
		cfg, err := e.resolveAgent(context.Background(), "general")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Name != "General" {
			t.Errorf("name = %s, want General", cfg.Name)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := e.resolveAgent(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent agent")
		}
	})
}

func TestChatInsufficientBalance(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	billing := &mockBilling{balance: 0} // zero balance
	e := newTestExecutor(&mockProvider{name: "test"}, &mockToolRegistry{}, billing, reg, mock)

	_, err := e.Chat(context.Background(), 1, 1, "general", ChatRequest{Message: "hi"})
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestChatBalanceCheckError(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	billing := &mockBilling{balanceErr: fmt.Errorf("billing down")}
	e := newTestExecutor(&mockProvider{name: "test"}, &mockToolRegistry{}, billing, reg, mock)

	_, err := e.Chat(context.Background(), 1, 1, "general", ChatRequest{Message: "hi"})
	if err == nil {
		t.Fatal("expected error from balance check")
	}
}

func TestChatSuccess(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096, CostMultiplier: 1.0},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	sessionID := uuid.New()
	queueChatDBCalls(mock, sessionID)

	prov := &mockProvider{name: "test-provider"}
	billing := &mockBilling{balance: 10000}
	e := newTestExecutor(prov, &mockToolRegistry{}, billing, reg, mock)

	resp, err := e.Chat(context.Background(), 1, 1, "general", ChatRequest{Message: "hello"})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Message != "mock response" {
		t.Errorf("message = %s, want 'mock response'", resp.Message)
	}
	if resp.Agent != "general" {
		t.Errorf("agent = %s, want general", resp.Agent)
	}
	if resp.SessionID == "" {
		t.Error("session_id should not be empty")
	}
	if resp.TokensUsed <= 0 {
		t.Error("tokens_used should be > 0")
	}
}

func TestChatAgentNotFound(t *testing.T) {
	reg := newTestRegistry(map[string]AgentConfig{})
	mock := testutil.NewMockDBTX()
	e := newTestExecutor(&mockProvider{name: "test"}, &mockToolRegistry{}, &mockBilling{balance: 1000}, reg, mock)

	_, err := e.Chat(context.Background(), 1, 1, "nonexistent", ChatRequest{Message: "hi"})
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestChatWithToolUse(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096, CostMultiplier: 1.0, Tools: []string{"get_time"}},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	sessionID := uuid.New()
	queueChatDBCalls(mock, sessionID)

	callCount := 0
	prov := &mockProvider{
		name: "test",
		generateFn: func(_ context.Context, req provider.Request) (*provider.Response, error) {
			callCount++
			if callCount == 1 {
				// First round: request tool use
				return &provider.Response{
					Content:    "",
					StopReason: provider.StopToolUse,
					ToolCalls: []provider.ToolCall{
						{ID: "tc-1", Name: "get_time", Input: map[string]any{}},
					},
					Usage: provider.Usage{InputTokens: 50, OutputTokens: 20},
				}, nil
			}
			// Second round: final response
			return &provider.Response{
				Content:    "The time is 3pm",
				StopReason: provider.StopEndTurn,
				Usage:      provider.Usage{InputTokens: 80, OutputTokens: 30},
			}, nil
		},
	}

	toolExecuted := false
	tools := &mockToolRegistry{
		executeFn: func(_ context.Context, name string, _, _ int64, _ map[string]any) (string, error) {
			toolExecuted = true
			if name != "get_time" {
				t.Errorf("tool name = %s, want get_time", name)
			}
			return `{"time": "15:00"}`, nil
		},
	}
	billing := &mockBilling{balance: 10000}
	e := newTestExecutor(prov, tools, billing, reg, mock)

	resp, err := e.Chat(context.Background(), 1, 1, "general", ChatRequest{Message: "what time is it?"})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if !toolExecuted {
		t.Error("expected tool to be executed")
	}
	if resp.Message != "The time is 3pm" {
		t.Errorf("message = %s, want 'The time is 3pm'", resp.Message)
	}
	if callCount != 2 {
		t.Errorf("provider called %d times, want 2", callCount)
	}
}

func TestChatDeductError(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096, CostMultiplier: 1.0},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	sessionID := uuid.New()

	// Queue session + messages (but deduction will fail before save)
	mock.OnQueryRow(testutil.NewRow(chatSessionRow(sessionID)...))  // CreateChatSession
	mock.OnQuery(testutil.NewEmptyRows(), nil)                       // ListChatMessages

	billing := &mockBilling{balance: 10000, deductErr: fmt.Errorf("deduct failed")}
	e := newTestExecutor(&mockProvider{name: "test"}, &mockToolRegistry{}, billing, reg, mock)

	_, err := e.Chat(context.Background(), 1, 1, "general", ChatRequest{Message: "hi"})
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestStreamChatSuccess(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096, CostMultiplier: 1.0},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	sessionID := uuid.New()
	queueChatDBCalls(mock, sessionID)

	billing := &mockBilling{balance: 10000}
	e := newTestExecutor(&mockProvider{name: "test"}, &mockToolRegistry{}, billing, reg, mock)

	var chunks []provider.StreamChunk
	resp, err := e.StreamChat(context.Background(), 1, 1, "general", ChatRequest{Message: "hello"}, func(chunk provider.StreamChunk) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("StreamChat() error: %v", err)
	}
	if resp.Message != "streamed" {
		t.Errorf("message = %s, want 'streamed'", resp.Message)
	}
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestChatProviderError(t *testing.T) {
	agents := map[string]AgentConfig{
		"general": {Slug: "general", MaxRounds: 5, MaxTokens: 4096, CostMultiplier: 1.0},
	}
	reg := newTestRegistry(agents)
	mock := testutil.NewMockDBTX()
	sessionID := uuid.New()

	mock.OnQueryRow(testutil.NewRow(chatSessionRow(sessionID)...))
	mock.OnQuery(testutil.NewEmptyRows(), nil)

	prov := &mockProvider{
		name: "test",
		generateFn: func(_ context.Context, _ provider.Request) (*provider.Response, error) {
			return nil, fmt.Errorf("provider error")
		},
	}
	billing := &mockBilling{balance: 10000}
	e := newTestExecutor(prov, &mockToolRegistry{}, billing, reg, mock)

	_, err := e.Chat(context.Background(), 1, 1, "general", ChatRequest{Message: "hi"})
	if err == nil {
		t.Fatal("expected provider error")
	}
}
