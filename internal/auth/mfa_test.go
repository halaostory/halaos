package auth

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// RFC 6238 Appendix B test vectors (SHA1, 8 digits → last 6 digits).
func TestTOTP_RFC6238(t *testing.T) {
	secret := []byte("12345678901234567890")
	for ts, want8 := range map[int64]string{59: "94287082", 1111111109: "07081804", 1234567890: "89005924", 2000000000: "69279037"} {
		if got := totpAt(secret, ts/30); got != want8[2:] {
			t.Errorf("t=%d got %s want %s", ts, got, want8[2:])
		}
	}
}

func TestVerifyTOTP_Window(t *testing.T) {
	secret, _ := hex.DecodeString("3132333435363738393031323334353637383930")
	now := time.Unix(1111111109, 0)
	cur := now.Unix() / 30
	if verifyTOTP(secret, totpAt(secret, cur), now) != cur {
		t.Error("current step")
	}
	if verifyTOTP(secret, totpAt(secret, cur-1), now) != cur-1 {
		t.Error("previous step (clock drift)")
	}
	if verifyTOTP(secret, totpAt(secret, cur-3), now) != -1 {
		t.Error("old code must fail")
	}
	if verifyTOTP(secret, "12345", now) != -1 {
		t.Error("short code")
	}
}

func TestMFATokenAndSecretSealing(t *testing.T) {
	h := &Handler{jwt: NewJWTService("test-secret-0123456789abcdef", time.Hour, time.Hour)}
	now := time.Now()
	tok := h.mfaToken(42, now)
	if uid, ok := h.parseMFAToken(tok, now); !ok || uid != 42 {
		t.Fatal("valid token rejected")
	}
	if _, ok := h.parseMFAToken(tok, now.Add(6*time.Minute)); ok {
		t.Error("expired token accepted")
	}
	if _, ok := h.parseMFAToken(strings.Replace(tok, "42.", "43.", 1), now); ok {
		t.Error("tampered token accepted")
	}
	sealed, err := h.sealSecret([]byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := h.openSecret(sealed); err != nil || string(got) != "abc" {
		t.Fatalf("%s %v", got, err)
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := newRecoveryCodes()
	if err != nil || len(codes) != 8 || len(hashes) != 8 {
		t.Fatal(err)
	}
	if len(codes[0]) != 9 || codes[0][4] != '-' {
		t.Errorf("format %q", codes[0])
	}
	if hashRecovery(strings.ToLower(strings.ReplaceAll(codes[0], "-", ""))) != hashes[0] {
		t.Error("recovery codes must be case/dash insensitive")
	}
}
