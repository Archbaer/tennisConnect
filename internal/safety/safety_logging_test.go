package safety

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestSafetyEventsAreLoggedWithoutDetails(t *testing.T) {
	svc := newTestSafety(t)
	logs := captureDefaultLog(t)
	reporter := testutil.InsertUser(t, "reporter@example.com")
	target := testutil.InsertUser(t, "target@example.com")
	insertProfile(t, target, "Target")

	const details = "Call me on 07700 900123 or write to reporter@example.com"
	if response := reportRequest(svc, reporter, target, "harassment", details); response.Code != http.StatusCreated {
		t.Fatalf("report status = %d, body = %s", response.Code, response.Body.String())
	}
	output := logs.String()
	if !strings.Contains(output, `"event":"safety.report_submitted"`) {
		t.Fatalf("missing report event: %s", output)
	}
	if !strings.Contains(output, `"category":"harassment"`) {
		t.Fatalf("missing category: %s", output)
	}
	if !strings.Contains(output, `"actor_ref":"`+observability.ActorRef(reporter)+`"`) ||
		!strings.Contains(output, `"target_ref":"`+observability.ActorRef(target)+`"`) {
		t.Fatalf("missing actor/target refs: %s", output)
	}
	for _, secret := range []string{details, "07700 900123", "reporter@example.com", "target@example.com"} {
		if strings.Contains(output, secret) {
			t.Fatalf("report detail leaked: %s", output)
		}
	}

	var reportID string
	if err := testutil.DB.QueryRow("SELECT id FROM reports").Scan(&reportID); err != nil {
		t.Fatal(err)
	}
	reviewerID := insertReviewer(t)

	dismiss := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/reports/"+reportID+"/dismiss", nil)
	dismiss.SetPathValue("id", reportID)
	dismissResponse := httptest.NewRecorder()
	svc.Dismiss(dismissResponse, dismiss, reviewerID)
	if dismissResponse.Code != http.StatusOK {
		t.Fatalf("dismiss status = %d", dismissResponse.Code)
	}

	if _, err := testutil.DB.Exec("UPDATE reports SET status = 'dismissed' WHERE id = $1", reportID); err != nil {
		t.Fatal(err)
	}
	if response := reportRequest(svc, reporter, target, "underage", ""); response.Code != http.StatusCreated {
		t.Fatalf("second report status = %d", response.Code)
	}
	var secondID string
	if err := testutil.DB.QueryRow("SELECT id FROM reports WHERE status = 'open'").Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	ban := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/reports/"+secondID+"/ban", nil)
	ban.SetPathValue("id", secondID)
	banResponse := httptest.NewRecorder()
	svc.Ban(banResponse, ban, reviewerID)
	if banResponse.Code != http.StatusOK {
		t.Fatalf("ban status = %d", banResponse.Code)
	}

	output = logs.String()
	for _, event := range []string{
		`"event":"safety.report_dismissed"`,
		`"event":"safety.reviewer_banned"`,
	} {
		if !strings.Contains(output, event) {
			t.Fatalf("missing %s: %s", event, output)
		}
	}
	if !strings.Contains(output, `"actor_ref":"`+observability.ActorRef(reviewerID)+`"`) {
		t.Fatalf("missing reviewer ref: %s", output)
	}
}
