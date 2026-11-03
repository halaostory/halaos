package push

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/store"
)

func TestFCMNativePush(t *testing.T) {
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

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	var tokenCalls, sends atomic.Int32
	var gone atomic.Bool
	var lastBody map[string]any
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		tok, err := jwt.Parse(r.Form.Get("assertion"), func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, jwt.WithValidMethods([]string{"RS256"}))
		if err != nil || !tok.Valid {
			w.WriteHeader(401)
			return
		}
		c := tok.Claims.(jwt.MapClaims)
		if c["iss"] != "push@proj.iam.gserviceaccount.com" || c["scope"] != "https://www.googleapis.com/auth/firebase.messaging" {
			w.WriteHeader(403)
			return
		}
		tokenCalls.Add(1)
		_, _ = w.Write([]byte(`{"access_token":"ya29.test","expires_in":3600}`))
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ya29.test" {
			w.WriteHeader(401)
			return
		}
		sends.Add(1)
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &lastBody)
		if gone.Load() {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"name":"projects/proj/messages/1"}`))
	})
	sa, _ := json.Marshal(map[string]string{"project_id": "proj", "client_email": "push@proj.iam.gserviceaccount.com", "private_key": string(pemKey), "token_uri": srv.URL + "/token"})
	fcm, err := NewFCM(sa)
	if err != nil {
		t.Fatal(err)
	}
	fcm.SendURL, fcm.HTTP = srv.URL+"/send", srv.Client()
	s := &Service{q: q, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), FCM: fcm, HTTP: srv.Client()}

	var cid, uid int64
	sfx := fmt.Sprint(time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, "FCM Co "+sfx).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, cid) //nolint:errcheck
	if err := pool.QueryRow(ctx, `INSERT INTO users (company_id, email, password_hash, first_name, last_name, role) VALUES ($1, $2, 'x', 'F', 'C', 'employee') RETURNING id`, cid, "f"+sfx+"@t.test").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertNativePushToken(ctx, store.UpsertNativePushTokenParams{CompanyID: cid, UserID: uid, Token: "device-" + sfx, Platform: "ios"}); err != nil {
		t.Fatal(err)
	}
	m := Message{Title: "Payslip ready", Body: "Nov 1-15", URL: LinkFor("payroll"), Tag: "payroll"}
	if n := s.Send(ctx, cid, uid, m); n != 1 {
		t.Fatalf("sent %d", n)
	}
	msg := lastBody["message"].(map[string]any)
	if msg["token"] != "device-"+sfx || msg["notification"].(map[string]any)["title"] != "Payslip ready" || msg["data"].(map[string]any)["url"] != "/m/payslips" {
		t.Fatalf("payload %v", msg)
	}
	s.Send(ctx, cid, uid, m)
	if tokenCalls.Load() != 1 {
		t.Errorf("access token must be cached, exchanged %d times", tokenCalls.Load())
	}
	gone.Store(true)
	s.Send(ctx, cid, uid, m)
	var left int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM native_push_tokens WHERE user_id = $1`, uid).Scan(&left)
	if left != 0 {
		t.Fatalf("unregistered token kept")
	}
}
