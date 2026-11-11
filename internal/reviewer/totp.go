package reviewer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tennisconnect/internal/httpapi"
)

const totpPeriod = 30

var base32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

func totpKey(emailKey []byte) []byte {
	mac := hmac.New(sha256.New, emailKey)
	mac.Write([]byte("reviewer-totp-key-v1"))
	return mac.Sum(nil)
}

func encryptSecret(secret string, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(totpKey(key))
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
	return gcm.Seal(nonce, nonce, []byte(secret), []byte("reviewer-totp-v1")), nil
}

func decryptSecret(encrypted, key []byte) (string, error) {
	block, err := aes.NewCipher(totpKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(encrypted) < gcm.NonceSize() {
		return "", errors.New("invalid secret")
	}
	plain, err := gcm.Open(nil, encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():], []byte("reviewer-totp-v1"))
	if err != nil {
		return "", errors.New("invalid secret")
	}
	return string(plain), nil
}

func newTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32NoPadding.EncodeToString(raw), nil
}

func totpCode(secret string, step int64) (string, error) {
	key, err := base32NoPadding.DecodeString(secret)
	if err != nil {
		return "", errors.New("invalid secret")
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return strconv.FormatUint(uint64(value%1_000_000), 10), nil
}

// validateTOTP accepts the current step and one step either side. It returns
// the matched step so the caller can reject replay.
func validateTOTP(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	step := now.Unix() / totpPeriod
	for _, candidate := range []int64{step - 1, step, step + 1} {
		if candidate <= lastStep {
			continue
		}
		expected, err := totpCode(secret, candidate)
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return candidate, true
		}
	}
	return 0, false
}

// EnrollMFA starts TOTP enrollment and returns the secret once.
func (s *Service) EnrollMFA(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	reviewerID, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	var enrolledAt sql.NullTime
	if err := s.DB.QueryRowContext(ctx, "SELECT mfa_enrolled_at FROM reviewers WHERE id = $1", reviewerID).Scan(&enrolledAt); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if enrolledAt.Valid {
		httpapi.WriteError(w, http.StatusConflict, "mfa already enrolled")
		return
	}
	secret, err := newTOTPSecret()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	encrypted, err := encryptSecret(secret, s.EmailEncryptionKey)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	if _, err := s.DB.ExecContext(ctx, "UPDATE reviewers SET totp_secret_encrypted = $1 WHERE id = $2",
		encrypted, reviewerID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	label := url.QueryEscape("Tennis Connect:reviewer-" + reviewerID[:8])
	uri := "otpauth://totp/" + label + "?secret=" + secret + "&issuer=Tennis+Connect&digits=6&period=30"
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"secret": secret, "otpauth_uri": uri})
}

// VerifyMFA confirms enrollment or elevates an enrolled session.
func (s *Service) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input.Code = strings.TrimSpace(input.Code)
	reviewerID, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	if allowed, retry, err := s.allowAttempt(ctx, "mfa", reviewerID, 5, time.Minute); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	} else if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		httpapi.WriteError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	var encrypted []byte
	var enrolledAt sql.NullTime
	var lastStep int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT totp_secret_encrypted, mfa_enrolled_at, last_totp_step
		FROM reviewers WHERE id = $1`, reviewerID).Scan(&encrypted, &enrolledAt, &lastStep)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if len(encrypted) == 0 {
		httpapi.WriteError(w, http.StatusConflict, "enroll mfa first")
		return
	}
	secret, err := decryptSecret(encrypted, s.EmailEncryptionKey)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	step, valid := validateTOTP(secret, input.Code, time.Now().UTC(), lastStep)
	if !valid {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	updated, err := tx.ExecContext(ctx, `
		UPDATE reviewers SET last_totp_step = $1,
			mfa_enrolled_at = COALESCE(mfa_enrolled_at, now())
		WHERE id = $2 AND last_totp_step < $1`, step, reviewerID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if affected, err := updated.RowsAffected(); err != nil || affected != 1 {
		// A concurrent request consumed this time step first.
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE reviewer_sessions SET mfa_at = now()
		WHERE token_hash = $1`, tokenHash(cookie.Value)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "mfa verified"})
}
