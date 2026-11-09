package config

import (
	"encoding/base64"
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
	t.Setenv("S3_BUCKET", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("SIGHTENGINE_API_USER", "")
	t.Setenv("SIGHTENGINE_API_SECRET", "")
	t.Setenv("MAPBOX_ACCESS_TOKEN", "")
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

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an empty DATABASE_URL")
	}
}

func TestLoadConfigDefaultsPort(t *testing.T) {
	setValidConfigEnv(t)
	const databaseURL = "postgres://user:password@localhost:5432/tennisconnect"
	t.Setenv("DATABASE_URL", databaseURL)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if got.DatabaseURL != databaseURL {
		t.Fatalf("DatabaseURL = %q, want %q", got.DatabaseURL, databaseURL)
	}
	if got.Address != ":8080" {
		t.Fatalf("Address = %q, want :8080", got.Address)
	}
}

func TestLoadConfigAuthSettings(t *testing.T) {
	setValidConfigEnv(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if got.AppBaseURL != "http://localhost:8080" {
		t.Fatalf("AppBaseURL = %q", got.AppBaseURL)
	}
	if len(got.EmailEncryptionKey) != 32 || len(got.EmailHMACKey) != 32 {
		t.Fatal("Load() did not decode 32-byte auth keys")
	}
	if got.CookieSecure {
		t.Fatal("CookieSecure = true, want false for local configuration")
	}
	if got.TrustProxy {
		t.Fatal("TrustProxy = true, want false")
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
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%q", test.name, test.value)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536"} {
		t.Run(port, func(t *testing.T) {
			setValidConfigEnv(t)
			t.Setenv("PORT", port)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted PORT=%q", port)
			}
		})
	}
}

func TestLoadConfigRejectsNonPostgresURL(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("DATABASE_URL", "https://localhost/tennisconnect")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a non-PostgreSQL DATABASE_URL")
	}
}

func TestLoadConfigSecureDefaults(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("TRUST_PROXY_HEADERS", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !got.CookieSecure {
		t.Fatal("CookieSecure defaulted to false")
	}
	if got.TrustProxy {
		t.Fatal("TrustProxy defaulted to true")
	}
}

func TestLoadConfigS3Settings(t *testing.T) {
	setValidConfigEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() without S3: %v", err)
	}
	if cfg.S3Bucket != "" || cfg.S3Region != "" {
		t.Fatalf("unexpected S3 config: bucket %q region %q", cfg.S3Bucket, cfg.S3Region)
	}

	t.Setenv("S3_BUCKET", "tennisconnect-photos")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted S3_BUCKET without AWS_REGION")
	}

	t.Setenv("AWS_REGION", "eu-west-2")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with S3: %v", err)
	}
	if cfg.S3Bucket != "tennisconnect-photos" || cfg.S3Region != "eu-west-2" {
		t.Fatalf("S3 config = bucket %q region %q", cfg.S3Bucket, cfg.S3Region)
	}
}

func TestLoadConfigModerationIsBothOrNeither(t *testing.T) {
	setValidConfigEnv(t)
	if _, err := Load(); err != nil {
		t.Fatalf("Load() without moderation: %v", err)
	}

	t.Setenv("SIGHTENGINE_API_USER", "user")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a moderation user without a secret")
	}

	t.Setenv("SIGHTENGINE_API_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with moderation: %v", err)
	}
	if cfg.SightengineUser != "user" || cfg.SightengineSecret != "secret" {
		t.Fatalf("moderation config = %q / %q", cfg.SightengineUser, cfg.SightengineSecret)
	}
}

func TestLoadConfigMapboxTokenIsOptional(t *testing.T) {
	setValidConfigEnv(t)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() without a token: %v", err)
	}
	if got.MapboxAccessToken != "" {
		t.Fatalf("MapboxAccessToken = %q, want empty", got.MapboxAccessToken)
	}

	t.Setenv("MAPBOX_ACCESS_TOKEN", "  pk.test-token  ")
	got, err = Load()
	if err != nil {
		t.Fatalf("Load() with a token: %v", err)
	}
	if got.MapboxAccessToken != "pk.test-token" {
		t.Fatalf("MapboxAccessToken = %q", got.MapboxAccessToken)
	}
}
