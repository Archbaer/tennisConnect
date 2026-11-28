package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buffer bytes.Buffer
	logger, err := New("debug", "json", &buffer)
	if err != nil {
		t.Fatal(err)
	}
	return logger, &buffer
}

// bannedKeys mirrors the documented redaction list. Every one must be
// normalised, matched case-insensitively and replaced.
var bannedKeys = []string{
	"password", "passwd", "secret", "token", "authorization", "cookie",
	"set-cookie", "session", "email", "e_mail", "address", "phone", "dob",
	"birth", "bio", "details", "instagram", "url", "upload_url", "presigned",
	"api_key", "apikey", "key", "signature", "latitude", "longitude", "lat",
	"lng", "coordinates", "ip",
}

func TestBannedKeysAreRedacted(t *testing.T) {
	for _, key := range bannedKeys {
		t.Run(key, func(t *testing.T) {
			logger, buffer := testLogger(t)
			logger.Info("probe", key, "SENSITIVE-VALUE")
			if strings.Contains(buffer.String(), "SENSITIVE-VALUE") {
				t.Fatalf("%q leaked: %s", key, buffer.String())
			}
			if !strings.Contains(buffer.String(), redacted) {
				t.Fatalf("%q not redacted: %s", key, buffer.String())
			}
		})
	}
}

func TestBannedKeysAreCaseInsensitive(t *testing.T) {
	logger, buffer := testLogger(t)
	logger.Info("probe", "Authorization", "SENSITIVE-VALUE", "E-MAIL", "other-secret")
	output := buffer.String()
	for _, leaked := range []string{"SENSITIVE-VALUE", "other-secret"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("%q leaked: %s", leaked, output)
		}
	}
}

func TestNestedGroupIsRedacted(t *testing.T) {
	logger, buffer := testLogger(t)
	logger.Info("probe", slog.Group("request",
		slog.String("email", "nested@example.com"),
		slog.Group("auth", slog.String("session_id", "abc123")),
		slog.String("route", "POST /api/v1/auth/verify"),
	))
	output := buffer.String()
	if strings.Contains(output, "nested@example.com") || strings.Contains(output, "abc123") {
		t.Fatalf("nested group leaked: %s", output)
	}
	if !strings.Contains(output, "POST /api/v1/auth/verify") {
		t.Fatalf("legitimate nested field was dropped: %s", output)
	}
}

func TestInnocentKeysWithSensitiveValuesAreRedacted(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "email", value: "contact alex@example.com now"},
		{name: "presigned url", value: "https://bucket.s3.eu-west-2.amazonaws.com/photo?X-Amz-Signature=deadbeef"},
		{name: "bearer token", value: "Bearer abcdefghijklmnopqrstuvwxyz0123456789"},
		{name: "opaque token", value: "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_ab"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger, buffer := testLogger(t)
			logger.Info("probe", "note", test.value)
			if strings.Contains(buffer.String(), test.value) {
				t.Fatalf("value leaked: %s", buffer.String())
			}
			if !strings.Contains(buffer.String(), redacted) {
				t.Fatalf("value not redacted: %s", buffer.String())
			}
		})
	}
}

func TestLegitimateFieldsPassThrough(t *testing.T) {
	logger, buffer := testLogger(t)
	const requestID = "0123456789abcdef0123456789abcdef"
	logger.Info("http_request",
		"event", "http_request",
		"request_id", requestID,
		"route", "GET /api/v1/discovery",
		"status", 200,
		"actor_ref", "fedcba9876543210",
		"duration_ms", int64(12),
	)
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &event); err != nil {
		t.Fatalf("decode log: %v: %s", err, buffer.String())
	}
	for key, want := range map[string]any{
		"event":       "http_request",
		"request_id":  requestID,
		"route":       "GET /api/v1/discovery",
		"status":      float64(200),
		"actor_ref":   "fedcba9876543210",
		"duration_ms": float64(12),
	} {
		if event[key] != want {
			t.Fatalf("%s = %v, want %v; log: %s", key, event[key], want, buffer.String())
		}
	}
}

func TestMiddlewareNeverLogsQueryHeadersCookieOrBody(t *testing.T) {
	logger, buffer := testLogger(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /secret", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "tennisconnect_session", Value: "response-cookie-secret"})
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/secret?token=query-token-secret&email=query@example.com",
		strings.NewReader(`{"email":"body@example.com","password":"body-password-secret"}`))
	request.Header.Set("Authorization", "Bearer header-token-secret")
	request.Header.Set("Cookie", "tennisconnect_session=request-cookie-secret")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	Middleware(logger, mux).ServeHTTP(recorder, request)

	output := buffer.String()
	for _, leaked := range []string{
		"query-token-secret",
		"query@example.com",
		"body@example.com",
		"body-password-secret",
		"header-token-secret",
		"request-cookie-secret",
		"response-cookie-secret",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("%q leaked into request log: %s", leaked, output)
		}
	}
	if !strings.Contains(output, `"event":"http_request"`) {
		t.Fatalf("request event missing: %s", output)
	}
}
