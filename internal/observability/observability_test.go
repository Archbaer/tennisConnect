package observability

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareLogsSafeRequestFields(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := New("info", "json", &buffer)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	handler := Middleware(logger, mux)
	request := httptest.NewRequest(http.MethodGet, "/ok?token=super-secret", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &event); err != nil {
		t.Fatalf("decode log: %v: %s", err, buffer.String())
	}
	if event["event"] != "http_request" || event["route"] != "GET /ok" || event["status"] != float64(201) {
		t.Fatalf("unexpected event: %+v", event)
	}
	if strings.Contains(buffer.String(), "super-secret") {
		t.Fatal("query string leaked into logs")
	}
}

func TestMiddlewareLogsActorReference(t *testing.T) {
	var buffer bytes.Buffer
	logger, _ := New("info", "json", &buffer)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /actor", func(w http.ResponseWriter, r *http.Request) {
		WithActor(r.Context(), "00000000-0000-4000-8000-000000000001")
	})
	Middleware(logger, mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/actor", nil))
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &event); err != nil {
		t.Fatal(err)
	}
	ref, _ := event["actor_ref"].(string)
	if len(ref) != 16 {
		t.Fatalf("actor_ref = %q", ref)
	}
	if strings.Contains(ref, "0000") {
		t.Fatal("raw user id leaked")
	}
}

func TestMiddlewareRecoversPanic(t *testing.T) {
	var buffer bytes.Buffer
	logger, _ := New("info", "json", &buffer)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	response := httptest.NewRecorder()
	Middleware(logger, mux).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if !strings.Contains(buffer.String(), `"msg":"panic"`) {
		t.Fatalf("panic not logged: %s", buffer.String())
	}
	if strings.Contains(buffer.String(), "boom") {
		t.Fatal("panic value leaked")
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	if _, err := New("verbose", "json", &bytes.Buffer{}); err == nil {
		t.Fatal("invalid level accepted")
	}
	if _, err := New("info", "xml", &bytes.Buffer{}); err == nil {
		t.Fatal("invalid format accepted")
	}
}
