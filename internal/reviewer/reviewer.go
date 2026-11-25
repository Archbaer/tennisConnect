// Package reviewer authenticates the separate human review accounts used for
// safety reports. Reviewers never hold player profiles and every review action
// requires a session that passed TOTP MFA.
package reviewer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"tennisconnect/internal/auth"
	"tennisconnect/internal/httpapi"
)

const (
	CookieName    = "tennisconnect_reviewer"
	sessionLife   = 8 * time.Hour
	tokenLife     = 15 * time.Minute
	requestBudget = 5 * time.Second
)

var verifyPage = template.Must(template.New("verify").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Reviewer sign in</title></head>
<body><main><h1>Reviewer sign in</h1>
<form method="post" action="/api/v1/reviewer/auth/verify">
<input type="hidden" name="token" value="{{.Token}}">
<input type="hidden" name="redirect" value="/reviewer">
<button type="submit">Continue</button>
</form></main></body></html>`))

// Service handles reviewer authentication and MFA.
type Service struct {
	DB                 *sql.DB
	AppBaseURL         string
	EmailEncryptionKey []byte
	EmailHMACKey       []byte
	CookieSecure       bool
	TrustProxy         bool
	SendMagicLink      func(context.Context, string, string) error
}

type tokenPair struct {
	token string
	hash  [32]byte
}

func newTokenPair() (tokenPair, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return tokenPair{}, errors.New("could not generate token")
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return tokenPair{token: token, hash: sha256.Sum256([]byte(token))}, nil
}

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func validToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32
}

func (s *Service) allowAttempt(ctx context.Context, kind, identifier string, limit int, window time.Duration) (bool, int, error) {
	now := time.Now().UTC()
	windowStart := now.Truncate(window)
	var attempts int
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO reviewer_rate_limits (key_hash, window_start, attempts)
		VALUES ($1, $2, 1)
		ON CONFLICT (key_hash, window_start)
		DO UPDATE SET attempts = reviewer_rate_limits.attempts + 1
		RETURNING attempts`, s.limitHMAC(kind, identifier), windowStart).Scan(&attempts)
	if err != nil {
		return false, 0, err
	}
	retryAfter := int(windowStart.Add(window).Sub(now).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return attempts <= limit, retryAfter, nil
}

func (s *Service) limitHMAC(kind, identifier string) []byte {
	return auth.EmailLookupHMAC("reviewer-rate-limit\x00"+kind+"\x00"+identifier, s.EmailHMACKey)
}

// RequestLink sends a reviewer sign-in link. The response is identical for
// known and unknown addresses so the endpoint cannot enumerate reviewers.
func (s *Service) RequestLink(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	email, err := auth.NormalizeEmail(input.Email)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	ip := auth.ClientIP(r, s.TrustProxy)
	if ip == "" {
		ip = "unknown"
	}
	if allowed, retry, err := s.allowAttempt(ctx, "ip", ip, 20, time.Hour); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	} else if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		httpapi.WriteError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	if allowed, retry, err := s.allowAttempt(ctx, "email", email, 5, time.Hour); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	} else if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		httpapi.WriteError(w, http.StatusTooManyRequests, "too many requests")
		return
	}

	var reviewerID string
	err = s.DB.QueryRowContext(ctx, "SELECT id FROM reviewers WHERE email_lookup_hmac = $1",
		auth.EmailLookupHMAC(email, s.EmailHMACKey)).Scan(&reviewerID)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "check your email"})
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	pair, err := newTokenPair()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	if _, err := s.DB.ExecContext(ctx, `
		INSERT INTO reviewer_login_tokens (token_hash, reviewer_id, expires_at)
		VALUES ($1, $2, $3)`, pair.hash[:], reviewerID, time.Now().Add(tokenLife)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	link := s.AppBaseURL + "/reviewer/verify?token=" + pair.token
	if err := s.SendMagicLink(ctx, email, link); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "check your email"})
}

