package bot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

func newTestLinkHandler(db *testutil.MockDBTX) *LinkHandler {
	queries := store.New(db)
	return &LinkHandler{
		queries: queries,
		logger:  slog.Default(),
		cfg: &config.BotConfig{
			Enabled:             true,
			TelegramBotToken:    "shared-token",
			TelegramBotUsername: "halaosbot",
		},
	}
}

// ── TestBotToken ──

func TestTestBotToken_Success(t *testing.T) {
	// Mock Telegram API server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"result": map[string]interface{}{
				"username":   "test_bot",
				"first_name": "Test Bot",
			},
		})
	}))
	defer ts.Close()

	db := testutil.NewMockDBTX()
	h := newTestLinkHandler(db)
	// Override httpClient to point to our test server
	h.httpClient = ts.Client()

	// We also need to override the URL. Since TestBotToken constructs the URL,
	// we'll use a transport that redirects all requests to our test server.
	h.httpClient.Transport = &testTransport{baseURL: ts.URL}

	c, w := testutil.NewGinContext("POST", "/admin/bot/test-token", map[string]string{
		"bot_token": "123456:ABC-DEF",
	}, testutil.DefaultAdmin)

	h.TestBotToken(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := testutil.ResponseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", body)
	}
	if data["bot_username"] != "test_bot" {
		t.Errorf("expected bot_username=test_bot, got %v", data["bot_username"])
	}
	if data["bot_name"] != "Test Bot" {
		t.Errorf("expected bot_name=Test Bot, got %v", data["bot_name"])
	}
}

func TestTestBotToken_InvalidToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"description": "Unauthorized",
		})
	}))
	defer ts.Close()

	db := testutil.NewMockDBTX()
	h := newTestLinkHandler(db)
	h.httpClient = ts.Client()
	h.httpClient.Transport = &testTransport{baseURL: ts.URL}

	c, w := testutil.NewGinContext("POST", "/admin/bot/test-token", map[string]string{
		"bot_token": "invalid-token",
	}, testutil.DefaultAdmin)

	h.TestBotToken(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestBotToken_EmptyToken(t *testing.T) {
	db := testutil.NewMockDBTX()
	h := newTestLinkHandler(db)

	c, w := testutil.NewGinContext("POST", "/admin/bot/test-token", map[string]string{
		"bot_token": "",
	}, testutil.DefaultAdmin)

	h.TestBotToken(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestBotToken_NoBody(t *testing.T) {
	db := testutil.NewMockDBTX()
	h := newTestLinkHandler(db)

	c, w := testutil.NewGinContext("POST", "/admin/bot/test-token", nil, testutil.DefaultAdmin)

	h.TestBotToken(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── GetBotInfo ──

func TestGetBotInfo_WithConfig(t *testing.T) {
	db := testutil.NewMockDBTX()
	// Queue a row for GetBotConfig query (10 columns)
	db.OnQueryRow(testutil.NewRow(
		int64(1),         // ID
		int64(1),         // CompanyID
		"telegram",       // Platform
		"tok:xxx",        // BotToken
		"mybot",          // BotUsername
		true,             // IsActive
		"",               // WebhookUrl
		"polling",        // Mode
		time.Now(),       // CreatedAt
		time.Now(),       // UpdatedAt
	))

	h := newTestLinkHandler(db)

	c, w := testutil.NewGinContext("GET", "/bot/info", nil, testutil.DefaultAdmin)

	h.GetBotInfo(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := testutil.ResponseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", body)
	}
	if data["bot_username"] != "@mybot" {
		t.Errorf("expected @mybot, got %v", data["bot_username"])
	}
	if data["is_active"] != true {
		t.Errorf("expected is_active=true, got %v", data["is_active"])
	}
	if data["is_shared"] != false {
		t.Errorf("expected is_shared=false, got %v", data["is_shared"])
	}
}

func TestGetBotInfo_NoConfig_SharedFallback(t *testing.T) {
	db := testutil.NewMockDBTX()
	// Queue an error row (no config found)
	db.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("no rows")))

	h := newTestLinkHandler(db)

	c, w := testutil.NewGinContext("GET", "/bot/info", nil, testutil.DefaultEmployee)

	h.GetBotInfo(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := testutil.ResponseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", body)
	}
	// Shared bot fallback
	if data["bot_username"] != "@halaosbot" {
		t.Errorf("expected @halaosbot, got %v", data["bot_username"])
	}
	if data["is_active"] != true {
		t.Errorf("expected is_active=true (shared bot enabled), got %v", data["is_active"])
	}
	if data["is_shared"] != true {
		t.Errorf("expected is_shared=true, got %v", data["is_shared"])
	}
}

// testTransport redirects all HTTP requests to a test server.
type testTransport struct {
	baseURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server, preserving path
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}
