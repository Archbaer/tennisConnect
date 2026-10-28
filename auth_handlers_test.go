package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type capturedEmail struct {
	recipient string
	link      string
}

func newTestAuth(t *testing.T) (*authService, *[]capturedEmail) {
	t.Helper()
	resetAuthTables(t)
	var emails []capturedEmail
	auth := &authService{
		db:                 testDB,
		appBaseURL:         "http://localhost:8080",
		emailEncryptionKey: bytesOf(1, 32),
		emailHMACKey:       bytesOf(2, 32),
		sendMagicLink: func(_ context.Context, recipient, link string) error {
			emails = append(emails, capturedEmail{recipient: recipient, link: link})
			return nil
		},
	}
	return auth, &emails
}

func linkRequest(auth *authService, body string) *httptest.ResponseRecorder {
	return linkRequestFromIP(auth, body, "192.0.2.10")
}

func linkRequestFromIP(auth *authService, body, ip string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/link", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = ip + ":1234"
	response := httptest.NewRecorder()
	auth.requestLink(response, request)
	return response
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil || parsed.Query().Get("token") == "" {
		t.Fatalf("invalid magic link %q", link)
	}
	return parsed.Query().Get("token")
}

func TestRequestLinkStoresOnlyHashesAndEncryptedEmail(t *testing.T) {
	auth, emails := newTestAuth(t)
	start := time.Now()
	response := linkRequest(auth, `{"email":"Player@Example.COM"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() != "{\"message\":\"check your email\"}\n" {
		t.Fatalf("unexpected response body %q", response.Body.String())
	}
	if len(*emails) != 1 || (*emails)[0].recipient != "Player@example.com" {
		t.Fatalf("emails = %#v", *emails)
	}
	token := tokenFromLink(t, (*emails)[0].link)
	hash := sha256.Sum256([]byte(token))
	var encrypted []byte
	var expires time.Time
	err := testDB.QueryRow("SELECT email_encrypted, expires_at FROM login_tokens WHERE token_hash = $1", hash[:]).Scan(&encrypted, &expires)
	if err != nil {
		t.Fatalf("find stored token: %v", err)
	}
	if strings.Contains(string(encrypted), "player@example.com") {
		t.Fatal("stored token contains plaintext email")
	}
	if expires.Sub(start) < 14*time.Minute || expires.Sub(start) > 16*time.Minute {
		t.Fatalf("token expiry = %s, want about 15 minutes", expires.Sub(start))
	}
}

func TestRequestLinkInvalidatesPreviousToken(t *testing.T) {
	auth, emails := newTestAuth(t)
	linkRequest(auth, `{"email":"player@example.com"}`)
	linkRequest(auth, `{"email":"player@example.com"}`)
	if len(*emails) != 2 {
		t.Fatalf("email count = %d, want 2", len(*emails))
	}
	firstHash := sha256.Sum256([]byte(tokenFromLink(t, (*emails)[0].link)))
	secondHash := sha256.Sum256([]byte(tokenFromLink(t, (*emails)[1].link)))
	var count int
	if err := testDB.QueryRow("SELECT count(*) FROM login_tokens").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stored token count = %d, want 1", count)
	}
	if err := testDB.QueryRow("SELECT count(*) FROM login_tokens WHERE token_hash = $1", secondHash[:]).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("latest token is not stored")
	}
	if err := testDB.QueryRow("SELECT count(*) FROM login_tokens WHERE token_hash = $1", firstHash[:]).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("previous token remains valid")
	}
}

func TestRequestLinkRejectsInvalidEmail(t *testing.T) {
	auth, emails := newTestAuth(t)
	response := linkRequest(auth, `{"email":"not-an-email"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if len(*emails) != 0 {
		t.Fatal("invalid email triggered provider send")
	}
}

func TestRequestLinkRejectsMalformedJSON(t *testing.T) {
	auth, emails := newTestAuth(t)
	for _, body := range []string{`{`, `{"email":"player@example.com","admin":true}`, `{"email":"player@example.com"} {}`} {
		response := linkRequest(auth, body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %q returned %d, want 400", body, response.Code)
		}
	}
	if len(*emails) != 0 {
		t.Fatal("malformed JSON triggered provider send")
	}
}

func TestRequestLinkLimitsEachEmail(t *testing.T) {
	auth, emails := newTestAuth(t)
	for i := 0; i < 5; i++ {
		if response := linkRequest(auth, `{"email":"player@example.com"}`); response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d", i+1, response.Code)
		}
	}
	response := linkRequest(auth, `{"email":"player@example.com"}`)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth request status = %d, want 429", response.Code)
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("429 response has no Retry-After")
	}
	if len(*emails) != 5 {
		t.Fatalf("provider sends = %d, want 5", len(*emails))
	}
}

func TestRequestLinkLimitsEachIP(t *testing.T) {
	auth, emails := newTestAuth(t)
	for i := 0; i < 20; i++ {
		body := `{"email":"player` + string(rune('a'+i)) + `@example.com"}`
		if response := linkRequest(auth, body); response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d", i+1, response.Code)
		}
	}
	response := linkRequest(auth, `{"email":"another@example.com"}`)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("request 21 status = %d, want 429", response.Code)
	}
	if len(*emails) != 20 {
		t.Fatalf("provider sends = %d, want 20", len(*emails))
	}
}

func TestLookupHMACUsesDomainSeparation(t *testing.T) {
	auth, _ := newTestAuth(t)
	got := auth.lookupHMAC("player@example.com")
	mac := hmac.New(sha256.New, bytesOf(2, 32))
	mac.Write([]byte("email-lookup-v1\x00player@example.com"))
	want := mac.Sum(nil)
	if !hmac.Equal(got, want) {
		t.Fatal("lookup HMAC does not use versioned email domain separation")
	}
}

func TestBlockedIPCannotConsumeAnotherEmailQuota(t *testing.T) {
	auth, emails := newTestAuth(t)
	for i := 0; i < 20; i++ {
		body := `{"email":"player` + string(rune('a'+i)) + `@example.com"}`
		if response := linkRequest(auth, body); response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d", i+1, response.Code)
		}
	}
	for i := 0; i < 5; i++ {
		if response := linkRequest(auth, `{"email":"victim@example.com"}`); response.Code != http.StatusTooManyRequests {
			t.Fatalf("blocked request %d status = %d, want 429", i+1, response.Code)
		}
	}
	response := linkRequestFromIP(auth, `{"email":"victim@example.com"}`, "203.0.113.50")
	if response.Code != http.StatusOK {
		t.Fatalf("victim from fresh IP status = %d, want 200", response.Code)
	}
	if len(*emails) != 21 {
		t.Fatalf("provider sends = %d, want 21", len(*emails))
	}
}
