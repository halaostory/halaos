package config

import (
	"os"
	"testing"
	"time"
)

func TestPostgresConfig_DSN(t *testing.T) {
	p := PostgresConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "myuser",
		Password: "mypass",
		DB:       "mydb",
		SSLMode:  "disable",
	}

	expected := "postgres://myuser:mypass@localhost:5432/mydb?sslmode=disable"
	if got := p.DSN(); got != expected {
		t.Errorf("DSN() = %q, want %q", got, expected)
	}
}

func TestPostgresConfig_SafeDSN(t *testing.T) {
	p := PostgresConfig{
		Host:     "db.example.com",
		Port:     "5432",
		User:     "admin",
		Password: "s3cr3t",
		DB:       "production",
		SSLMode:  "require",
	}

	got := p.SafeDSN()
	if got != "postgres://admin:***@db.example.com:5432/production?sslmode=require" {
		t.Errorf("SafeDSN() = %q", got)
	}
	// Password should NOT appear in safe DSN
	if containsStr(got, "s3cr3t") {
		t.Error("SafeDSN contains the actual password")
	}
}

func TestGetEnv(t *testing.T) {
	// Set a test env var
	os.Setenv("TEST_CONFIG_VAR", "hello")
	defer os.Unsetenv("TEST_CONFIG_VAR")

	if got := getEnv("TEST_CONFIG_VAR", "default"); got != "hello" {
		t.Errorf("getEnv existing = %q, want %q", got, "hello")
	}
	if got := getEnv("TEST_CONFIG_MISSING", "fallback"); got != "fallback" {
		t.Errorf("getEnv missing = %q, want %q", got, "fallback")
	}
}

func TestGetEnvInt64(t *testing.T) {
	os.Setenv("TEST_INT64", "42")
	defer os.Unsetenv("TEST_INT64")

	if got := getEnvInt64("TEST_INT64", 0); got != 42 {
		t.Errorf("getEnvInt64 = %d, want 42", got)
	}
	if got := getEnvInt64("TEST_MISSING_INT", 99); got != 99 {
		t.Errorf("getEnvInt64 missing = %d, want 99", got)
	}

	// Invalid value should return fallback
	os.Setenv("TEST_INT64_BAD", "not-a-number")
	defer os.Unsetenv("TEST_INT64_BAD")
	if got := getEnvInt64("TEST_INT64_BAD", 77); got != 77 {
		t.Errorf("getEnvInt64 invalid = %d, want 77", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")
	os.Setenv("TEST_BOOL_BAD", "maybe")
	defer func() {
		os.Unsetenv("TEST_BOOL_TRUE")
		os.Unsetenv("TEST_BOOL_FALSE")
		os.Unsetenv("TEST_BOOL_BAD")
	}()

	if got := getEnvBool("TEST_BOOL_TRUE", false); !got {
		t.Error("getEnvBool true = false")
	}
	if got := getEnvBool("TEST_BOOL_FALSE", true); got {
		t.Error("getEnvBool false = true")
	}
	if got := getEnvBool("TEST_BOOL_BAD", true); !got {
		t.Error("getEnvBool invalid should return fallback true")
	}
	if got := getEnvBool("TEST_MISSING_BOOL", true); !got {
		t.Error("getEnvBool missing should return fallback true")
	}
}

func TestGetEnvDuration(t *testing.T) {
	os.Setenv("TEST_DUR", "5m")
	defer os.Unsetenv("TEST_DUR")

	if got := getEnvDuration("TEST_DUR", time.Hour); got != 5*time.Minute {
		t.Errorf("getEnvDuration = %v, want 5m", got)
	}
	if got := getEnvDuration("TEST_MISSING_DUR", time.Hour); got != time.Hour {
		t.Errorf("getEnvDuration missing = %v, want 1h", got)
	}

	os.Setenv("TEST_DUR_BAD", "invalid")
	defer os.Unsetenv("TEST_DUR_BAD")
	if got := getEnvDuration("TEST_DUR_BAD", 30*time.Second); got != 30*time.Second {
		t.Errorf("getEnvDuration invalid = %v, want 30s", got)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Unset common env vars to test defaults
	vars := []string{"SERVER_HOST", "SERVER_PORT", "POSTGRES_HOST", "POSTGRES_PORT"}
	saved := make(map[string]string)
	for _, v := range vars {
		saved[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	cfg := Load()

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("server host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != "8080" {
		t.Errorf("server port = %q, want 8080", cfg.Server.Port)
	}
	if cfg.RateLimit.Enabled != true {
		t.Error("rate limit should be enabled by default")
	}
	if cfg.Billing.FreeTierTokens != 1000 {
		t.Errorf("free tier tokens = %d, want 1000", cfg.Billing.FreeTierTokens)
	}
}

func TestValidate_MissingJWTSecret(t *testing.T) {
	cfg := &Config{
		JWT:      JWTConfig{Secret: ""},
		Postgres: PostgresConfig{Password: "somepass"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("validate with empty JWT_SECRET should fail")
	}
}

func TestValidate_MissingDBPassword(t *testing.T) {
	cfg := &Config{
		JWT:      JWTConfig{Secret: "some-secret"},
		Postgres: PostgresConfig{Password: ""},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("validate with empty POSTGRES_PASSWORD should fail")
	}
}

func TestValidate_AllSet(t *testing.T) {
	cfg := &Config{
		JWT:      JWTConfig{Secret: "test-secret-at-least-32-chars!!"},
		Postgres: PostgresConfig{Password: "testpass"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("validate with all required fields should pass: %v", err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
