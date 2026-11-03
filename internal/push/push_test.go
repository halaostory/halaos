package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

func TestSendAndPruneExpired(t *testing.T) {
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
	t.Setenv("VAPID_PUBLIC_KEY", "")
	s, err := New(ctx, q, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://people.example.com")
	if err != nil || s.public == "" {
		t.Fatalf("keys: %v", err)
	}
	again, _ := New(ctx, q, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	if again.public != s.public {
		t.Fatal("VAPID keys must be stable across restarts")
	}

	var gone atomic.Bool
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "vapid t=") || r.Header.Get("Content-Encoding") != "aes128gcm" {
			w.WriteHeader(400)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "Leave Approved") {
			w.WriteHeader(400) // payload must be encrypted
			return
		}
		got.Add(1)
		if gone.Load() {
			w.WriteHeader(http.StatusGone)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	var cid, uid int64
	sfx := fmt.Sprint(time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "Push Co "+sfx).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid) //nolint:errcheck
	if err := pool.QueryRow(ctx, `INSERT INTO users (company_id, email, password_hash, first_name, last_name, role) VALUES ($1, $2, 'x', 'P', 'U', 'employee') RETURNING id`, cid, "p"+sfx+"@t.test").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	b64 := base64.RawURLEncoding.EncodeToString
	if err := q.UpsertPushSubscription(ctx, store.UpsertPushSubscriptionParams{CompanyID: cid, UserID: uid, Endpoint: srv.URL + "/sub/" + sfx,
		P256dh: b64(key.PublicKey().Bytes()), Auth: b64(secret)}); err != nil {
		t.Fatal(err)
	}
	msg := Message{Title: "Leave Approved", Body: "Your leave was approved", URL: LinkFor("leave"), Tag: "leave"}
	if n := s.Send(ctx, cid, uid, msg); n != 1 || got.Load() != 1 {
		t.Fatalf("sent %d (server got %d)", n, got.Load())
	}
	if n := s.Send(ctx, cid+1, uid, msg); n != 0 {
		t.Fatal("pushed to a device registered under another company")
	}
	gone.Store(true)
	s.Send(ctx, cid, uid, msg)
	if n, _ := q.CountPushSubscriptions(ctx, uid); n != 0 {
		t.Fatalf("expired subscription kept (%d)", n)
	}
}
