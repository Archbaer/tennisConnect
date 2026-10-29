package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthRejectsCrossOriginPosts(t *testing.T) {
	auth, emails := newTestAuth(t)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
		body    string
	}{
		{name: "link", handler: auth.requestLink, path: "/api/v1/auth/link", body: `{"email":"player@example.com"}`},
		{name: "verify", handler: auth.verifyLink, path: "/api/v1/auth/verify", body: "token=abc"},
		{name: "register", handler: auth.register, path: "/api/v1/auth/register", body: "token=abc&birth_date=2000-01-01"},
		{name: "sign out", handler: auth.signOut, path: "/api/v1/auth/sign-out", body: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Origin", "https://attacker.example")
			response := httptest.NewRecorder()
			test.handler(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", response.Code)
			}
		})
	}
	if len(*emails) != 0 {
		t.Fatal("cross-origin request triggered email")
	}
}

func TestSessionEndpointValidatesServerSession(t *testing.T) {
	auth, emails := newTestAuth(t)
	userID := insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	cookie := responseSessionCookie(t, verifyTokenRequest(auth, token))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	auth.session(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid session status = %d, want 204", response.Code)
	}

	if _, err := testDB.Exec("UPDATE sessions SET expires_at = now() - interval '1 second' WHERE user_id = $1", userID); err != nil {
		t.Fatal(err)
	}
	expired := httptest.NewRecorder()
	auth.session(expired, request)
	if expired.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status = %d, want 401", expired.Code)
	}
}

func TestSessionEndpointRejectsDeletedUser(t *testing.T) {
	auth, emails := newTestAuth(t)
	userID := insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	cookie := responseSessionCookie(t, verifyTokenRequest(auth, token))
	if _, err := testDB.Exec("UPDATE users SET deleted_at = now() WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	auth.session(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("deleted user session status = %d, want 401", response.Code)
	}
}

func TestSessionEndpointRejectsInvalidCookie(t *testing.T) {
	auth, _ := newTestAuth(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.AddCookie(&http.Cookie{Name: "tennisconnect_session", Value: "invalid"})
	response := httptest.NewRecorder()
	auth.session(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid cookie status = %d, want 401", response.Code)
	}
}

func TestProductionSessionCookieIsSecure(t *testing.T) {
	auth, emails := newTestAuth(t)
	auth.cookieSecure = true
	insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	cookie := responseSessionCookie(t, verifyTokenRequest(auth, token))
	if !cookie.Secure {
		t.Fatal("production session cookie lacks Secure attribute")
	}
}

func TestRequestLinkResponseDoesNotRevealAccountExistence(t *testing.T) {
	auth, emails := newTestAuth(t)
	insertTestUser(t, auth, "known@example.com")
	known := linkRequest(auth, `{"email":"known@example.com"}`)
	unknown := linkRequest(auth, `{"email":"unknown@example.com"}`)
	if known.Code != unknown.Code || known.Body.String() != unknown.Body.String() {
		t.Fatalf("responses differ: known=%d %q unknown=%d %q", known.Code, known.Body.String(), unknown.Code, unknown.Body.String())
	}
	if len(*emails) != 2 {
		t.Fatal("both requests should use same email flow")
	}
}

func TestRequestLinkHidesProviderError(t *testing.T) {
	auth, _ := newTestAuth(t)
	auth.sendMagicLink = func(context.Context, string, string) error {
		return errors.New("private provider failure")
	}
	response := linkRequest(auth, `{"email":"player@example.com"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if strings.Contains(response.Body.String(), "private provider failure") {
		t.Fatal("provider error leaked to client")
	}
}

func TestVerifyUpdatesLastActiveTime(t *testing.T) {
	auth, emails := newTestAuth(t)
	userID := insertTestUser(t, auth, "player@example.com")
	if _, err := testDB.Exec("UPDATE users SET last_active_at = '2000-01-01' WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	token := requestMagicToken(t, auth, emails, "player@example.com")
	verifyTokenRequest(auth, token)
	var lastActive time.Time
	if err := testDB.QueryRow("SELECT last_active_at FROM users WHERE id = $1", userID).Scan(&lastActive); err != nil {
		t.Fatal(err)
	}
	if lastActive.Year() <= 2000 {
		t.Fatal("sign-in did not update last-active time")
	}
}

func TestVerifyResponsePreventsCaching(t *testing.T) {
	auth, emails := newTestAuth(t)
	insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	response := verifyTokenRequest(auth, token)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("authenticated response allows caching")
	}
}

func TestRegistrationFormActionUsesExpectedRoute(t *testing.T) {
	auth, emails := newTestAuth(t)
	token := requestMagicToken(t, auth, emails, "new@example.com")
	response := verifyTokenRequest(auth, token)
	if !strings.Contains(response.Body.String(), `action="/api/v1/auth/register"`) {
		t.Fatal("registration form posts to unexpected route")
	}
}

func TestCleanupExpiredAuthRows(t *testing.T) {
	auth, _ := newTestAuth(t)
	userID := insertTestUser(t, auth, "player@example.com")
	for i, expires := range []time.Time{time.Now().Add(-time.Hour), time.Now().Add(time.Hour)} {
		_, err := testDB.Exec(`
			INSERT INTO login_tokens (token_hash, email_encrypted, email_lookup_hmac, expires_at)
			VALUES ($1, $2, $3, $4)`, bytesOf(byte(10+i), 32), bytesOf(1, 20), bytesOf(byte(20+i), 32), expires)
		if err != nil {
			t.Fatal(err)
		}
		_, err = testDB.Exec("INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)", bytesOf(byte(30+i), 32), userID, expires)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, window := range []time.Time{time.Now().Add(-3 * time.Hour), time.Now().UTC().Truncate(time.Hour)} {
		_, err := testDB.Exec("INSERT INTO auth_rate_limits (key_hash, window_start, attempts) VALUES ($1, $2, 1)", bytesOf(byte(40+i), 32), window)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := auth.cleanupExpired(context.Background()); err != nil {
		t.Fatalf("cleanupExpired() returned error: %v", err)
	}
	for table, want := range map[string]int{"login_tokens": 1, "sessions": 1, "auth_rate_limits": 1} {
		var got int
		if err := testDB.QueryRow("SELECT count(*) FROM " + table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s rows = %d, want %d", table, got, want)
		}
	}
}

func TestSignOutDatabaseFailureKeepsCookieForRetry(t *testing.T) {
	auth, _ := newTestAuth(t)
	broken, err := sql.Open("pgx", "postgres://user:password@localhost:5432/tennisconnect_test")
	if err != nil {
		t.Fatal(err)
	}
	_ = broken.Close()
	auth.db = broken
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sign-out", nil)
	request.AddCookie(&http.Cookie{Name: "tennisconnect_session", Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	auth.signOut(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if cookies := response.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("database failure cleared session cookie: %#v", cookies)
	}
}
