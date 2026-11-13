package account

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

func newTestAccount(t *testing.T) *Service {
	t.Helper()
	testutil.ResetTables(t)
	return &Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080"}
}

func insertProfile(t *testing.T, userID, name string) {
	t.Helper()
	if _, err := testutil.DB.Exec(`
		INSERT INTO profiles (user_id, display_name, gender, skill_level, bio, gender_preference, min_age, max_age)
		VALUES ($1, $2, 'woman', 'intermediate', 'bio', 'all', 18, 120)`, userID, name); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRevokesSessions(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "player@example.com")
	if _, err := testutil.DB.Exec(`
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, now() + interval '1 hour')`, bytes.Repeat([]byte{1}, 32), userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/account", nil)
	response := httptest.NewRecorder()
	svc.Delete(response, request, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var deleted bool
	if err := testutil.DB.QueryRow("SELECT deleted_at IS NOT NULL FROM users WHERE id = $1", userID).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("account not deleted")
	}
	var sessions int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM sessions WHERE user_id = $1", userID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatal("sessions survived deletion")
	}
	if !strings.Contains(response.Body.String(), "recover_until") {
		t.Fatalf("missing recovery boundary: %s", response.Body.String())
	}
}

func TestCleanupPurgesAfterRecoveryWindow(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "old@example.com")
	if _, err := testutil.DB.Exec("UPDATE users SET deleted_at = now() - interval '31 days' WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CleanupExpired(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM users WHERE id = $1", userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("expired account survived cleanup")
	}
}

func TestAppealWindowAndOneSubmission(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "banned@example.com")
	if _, err := testutil.DB.Exec(`
		UPDATE users SET banned_at = now(), ban_category = 'harassment' WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	body := `{"body":"I would like this reviewed."}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/account/appeal", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	svc.SubmitAppeal(response, request, userID)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	second := httptest.NewRequest(http.MethodPost, "/api/v1/account/appeal", strings.NewReader(body))
	second.Header.Set("Content-Type", "application/json")
	secondResponse := httptest.NewRecorder()
	svc.SubmitAppeal(secondResponse, second, userID)
	if secondResponse.Code != http.StatusConflict {
		t.Fatalf("second appeal status = %d", secondResponse.Code)
	}

	other := testutil.InsertUser(t, "late@example.com")
	if _, err := testutil.DB.Exec(`
		UPDATE users SET banned_at = now() - interval '15 days', ban_category = 'harassment' WHERE id = $1`, other); err != nil {
		t.Fatal(err)
	}
	late := httptest.NewRequest(http.MethodPost, "/api/v1/account/appeal", strings.NewReader(body))
	late.Header.Set("Content-Type", "application/json")
	lateResponse := httptest.NewRecorder()
	svc.SubmitAppeal(lateResponse, late, other)
	if lateResponse.Code != http.StatusForbidden {
		t.Fatalf("late appeal status = %d, want 403", lateResponse.Code)
	}
}

func TestAppealStatusIsOwnerOnlyByConstruction(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "banned@example.com")
	if _, err := testutil.DB.Exec(`
		UPDATE users SET banned_at = now(), ban_category = 'harassment' WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/account/appeal", nil)
	response := httptest.NewRecorder()
	svc.AppealStatus(response, request, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "deadline") {
		t.Fatalf("missing deadline: %s", response.Body.String())
	}
}

func TestExportIsOwnerScopedAndOmitsPrivateData(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "player@example.com")
	insertProfile(t, userID, "Exporter")
	if _, err := testutil.DB.Exec(`
		INSERT INTO availability (profile_id, weekday, start_time, end_time)
		VALUES ($1, 2, '18:00', '20:00')`, userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/account/export", nil)
	response := httptest.NewRecorder()
	svc.Export(response, request, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{"Exporter", "availability", "photos", "reports_made"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("export missing %q: %s", expected, body)
		}
	}
	for _, forbidden := range []string{"email", "birth_date", "object_key", "latitude", "longitude", "s3"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("export leaked %q: %s", forbidden, body)
		}
	}
	if response.Header().Get("Content-Disposition") == "" {
		t.Fatal("export is not an attachment")
	}
}

func TestDeleteIsNotCallableTwice(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "player@example.com")
	if _, err := testutil.DB.Exec("UPDATE users SET deleted_at = now() - interval '1 day' WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/account", nil)
	response := httptest.NewRecorder()
	svc.Delete(response, request, userID)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
}

func TestAppealDeadlineUsesBanTime(t *testing.T) {
	svc := newTestAccount(t)
	userID := testutil.InsertUser(t, "banned@example.com")
	bannedAt := time.Now().Add(-13 * 24 * time.Hour)
	if _, err := testutil.DB.Exec(`
		UPDATE users SET banned_at = $1, ban_category = 'harassment' WHERE id = $2`, bannedAt, userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/account/appeal", nil)
	response := httptest.NewRecorder()
	svc.AppealStatus(response, request, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"open":true`) {
		t.Fatalf("appeal should be open: %s", response.Body.String())
	}
}
