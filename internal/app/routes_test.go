package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tennisconnect/internal/auth"
	"tennisconnect/internal/discovery"
	"tennisconnect/internal/location"
	"tennisconnect/internal/matches"
	"tennisconnect/internal/photos"
	"tennisconnect/internal/testutil"
)

func newTestServices(t *testing.T) (*auth.Service, *photos.Service, *location.Service, *discovery.Service, *matches.Service) {
	t.Helper()
	testutil.ResetTables(t)
	authSvc := &auth.Service{
		DB:                 testutil.DB,
		AppBaseURL:         "http://localhost:8080",
		EmailEncryptionKey: bytes.Repeat([]byte{1}, 32),
		EmailHMACKey:       bytes.Repeat([]byte{2}, 32),
		SendMagicLink:      func(context.Context, string, string) error { return nil },
	}
	return authSvc,
		&photos.Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080"},
		&location.Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080"},
		&discovery.Service{DB: testutil.DB},
		&matches.Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080"}
}

func insertTestSession(t *testing.T, userID string) *http.Cookie {
	t.Helper()
	sum := sha256.Sum256([]byte(fmt.Sprintf("test-session-%s", userID)))
	token := base64.RawURLEncoding.EncodeToString(sum[:])
	hash := sha256.Sum256([]byte(token))
	if _, err := testutil.DB.Exec("INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, now() + interval '1 hour')", hash[:], userID); err != nil {
		t.Fatalf("insert test session: %v", err)
	}
	return &http.Cookie{Name: "tennisconnect_session", Value: token}
}

func authenticatedRequest(t *testing.T, method, path, body string, cookie *http.Cookie) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	return request
}

