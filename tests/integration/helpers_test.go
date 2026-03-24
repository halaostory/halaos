package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var retryCount int

// doRequest performs an HTTP request with optional auth token and returns raw body + status.
func doRequest(method, reqURL, token string, body map[string]any) (json.RawMessage, int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	// Rate limit retry (max 3 times)
	if resp.StatusCode == 429 {
		retryCount++
		if retryCount > 3 {
			retryCount = 0
			return json.RawMessage(respBody), resp.StatusCode, fmt.Errorf("rate limited after 3 retries")
		}
		wait := 2
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if w, err := strconv.Atoi(ra); err == nil {
				wait = w
			}
		}
		time.Sleep(time.Duration(wait) * time.Second)
		return doRequest(method, reqURL, token, body)
	}
	retryCount = 0

	return json.RawMessage(respBody), resp.StatusCode, nil
}

func doPost(reqURL, token string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPost, reqURL, token, body)
}

// apiGet performs GET on /api/v1{path} with admin auth.
func apiGet(path string, query map[string]string) (json.RawMessage, int, error) {
	reqURL := baseURL + "/api/v1" + path
	if len(query) > 0 {
		params := url.Values{}
		for k, v := range query {
			if v != "" {
				params.Set(k, v)
			}
		}
		if encoded := params.Encode(); encoded != "" {
			reqURL += "?" + encoded
		}
	}
	return doRequest(http.MethodGet, reqURL, authToken, nil)
}

// apiPost performs POST on /api/v1{path} with admin auth.
func apiPost(path string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPost, baseURL+"/api/v1"+path, authToken, body)
}

// apiPut performs PUT on /api/v1{path} with admin auth.
func apiPut(path string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPut, baseURL+"/api/v1"+path, authToken, body)
}

// apiGetAs performs GET with a specific token (for employee-context calls).
func apiGetAs(token, path string, query map[string]string) (json.RawMessage, int, error) {
	reqURL := baseURL + "/api/v1" + path
	if len(query) > 0 {
		params := url.Values{}
		for k, v := range query {
			if v != "" {
				params.Set(k, v)
			}
		}
		if encoded := params.Encode(); encoded != "" {
			reqURL += "?" + encoded
		}
	}
	return doRequest(http.MethodGet, reqURL, token, nil)
}

// apiPostAs performs POST with a specific token.
func apiPostAs(token, path string, body map[string]any) (json.RawMessage, int, error) {
	return doRequest(http.MethodPost, baseURL+"/api/v1"+path, token, body)
}

// requireSuccess asserts HTTP 200 and success:true in the response envelope.
func requireSuccess(t *testing.T, resp json.RawMessage, status int) {
	t.Helper()
	require.Equal(t, 200, status, "expected HTTP 200, got %d: %s", status, string(resp))
	var envelope struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	require.True(t, envelope.Success, "expected success:true, body: %s", string(resp))
}

// requireCreated asserts HTTP 201 and success:true.
func requireCreated(t *testing.T, resp json.RawMessage, status int) {
	t.Helper()
	require.Equal(t, 201, status, "expected HTTP 201, got %d: %s", status, string(resp))
	var envelope struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	require.True(t, envelope.Success, "expected success:true, body: %s", string(resp))
}

// extractID extracts data.id from the standard response envelope.
func extractID(t *testing.T, resp json.RawMessage) int64 {
	t.Helper()
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	require.NotZero(t, envelope.Data.ID, "expected non-zero id")
	return envelope.Data.ID
}

// extractList extracts data as []any from the response.
func extractList(t *testing.T, resp json.RawMessage) []any {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	var list []any
	require.NoError(t, json.Unmarshal(envelope.Data, &list))
	return list
}

// extractData extracts the data field as raw JSON.
func extractData(t *testing.T, resp json.RawMessage) json.RawMessage {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp, &envelope))
	return envelope.Data
}

// PH name lists for test data generation.
var phFirstNames = []string{
	"Juan", "Maria", "Carlo", "Ana", "Jose", "Rosa",
	"Miguel", "Sofia", "Paolo", "Isabella", "Marco",
	"Gabriela", "Rafael", "Camille", "Antonio",
	"Patricia", "Gabriel", "Beatriz", "Francisco", "Elena",
	"Ricardo", "Luisa", "Diego", "Carmen", "Fernando",
	"Teresa", "Andres", "Victoria", "Eduardo", "Rosario",
	"Alejandro", "Catalina", "Roberto", "Dolores", "Enrique",
	"Mercedes", "Alfonso", "Pilar", "Sergio", "Esperanza",
	"Bernardo", "Soledad", "Vicente", "Consuelo", "Arturo",
	"Remedios", "Ignacio", "Amparo", "Raul", "Trinidad",
}

var phLastNames = []string{
	"Santos", "Reyes", "Cruz", "Bautista", "Gonzales",
	"Garcia", "Mendoza", "Torres", "Villanueva", "Ramos",
	"Aquino", "Dela Cruz", "Fernandez", "Castillo", "Rivera",
	"Lopez", "Morales", "Navarro", "Flores", "Perez",
}

// randomEmployee generates a unique employee payload.
func randomEmployee(seq int) map[string]any {
	ts := time.Now().UnixMilli()
	fn := phFirstNames[seq%len(phFirstNames)]
	ln := phLastNames[seq%len(phLastNames)]
	empType := "regular"
	if seq%10 >= 7 && seq%10 < 9 {
		empType = "probationary"
	} else if seq%10 == 9 {
		empType = "contractual"
	}

	year := 2024 + (seq % 3)
	month := (seq%12) + 1
	day := (seq%28) + 1

	return map[string]any{
		"employee_no":     fmt.Sprintf("I%d%02d", ts%1000000000, seq),
		"first_name":      fn,
		"last_name":       ln,
		"email":           fmt.Sprintf("int.%s.%s.%d.%d@test.halaos.com", fn, ln, seq, ts),
		"phone":           fmt.Sprintf("0917%07d", 1000000+seq),
		"birth_date":      fmt.Sprintf("%d-%02d-%02d", 1985+(seq%15), month, day),
		"gender":          []string{"male", "female"}[seq%2],
		"civil_status":    []string{"single", "married", "single", "married"}[seq%4],
		"nationality":     "Filipino",
		"hire_date":       fmt.Sprintf("%d-%02d-%02d", year, month, day),
		"employment_type": empType,
	}
}
