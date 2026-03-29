package bot

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

func newTestBotManager(db *testutil.MockDBTX) *BotManager {
	queries := store.New(db)
	cfg := &config.BotConfig{
		Enabled:             true,
		TelegramBotToken:    "test-shared-token",
		TelegramBotUsername: "halaosbot",
	}
	// dispatcher is nil — StartBot will panic on Run, but we override the map directly for unit tests
	return &BotManager{
		bots:    make(map[int64]runningBot),
		queries: queries,
		cfg:     cfg,
		logger:  slog.Default(),
	}
}

func TestStartBot(t *testing.T) {
	db := testutil.NewMockDBTX()
	m := newTestBotManager(db)

	// Simulate StartBot by inserting a running bot entry directly
	// (can't call StartBot because it spawns a real goroutine needing a dispatcher)
	cancelled := false
	m.bots[42] = runningBot{
		cancel: func() { cancelled = true },
		token:  "tok-42",
	}

	if !m.IsRunning(42) {
		t.Error("expected bot 42 to be running")
	}
	if m.IsRunning(99) {
		t.Error("expected bot 99 to NOT be running")
	}
	if cancelled {
		t.Error("should not have been cancelled yet")
	}
}

func TestStopBot(t *testing.T) {
	db := testutil.NewMockDBTX()
	m := newTestBotManager(db)

	cancelled := false
	m.bots[42] = runningBot{
		cancel: func() { cancelled = true },
		token:  "tok-42",
	}

	m.StopBot(42)

	if !cancelled {
		t.Error("expected cancel to be called")
	}
	if m.IsRunning(42) {
		t.Error("expected bot 42 to be stopped")
	}
}

func TestStopBot_Idempotent(t *testing.T) {
	db := testutil.NewMockDBTX()
	m := newTestBotManager(db)

	// Stopping a bot that doesn't exist should not panic
	m.StopBot(999)
}

func TestStopAll(t *testing.T) {
	db := testutil.NewMockDBTX()
	m := newTestBotManager(db)

	cancelled := make(map[int64]bool)
	for _, id := range []int64{0, 1, 2, 3} {
		id := id
		m.bots[id] = runningBot{
			cancel: func() { cancelled[id] = true },
			token:  "tok",
		}
	}

	m.StopAll()

	if len(m.bots) != 0 {
		t.Errorf("expected all bots stopped, got %d remaining", len(m.bots))
	}
	for _, id := range []int64{0, 1, 2, 3} {
		if !cancelled[id] {
			t.Errorf("expected bot %d to be cancelled", id)
		}
	}
}

func TestReload_StopsOnMissingConfig(t *testing.T) {
	db := testutil.NewMockDBTX()
	m := newTestBotManager(db)

	// Pre-populate a running bot
	cancelled := false
	m.bots[42] = runningBot{
		cancel: func() { cancelled = true },
		token:  "tok-42",
	}

	// Queue an error for GetBotConfig (no config found)
	db.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	m.Reload(42)

	if !cancelled {
		t.Error("expected running bot to be cancelled when config not found")
	}
	if m.IsRunning(42) {
		t.Error("expected bot 42 to be stopped after reload with no config")
	}
}

func TestIsRunning_EmptyManager(t *testing.T) {
	db := testutil.NewMockDBTX()
	m := newTestBotManager(db)

	if m.IsRunning(0) {
		t.Error("expected no bots running in empty manager")
	}
}
