package ratelimit

import (
	"testing"
	"time"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		LoginRate:   5,
		LoginWindow: 15 * time.Minute,
		APIRate:     100,
		APIWindow:   1 * time.Minute,
	}

	if cfg.LoginRate != 5 {
		t.Fatalf("expected login rate 5, got %d", cfg.LoginRate)
	}
	if cfg.APIRate != 100 {
		t.Fatalf("expected API rate 100, got %d", cfg.APIRate)
	}
	if !cfg.Enabled {
		t.Fatal("expected enabled=true")
	}
}

func TestNew(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		LoginRate:   5,
		LoginWindow: 15 * time.Minute,
		APIRate:     100,
		APIWindow:   1 * time.Minute,
	}

	limiter := New(nil, cfg)
	if limiter == nil {
		t.Fatal("expected non-nil limiter")
	}
	if limiter.config.LoginRate != 5 {
		t.Fatalf("expected login rate 5, got %d", limiter.config.LoginRate)
	}
}
