package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendMailerSendsPlainTextMagicLink(t *testing.T) {
	const magicLink = "http://localhost:8080/auth/verify?token=secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer re_test" {
			t.Error("missing Resend authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type is not application/json")
		}
		var body struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
			Text    string   `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body.From != "Tennis Connect <auth@example.com>" {
			t.Errorf("from = %q", body.From)
		}
		if len(body.To) != 1 || body.To[0] != "player@example.com" {
			t.Errorf("to = %#v", body.To)
		}
		if body.Subject == "" || !strings.Contains(body.Text, magicLink) {
			t.Error("email does not contain subject and magic link")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email-id"}`))
	}))
	defer server.Close()

	mailer := resendMailer{apiKey: "re_test", from: "Tennis Connect <auth@example.com>", endpoint: server.URL, client: server.Client()}
	if err := mailer.sendMagicLink(context.Background(), "player@example.com", magicLink); err != nil {
		t.Fatalf("sendMagicLink() returned error: %v", err)
	}
}

func TestResendMailerHidesProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("private provider response"))
	}))
	defer server.Close()

	mailer := resendMailer{apiKey: "re_test", from: "auth@example.com", endpoint: server.URL, client: server.Client()}
	err := mailer.sendMagicLink(context.Background(), "player@example.com", "http://localhost/token")
	if err == nil {
		t.Fatal("sendMagicLink() accepted provider failure")
	}
	if strings.Contains(err.Error(), "private provider response") || strings.Contains(err.Error(), "re_test") {
		t.Fatalf("sendMagicLink() leaked sensitive error details: %v", err)
	}
}
