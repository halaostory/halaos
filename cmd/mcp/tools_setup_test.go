package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallCLIEndpoint_Register(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/cli-register" {
			t.Errorf("expected path /api/v1/auth/cli-register, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no Authorization header for public endpoint")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"token":          "test-jwt",
				"api_key":        "halaos_test1234567890abcdef1234567890abcdef1234",
				"api_key_prefix": "halaos_test12",
			},
		})
	}))
	defer srv.Close()

	resp, err := callCLIEndpoint(srv.URL, "cli-register", map[string]interface{}{
		"email":    "test@example.com",
		"password": "TestPass123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp["success"].(bool) {
		t.Fatal("expected success=true")
	}
}

func TestCallCLIEndpoint_LoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/cli-login" {
			t.Errorf("expected path /api/v1/auth/cli-login, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   map[string]interface{}{"code": "invalid_credentials", "message": "Invalid email or password"},
		})
	}))
	defer srv.Close()

	_, err := callCLIEndpoint(srv.URL, "cli-login", map[string]interface{}{
		"email":    "test@example.com",
		"password": "wrong",
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if err.Error() != "Invalid email or password (HTTP 401)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCallCLIEndpoint_ConflictError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   map[string]interface{}{"code": "email_exists", "message": "Email already registered. Use cli-login instead."},
		})
	}))
	defer srv.Close()

	_, err := callCLIEndpoint(srv.URL, "cli-register", map[string]interface{}{
		"email":    "existing@example.com",
		"password": "TestPass123",
	})
	if err == nil {
		t.Fatal("expected error for 409 response")
	}
}
