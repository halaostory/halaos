package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey_Format(t *testing.T) {
	key := generateAPIKey()
	if !strings.HasPrefix(key, "halaos_") {
		t.Errorf("key should start with halaos_, got %q", key[:7])
	}
	if len(key) != 47 {
		t.Errorf("key should be 47 chars (halaos_ + 40 hex), got %d", len(key))
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := generateAPIKey()
		if seen[key] {
			t.Fatalf("duplicate key generated on iteration %d", i)
		}
		seen[key] = true
	}
}

func TestAPIKeyPrefix(t *testing.T) {
	key := "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	prefix := apiKeyPrefix(key)
	if prefix != "halaos_a1b2c3" {
		t.Errorf("expected halaos_a1b2c3, got %q", prefix)
	}
}

func TestAPIKeyPrefix_ShortKey(t *testing.T) {
	prefix := apiKeyPrefix("short")
	if prefix != "short" {
		t.Errorf("expected short, got %q", prefix)
	}
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	key := "halaos_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	h1 := hashAPIKey(key)
	h2 := hashAPIKey(key)
	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(h1))
	}
}

func TestHashAPIKey_DifferentKeys(t *testing.T) {
	h1 := hashAPIKey("halaos_aaaa")
	h2 := hashAPIKey("halaos_bbbb")
	if h1 == h2 {
		t.Error("different keys should produce different hashes")
	}
}