// ConfirmLink renders the form that consumes the token by POST.
func (s *Service) ConfirmLink(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	token := r.URL.Query().Get("token")
	if !validToken(token) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid or expired link")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = verifyPage.Execute(w, map[string]string{"Token": token})
}

// VerifyLink consumes the reviewer token and starts an un-elevated session.
func (s *Service) VerifyLink(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowedForm(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := r.ParseForm(); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	token := r.PostForm.Get("token")
	if !validToken(token) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	pair, err := newTokenPair()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()

	var reviewerID string
	err = tx.QueryRowContext(ctx, `
		DELETE FROM reviewer_login_tokens
		WHERE token_hash = $1 AND expires_at > now()
		RETURNING reviewer_id`, tokenHash(token)).Scan(&reviewerID)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reviewer_sessions (token_hash, reviewer_id, expires_at)
		VALUES ($1, $2, $3)`, pair.hash[:], reviewerID, time.Now().Add(sessionLife)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, "UPDATE reviewers SET last_active_at = now() WHERE id = $1", reviewerID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    pair.token,
		Path:     "/",
		MaxAge:   int(sessionLife.Seconds()),
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	if r.PostForm.Get("redirect") == "/reviewer" {
		http.Redirect(w, r, "/reviewer", http.StatusSeeOther)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "verify mfa"})
}

func (s *Service) requireSession(w http.ResponseWriter, r *http.Request) (string, bool) {
	httpapi.PreventCaching(w)
	cookie, err := r.Cookie(CookieName)
	if err != nil || !validToken(cookie.Value) {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var reviewerID string
	err = s.DB.QueryRowContext(ctx, `
		SELECT reviewer_id FROM reviewer_sessions
		WHERE token_hash = $1 AND expires_at > now()`, tokenHash(cookie.Value)).Scan(&reviewerID)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return "", false
	}
	return reviewerID, true
}

// RequireReviewer rejects sessions that have not passed TOTP MFA.
func (s *Service) RequireReviewer(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewerID, ok := s.requireSession(w, r)
		if !ok {
			return
		}
		var mfaAt sql.NullTime
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		err := s.DB.QueryRowContext(ctx, `
			SELECT mfa_at FROM reviewer_sessions
			WHERE token_hash = $1 AND expires_at > now()`,
			tokenHash(mustCookie(r))).Scan(&mfaAt)
		if err != nil || !mfaAt.Valid {
			httpapi.WriteError(w, http.StatusForbidden, "mfa required")
			return
		}
		next(w, r, reviewerID)
	}
}

func mustCookie(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SignOut revokes the reviewer session and clears the cookie.
func (s *Service) SignOut(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if cookie, err := r.Cookie(CookieName); err == nil && validToken(cookie.Value) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if _, err := s.DB.ExecContext(ctx, "DELETE FROM reviewer_sessions WHERE token_hash = $1",
			tokenHash(cookie.Value)); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
	}
	clearCookie(w, s.CookieSecure)
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "signed out"})
}

func clearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// CleanupExpired removes expired reviewer tokens, sessions and rate limits.
func (s *Service) CleanupExpired(ctx context.Context) error {
	for _, query := range []string{
		"DELETE FROM reviewer_login_tokens WHERE expires_at <= now()",
		"DELETE FROM reviewer_sessions WHERE expires_at <= now()",
		"DELETE FROM reviewer_rate_limits WHERE window_start < now() - interval '2 hours'",
	} {
		if _, err := s.DB.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

// Create adds a reviewer account. Only the operator CLI calls it.
func Create(ctx context.Context, db *sql.DB, email string, encryptionKey, hmacKey []byte) (string, error) {
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return "", errors.New("invalid email")
	}
	encrypted, err := auth.EncryptEmail(normalized, encryptionKey)
	if err != nil {
		return "", err
	}
	var id string
	err = db.QueryRowContext(ctx, `
		INSERT INTO reviewers (email_encrypted, email_lookup_hmac)
		VALUES ($1, $2) RETURNING id`,
		encrypted, auth.EmailLookupHMAC(normalized, hmacKey)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create reviewer: %w", err)
	}
	return id, nil
}
