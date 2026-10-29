package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setValidConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/tennisconnect")
	t.Setenv("PORT", "")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	t.Setenv("EMAIL_FROM", "Tennis Connect <auth@example.com>")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("EMAIL_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("EMAIL_HMAC_KEY", base64.StdEncoding.EncodeToString(bytesOf(2, 32)))
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("TRUST_PROXY_HEADERS", "false")
}

func bytesOf(value byte, count int) []byte {
	result := make([]byte, count)
	for i := range result {
		result[i] = value
	}
	return result
}

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("DATABASE_URL", "")

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() accepted an empty DATABASE_URL")
	}
}

func TestLoadConfigDefaultsPort(t *testing.T) {
	setValidConfigEnv(t)
	const databaseURL = "postgres://user:password@localhost:5432/tennisconnect"
	t.Setenv("DATABASE_URL", databaseURL)

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() returned error: %v", err)
	}
	if got.databaseURL != databaseURL {
		t.Fatalf("databaseURL = %q, want %q", got.databaseURL, databaseURL)
	}
	if got.address != ":8080" {
		t.Fatalf("address = %q, want :8080", got.address)
	}
}

func TestLoadConfigAuthSettings(t *testing.T) {
	setValidConfigEnv(t)

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() returned error: %v", err)
	}
	if got.appBaseURL != "http://localhost:8080" {
		t.Fatalf("appBaseURL = %q", got.appBaseURL)
	}
	if len(got.emailEncryptionKey) != 32 || len(got.emailHMACKey) != 32 {
		t.Fatal("loadConfig() did not decode 32-byte auth keys")
	}
	if got.cookieSecure {
		t.Fatal("cookieSecure = true, want false for local configuration")
	}
	if got.trustProxy {
		t.Fatal("trustProxy = true, want false")
	}
}

func TestLoadConfigRejectsInvalidAuthSettings(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "EMAIL_ENCRYPTION_KEY", value: "bad"},
		{name: "EMAIL_HMAC_KEY", value: "bad"},
		{name: "APP_BASE_URL", value: "javascript://example.com"},
		{name: "APP_BASE_URL", value: "https://user:password@example.com"},
		{name: "RESEND_API_KEY", value: ""},
		{name: "EMAIL_FROM", value: "not-an-email"},
		{name: "COOKIE_SECURE", value: "maybe"},
		{name: "TRUST_PROXY_HEADERS", value: "maybe"},
	}
	for _, test := range tests {
		t.Run(test.name+"="+test.value, func(t *testing.T) {
			setValidConfigEnv(t)
			t.Setenv(test.name, test.value)
			if _, err := loadConfig(); err == nil {
				t.Fatalf("loadConfig() accepted %s=%q", test.name, test.value)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536"} {
		t.Run(port, func(t *testing.T) {
			setValidConfigEnv(t)
			t.Setenv("PORT", port)

			if _, err := loadConfig(); err == nil {
				t.Fatalf("loadConfig() accepted PORT=%q", port)
			}
		})
	}
}

func TestLoadConfigRejectsNonPostgresURL(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("DATABASE_URL", "https://localhost/tennisconnect")

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() accepted a non-PostgreSQL DATABASE_URL")
	}
}

func TestLoadConfigSecureDefaults(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("TRUST_PROXY_HEADERS", "")

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() returned error: %v", err)
	}
	if !got.cookieSecure {
		t.Fatal("cookieSecure defaulted to false")
	}
	if got.trustProxy {
		t.Fatal("trustProxy defaulted to true")
	}
}

func TestHealthHandlerDatabaseHealthy(t *testing.T) {
	handler := healthHandler(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("database ping context has no deadline")
		}
		return nil
	})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "ok\n" {
		t.Fatalf("body = %q, want %q", response.Body.String(), "ok\n")
	}
}

func TestHealthHandlerDatabaseUnavailable(t *testing.T) {
	handler := healthHandler(func(context.Context) error {
		return errors.New("private database error")
	})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if response.Body.String() != "unavailable\n" {
		t.Fatalf("body = %q, want %q", response.Body.String(), "unavailable\n")
	}
}
