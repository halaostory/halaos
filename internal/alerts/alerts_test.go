package alerts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/store"
)

func TestValidWebhook(t *testing.T) {
	ok := map[string]string{
		"https://hooks.slack.com/services/T/B/x":                                          KindSlack,
		"https://prod-12.southeastasia.logic.azure.com:443/workflows/abc/triggers/manual": KindTeams,
		"https://default1.environment.api.powerplatform.com/powerautomate/automations/x":  KindTeams,
		"https://acme.webhook.office.com/webhookb2/x":                                     KindTeams,
		"https://chat.googleapis.com/v1/spaces/AAA/messages?key=k&token=t":                KindGoogleChat,
		"https://open.larksuite.com/open-apis/bot/v2/hook/abc":                            KindLark,
		"https://open.feishu.cn/open-apis/bot/v2/hook/abc":                                KindLark,
	}
	for u, k := range ok {
		if !validWebhook(k, u) {
			t.Errorf("%s %s should be accepted", k, u)
		}
	}
	bad := map[string]string{
		"http://hooks.slack.com/services/x":                   KindSlack, // not https
		"https://hooks.slack.com.evil.io/x":                   KindSlack,
		"https://user@hooks.slack.com/x":                      KindSlack,
		"https://chat.googleapis.com/v1/other":                KindGoogleChat,
		"https://logic.azure.com.evil.io/x":                   KindTeams,
		"https://open.larksuite.com/open-apis/im/v1/messages": KindLark,
		"https://hooks.slack.com/services/x#":                 KindTeams, // wrong kind
		"https://169.254.169.254/latest/meta-data":            KindSlack,
		"https://api.telegram.org/bot1:x/sendMessage":         KindTelegram, // bot kinds take no URL
	}
	for u, k := range bad {
		if validWebhook(k, u) {
			t.Errorf("%s %s must be refused", k, u)
		}
	}
	if err := validateBot(KindTelegram, "123456789:AAH4b3XqLr0hT1yU6kZ9mN2pQ8sV5wE7c-_", "-1001234567890"); err != nil {
		t.Error(err)
	}
	if validateBot(KindTelegram, "123:x/../../evil", "-100123") == nil || validateBot(KindTelegram, "", "general") == nil {
		t.Error("bad telegram input accepted")
	}
	if err := validateBot(KindLINE, "", "C"+strings.Repeat("a1", 16)); err != nil {
		t.Error(err)
	}
	if validateBot(KindLINE, "", "group-1") == nil {
		t.Error("bad LINE group accepted")
	}
}

// capture records what each platform receives.
type capture struct {
	path, auth string
	body       map[string]any
}

