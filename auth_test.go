package main

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeEmail(t *testing.T) {
	got, err := normalizeEmail(" Player@Example.COM ")
	if err != nil {
		t.Fatalf("normalizeEmail() returned error: %v", err)
	}
	if got != "Player@example.com" {
		t.Fatalf("normalizeEmail() = %q, want Player@example.com", got)
	}
}

func TestNormalizeEmailPreservesCaseSensitiveLocalPart(t *testing.T) {
	upper, err := normalizeEmail("Player@Example.COM")
	if err != nil {
		t.Fatal(err)
	}
	lower, err := normalizeEmail("player@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if upper == lower {
		t.Fatal("normalizeEmail() merged distinct case-sensitive mailboxes")
	}
}

func TestNormalizeEmailRejectsInvalidInput(t *testing.T) {
	for _, email := range []string{"", "not-an-email", "Player <player@example.com>", "a@example.com, b@example.com"} {
		t.Run(email, func(t *testing.T) {
			if _, err := normalizeEmail(email); err == nil {
				t.Fatalf("normalizeEmail(%q) accepted invalid input", email)
			}
		})
	}
}

func TestIsAdult(t *testing.T) {
	now := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		dob  time.Time
		want bool
	}{
		{name: "eighteenth birthday", dob: time.Date(2008, time.October, 27, 0, 0, 0, 0, time.UTC), want: true},
		{name: "day before eighteenth birthday", dob: time.Date(2008, time.October, 28, 0, 0, 0, 0, time.UTC), want: false},
		{name: "future date", dob: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC), want: false},
		{name: "older adult", dob: time.Date(1950, time.January, 1, 0, 0, 0, 0, time.UTC), want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isAdult(test.dob, now); got != test.want {
				t.Fatalf("isAdult() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestNewToken(t *testing.T) {
	token, hash, err := newToken()
	if err != nil {
		t.Fatalf("newToken() returned error: %v", err)
	}
	if len(token) != 43 {
		t.Fatalf("token length = %d, want 43", len(token))
	}
	if len(hash) != 32 {
		t.Fatalf("hash length = %d, want 32", len(hash))
	}
	otherToken, otherHash, err := newToken()
	if err != nil {
		t.Fatalf("newToken() returned error: %v", err)
	}
	if token == otherToken || bytes.Equal(hash[:], otherHash[:]) {
		t.Fatal("newToken() repeated a token or hash")
	}
}

func TestEncryptEmailUsesRandomNonce(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	first, err := encryptEmail("player@example.com", key)
	if err != nil {
		t.Fatalf("encryptEmail() returned error: %v", err)
	}
	second, err := encryptEmail("player@example.com", key)
	if err != nil {
		t.Fatalf("encryptEmail() returned error: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("encryptEmail() produced deterministic ciphertext")
	}
	if bytes.Contains(first, []byte("player@example.com")) {
		t.Fatal("encryptEmail() exposed plaintext")
	}
}

func TestClientIP(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/v1/auth/link", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.2")

	if got := clientIP(request, false); got != "192.0.2.10" {
		t.Fatalf("clientIP() without proxy trust = %q, want 192.0.2.10", got)
	}
	if got := clientIP(request, true); got != "203.0.113.2" {
		t.Fatalf("clientIP() with proxy trust = %q, want 203.0.113.2", got)
	}
}

func TestClientIPIgnoresMalformedRemoteAddress(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/v1/auth/link", nil)
	request.RemoteAddr = "not-an-ip"
	if got := clientIP(request, false); got != "" {
		t.Fatalf("clientIP() = %q, want empty string", got)
	}
}

func TestNormalizeEmailRejectsOverlongInput(t *testing.T) {
	email := strings.Repeat("a", 245) + "@example.com"
	if _, err := normalizeEmail(email); err == nil {
		t.Fatal("normalizeEmail() accepted an address over 254 bytes")
	}
}
