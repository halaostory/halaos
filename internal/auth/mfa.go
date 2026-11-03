package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // TOTP (RFC 6238) uses HMAC-SHA1; authenticator apps expect it
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Two-factor sign-in with an authenticator app (TOTP, RFC 6238: 6 digits,
// 30-second steps, ±1 step of clock drift). Secrets are encrypted at rest
// with a key derived from the JWT secret; each step can be used once.
// Eight one-time recovery codes are issued when MFA is turned on.

const (
	totpStep     = 30
	totpDigits   = 6
	mfaTokenTTL  = 5 * time.Minute
	issuerName   = "HalaPeople"
	recoveryN    = 8
	recoveryBits = 5 // bytes per recovery code
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// totpAt returns the code for a time step.
func totpAt(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	m := hmac.New(sha1.New, secret)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := (binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff) % 1_000_000
	return fmt.Sprintf("%06d", v)
}

// verifyTOTP returns the matching step (now ±1) or -1.
func verifyTOTP(secret []byte, code string, now time.Time) int64 {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return -1
	}
	cur := now.Unix() / totpStep
	for _, d := range []int64{0, -1, 1} {
		if subtle.ConstantTimeCompare([]byte(totpAt(secret, cur+d)), []byte(code)) == 1 {
			return cur + d
		}
	}
	return -1
}

func (h *Handler) mfaKey() []byte {
	k := sha256.Sum256([]byte("halapeople-mfa:" + h.jwt.secret))
	return k[:]
}

func (h *Handler) sealSecret(secret []byte) ([]byte, error) {
	block, err := aes.NewCipher(h.mfaKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, secret, nil), nil
}

func (h *Handler) openSecret(sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(h.mfaKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("mfa secret corrupt")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}

// mfaToken binds the second step to a user for a few minutes after the
// password check: "<uid>.<exp>.<hmac>".
func (h *Handler) mfaToken(userID int64, now time.Time) string {
	payload := fmt.Sprintf("%d.%d", userID, now.Add(mfaTokenTTL).Unix())
	m := hmac.New(sha256.New, h.mfaKey())
	m.Write([]byte("token:" + payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (h *Handler) parseMFAToken(tok string, now time.Time) (int64, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return 0, false
	}
	m := hmac.New(sha256.New, h.mfaKey())
	m.Write([]byte("token:" + parts[0] + "." + parts[1]))
	want := base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(want), []byte(parts[2])) != 1 {
		return 0, false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > exp {
		return 0, false
	}
	uid, err := strconv.ParseInt(parts[0], 10, 64)
	return uid, err == nil
}

func hashRecovery(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))))
	return hex.EncodeToString(sum[:])
}

func newRecoveryCodes() (codes, hashes []string, err error) {
	for i := 0; i < recoveryN; i++ {
		b := make([]byte, recoveryBits)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		c := b32.EncodeToString(b) // 8 chars
		c = c[:4] + "-" + c[4:]
		codes = append(codes, c)
		hashes = append(hashes, hashRecovery(c))
	}
	return codes, hashes, nil
}

