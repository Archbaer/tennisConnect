package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tennisconnect/internal/httpapi"
)

func normalizeEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > 254 {
		return "", errors.New("invalid email")
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address != trimmed {
		return "", errors.New("invalid email")
	}
	separator := strings.LastIndex(address.Address, "@")
	if separator < 1 || separator == len(address.Address)-1 {
		return "", errors.New("invalid email")
	}
	// SMTP local parts can be case-sensitive; only the domain is case-insensitive.
	return address.Address[:separator] + "@" + strings.ToLower(address.Address[separator+1:]), nil
}

func isAdult(dob, now time.Time) bool {
	if dob.After(now) {
		return false
	}
	age := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		age--
	}
	return age >= 18
}

func newToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, errors.New("could not generate token")
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, sha256.Sum256([]byte(token)), nil
}

func encryptEmail(email string, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("email encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("could not configure email encryption")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("could not configure email encryption")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, errors.New("could not encrypt email")
	}
	ciphertext := append([]byte{}, nonce...)
	return gcm.Seal(ciphertext, nonce, []byte(email), []byte("email-v1")), nil
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		forwarded := r.Header.Get("X-Forwarded-For")
		parts := strings.Split(forwarded, ",")
		// The trusted proxy appends the rightmost address; earlier values are client-controlled.
		if address, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
			return address.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return ""
	}
	return address.String()
}

type Service struct {
	DB                 *sql.DB
	AppBaseURL         string
	EmailEncryptionKey []byte
	EmailHMACKey       []byte
	CookieSecure       bool
	TrustProxy         bool
	SendMagicLink      func(context.Context, string, string) error
}

func (a *Service) lookupHMAC(email string) []byte {
	mac := hmac.New(sha256.New, a.EmailHMACKey)
	mac.Write([]byte("email-lookup-v1\x00" + email))
	return mac.Sum(nil)
}

func (a *Service) rateLimitHMAC(kind, identifier string) []byte {
	mac := hmac.New(sha256.New, a.EmailHMACKey)
	mac.Write([]byte("auth-rate-limit-v1\x00" + kind + "\x00" + identifier))
	return mac.Sum(nil)
}

func (a *Service) allowAuthAttempt(ctx context.Context, kind, identifier string, limit int, now time.Time) (bool, int, error) {
	windowStart := now.UTC().Truncate(time.Hour)
	var attempts int
	err := a.DB.QueryRowContext(ctx, `
		INSERT INTO auth_rate_limits (key_hash, window_start, attempts)
		VALUES ($1, $2, 1)
		ON CONFLICT (key_hash, window_start)
		DO UPDATE SET attempts = auth_rate_limits.attempts + 1
		RETURNING attempts`, a.rateLimitHMAC(kind, identifier), windowStart).Scan(&attempts)
	if err != nil {
		return false, 0, err
	}
	retryAfter := int(windowStart.Add(time.Hour).Sub(now).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return attempts <= limit, retryAfter, nil
}

func (a *Service) limitIP(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	ip := clientIP(r, a.TrustProxy)
	if ip == "" {
		ip = "unknown"
	}
	allowed, retryAfter, err := a.allowAuthAttempt(ctx, "ip", ip, 20, time.Now().UTC())
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		httpapi.WriteError(w, http.StatusTooManyRequests, "too many requests")
	}
	return allowed
}

func (a *Service) CleanupExpired(ctx context.Context) error {
	for _, query := range []string{
		"DELETE FROM login_tokens WHERE expires_at <= now()",
		"DELETE FROM sessions WHERE expires_at <= now()",
		"DELETE FROM auth_rate_limits WHERE window_start < now() - interval '2 hours'",
	} {
		if _, err := a.DB.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

func (a *Service) RequestLink(w http.ResponseWriter, r *http.Request) {
	if !a.validOrigin(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var input struct {
		Email string `json:"email"`
	}
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if !a.limitIP(ctx, w, r) {
		return
	}
	now := time.Now().UTC()
	allowed, retryAfter, err := a.allowAuthAttempt(ctx, "email", email, 5, now)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		httpapi.WriteError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	token, hash, err := newToken()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	encrypted, err := encryptEmail(email, a.EmailEncryptionKey)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	_, err = a.DB.ExecContext(ctx, `
		INSERT INTO login_tokens (token_hash, email_encrypted, email_lookup_hmac, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email_lookup_hmac) DO UPDATE SET
			token_hash = EXCLUDED.token_hash,
			email_encrypted = EXCLUDED.email_encrypted,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at`, hash[:], encrypted, a.lookupHMAC(email), now.Add(15*time.Minute))
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	magicLink := a.AppBaseURL + "/auth/verify?token=" + url.QueryEscape(token)
	if err := a.SendMagicLink(ctx, email, magicLink); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "sign-in email unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "check your email"})
}

func (a *Service) validOrigin(r *http.Request) bool {
	return httpapi.OriginAllowed(r, a.AppBaseURL)
}
