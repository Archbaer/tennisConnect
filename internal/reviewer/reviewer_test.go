package reviewer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

func newTestReviewer(t *testing.T) (*Service, *[]string) {
	t.Helper()
	testutil.ResetTables(t)
	links := &[]string{}
	svc := &Service{
		DB:                 testutil.DB,
		AppBaseURL:         "http://localhost:8080",
		EmailEncryptionKey: bytes.Repeat([]byte{1}, 32),
		EmailHMACKey:       bytes.Repeat([]byte{2}, 32),
		SendMagicLink: func(_ context.Context, _ string, link string) error {
			*links = append(*links, link)
			return nil
		},
	}
	return svc, links
}

func TestTOTPValidationAndReplay(t *testing.T) {
	secret, err := newTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	code, err := totpCode(secret, now.Unix()/totpPeriod)
	if err != nil {
		t.Fatal(err)
	}
	step, ok := validateTOTP(secret, code, now, 0)
	if !ok {
		t.Fatal("valid code rejected")
	}
	if _, ok := validateTOTP(secret, code, now, step); ok {
		t.Fatal("replayed code accepted")
	}
	if _, ok := validateTOTP(secret, "00000x", now, 0); ok {
		t.Fatal("non-numeric code accepted")
	}
}

func TestSecretEncryption(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	encrypted, err := encryptSecret("JBSWY3DPEHPK3PXP", key)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decryptSecret(encrypted, key)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("secret = %q", plain)
	}
	if _, err := decryptSecret(encrypted, bytes.Repeat([]byte{4}, 32)); err == nil {
		t.Fatal("wrong key decrypted secret")
	}
}

func TestReviewerFlowRequiresAndAcceptsMFA(t *testing.T) {
	svc, links := newTestReviewer(t)
	if _, err := Create(context.Background(), testutil.DB, "review@example.com", svc.EmailEncryptionKey, svc.EmailHMACKey); err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(`{"email":"review@example.com"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/auth/link", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	svc.RequestLink(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("request link status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(*links) != 1 {
		t.Fatalf("links = %d", len(*links))
	}
	link, err := url.Parse((*links)[0])
	if err != nil {
		t.Fatal(err)
	}
	token := link.Query().Get("token")

	form := url.Values{"token": {token}}
	verify := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/auth/verify", strings.NewReader(form.Encode()))
	verify.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	verifyResponse := httptest.NewRecorder()
	svc.VerifyLink(verifyResponse, verify)
	if verifyResponse.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", verifyResponse.Code, verifyResponse.Body.String())
	}
	cookies := verifyResponse.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no reviewer session cookie")
	}
	cookie := cookies[0]

	// A session that has not passed MFA cannot reach review handlers.
	protected := svc.RequireReviewer(func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	})
	blocked := httptest.NewRequest(http.MethodGet, "/api/v1/reviewer/reports", nil)
	blocked.AddCookie(cookie)
	blockedResponse := httptest.NewRecorder()
	protected(blockedResponse, blocked)
	if blockedResponse.Code != http.StatusForbidden {
		t.Fatalf("pre-MFA status = %d, want 403", blockedResponse.Code)
	}

	enroll := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/mfa/enroll", nil)
	enroll.AddCookie(cookie)
	enrollResponse := httptest.NewRecorder()
	svc.EnrollMFA(enrollResponse, enroll)
	if enrollResponse.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, body = %s", enrollResponse.Code, enrollResponse.Body.String())
	}
	var enrollment struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(enrollResponse.Body.Bytes(), &enrollment); err != nil {
		t.Fatal(err)
	}
	if enrollment.Secret == "" {
		t.Fatal("empty secret")
	}
	code, err := totpCode(enrollment.Secret, time.Now().Unix()/totpPeriod)
	if err != nil {
		t.Fatal(err)
	}
	mfaBody := strings.NewReader(`{"code":"` + code + `"}`)
	mfa := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/mfa/verify", mfaBody)
	mfa.Header.Set("Content-Type", "application/json")
	mfa.AddCookie(cookie)
	mfaResponse := httptest.NewRecorder()
	svc.VerifyMFA(mfaResponse, mfa)
	if mfaResponse.Code != http.StatusOK {
		t.Fatalf("mfa status = %d, body = %s", mfaResponse.Code, mfaResponse.Body.String())
	}

	allowed := httptest.NewRequest(http.MethodGet, "/api/v1/reviewer/reports", nil)
	allowed.AddCookie(cookie)
	allowedResponse := httptest.NewRecorder()
	protected(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent {
		t.Fatalf("post-MFA status = %d, want 204", allowedResponse.Code)
	}
}

func TestReviewerRequestLinkDoesNotEnumerate(t *testing.T) {
	svc, links := newTestReviewer(t)
	body := strings.NewReader(`{"email":"missing@example.com"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/auth/link", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	svc.RequestLink(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if len(*links) != 0 {
		t.Fatal("unknown address received a link")
	}
}

func TestReviewerOriginsAreChecked(t *testing.T) {
	svc, _ := newTestReviewer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reviewer/auth/link", strings.NewReader(`{"email":"review@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	svc.RequestLink(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}
