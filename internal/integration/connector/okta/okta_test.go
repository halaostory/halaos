package okta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/halaostory/halaos/internal/integration/connector"
)

// fakeOkta keeps users in memory and checks the SSWS token.
func fakeOkta(t *testing.T) (*httptest.Server, map[string]map[string]any) {
	var mu sync.Mutex
	users := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "SSWS tok" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"errorCode":"E0000011","errorSummary":"Invalid token provided"}`))
			return
		}
		find := func(key string) map[string]any {
			for id, u := range users {
				if id == key || u["profile"].(map[string]any)["login"] == key {
					return u
				}
			}
			return nil
		}
		p := strings.TrimPrefix(r.URL.Path, "/api/v1/users")
		switch {
		case r.Method == "GET" && p == "":
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == "POST" && p == "":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := "00u" + string(rune('a'+len(users)))
			u := map[string]any{"id": id, "status": "ACTIVE", "profile": body["profile"]}
			users[id] = u
			_ = json.NewEncoder(w).Encode(u)
		case strings.HasSuffix(p, "/lifecycle/deactivate"):
			if u := find(strings.TrimSuffix(strings.TrimPrefix(p, "/"), "/lifecycle/deactivate")); u != nil {
				u["status"] = "DEPROVISIONED"
				return
			}
			w.WriteHeader(404)
		case strings.HasSuffix(p, "/lifecycle/activate"):
			if u := find(strings.TrimSuffix(strings.TrimPrefix(p, "/"), "/lifecycle/activate")); u != nil {
				u["status"] = "ACTIVE"
				return
			}
			w.WriteHeader(404)
		case r.Method == "GET":
			if u := find(strings.TrimPrefix(p, "/")); u != nil {
				_ = json.NewEncoder(w).Encode(u)
				return
			}
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"errorCode":"E0000007","errorSummary":"Not found"}`))
		case r.Method == "DELETE":
			delete(users, strings.TrimPrefix(p, "/"))
			w.WriteHeader(204)
		default:
			w.WriteHeader(400)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, users
}

func TestOktaLifecycle(t *testing.T) {
	srv, users := fakeOkta(t)
	c := NewWithBase(srv.URL, "tok", srv.Client())
	ctx := context.Background()
	if err := c.Health(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := c.Provision(ctx, connector.ProvisionRequest{Email: "ana@acme.test", FirstName: "Ana", LastName: "Cruz", Department: "Ops", Position: "Analyst"})
	if err != nil || res.ExternalID == "" || len(users) != 1 {
		t.Fatalf("%v %+v", err, res)
	}
	prof := users[res.ExternalID]["profile"].(map[string]any)
	if prof["login"] != "ana@acme.test" || prof["department"] != "Ops" || prof["title"] != "Analyst" {
		t.Errorf("profile %+v", prof)
	}
	// provisioning again finds the same account
	again, err := c.Provision(ctx, connector.ProvisionRequest{Email: "ana@acme.test", FirstName: "Ana", LastName: "Cruz"})
	if err != nil || again.ExternalID != res.ExternalID || len(users) != 1 {
		t.Fatalf("idempotent: %v %+v", err, again)
	}
	if _, err := c.Deprovision(ctx, connector.DeprovisionRequest{ExternalID: res.ExternalID}); err != nil {
		t.Fatal(err)
	}
	if s, _ := c.SyncAccount(ctx, connector.SyncRequest{ExternalID: res.ExternalID}); s.AccountStatus != "disabled" {
		t.Errorf("after deactivate: %+v", s)
	}
	// rehire re-activates
	re, err := c.Provision(ctx, connector.ProvisionRequest{Email: "ana@acme.test"})
	if err != nil || !strings.Contains(re.Message, "Re-activated") || users[res.ExternalID]["status"] != "ACTIVE" {
		t.Fatalf("rehire: %v %+v", err, re)
	}
	if out, err := c.Deprovision(ctx, connector.DeprovisionRequest{ExternalID: res.ExternalID, DeprovisionMode: "delete"}); err != nil || out.Status != "deleted" || len(users) != 0 {
		t.Fatalf("delete: %v %+v", err, out)
	}
	if s, _ := c.SyncAccount(ctx, connector.SyncRequest{ExternalID: res.ExternalID}); s.AccountStatus != "deleted" {
		t.Errorf("after delete: %+v", s)
	}
	bad := NewWithBase(srv.URL, "wrong", srv.Client())
	if err := bad.Health(ctx); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Errorf("bad token: %v", err)
	}
}

func TestNewValidatesDomain(t *testing.T) {
	if _, err := New(connector.Credentials{APIKey: "x", Extra: map[string]string{"domain": "acme.okta.com/evil"}}); err == nil {
		t.Error("path in domain accepted")
	}
	if _, err := New(connector.Credentials{APIKey: "x"}); err == nil {
		t.Error("missing domain accepted")
	}
	c, err := New(connector.Credentials{APIKey: "x", Extra: map[string]string{"domain": "https://acme.okta.com"}})
	if err != nil || c.(*Connector).base != "https://acme.okta.com" {
		t.Errorf("%v", err)
	}
}
