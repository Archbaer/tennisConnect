package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"html/template"
	"mime"
	"net/http"
	"time"
)

var confirmPage = template.Must(template.New("confirm").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Continue sign-in</title></head>
<body><main><h1>Continue sign-in</h1><form method="post" action="/api/v1/auth/verify"><input type="hidden" name="token" value="{{.}}"><button type="submit">Continue</button></form></main></body></html>`))

var registrationPage = template.Must(template.New("register").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Complete registration</title></head>
<body><main><h1>Complete registration</h1><form method="post" action="/api/v1/auth/register"><input type="hidden" name="token" value="{{.}}"><label for="birth-date">Date of birth</label><input id="birth-date" name="birth_date" type="date" required><button type="submit">Create account</button></form></main></body></html>`))

type loginIdentity struct {
	emailEncrypted []byte
	emailHMAC      []byte
}

func validAuthToken(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func preventAuthCaching(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func renderAuthPage(w http.ResponseWriter, page *template.Template, token string) {
	preventAuthCaching(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = page.Execute(w, token)
}

func (a *authService) confirmLink(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if !validAuthToken(token) {
		preventAuthCaching(w)
		writeError(w, http.StatusBadRequest, "invalid or expired link")
		return
	}
	renderAuthPage(w, confirmPage, token)
}

func parseAuthForm(w http.ResponseWriter, r *http.Request) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/x-www-form-urlencoded" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	return r.ParseForm() == nil
}

func consumeLoginToken(ctx context.Context, tx *sql.Tx, hash [32]byte) (loginIdentity, error) {
	var identity loginIdentity
	err := tx.QueryRowContext(ctx, `
		DELETE FROM login_tokens WHERE token_hash = $1 AND expires_at > now()
		RETURNING email_encrypted, email_lookup_hmac`, hash[:]).Scan(&identity.emailEncrypted, &identity.emailHMAC)
	return identity, err
}

func insertSession(ctx context.Context, tx *sql.Tx, userID string, hash [32]byte, expires time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, hash[:], userID, expires)
	return err
}

func setSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tennisconnect_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(14 * 24 * time.Hour),
		MaxAge:   14 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tennisconnect_session",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *authService) verifyLink(w http.ResponseWriter, r *http.Request) {
	preventAuthCaching(w)
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !a.limitIP(ctx, w, r) {
		return
	}
	if !parseAuthForm(w, r) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	token := r.PostForm.Get("token")
	if !validAuthToken(token) {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	sessionToken, sessionHash, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	identity, err := consumeLoginToken(ctx, tx, sha256.Sum256([]byte(token)))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var userID string
	var deletedAt sql.NullTime
	err = tx.QueryRowContext(ctx, "SELECT id, deleted_at FROM users WHERE email_lookup_hmac = $1", identity.emailHMAC).Scan(&userID, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		renderAuthPage(w, registrationPage, token)
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if deletedAt.Valid {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET last_active_at = now() WHERE id = $1", userID); err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := insertSession(ctx, tx, userID, sessionHash, time.Now().Add(14*24*time.Hour)); err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	setSessionCookie(w, sessionToken, a.cookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"message": "signed in"})
}

func (a *authService) register(w http.ResponseWriter, r *http.Request) {
	preventAuthCaching(w)
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !a.limitIP(ctx, w, r) {
		return
	}
	if !parseAuthForm(w, r) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	token := r.PostForm.Get("token")
	birthDate, err := time.Parse("2006-01-02", r.PostForm.Get("birth_date"))
	if !validAuthToken(token) {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil || !isAdult(birthDate, time.Now()) {
		writeError(w, http.StatusBadRequest, "must be 18 or older")
		return
	}
	sessionToken, sessionHash, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	identity, err := consumeLoginToken(ctx, tx, sha256.Sum256([]byte(token)))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var userID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, email_verified_at, last_active_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (email_lookup_hmac) DO NOTHING
		RETURNING id`, identity.emailEncrypted, identity.emailHMAC, birthDate).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		var deletedAt sql.NullTime
		err = tx.QueryRowContext(ctx, "SELECT id, deleted_at FROM users WHERE email_lookup_hmac = $1", identity.emailHMAC).Scan(&userID, &deletedAt)
		if err == nil && deletedAt.Valid {
			writeError(w, http.StatusUnauthorized, "invalid or expired link")
			return
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET last_active_at = now() WHERE id = $1", userID); err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := insertSession(ctx, tx, userID, sessionHash, time.Now().Add(14*24*time.Hour)); err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	setSessionCookie(w, sessionToken, a.cookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"message": "signed in"})
}

func (a *authService) signOut(w http.ResponseWriter, r *http.Request) {
	preventAuthCaching(w)
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	cookie, err := r.Cookie("tennisconnect_session")
	if err == nil && validAuthToken(cookie.Value) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		hash := sha256.Sum256([]byte(cookie.Value))
		if _, err := a.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = $1", hash[:]); err != nil {
			writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
	}
	clearSessionCookie(w, a.cookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"message": "signed out"})
}

func (a *authService) authenticatedUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	preventAuthCaching(w)
	cookie, err := r.Cookie("tennisconnect_session")
	if err != nil || !validAuthToken(cookie.Value) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	hash := sha256.Sum256([]byte(cookie.Value))
	var userID string
	err = a.db.QueryRowContext(ctx, `
		SELECT u.id FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.deleted_at IS NULL`, hash[:]).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return "", false
	}
	return userID, true
}

func (a *authService) requireUser(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := a.authenticatedUser(w, r)
		if !ok {
			return
		}
		next(w, r, userID)
	}
}

func (a *authService) session(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.authenticatedUser(w, r); !ok {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
