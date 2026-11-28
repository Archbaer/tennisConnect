package auth

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tennisconnect/internal/observability"
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

func TestAuthEventsAreLoggedWithoutAddressesOrTokens(t *testing.T) {
	auth, emails := newTestAuth(t)
	logs := captureDefaultLog(t)
	const address = "secret.player@example.com"
	userID := insertTestUser(t, auth, address)
	token := requestMagicToken(t, auth, emails, address)
	response := verifyTokenRequest(auth, token)
	if response.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", response.Code, response.Body.String())
	}
	cookie := responseSessionCookie(t, response)

	if replay := verifyTokenRequest(auth, token); replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", replay.Code)
	}

	signOut := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sign-out", nil)
	signOut.AddCookie(cookie)
	auth.SignOut(httptest.NewRecorder(), signOut)

	output := logs.String()
	for _, event := range []string{
		`"event":"auth.link_requested"`,
		`"event":"auth.login_succeeded"`,
		`"event":"auth.login_failed"`,
		`"event":"auth.logout"`,
	} {
		if !strings.Contains(output, event) {
			t.Fatalf("missing %s in logs: %s", event, output)
		}
	}
	if !strings.Contains(output, `"reason":"invalid_or_expired_token"`) {
		t.Fatalf("missing failure reason: %s", output)
	}
	actorRef := observability.ActorRef(userID)
	if !strings.Contains(output, `"actor_ref":"`+actorRef+`"`) {
		t.Fatalf("missing actor_ref %s: %s", actorRef, output)
	}
	for _, secret := range []string{address, token, cookie.Value} {
		if secret != "" && strings.Contains(output, secret) {
			t.Fatalf("secret leaked into logs: %s", output)
		}
	}
}

func TestAuthRateLimitAndUnderageAreEnumsNotInput(t *testing.T) {
	auth, emails := newTestAuth(t)
	logs := captureDefaultLog(t)
	token := requestMagicToken(t, auth, emails, "adult@example.com")
	underage := registerRequest(auth, token, "2015-01-01")
	if underage.Code != http.StatusBadRequest {
		t.Fatalf("underage status = %d", underage.Code)
	}
	output := logs.String()
	if !strings.Contains(output, `"reason":"underage"`) {
		t.Fatalf("missing underage reason: %s", output)
	}
	if strings.Contains(output, "2015-01-01") {
		t.Fatalf("date of birth leaked: %s", output)
	}
}
