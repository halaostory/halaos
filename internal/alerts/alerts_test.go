package alerts

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
