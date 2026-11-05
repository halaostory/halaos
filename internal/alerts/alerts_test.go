package alerts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPost_OnlySlackWebhooks(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	s := NewService(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://people.example.com/")

	if err := s.post(context.Background(), srv.URL+"/services/x", "hi"); err == nil {
		t.Fatal("non-Slack URL must be refused")
	}
	old := webhookPrefix
	webhookPrefix = srv.URL + "/"
	defer func() { webhookPrefix = old }()
	if err := s.post(context.Background(), srv.URL+"/services/x", s.message("Payroll ready", "/payroll")); err != nil {
		t.Fatal(err)
	}
	if got["text"] != "Payroll ready\n<https://people.example.com/payroll|Open in HalaPeople>" {
		t.Errorf("text %q", got["text"])
	}
}

func TestNotifyNilSafe(t *testing.T) {
	var s *Service
	s.Notify(1, EventNotice, "x", "") // must not panic
	if !strings.HasPrefix(webhookPrefix, "https://hooks.slack.com/") {
		t.Error("default prefix changed")
	}
}

// A routine's "post in Slack" step posts even when the company did not
// subscribe to any alert event, and is refused without a connected channel.
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
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]string
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &m)
		got = append(got, m["text"])
	}))
	defer srv.Close()
	old := webhookPrefix
	webhookPrefix = srv.URL + "/"
	defer func() { webhookPrefix = old }()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	s := NewServiceFromKey(q, key, slog.New(slog.NewTextHandler(io.Discard, nil)), "")

	var cid int64
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, fmt.Sprint("Slack Co ", time.Now().UnixNano())).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid) //nolint:errcheck
	if err := s.PostMessage(ctx, cid, "hello", ""); !errors.Is(err, ErrSlackNotConnected) {
		t.Fatalf("no channel: %v", err)
	}
	enc, _ := crypto.NewCredentialEncryptor(key)
	hook, _ := enc.Encrypt([]byte(srv.URL + "/services/T/B/x"))
	if _, err := q.UpsertAlertChannel(ctx, store.UpsertAlertChannelParams{CompanyID: cid, SlackWebhookEnc: hook, SlackHint: "x", Events: []string{}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.PostMessage(ctx, cid, "Welcome Ana to the team!", ""); err != nil || len(got) != 1 || got[0] != "Welcome Ana to the team!" {
		t.Fatalf("post: %v %v", err, got)
	}
}