func TestMuxRoutesAuthEndpoints(t *testing.T) {
	authSvc, photoSvc, locationSvc, discoverySvc, matchSvc := newTestServices(t)
	handler := NewMux(authSvc, photoSvc, locationSvc, discoverySvc, matchSvc, testutil.DB.PingContext)
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		want        int
	}{
		{name: "health", method: http.MethodGet, path: "/healthz", want: http.StatusOK},
		{name: "request link", method: http.MethodPost, path: "/api/v1/auth/link", body: `{"email":"player@example.com"}`, contentType: "application/json", want: http.StatusOK},
		{name: "confirm", method: http.MethodGet, path: "/auth/verify?token=invalid", want: http.StatusBadRequest},
		{name: "verify", method: http.MethodPost, path: "/api/v1/auth/verify", body: "token=invalid", contentType: "application/x-www-form-urlencoded", want: http.StatusUnauthorized},
		{name: "register", method: http.MethodPost, path: "/api/v1/auth/register", body: "token=invalid&birth_date=2000-01-01", contentType: "application/x-www-form-urlencoded", want: http.StatusUnauthorized},
		{name: "sign out", method: http.MethodPost, path: "/api/v1/auth/sign-out", want: http.StatusOK},
		{name: "session", method: http.MethodGet, path: "/api/v1/auth/session", want: http.StatusUnauthorized},
		{name: "get profile unauthorized", method: http.MethodGet, path: "/api/v1/profile", want: http.StatusUnauthorized},
		{name: "put profile unauthorized", method: http.MethodPut, path: "/api/v1/profile", body: `{}`, contentType: "application/json", want: http.StatusUnauthorized},
		{name: "upload url unauthorized", method: http.MethodPost, path: "/api/v1/photos/upload-url", want: http.StatusUnauthorized},
		{name: "process photo unauthorized", method: http.MethodPost, path: "/api/v1/photos/00000000-0000-4000-8000-000000000000/process", want: http.StatusUnauthorized},
		{name: "list photos unauthorized", method: http.MethodGet, path: "/api/v1/photos", want: http.StatusUnauthorized},
		{name: "photo url unauthorized", method: http.MethodGet, path: "/api/v1/photos/00000000-0000-4000-8000-000000000000/url", want: http.StatusUnauthorized},
		{name: "discovery unauthorized", method: http.MethodGet, path: "/api/v1/discovery", want: http.StatusUnauthorized},
		{name: "reaction unauthorized", method: http.MethodPost, path: "/api/v1/reactions", body: `{}`, contentType: "application/json", want: http.StatusUnauthorized},
		{name: "match list unauthorized", method: http.MethodGet, path: "/api/v1/matches", want: http.StatusUnauthorized},
		{name: "match detail unauthorized", method: http.MethodGet, path: "/api/v1/matches/00000000-0000-4000-8000-000000000000", want: http.StatusUnauthorized},
		{name: "match instagram unauthorized", method: http.MethodGet, path: "/api/v1/matches/00000000-0000-4000-8000-000000000000/instagram", want: http.StatusUnauthorized},
		{name: "block unauthorized", method: http.MethodPost, path: "/api/v1/blocks", body: `{}`, contentType: "application/json", want: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.RemoteAddr = "192.0.2.10:1234"
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestNoPublicRouteServesPhotoObjects(t *testing.T) {
	authSvc := &auth.Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080"}
	handler := NewMux(authSvc, &photos.Service{DB: testutil.DB}, &location.Service{DB: testutil.DB}, &discovery.Service{DB: testutil.DB}, &matches.Service{DB: testutil.DB}, func(context.Context) error { return nil })
	for _, path := range []string{"/api/v1/photos/00000000-0000-4000-8000-000000000000", "/api/v1/photos/00000000-0000-4000-8000-000000000000/process"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s status = %d", path, response.Code)
		}
		if response.Body.Len() > 0 && bytes.HasPrefix(response.Body.Bytes(), []byte{0xff, 0xd8}) {
			t.Fatalf("GET %s served image bytes", path)
		}
	}
}

func TestLocationRoutesRequireSession(t *testing.T) {
	authSvc, photoSvc, locationSvc, discoverySvc, matchSvc := newTestServices(t)
	handler := NewMux(authSvc, photoSvc, locationSvc, discoverySvc, matchSvc, testutil.DB.PingContext)
	userID := testutil.InsertUser(t, "routes@example.com")
	cookie := insertTestSession(t, userID)

	for _, test := range []struct {
		name string
		path string
	}{
		{name: "put location", path: "/api/v1/location"},
		{name: "get location", path: "/api/v1/location"},
		{name: "put travel", path: "/api/v1/location/travel"},
		{name: "delete travel", path: "/api/v1/location/travel"},
		{name: "discovery", path: "/api/v1/discovery"},
	} {
		t.Run("unauthenticated "+test.name, func(t *testing.T) {
			method := http.MethodGet
			if strings.HasPrefix(test.name, "put") {
				method = http.MethodPut
			}
			if strings.HasPrefix(test.name, "delete") {
				method = http.MethodDelete
			}
			request := httptest.NewRequest(method, test.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", response.Code)
			}
		})
	}

	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "put location", path: "/api/v1/location", body: `{"area":"Battersea"}`},
		{name: "put travel", path: "/api/v1/location/travel", body: `{"area":"Edinburgh"}`},
		{name: "delete travel", path: "/api/v1/location/travel", body: ""},
	} {
		t.Run("cross origin "+test.name, func(t *testing.T) {
			method := http.MethodPut
			if strings.HasPrefix(test.name, "delete") {
				method = http.MethodDelete
			}
			request := authenticatedRequest(t, method, test.path, test.body, cookie)
			request.Header.Set("Origin", "https://attacker.example")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", response.Code)
			}
		})
	}

	getRequest := authenticatedRequest(t, http.MethodGet, "/api/v1/location", "", cookie)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusNotFound {
		t.Fatalf("GET location status = %d, want 404", getResponse.Code)
	}
	if getResponse.Header().Get("Cache-Control") != "no-store" || getResponse.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("location response headers = %v", getResponse.Header())
	}

	discoveryRequest := authenticatedRequest(t, http.MethodGet, "/api/v1/discovery", "", cookie)
	discoveryResponse := httptest.NewRecorder()
	handler.ServeHTTP(discoveryResponse, discoveryRequest)
	if discoveryResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured discovery status = %d, want 503", discoveryResponse.Code)
	}
	if discoveryResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("discovery response headers = %v", discoveryResponse.Header())
	}
}
