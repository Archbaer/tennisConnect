package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func requestMagicToken(t *testing.T, auth *authService, emails *[]capturedEmail, email string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q}`, email)
	if response := linkRequest(auth, body); response.Code != http.StatusOK {
		t.Fatalf("request link status = %d, body = %s", response.Code, response.Body.String())
	}
	return tokenFromLink(t, (*emails)[len(*emails)-1].link)
}

func insertTestUser(t *testing.T, auth *authService, email string) string {
	t.Helper()
	encrypted, err := encryptEmail(email, auth.emailEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = testDB.QueryRow(`
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, email_verified_at)
		VALUES ($1, $2, $3, now()) RETURNING id`, encrypted, auth.lookupHMAC(email), time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)).Scan(&id)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return id
}

func authFormRequest(handler http.HandlerFunc, path string, values url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}

func verifyTokenRequest(auth *authService, token string) *httptest.ResponseRecorder {
	return authFormRequest(auth.verifyLink, "/api/v1/auth/verify", url.Values{"token": {token}})
}

func registerRequest(auth *authService, token, birthDate string) *httptest.ResponseRecorder {
	return authFormRequest(auth.register, "/api/v1/auth/register", url.Values{"token": {token}, "birth_date": {birthDate}})
}

func countAuthRows(t *testing.T, table string) int {
	t.Helper()
	if table != "users" && table != "sessions" && table != "login_tokens" {
		t.Fatalf("invalid test table %q", table)
	}
	var count int
	if err := testDB.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func responseSessionCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "tennisconnect_session" {
			return cookie
		}
	}
	t.Fatal("response has no session cookie")
	return nil
}

func TestConfirmLinkDoesNotConsumeToken(t *testing.T) {
	auth, emails := newTestAuth(t)
	token := requestMagicToken(t, auth, emails, "player@example.com")
	request := httptest.NewRequest(http.MethodGet, "/auth/verify?token="+url.QueryEscape(token), nil)
	response := httptest.NewRecorder()
	auth.confirmLink(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `name="token" value="`+token+`"`) {
		t.Fatal("confirmation form does not contain token")
	}
	if response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("confirmation page lacks no-referrer protection")
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("confirmation page lacks restrictive CSP")
	}
	if got := countAuthRows(t, "login_tokens"); got != 1 {
		t.Fatalf("GET consumed login token; rows = %d", got)
	}
}

func TestVerifyExistingUserCreatesSessionAndRejectsReplay(t *testing.T) {
	auth, emails := newTestAuth(t)
	userID := insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	response := verifyTokenRequest(auth, token)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	cookie := responseSessionCookie(t, response)
	if !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("unsafe cookie attributes: %#v", cookie)
	}
	if cookie.MaxAge != 14*24*60*60 {
		t.Fatalf("cookie MaxAge = %d, want 14 days", cookie.MaxAge)
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	var storedUser string
	if err := testDB.QueryRow("SELECT user_id FROM sessions WHERE token_hash = $1", hash[:]).Scan(&storedUser); err != nil {
		t.Fatalf("find session: %v", err)
	}
	if storedUser != userID {
		t.Fatal("session belongs to wrong user")
	}
	if got := countAuthRows(t, "login_tokens"); got != 0 {
		t.Fatal("used login token remains stored")
	}

	replay := verifyTokenRequest(auth, token)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", replay.Code)
	}
	if got := countAuthRows(t, "sessions"); got != 1 {
		t.Fatalf("replay changed session count to %d", got)
	}
}

func TestVerifyNewEmailRequestsBirthDate(t *testing.T) {
	auth, emails := newTestAuth(t)
	token := requestMagicToken(t, auth, emails, "new@example.com")
	response := verifyTokenRequest(auth, token)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `name="birth_date"`) {
		t.Fatal("response does not request date of birth")
	}
	if got := countAuthRows(t, "login_tokens"); got != 1 {
		t.Fatal("registration token was consumed before DOB validation")
	}
	if got := countAuthRows(t, "users"); got != 0 {
		t.Fatal("user created before DOB validation")
	}
}

func TestRegisterRejectsUnderageWithoutConsumingToken(t *testing.T) {
	auth, emails := newTestAuth(t)
	token := requestMagicToken(t, auth, emails, "new@example.com")
	underage := time.Now().AddDate(-17, 0, 0).Format("2006-01-02")
	response := registerRequest(auth, token, underage)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if got := countAuthRows(t, "login_tokens"); got != 1 {
		t.Fatal("underage attempt consumed token")
	}
	if got := countAuthRows(t, "users"); got != 0 {
		t.Fatal("underage user created")
	}
}

func TestRegisterAdultCreatesUserAndSession(t *testing.T) {
	auth, emails := newTestAuth(t)
	token := requestMagicToken(t, auth, emails, "new@example.com")
	adult := time.Now().AddDate(-18, 0, 0).Format("2006-01-02")
	response := registerRequest(auth, token, adult)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	responseSessionCookie(t, response)
	if got := countAuthRows(t, "users"); got != 1 {
		t.Fatalf("users = %d, want 1", got)
	}
	if got := countAuthRows(t, "sessions"); got != 1 {
		t.Fatalf("sessions = %d, want 1", got)
	}
	if got := countAuthRows(t, "login_tokens"); got != 0 {
		t.Fatal("registration token remains stored")
	}
	if replay := registerRequest(auth, token, adult); replay.Code != http.StatusUnauthorized {
		t.Fatalf("registration replay status = %d, want 401", replay.Code)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	auth, emails := newTestAuth(t)
	insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	if _, err := testDB.Exec("UPDATE login_tokens SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	response := verifyTokenRequest(auth, token)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := countAuthRows(t, "sessions"); got != 0 {
		t.Fatal("expired token created session")
	}
}

func TestVerifyRejectsDeletedUser(t *testing.T) {
	auth, emails := newTestAuth(t)
	userID := insertTestUser(t, auth, "player@example.com")
	if _, err := testDB.Exec("UPDATE users SET deleted_at = now() WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	token := requestMagicToken(t, auth, emails, "player@example.com")
	response := verifyTokenRequest(auth, token)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := countAuthRows(t, "sessions"); got != 0 {
		t.Fatal("deleted user created session")
	}
}

func TestConcurrentVerificationCreatesOneSession(t *testing.T) {
	auth, emails := newTestAuth(t)
	insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	responses := make([]*httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	for i := range responses {
		wait.Add(1)
		go func() {
			defer wait.Done()
			responses[i] = verifyTokenRequest(auth, token)
		}()
	}
	wait.Wait()
	var success, rejected int
	for _, response := range responses {
		switch response.Code {
		case http.StatusOK:
			success++
		case http.StatusUnauthorized:
			rejected++
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("success = %d, rejected = %d, want one each", success, rejected)
	}
	if got := countAuthRows(t, "sessions"); got != 1 {
		t.Fatalf("sessions = %d, want 1", got)
	}
}

func TestSignOutDeletesSessionAndClearsCookie(t *testing.T) {
	auth, emails := newTestAuth(t)
	insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	cookie := responseSessionCookie(t, verifyTokenRequest(auth, token))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sign-out", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	auth.signOut(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := countAuthRows(t, "sessions"); got != 0 {
		t.Fatal("sign-out did not delete server session")
	}
	cleared := responseSessionCookie(t, response)
	if cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Fatalf("session cookie not cleared: %#v", cleared)
	}
}

func TestRegisterRejectsMalformedBirthDate(t *testing.T) {
	auth, emails := newTestAuth(t)
	token := requestMagicToken(t, auth, emails, "new@example.com")
	response := registerRequest(auth, token, "27/10/2000")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if got := countAuthRows(t, "users"); got != 0 {
		t.Fatal("malformed DOB created user")
	}
}

func TestRegistrationUsesEmailHMACConstraint(t *testing.T) {
	auth, emails := newTestAuth(t)
	insertTestUser(t, auth, "player@example.com")
	token := requestMagicToken(t, auth, emails, "player@example.com")
	response := registerRequest(auth, token, time.Now().AddDate(-20, 0, 0).Format("2006-01-02"))
	if response.Code != http.StatusOK {
		t.Fatalf("existing account registration status = %d", response.Code)
	}
	if got := countAuthRows(t, "users"); got != 1 {
		t.Fatalf("duplicate user created; users = %d", got)
	}
}

func TestVerifyRejectsUnknownToken(t *testing.T) {
	auth, _ := newTestAuth(t)
	response := verifyTokenRequest(auth, strings.Repeat("a", 43))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