// mfaEnabled reports whether the user must pass a second factor.
func (h *Handler) mfaEnabled(ctx context.Context, userID int64) (bool, error) {
	m, err := h.queries.GetUserMFA(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return m.EnabledAt.Valid, nil
}

// checkSecondFactor accepts a current TOTP code (each step once) or an
// unused recovery code (consumed).
func (h *Handler) checkSecondFactor(ctx context.Context, m store.UserMfa, code string) bool {
	secret, err := h.openSecret(m.SecretEnc)
	if err != nil {
		h.logger.Error("mfa: open secret", "user_id", m.UserID, "error", err)
		return false
	}
	if step := verifyTOTP(secret, code, time.Now()); step >= 0 {
		n, err := h.queries.SetUserMFAStep(ctx, store.SetUserMFAStepParams{UserID: m.UserID, LastStep: step})
		return err == nil && n == 1
	}
	hc := hashRecovery(code)
	if i := slices.Index(m.RecoveryHashes, hc); i >= 0 && len(strings.TrimSpace(code)) >= 8 {
		rest := slices.Delete(slices.Clone(m.RecoveryHashes), i, i+1)
		return h.queries.SetUserMFARecovery(ctx, store.SetUserMFARecoveryParams{UserID: m.UserID, RecoveryHashes: rest}) == nil
	}
	return false
}

// MFAStatus handles GET /auth/mfa.
func (h *Handler) MFAStatus(c *gin.Context) {
	m, err := h.queries.GetUserMFA(c.Request.Context(), GetUserID(c))
	if err != nil {
		response.OK(c, gin.H{"enabled": false, "recovery_codes_left": 0})
		return
	}
	response.OK(c, gin.H{"enabled": m.EnabledAt.Valid, "recovery_codes_left": len(m.RecoveryHashes)})
}

// MFASetup handles POST /auth/mfa/setup: a new secret to scan.
func (h *Handler) MFASetup(c *gin.Context) {
	ctx := c.Request.Context()
	uid := GetUserID(c)
	if on, err := h.mfaEnabled(ctx, uid); err != nil || on {
		response.BadRequest(c, "Two-factor sign-in is already on; turn it off first to re-enrol")
		return
	}
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		response.InternalError(c, "Failed to create secret")
		return
	}
	sealed, err := h.sealSecret(secret)
	if err != nil {
		response.InternalError(c, "Failed to create secret")
		return
	}
	if err := h.queries.StartUserMFASetup(ctx, store.StartUserMFASetupParams{UserID: uid, SecretEnc: sealed}); err != nil {
		response.InternalError(c, "Failed to start setup")
		return
	}
	enc := b32.EncodeToString(secret)
	label := url.PathEscape(issuerName + ":" + GetEmail(c))
	uri := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&digits=6&period=30", label, enc, url.QueryEscape(issuerName))
	response.OK(c, gin.H{"secret": enc, "otpauth_url": uri})
}

type mfaCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// MFAEnable handles POST /auth/mfa/enable {code}: confirms the app works.
func (h *Handler) MFAEnable(c *gin.Context) {
	var req mfaCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Enter the 6-digit code from your authenticator app")
		return
	}
	ctx := c.Request.Context()
	uid := GetUserID(c)
	m, err := h.queries.GetUserMFA(ctx, uid)
	if err != nil || m.EnabledAt.Valid {
		response.BadRequest(c, "Start setup first")
		return
	}
	secret, err := h.openSecret(m.SecretEnc)
	if err != nil {
		response.InternalError(c, "Setup is corrupt; start again")
		return
	}
	step := verifyTOTP(secret, req.Code, time.Now())
	if step < 0 {
		response.BadRequest(c, "That code didn't match. Check your phone's clock and try the newest code.")
		return
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		response.InternalError(c, "Failed to create recovery codes")
		return
	}
	if err := h.queries.EnableUserMFA(ctx, store.EnableUserMFAParams{UserID: uid, LastStep: step, RecoveryHashes: hashes}); err != nil {
		response.InternalError(c, "Failed to turn on two-factor sign-in")
		return
	}
	response.OK(c, gin.H{"enabled": true, "recovery_codes": codes})
}

// MFADisable handles POST /auth/mfa/disable {code}.
func (h *Handler) MFADisable(c *gin.Context) {
	var req mfaCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Enter a code from your authenticator app or a recovery code")
		return
	}
	ctx := c.Request.Context()
	uid := GetUserID(c)
	m, err := h.queries.GetUserMFA(ctx, uid)
	if err != nil {
		response.OK(c, gin.H{"enabled": false})
		return
	}
	if m.EnabledAt.Valid && !h.checkSecondFactor(ctx, m, req.Code) {
		response.Error(c, http.StatusUnauthorized, "invalid_mfa_code", "That code didn't match")
		return
	}
	if err := h.queries.DeleteUserMFA(ctx, uid); err != nil {
		response.InternalError(c, "Failed to turn off two-factor sign-in")
		return
	}
	response.OK(c, gin.H{"enabled": false})
}

type mfaVerifyRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// MFAVerify handles POST /auth/mfa/verify: the second sign-in step.
func (h *Handler) MFAVerify(c *gin.Context) {
	var req mfaVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "mfa_token and code are required")
		return
	}
	uid, ok := h.parseMFAToken(req.MFAToken, time.Now())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "mfa_expired", "Sign-in expired; enter your password again")
		return
	}
	ctx := c.Request.Context()
	m, err := h.queries.GetUserMFA(ctx, uid)
	if err != nil || !m.EnabledAt.Valid || !h.checkSecondFactor(ctx, m, req.Code) {
		response.Error(c, http.StatusUnauthorized, "invalid_mfa_code", "That code didn't match")
		return
	}
	user, err := h.queries.GetUserByID(ctx, uid)
	if err != nil || user.Status != "active" {
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Account not available")
		return
	}
	h.completeLogin(c, user)
}
