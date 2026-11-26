package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tennisconnect/internal/testutil"
)

func recoveryCookieFor(t *testing.T, auth *Service, emails *[]capturedEmail, email string) (*http.Cookie, string) {
	t.Helper()
	userID := insertTestUser(t, auth, email)
	if _, err := testutil.DB.Exec("UPDATE users SET deleted_at = now() - interval '1 day' WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	token := requestMagicToken(t, auth, emails, email)
	response := verifyTokenRequest(auth, token)
	if response.Code != http.StatusOK {
		t.Fatalf("verify status = %d: %s", response.Code, response.Body.String())
	}
	cookie := responseCookie(t, response, recoveryCookie)
	if cookie == nil {
		t.Fatal("missing recovery cookie")
	}
	return cookie, userID
}

func TestRecoveryStatusAndRecover(t *testing.T) {
	auth, emails := newTestAuth(t)
	cookie, userID := recoveryCookieFor(t, auth, emails, "recover@example.com")

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/account/recovery", nil)
	statusRequest.AddCookie(cookie)
	status := httptest.NewRecorder()
	auth.RecoveryStatus(status, statusRequest)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), "recover_until") {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}

	recoverRequest := httptest.NewRequest(http.MethodPost, "/api/v1/account/recovery", nil)
	recoverRequest.Header.Set("Origin", auth.AppBaseURL)
	recoverRequest.AddCookie(cookie)
	recovered := httptest.NewRecorder()
	auth.Recover(recovered, recoverRequest)
	if recovered.Code != http.StatusOK {
		t.Fatalf("recover = %d %s", recovered.Code, recovered.Body.String())
	}
	cleared := responseCookie(t, recovered, recoveryCookie)
	if cleared == nil || cleared.MaxAge != -1 {
		t.Fatal("recovery cookie was not cleared")
	}
	session := responseCookie(t, recovered, "tennisconnect_session")
	if session == nil || session.Value == "" {
		t.Fatal("recover did not create a normal session")
	}

	var deleted bool
	if err := testutil.DB.QueryRow("SELECT deleted_at IS NOT NULL FROM users WHERE id = $1", userID).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("recover did not clear deleted_at")
	}

	// Replaying the recovery cookie after a successful recover must fail.
	replayRequest := httptest.NewRequest(http.MethodGet, "/api/v1/account/recovery", nil)
	replayRequest.AddCookie(cookie)
	replay := httptest.NewRecorder()
	auth.RecoveryStatus(replay, replayRequest)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", replay.Code)
	}
}

func TestContinueDeletingRevokesRecoveryOnly(t *testing.T) {
	auth, emails := newTestAuth(t)
	cookie, userID := recoveryCookieFor(t, auth, emails, "continue@example.com")

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/account/recovery", nil)
	request.Header.Set("Origin", auth.AppBaseURL)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	auth.ContinueDeleting(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("continue = %d %s", response.Code, response.Body.String())
	}
	var deleted bool
	if err := testutil.DB.QueryRow("SELECT deleted_at IS NOT NULL FROM users WHERE id = $1", userID).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("continue deleting restored the account")
	}
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM recovery_sessions WHERE user_id = $1", userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("recovery sessions = %d, want 0", count)
	}

	replayRequest := httptest.NewRequest(http.MethodGet, "/api/v1/account/recovery", nil)
	replayRequest.AddCookie(cookie)
	replay := httptest.NewRecorder()
	auth.RecoveryStatus(replay, replayRequest)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", replay.Code)
	}
}

func TestRecoveryRejectsExpiredAndCrossOriginWrites(t *testing.T) {
	auth, emails := newTestAuth(t)
	cookie, userID := recoveryCookieFor(t, auth, emails, "expired@example.com")
	if _, err := testutil.DB.Exec(`
		UPDATE recovery_sessions SET expires_at = now() - interval '1 second' WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/account/recovery", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	auth.RecoveryStatus(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired status = %d, want 401", response.Code)
	}

	auth, emails = newTestAuth(t)
	cookie, _ = recoveryCookieFor(t, auth, emails, "origin@example.com")
	crossRequest := httptest.NewRequest(http.MethodPost, "/api/v1/account/recovery", nil)
	crossRequest.Header.Set("Origin", "https://attacker.example")
	crossRequest.AddCookie(cookie)
	crossOrigin := httptest.NewRecorder()
	auth.Recover(crossOrigin, crossRequest)
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("cross-origin recover = %d, want 403", crossOrigin.Code)
	}
}
