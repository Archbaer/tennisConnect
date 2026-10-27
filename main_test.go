package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() accepted an empty DATABASE_URL")
	}
}

func TestLoadConfigDefaultsPort(t *testing.T) {
	const databaseURL = "postgres://user:password@localhost:5432/tennisconnect"
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("PORT", "")

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

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536"} {
		t.Run(port, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/tennisconnect")
			t.Setenv("PORT", port)

			if _, err := loadConfig(); err == nil {
				t.Fatalf("loadConfig() accepted PORT=%q", port)
			}
		})
	}
}

func TestLoadConfigRejectsNonPostgresURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "https://localhost/tennisconnect")
	t.Setenv("PORT", "")

	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() accepted a non-PostgreSQL DATABASE_URL")
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
