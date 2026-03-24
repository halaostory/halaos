package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

const apiKeyPrefixStr = "halaos_"

// generateAPIKey returns "halaos_" + 40 random hex chars (20 bytes).
func generateAPIKey() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return apiKeyPrefixStr + hex.EncodeToString(b)
}

// apiKeyPrefix returns the first 13 chars for display (e.g. "halaos_a1b2c3").
func apiKeyPrefix(key string) string {
	if len(key) <= 13 {
		return key
	}
	return key[:13]
}

// hashAPIKey returns the SHA-256 hex digest of the key.
func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
