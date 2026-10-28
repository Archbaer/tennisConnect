package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMuxRoutesAuthEndpoints(t *testing.T) {
	auth, _ := newTestAuth(t)
	handler := newMux(auth, testDB.PingContext)
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
