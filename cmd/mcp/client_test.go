package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/employees", r.URL.Path)
		assert.Equal(t, "Bearer halaos_testkey123", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "active", r.URL.Query().Get("status"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []map[string]any{{"id": 1, "name": "Test"}},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "halaos_testkey123")
	data, err := c.Get("/employees", map[string]string{
		"status": "active",
		"page":   "1",
	})
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.True(t, resp["success"].(bool))
}

func TestClient_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/attendance/clock-in", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Working from home", body["notes"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"clock_in": "2026-03-24T09:00:00Z"},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "halaos_testkey123")
	data, err := c.Post("/attendance/clock-in", map[string]any{
		"notes": "Working from home",
	})
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.True(t, resp["success"].(bool))
}

func TestClient_Get_EmptyQueryValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty values should not be included in query
		assert.Empty(t, r.URL.Query().Get("status"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "halaos_testkey123")
	_, err := c.Get("/employees", map[string]string{
		"status": "",
		"page":   "1",
	})
	require.NoError(t, err)
}

func TestClient_Get_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"message":"invalid api key"}}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "halaos_badkey")
	_, err := c.Get("/employees", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error (HTTP 401)")
	assert.Contains(t, err.Error(), "invalid api key")
}

func TestClient_Post_NilBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "halaos_testkey123")
	_, err := c.Post("/attendance/clock-in", nil)
	require.NoError(t, err)
}

func TestClient_TrailingSlash(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/employees", r.URL.Path)
		w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()

	// baseURL with trailing slash should be trimmed
	c := NewClient(ts.URL+"/", "halaos_testkey123")
	_, err := c.Get("/employees", nil)
	require.NoError(t, err)
}
