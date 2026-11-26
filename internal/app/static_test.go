package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>site</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log('ok')"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSPAHandlerServesAssetsAndFallback(t *testing.T) {
	handler := spaHandler(testSite(t))
	for _, test := range []struct {
		path string
		body string
	}{
		{path: "/", body: "site"},
		{path: "/discover", body: "site"},
		{path: "/reviewer", body: "site"},
		{path: "/app.js", body: "console.log"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", test.path, response.Code)
		}
		if !strings.Contains(response.Body.String(), test.body) {
			t.Fatalf("GET %s body = %q", test.path, response.Body.String())
		}
		if response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("GET %s missing nosniff", test.path)
		}
		if !strings.Contains(response.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatalf("GET %s missing frame-ancestors", test.path)
		}
	}
}

func TestSPAHandlerDoesNotShadowAPIs(t *testing.T) {
	handler := spaHandler(testSite(t))
	for _, path := range []string{"/api/v1/unknown", "/auth/verify", "/reviewer/verify", "/healthz"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, response.Code)
		}
		if strings.Contains(response.Body.String(), "site") {
			t.Fatalf("GET %s served the SPA fallback", path)
		}
	}
}

func TestSPAHandlerRejectsDotfilesAndWrites(t *testing.T) {
	handler := spaHandler(testSite(t))
	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/.env"},
		{method: http.MethodGet, path: "/../go.mod"},
		{method: http.MethodPost, path: "/discover"},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want 404", test.method, test.path, response.Code)
		}
	}
}

func TestValidateWebDir(t *testing.T) {
	if err := validateWebDir(""); err != nil {
		t.Fatal(err)
	}
	if err := validateWebDir(t.TempDir()); err == nil {
		t.Fatal("empty directory accepted")
	}
	if err := validateWebDir(testSite(t)); err != nil {
		t.Fatal(err)
	}
}