func fakeChat(t *testing.T, reply string) (*httptest.Server, *[]capture) {
	var got []capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		got = append(got, capture{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: m})
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestPostPerPlatform(t *testing.T) {
	s := NewService(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://people.example.com/")
	ctx := context.Background()
	m := s.msg("✅ *Payroll* ready <now> & done", "/payroll")
	const link = "https://people.example.com/payroll"

	srv, got := fakeChat(t, `{"ok":true,"code":0}`)
	// webhook kinds: refused until the URL is on the platform's host
	for _, k := range []string{KindSlack, KindTeams, KindGoogleChat, KindLark} {
		if err := s.post(ctx, target{kind: k, secret: srv.URL + "/hook"}, m); err == nil {
			t.Fatalf("%s: foreign URL accepted", k)
		}
	}
	testWebhookPrefix = srv.URL + "/"
	oldTG, oldLINE := telegramBase, lineBase
	telegramBase, lineBase = srv.URL, srv.URL
	defer func() { testWebhookPrefix, telegramBase, lineBase = "", oldTG, oldLINE }()

	for _, tc := range []struct {
		t     target
		check func(c capture) bool
	}{
		{target{kind: KindSlack, secret: srv.URL + "/s"}, func(c capture) bool {
			return c.body["text"] == "✅ *Payroll* ready <now> & done\n<"+link+"|Open in HalaPeople>"
		}},
		{target{kind: KindGoogleChat, secret: srv.URL + "/g"}, func(c capture) bool {
			return strings.HasSuffix(c.body["text"].(string), "<"+link+"|Open in HalaPeople>")
		}},
		{target{kind: KindTeams, secret: srv.URL + "/t"}, func(c capture) bool {
			card := c.body["attachments"].([]any)[0].(map[string]any)["content"].(map[string]any)
			tb := card["body"].([]any)[0].(map[string]any)
			act := card["actions"].([]any)[0].(map[string]any)
			return c.body["type"] == "message" && tb["text"] == "✅ **Payroll** ready <now> & done" && act["url"] == link
		}},
		{target{kind: KindLark, secret: srv.URL + "/l"}, func(c capture) bool {
			return c.body["msg_type"] == "text" && c.body["content"].(map[string]any)["text"] == "✅ Payroll ready <now> & done\nOpen in HalaPeople: "+link
		}},
		{target{kind: KindTelegram, secret: "123456789:tok", chat: "-100555"}, func(c capture) bool {
			return c.path == "/bot123456789:tok/sendMessage" && c.body["chat_id"] == "-100555" && c.body["parse_mode"] == "HTML" &&
				c.body["text"] == "✅ <b>Payroll</b> ready &lt;now&gt; &amp; done\n<a href=\""+link+"\">Open in HalaPeople</a>"
		}},
		{target{kind: KindLINE, secret: "line-token", chat: "Cabc"}, func(c capture) bool {
			msg := c.body["messages"].([]any)[0].(map[string]any)
			return c.path == "/v2/bot/message/push" && c.auth == "Bearer line-token" && c.body["to"] == "Cabc" &&
				msg["text"] == "✅ Payroll ready <now> & done\nOpen in HalaPeople: "+link
		}},
	} {
		n := len(*got)
		if err := s.post(ctx, tc.t, m); err != nil || len(*got) != n+1 || !tc.check((*got)[n]) {
			t.Errorf("%s: err=%v got=%+v", tc.t.kind, err, (*got)[len(*got)-1])
		}
	}

	// Lark and Telegram report errors in a 200 body
	srv2, _ := fakeChat(t, `{"ok":false,"description":"chat not found","code":19021,"msg":"sign match fail"}`)
	testWebhookPrefix, telegramBase = srv2.URL+"/", srv2.URL
	if err := s.post(ctx, target{kind: KindLark, secret: srv2.URL + "/l"}, m); err == nil || !strings.Contains(err.Error(), "19021") {
		t.Errorf("lark error: %v", err)
	}
	if err := s.post(ctx, target{kind: KindTelegram, secret: "1:secret", chat: "-1"}, m); err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("telegram error: %v", err)
	}
}

func TestTelegramGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"update_id":1,"my_chat_member":{"chat":{"id":-1001,"type":"supergroup","title":"Acme HR"}}},
			{"update_id":2,"message":{"chat":{"id":-1001,"type":"supergroup","title":"Acme HR"}}},
			{"update_id":3,"message":{"chat":{"id":42,"type":"private"}}},
			{"update_id":4,"message":{"chat":{"id":-77,"type":"group","title":"Ops"}}}]}`))
	}))
	defer srv.Close()
	old := telegramBase
	telegramBase = srv.URL
	defer func() { telegramBase = old }()
	s := NewService(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	g, err := s.telegramGroups(context.Background(), "1:x")
	if err != nil || len(g) != 2 || g[0]["chat_id"] != "-1001" || g[1]["title"] != "Ops" {
		t.Fatalf("%v %v", g, err)
	}
}

func TestNotifyNilSafe(t *testing.T) {
	var s *Service
	s.Notify(1, EventNotice, "x", "") // must not panic
	if s.ChannelName(context.Background(), 1) != "" {
		t.Error("nil service has no channel")
	}
}

// A routine's "post in the team chat" step posts even when the company did
// not subscribe to any alert event, and is refused without a connected channel.
func TestPostMessage(t *testing.T) {
	dsn := os.Getenv("HALAOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("HALAOS_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := store.New(pool)
	srv, got := fakeChat(t, `{"ok":true}`)
	old := telegramBase
	telegramBase = srv.URL
	defer func() { telegramBase = old }()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	s := NewServiceFromKey(q, key, slog.New(slog.NewTextHandler(io.Discard, nil)), "")

	var cid int64
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, fmt.Sprint("Chat Co ", time.Now().UnixNano())).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid) //nolint:errcheck
	if err := s.PostMessage(ctx, cid, "hello", ""); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("no channel: %v", err)
	}
	enc, _ := crypto.NewCredentialEncryptor(key)
	tok, _ := enc.Encrypt([]byte("123456789:tok"))
	if _, err := q.UpsertAlertChannel(ctx, store.UpsertAlertChannelParams{CompanyID: cid, Kind: KindTelegram, SecretEnc: tok, Hint: "…:tok", Target: "-100777", Events: []string{}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.PostMessage(ctx, cid, "Welcome Ana to the team!", ""); err != nil || len(*got) != 1 || (*got)[0].body["text"] != "Welcome Ana to the team!" || (*got)[0].body["chat_id"] != "-100777" {
		t.Fatalf("post: %v %+v", err, *got)
	}
	if s.ChannelName(ctx, cid) != "Telegram" {
		t.Error("channel name")
	}
	// events filter still applies to Notify-style sends
	if err := s.send(ctx, cid, EventNotice, "x", ""); !errors.Is(err, errNotConfigured) {
		t.Errorf("unsubscribed event sent: %v", err)
	}
	// keeping the secret while updating events keeps kind and target
	if _, err := q.UpsertAlertChannel(ctx, store.UpsertAlertChannelParams{CompanyID: cid, Kind: KindTelegram, Target: "-100777", Events: []string{EventNotice}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if ch, _ := q.GetAlertChannel(ctx, cid); len(ch.SecretEnc) == 0 || ch.Hint != "…:tok" {
		t.Errorf("secret lost: %+v", ch)
	}
}
