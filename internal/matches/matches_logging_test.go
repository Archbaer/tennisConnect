package matches

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"tennisconnect/internal/observability"
	"tennisconnect/internal/testutil"
)

func captureDefaultLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	logger, err := observability.New("debug", "json", &buffer)
	if err != nil {
		t.Fatal(err)
	}
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buffer
}

func TestBlockEventUsesActorRefsOnly(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	logs := captureDefaultLog(t)
	blocker, blocked := insertUser(t, "logger-blocker"), insertUser(t, "logger-blocked")
	insertProfile(t, blocker, "Blocker", "")
	insertProfile(t, blocked, "Blocked", "")

	if response := block(t, service, blocker, blocked); response.Code != http.StatusNoContent {
		t.Fatalf("block status = %d, body = %s", response.Code, response.Body.String())
	}

	output := logs.String()
	if !strings.Contains(output, `"event":"safety.block_created"`) {
		t.Fatalf("missing block event: %s", output)
	}
	for _, ref := range []string{observability.ActorRef(blocker), observability.ActorRef(blocked)} {
		if !strings.Contains(output, ref) {
			t.Fatalf("missing actor ref %s: %s", ref, output)
		}
	}
	if strings.Contains(output, blocker) || strings.Contains(output, blocked) {
		t.Fatalf("raw user id leaked into logs: %s", output)
	}
}
