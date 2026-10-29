package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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

func loadAuthConfig(cfg *config) error {
	baseURL, err := url.Parse(os.Getenv("APP_BASE_URL"))
	if err != nil || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" ||
		baseURL.Fragment != "" || baseURL.ForceQuery || (baseURL.Path != "" && baseURL.Path != "/") ||
		(baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return errors.New("APP_BASE_URL must be an HTTP(S) origin without credentials or path")
	}
	cfg.appBaseURL = strings.TrimSuffix(baseURL.String(), "/")

	cfg.emailFrom = strings.TrimSpace(os.Getenv("EMAIL_FROM"))
	address, err := mail.ParseAddress(cfg.emailFrom)
	if err != nil || address.Address == "" || len(cfg.emailFrom) > 320 {
		return errors.New("EMAIL_FROM must be a valid sender address")
	}
	cfg.resendAPIKey = strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	if cfg.resendAPIKey == "" {
		return errors.New("RESEND_API_KEY is required")
	}

	if cfg.emailEncryptionKey, err = decodeAuthKey("EMAIL_ENCRYPTION_KEY"); err != nil {
		return err
	}
	if cfg.emailHMACKey, err = decodeAuthKey("EMAIL_HMAC_KEY"); err != nil {
		return err
	}
	if cfg.cookieSecure, err = authBoolEnv("COOKIE_SECURE", true); err != nil {
		return err
	}
	if cfg.trustProxy, err = authBoolEnv("TRUST_PROXY_HEADERS", false); err != nil {
		return err
	}
	return nil
}

func decodeAuthKey(name string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv(name))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be a base64-encoded 32-byte key", name)
	}
	return key, nil
}

func authBoolEnv(name string, defaultValue bool) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return value, nil
}

type authService struct {
	db                 *sql.DB
	appBaseURL         string
	emailEncryptionKey []byte
	emailHMACKey       []byte
	cookieSecure       bool
	trustProxy         bool
	sendMagicLink      func(context.Context, string, string) error
}

func (a *authService) lookupHMAC(email string) []byte {
	mac := hmac.New(sha256.New, a.emailHMACKey)
	mac.Write([]byte("email-lookup-v1\x00" + email))
	return mac.Sum(nil)
}

func (a *authService) rateLimitHMAC(kind, identifier string) []byte {
	mac := hmac.New(sha256.New, a.emailHMACKey)
	mac.Write([]byte("auth-rate-limit-v1\x00" + kind + "\x00" + identifier))
	return mac.Sum(nil)
}

func (a *authService) allowAuthAttempt(ctx context.Context, kind, identifier string, limit int, now time.Time) (bool, int, error) {
	windowStart := now.UTC().Truncate(time.Hour)
	var attempts int
	err := a.db.QueryRowContext(ctx, `
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

func (a *authService) limitIP(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	ip := clientIP(r, a.trustProxy)
	if ip == "" {
		ip = "unknown"
	}
	allowed, retryAfter, err := a.allowAuthAttempt(ctx, "ip", ip, 20, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeError(w, http.StatusTooManyRequests, "too many requests")
	}
	return allowed
}

func (a *authService) cleanupExpired(ctx context.Context) error {
	for _, query := range []string{
		"DELETE FROM login_tokens WHERE expires_at <= now()",
		"DELETE FROM sessions WHERE expires_at <= now()",
		"DELETE FROM auth_rate_limits WHERE window_start < now() - interval '2 hours'",
	} {
		if _, err := a.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

func (a *authService) requestLink(w http.ResponseWriter, r *http.Request) {
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var input struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &input) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if !a.limitIP(ctx, w, r) {
		return
	}
	now := time.Now().UTC()
	allowed, retryAfter, err := a.allowAuthAttempt(ctx, "email", email, 5, now)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	token, hash, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	encrypted, err := encryptEmail(email, a.emailEncryptionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO login_tokens (token_hash, email_encrypted, email_lookup_hmac, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email_lookup_hmac) DO UPDATE SET
			token_hash = EXCLUDED.token_hash,
			email_encrypted = EXCLUDED.email_encrypted,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at`, hash[:], encrypted, a.lookupHMAC(email), now.Add(15*time.Minute))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	magicLink := a.appBaseURL + "/auth/verify?token=" + url.QueryEscape(token)
	if err := a.sendMagicLink(ctx, email, magicLink); err != nil {
		writeError(w, http.StatusServiceUnavailable, "sign-in email unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "check your email"})
}

func requestOriginAllowed(r *http.Request, expected string) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || strings.EqualFold(origin, expected)
}

func (a *authService) validOrigin(r *http.Request) bool {
	return requestOriginAllowed(r, a.appBaseURL)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
