package safety

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tennisconnect/internal/testutil"
)

func insertReviewer(t *testing.T) string {
	t.Helper()
	token := sha256.Sum256([]byte("reviewer@example.com"))
	var id string
	if err := testutil.DB.QueryRow(`
		INSERT INTO reviewers (email_encrypted, email_lookup_hmac)
		VALUES ($1, $2) RETURNING id`, []byte{1}, token[:]).Scan(&id); err != nil {
		t.Fatalf("insert reviewer: %v", err)
	}
	return id
}

func newTestSafety(t *testing.T) *Service {
	t.Helper()
	testutil.ResetTables(t)
	return &Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080", HMACKey: bytes.Repeat([]byte{2}, 32)}
}

func insertProfile(t *testing.T, userID, name string) {
	t.Helper()
	if _, err := testutil.DB.Exec(`
		INSERT INTO profiles (user_id, display_name, gender, skill_level, bio, gender_preference, min_age, max_age)
		VALUES ($1, $2, 'woman', 'intermediate', '', 'all', 18, 120)`, userID, name); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
}

func reportRequest(svc *Service, reporterID, targetID, category, details string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"target_id":%q,"category":%q,"details":%q}`, targetID, category, details)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	svc.Report(response, request, reporterID)
	return response
}

func TestReportCreationAndDuplicate(t *testing.T) {
	svc := newTestSafety(t)
	reporter := testutil.InsertUser(t, "reporter@example.com")
	target := testutil.InsertUser(t, "target@example.com")
	insertProfile(t, target, "Target")

	response := reportRequest(svc, reporter, target, "harassment", "details")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if duplicate := reportRequest(svc, reporter, target, "harassment", ""); duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", duplicate.Code)
	}
	if self := reportRequest(svc, reporter, reporter, "other", ""); self.Code != http.StatusBadRequest {
		t.Fatalf("self report status = %d", self.Code)
	}
	if invalid := reportRequest(svc, reporter, target, "not-a-category", ""); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid category status = %d", invalid.Code)
	}
	var events int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM report_events WHERE action = 'submitted'").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("submitted events = %d, want 1", events)
	}
}

func TestReviewerQueueDoesNotExposeReporter(t *testing.T) {
	svc := newTestSafety(t)
	reporter := testutil.InsertUser(t, "reporter@example.com")
	target := testutil.InsertUser(t, "target@example.com")
	insertProfile(t, target, "Target")
	if response := reportRequest(svc, reporter, target, "fake_profile", ""); response.Code != http.StatusCreated {
		t.Fatalf("report status = %d", response.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/reviewer/reports", nil)
	response := httptest.NewRecorder()
	svc.List(response, request, "unused")
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), reporter) || strings.Contains(response.Body.String(), "reporter_id") {
		t.Fatalf("reviewer queue exposed reporter: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Target") {
		t.Fatalf("reviewer queue missing target: %s", response.Body.String())
	}
}

func TestDismissAndBanTransitions(t *testing.T) {
	svc := newTestSafety(t)
	reporter := testutil.InsertUser(t, "reporter@example.com")
	target := testutil.InsertUser(t, "target@example.com")
	insertProfile(t, target, "Target")
	reportRequest(svc, reporter, target, "harassment", "")
	var reportID string
	if err := testutil.DB.QueryRow("SELECT id FROM reports").Scan(&reportID); err != nil {
		t.Fatal(err)
	}
	reviewerID := insertReviewer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/reports/"+reportID+"/dismiss", nil)
	request.SetPathValue("id", reportID)
	response := httptest.NewRecorder()
	svc.Dismiss(response, request, reviewerID)
	if response.Code != http.StatusOK {
		t.Fatalf("dismiss status = %d, body = %s", response.Code, response.Body.String())
	}
	if repeat := httptest.NewRecorder(); func() int {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/reports/"+reportID+"/dismiss", nil)
		r.SetPathValue("id", reportID)
		svc.Dismiss(repeat, r, reviewerID)
		return repeat.Code
	}() != http.StatusConflict {
		t.Fatal("reviewed report accepted a second decision")
	}

	// A fresh report can ban the same target.
	if _, err := testutil.DB.Exec("UPDATE reports SET status = 'dismissed' WHERE id = $1", reportID); err != nil {
		t.Fatal(err)
	}
	second := reportRequest(svc, reporter, target, "underage", "")
	if second.Code != http.StatusCreated {
		t.Fatalf("second report status = %d, body = %s", second.Code, second.Body.String())
	}
	if _, err := testutil.DB.Exec(`
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, now() + interval '1 hour')`, bytes.Repeat([]byte{9}, 32), target); err != nil {
		t.Fatal(err)
	}
	if _, err := testutil.DB.Exec(`
		INSERT INTO matches (user_low, user_high)
		SELECT LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid)`, reporter, target); err != nil {
		t.Fatal(err)
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
		t.Fatalf("ban status = %d, body = %s", banResponse.Code, banResponse.Body.String())
	}
	var banned bool
	if err := testutil.DB.QueryRow("SELECT banned_at IS NOT NULL FROM users WHERE id = $1", target).Scan(&banned); err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Fatal("ban marker missing")
	}
	var sessions int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM sessions WHERE user_id = $1", target).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatal("ban did not revoke sessions")
	}
	var matches int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM matches WHERE user_low = $1 OR user_high = $1", target).Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if matches != 0 {
		t.Fatal("ban did not close matches")
	}
}

func TestReportRateLimit(t *testing.T) {
	svc := newTestSafety(t)
	reporter := testutil.InsertUser(t, "reporter@example.com")
	for i := 0; i < reportLimit; i++ {
		target := testutil.InsertUser(t, fmt.Sprintf("target-%d@example.com", i))
		insertProfile(t, target, fmt.Sprintf("Target %d", i))
		if response := reportRequest(svc, reporter, target, "other", ""); response.Code != http.StatusCreated {
			t.Fatalf("report %d status = %d", i, response.Code)
		}
	}
	target := testutil.InsertUser(t, "target-final@example.com")
	insertProfile(t, target, "Target final")
	response := reportRequest(svc, reporter, target, "other", "")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", response.Code)
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}
