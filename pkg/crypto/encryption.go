package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
)

// GenerateKey creates a random 32-byte AES-256 key.
func GenerateKey() []byte {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return key
}

// Encrypt encrypts plaintext using AES-256-GCM. Returns nonce+ciphertext.
func Encrypt(key []byte, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt decrypts AES-256-GCM ciphertext (nonce prepended).
func Decrypt(key []byte, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// MaskSSN returns "XXX-XX-1234" format showing only the last 4 digits.
func MaskSSN(ssn string) string {
	digits := strings.ReplaceAll(strings.ReplaceAll(ssn, "-", ""), " ", "")
	if len(digits) < 4 {
		return "XXX-XX-XXXX"
	}
	return "XXX-XX-" + digits[len(digits)-4:]
}
