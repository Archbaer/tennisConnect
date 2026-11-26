package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"tennisconnect/internal/httpapi"
)

const (
	recoveryDays       = 30
	recoverySessionTTL = 15 * time.Minute
	recoveryCookie     = "tennisconnect_recovery"
)

func setRecoveryCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     recoveryCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(recoverySessionTTL),
		MaxAge:   int(recoverySessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearRecoveryCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     recoveryCookie,
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func createRecoverySession(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	token, hash, err := newToken()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM recovery_sessions WHERE user_id = $1", userID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO recovery_sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, now() + interval '15 minutes')`, hash[:], userID); err != nil {
		return "", err
	}
	return token, nil
}

// loadRecovery authenticates a recovery-only cookie. A deleted account cannot
// use a normal session; this is the only credential that can read recovery state.
func (a *Service) loadRecovery(w http.ResponseWriter, r *http.Request) (string, time.Time, bool) {
	httpapi.PreventCaching(w)
	cookie, err := r.Cookie(recoveryCookie)
	if err != nil || !validAuthToken(cookie.Value) {
		httpapi.WriteError(w, http.StatusUnauthorized, "recovery link expired")
		return "", time.Time{}, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	hash := sha256.Sum256([]byte(cookie.Value))
	var userID string
	var deletedAt time.Time
	err = a.DB.QueryRowContext(ctx, `
		SELECT rs.user_id, u.deleted_at
		FROM recovery_sessions rs
		JOIN users u ON u.id = rs.user_id
		WHERE rs.token_hash = $1 AND rs.expires_at > now() AND u.deleted_at IS NOT NULL`,
		hash[:]).Scan(&userID, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusUnauthorized, "recovery link expired")
		return "", time.Time{}, false
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return "", time.Time{}, false
	}
	if time.Since(deletedAt) > recoveryDays*24*time.Hour {
		httpapi.WriteError(w, http.StatusUnauthorized, "recovery link expired")
		return "", time.Time{}, false
	}
	return userID, deletedAt, true
}

// RecoveryStatus serves GET /api/v1/account/recovery.
func (a *Service) RecoveryStatus(w http.ResponseWriter, r *http.Request) {
	_, deletedAt, ok := a.loadRecovery(w, r)
	if !ok {
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{
		"recover_until": deletedAt.Add(recoveryDays * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
}

// Recover serves POST /api/v1/account/recovery. It is the only path that
// restores a deleted account.
func (a *Service) Recover(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !a.validOrigin(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	userID, _, ok := a.loadRecovery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
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
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET deleted_at = NULL, last_active_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL`, userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM recovery_sessions WHERE user_id = $1", userID); err != nil {
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
	clearRecoveryCookie(w, a.CookieSecure)
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "account recovered"})
}

// ContinueDeleting serves DELETE /api/v1/account/recovery. It revokes the
// recovery credential and leaves the account scheduled for deletion.
func (a *Service) ContinueDeleting(w http.ResponseWriter, r *http.Request) {
	httpapi.PreventCaching(w)
	if !a.validOrigin(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	userID, _, ok := a.loadRecovery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if _, err := a.DB.ExecContext(ctx, "DELETE FROM recovery_sessions WHERE user_id = $1", userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	clearRecoveryCookie(w, a.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
