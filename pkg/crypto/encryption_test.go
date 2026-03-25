package crypto

import (
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := GenerateKey()
	plaintext := "123-45-6789"

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if got != plaintext {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestEncrypt_DifferentNonce(t *testing.T) {
	key := GenerateKey()
	plaintext := "123-45-6789"

	ct1, _ := Encrypt(key, plaintext)
	ct2, _ := Encrypt(key, plaintext)

	if string(ct1) == string(ct2) {
		t.Error("expected different ciphertexts due to random nonce")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := GenerateKey()
	key2 := GenerateKey()
	plaintext := "123-45-6789"

	ciphertext, _ := Encrypt(key1, plaintext)
	_, err := Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := GenerateKey()
	ciphertext, _ := Encrypt(key, "123-45-6789")

	ciphertext[len(ciphertext)-1] ^= 0xFF // flip last byte
	_, err := Decrypt(key, ciphertext)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestMaskSSN(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123-45-6789", "XXX-XX-6789"},
		{"123456789", "XXX-XX-6789"},
		{"12345", "XXX-XX-2345"},
		{"", "XXX-XX-XXXX"},
	}
	for _, tt := range tests {
		got := MaskSSN(tt.input)
		if got != tt.want {
			t.Errorf("MaskSSN(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
