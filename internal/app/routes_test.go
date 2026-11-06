package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tennisconnect/internal/auth"
	"tennisconnect/internal/photos"
	"tennisconnect/internal/testutil"
)

func newTestServices(t *testing.T) (*auth.Service, *photos.Service) {
	t.Helper()
	testutil.ResetTables(t)
	authSvc := &auth.Service{
		DB:                 testutil.DB,
		AppBaseURL:         "http://localhost:8080",
		EmailEncryptionKey: bytes.Repeat([]byte{1}, 32),
		EmailHMACKey:       bytes.Repeat([]byte{2}, 32),
		SendMagicLink:      func(context.Context, string, string) error { return nil },
	}
	return authSvc, &photos.Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080"}
}

func TestMuxRoutesAuthEndpoints(t *testing.T) {
	authSvc, photoSvc := newTestServices(t)
	handler := NewMux(authSvc, photoSvc, testutil.DB.PingContext)
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
	handler := NewMux(authSvc, &photos.Service{DB: testutil.DB}, func(context.Context) error { return nil })
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
