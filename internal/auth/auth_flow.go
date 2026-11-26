package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"html/template"
	"mime"
	"net/http"
	"strings"
	"time"

	"tennisconnect/internal/httpapi"
	"tennisconnect/internal/observability"
)

var confirmPage = template.Must(template.New("confirm").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Continue sign-in</title>
<style>
:root{color-scheme:light;font-family:ui-sans-serif,system-ui,sans-serif;background:#f7f9f4;color:#0a1714}
*{box-sizing:border-box}
body{margin:0;min-height:100dvh;display:grid;place-items:center;padding:1.5rem;background:linear-gradient(160deg,#f7f9f4,#e7efe8)}
main{width:min(100%,26rem);background:#fff;border:1px solid #d7e0d8;border-radius:28px;padding:1.5rem;box-shadow:0 18px 48px rgb(6 40 31 / 10%)}
p{margin:0 0 .75rem;color:#4f625b;line-height:1.5}
.eyebrow{margin:0 0 .5rem;color:#146b4a;font-size:.76rem;font-weight:700;letter-spacing:.09em;text-transform:uppercase}
h1{margin:0 0 .75rem;font-size:2rem;line-height:1.05;letter-spacing:-.03em}
form{margin-top:1.25rem}
button{width:100%;min-height:3rem;border:0;border-radius:999px;background:#0b3b2e;color:#fff;font:inherit;font-weight:700;cursor:pointer}
button:hover{background:#06281f}
</style></head>
<body><main><p class="eyebrow">Tennis Connect</p><h1>Continue sign-in</h1><p>This link expires in 15 minutes and can be used once.</p><form method="post" action="/api/v1/auth/verify"><input type="hidden" name="token" value="{{.}}"><button type="submit">Continue</button></form></main></body></html>`))

var registrationPage = template.Must(template.New("register").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Complete registration</title>
<style>
:root{color-scheme:light;font-family:ui-sans-serif,system-ui,sans-serif;background:#f7f9f4;color:#0a1714}
*{box-sizing:border-box}
body{margin:0;min-height:100dvh;display:grid;place-items:center;padding:1.5rem;background:linear-gradient(160deg,#f7f9f4,#e7efe8)}
main{width:min(100%,26rem);background:#fff;border:1px solid #d7e0d8;border-radius:28px;padding:1.5rem;box-shadow:0 18px 48px rgb(6 40 31 / 10%)}
p{margin:0 0 .75rem;color:#4f625b;line-height:1.5}
.eyebrow{margin:0 0 .5rem;color:#146b4a;font-size:.76rem;font-weight:700;letter-spacing:.09em;text-transform:uppercase}
h1{margin:0 0 .75rem;font-size:2rem;line-height:1.05;letter-spacing:-.03em}
label{display:grid;gap:.35rem;font-weight:600}
input{width:100%;min-height:2.9rem;padding:.6rem .8rem;border:1px solid #667a72;border-radius:14px;background:#fff;color:#0a1714;font:inherit}
form{display:grid;gap:1rem;margin-top:1.25rem}
button{width:100%;min-height:3rem;border:0;border-radius:999px;background:#0b3b2e;color:#fff;font:inherit;font-weight:700;cursor:pointer}
button:hover{background:#06281f}
</style></head>
<body><main><p class="eyebrow">Tennis Connect · Adults only</p><h1>Confirm your age</h1><p>We ask once. Your date of birth is never shown to anyone.</p><form method="post" action="/api/v1/auth/register"><input type="hidden" name="token" value="{{.}}"><label for="birth-date">Date of birth<input id="birth-date" name="birth_date" type="date" required></label><button type="submit">Create account</button></form></main></body></html>`))

type loginIdentity struct {
	emailEncrypted []byte
	emailHMAC      []byte
}

func validAuthToken(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func renderAuthPage(w http.ResponseWriter, page *template.Template, token string) {
	httpapi.PreventCaching(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = page.Execute(w, token)
}

func (a *Service) ConfirmLink(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if !validAuthToken(token) {
		httpapi.PreventCaching(w)
		httpapi.WriteError(w, http.StatusBadRequest, "invalid or expired link")
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

func (a *Service) VerifyLink(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !a.validFormOrigin(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !a.limitIP(ctx, w, r) {
		return
	}
	if !parseAuthForm(w, r) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	token := r.PostForm.Get("token")
	if !validAuthToken(token) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	sessionToken, sessionHash, err := newToken()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}

	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	identity, err := consumeLoginToken(ctx, tx, sha256.Sum256([]byte(token)))
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
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
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if deletedAt.Valid {
		// Signing in inside the window issues a recovery-only credential. The
		// account stays deleted until the owner explicitly chooses to recover.
		if time.Since(deletedAt.Time) > recoveryDays*24*time.Hour {
			httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
			return
		}
		recoveryToken, err := createRecoverySession(ctx, tx, userID)
		if err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		if err := tx.Commit(); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		setRecoveryCookie(w, recoveryToken, a.CookieSecure)
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, a.AppBaseURL+"/account/recovery", http.StatusSeeOther)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"recovery": true})
		return
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET last_active_at = now() WHERE id = $1", userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := insertSession(ctx, tx, userID, sessionHash, time.Now().Add(14*24*time.Hour)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	setSessionCookie(w, sessionToken, a.CookieSecure)
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, a.AppBaseURL+"/discover", http.StatusSeeOther)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "signed in"})
}

func (a *Service) Register(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !a.validFormOrigin(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !a.limitIP(ctx, w, r) {
		return
	}
	if !parseAuthForm(w, r) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	token := r.PostForm.Get("token")
	birthDate, err := time.Parse("2006-01-02", r.PostForm.Get("birth_date"))
	if !validAuthToken(token) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil || !isAdult(birthDate, time.Now()) {
		httpapi.WriteError(w, http.StatusBadRequest, "must be 18 or older")
		return
	}
	sessionToken, sessionHash, err := newToken()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "temporarily unavailable")
		return
	}

	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	identity, err := consumeLoginToken(ctx, tx, sha256.Sum256([]byte(token)))
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var userID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, email_verified_at, last_active_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (email_lookup_hmac) DO NOTHING
		RETURNING id`, identity.emailEncrypted, identity.emailHMAC, birthDate).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		var deletedAt, bannedAt sql.NullTime
		err = tx.QueryRowContext(ctx, "SELECT id, deleted_at, banned_at FROM users WHERE email_lookup_hmac = $1", identity.emailHMAC).Scan(&userID, &deletedAt, &bannedAt)
		if err == nil && (deletedAt.Valid || bannedAt.Valid) {
			httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
			return
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET last_active_at = now() WHERE id = $1", userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := insertSession(ctx, tx, userID, sessionHash, time.Now().Add(14*24*time.Hour)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	setSessionCookie(w, sessionToken, a.CookieSecure)
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "signed in"})
}

func (a *Service) SignOut(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !a.validOrigin(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	cookie, err := r.Cookie("tennisconnect_session")
	if err == nil && validAuthToken(cookie.Value) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		hash := sha256.Sum256([]byte(cookie.Value))
		if _, err := a.DB.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = $1", hash[:]); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
	}
	clearSessionCookie(w, a.CookieSecure)
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "signed out"})
}

func (a *Service) authenticatedAccount(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	httpapi.PreventCaching(w)
	cookie, err := r.Cookie("tennisconnect_session")
	if err != nil || !validAuthToken(cookie.Value) {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return "", false, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	hash := sha256.Sum256([]byte(cookie.Value))
	var userID string
	var bannedAt sql.NullTime
	err = a.DB.QueryRowContext(ctx, `
		SELECT u.id, u.banned_at FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.deleted_at IS NULL`, hash[:]).Scan(&userID, &bannedAt)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return "", false, false
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return "", false, false
	}
	return userID, bannedAt.Valid, true
}

// RequireUser is the player boundary. Banned accounts keep a session only to
// read and submit an appeal through RequireAccount.
func (a *Service) RequireUser(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, banned, ok := a.authenticatedAccount(w, r)
		if !ok {
			return
		}
		if banned {
			httpapi.WriteError(w, http.StatusForbidden, "account suspended")
			return
		}
		next(w, r.WithContext(observability.WithActor(r.Context(), userID)), userID)
	}
}

// RequireAccount authenticates a live session without granting player APIs to
// banned accounts. Appeal handlers use it; every player route uses RequireUser.
func (a *Service) RequireAccount(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _, ok := a.authenticatedAccount(w, r)
		if !ok {
			return
		}
		next(w, r.WithContext(observability.WithActor(r.Context(), userID)), userID)
	}
}

func (a *Service) Session(w http.ResponseWriter, r *http.Request) {
	_, banned, ok := a.authenticatedAccount(w, r)
	if !ok {
		return
	}
	if banned {
		httpapi.WriteError(w, http.StatusForbidden, "account suspended")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
