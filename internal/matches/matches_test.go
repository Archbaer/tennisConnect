package matches

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

var emailCounter atomic.Int64

type fakePresigner struct {
	mu    sync.Mutex
	calls []string
	fail  bool
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, key)
	if f.fail {
		return "", fmt.Errorf("presign failed")
	}
	return "https://tennisconnect-photos.s3.eu-west-2.amazonaws.com/" + key + "?X-Amz-Expires=300", nil
}

func (f *fakePresigner) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func newService(t *testing.T) (*Service, *fakePresigner) {
	t.Helper()
	presigner := &fakePresigner{}
	return &Service{DB: testutil.DB, AppBaseURL: "http://localhost:8080", Presigner: presigner}, presigner
}

func insertUser(t *testing.T, name string) string {
	t.Helper()
	email := fmt.Sprintf("%s-%d@example.com", name, emailCounter.Add(1))
	var userID string
	err := testutil.DB.QueryRow(`
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, last_active_at)
		VALUES ('\x01', decode(md5($1) || md5($2), 'hex'), '1990-01-01', now())
		RETURNING id`, email, "salt:"+email).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

func insertProfile(t *testing.T, userID, name, handle string) {
	t.Helper()
	var instagram any
	if handle != "" {
		instagram = handle
	}
	_, err := testutil.DB.Exec(`
		INSERT INTO profiles (user_id, display_name, gender, skill_level, bio,
			gender_preference, min_age, max_age, skill_tolerance, instagram_handle)
		VALUES ($1, $2, 'woman', 'intermediate', 'Hello', 'all', 18, 120, 1, $3)`,
		userID, name, instagram)
	if err != nil {
		t.Fatalf("insert profile: %v", err)
	}
}

func insertApprovedPhoto(t *testing.T, userID, key string, order int) {
	t.Helper()
	if _, err := testutil.DB.Exec(`
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
		VALUES ($1, $2, $3, 'approved')`, userID, key, order); err != nil {
		t.Fatalf("insert photo: %v", err)
	}
}

func react(t *testing.T, service *Service, actorID, targetID, action string) (*httptest.ResponseRecorder, reactionResponse) {
	t.Helper()
	body := fmt.Sprintf(`{"target_id":%q,"action":%q}`, targetID, action)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reactions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.CreateReaction(response, request, actorID)
	var decoded reactionResponse
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode reaction response: %v", err)
		}
	}
	return response, decoded
}

func listMatches(t *testing.T, service *Service, userID string) (*httptest.ResponseRecorder, matchListResponse) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	response := httptest.NewRecorder()
	service.List(response, request, userID)
	var decoded matchListResponse
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode matches: %v (body %s)", err, response.Body.String())
		}
	}
	return response, decoded
}

func matchDetail(t *testing.T, service *Service, userID, matchID string) (*httptest.ResponseRecorder, matchDetailResponse) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID, nil)
	request.SetPathValue("id", matchID)
	response := httptest.NewRecorder()
	service.Detail(response, request, userID)
	var decoded matchDetailResponse
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode match detail: %v (body %s)", err, response.Body.String())
		}
	}
	return response, decoded
}

func discloseHandle(t *testing.T, service *Service, userID, matchID string) (*httptest.ResponseRecorder, instagramResponse) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+matchID+"/instagram", nil)
	request.SetPathValue("id", matchID)
	response := httptest.NewRecorder()
	service.Instagram(response, request, userID)
	var decoded instagramResponse
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode instagram: %v (body %s)", err, response.Body.String())
		}
	}
	return response, decoded
}

func block(t *testing.T, service *Service, blockerID, blockedID string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"target_id":%q}`, blockedID)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/blocks", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.Block(response, request, blockerID)
	return response
}

func TestReactionLikeWithoutReciprocalDoesNotMatch(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	response, decoded := react(t, service, alex, priya, "like")
	if response.Code != http.StatusOK {
		t.Fatalf("like returned %d: %s", response.Code, response.Body.String())
	}
	if decoded.Matched {
		t.Fatal("one-way like must not create a match")
	}
	if decoded.MatchID != nil {
		t.Fatalf("one-way like returned match id %q", *decoded.MatchID)
	}
}

func TestReactionReciprocalLikeCreatesExactlyOneMatch(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	first, firstResponse := react(t, service, alex, priya, "like")
	if first.Code != http.StatusOK || firstResponse.Matched {
		t.Fatalf("first like: %d %+v", first.Code, firstResponse)
	}
	second, secondResponse := react(t, service, priya, alex, "like")
	if second.Code != http.StatusOK {
		t.Fatalf("reciprocal like returned %d: %s", second.Code, second.Body.String())
	}
	if !secondResponse.Matched || secondResponse.MatchID == nil {
		t.Fatalf("reciprocal like did not match: %+v", secondResponse)
	}

	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM matches").Scan(&count); err != nil {
		t.Fatalf("count matches: %v", err)
	}
	if count != 1 {
		t.Fatalf("matches = %d, want 1", count)
	}
}

func TestReactionRejectsDuplicateTargetAction(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	if code, _ := react(t, service, alex, priya, "like"); code.Code != http.StatusOK {
		t.Fatalf("first like returned %d", code.Code)
	}
	duplicate, _ := react(t, service, alex, priya, "pass")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate reaction returned %d, want 409", duplicate.Code)
	}
	var action string
	if err := testutil.DB.QueryRow("SELECT action FROM reactions WHERE actor_id = $1", alex).Scan(&action); err != nil {
		t.Fatalf("read reaction: %v", err)
	}
	if action != "like" {
		t.Fatalf("pass overwrote the first reaction: %q", action)
	}
}

func TestReactionRejectsSelfAndUnknownInput(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex := insertUser(t, "alex")
	insertProfile(t, alex, "Alex", "")

	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "self", body: fmt.Sprintf(`{"target_id":%q,"action":"like"}`, alex), want: http.StatusBadRequest},
		{name: "unknown action", body: `{"target_id":"00000000-0000-4000-8000-000000000001","action":"superlike"}`, want: http.StatusBadRequest},
		{name: "missing target", body: `{"action":"like"}`, want: http.StatusBadRequest},
		{name: "malformed uuid", body: `{"target_id":"nope","action":"like"}`, want: http.StatusBadRequest},
		{name: "unknown target", body: `{"target_id":"00000000-0000-4000-8000-000000000001","action":"like"}`, want: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/reactions", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			service.CreateReaction(response, request, alex)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d (body %s)", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestConcurrentReciprocalLikesCreateOneMatch(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	var wait sync.WaitGroup
	start := make(chan struct{})
	responses := make([]reactionResponse, 2)
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, responses[0] = react(t, service, alex, priya, "like")
	}()
	go func() {
		defer wait.Done()
		<-start
		_, responses[1] = react(t, service, priya, alex, "like")
	}()
	close(start)
	wait.Wait()

	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM matches").Scan(&count); err != nil {
		t.Fatalf("count matches: %v", err)
	}
	if count != 1 {
		t.Fatalf("concurrent reciprocal likes created %d matches, want 1", count)
	}
	// Exactly one caller learns about the match (the second like is the one that
	// completes the pair); both must still be able to read it afterwards.
	matched := 0
	for _, response := range responses {
		if response.Matched {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("%d of 2 responses reported a match, want exactly 1", matched)
	}
	for _, viewer := range []string{alex, priya} {
		if _, list := listMatches(t, service, viewer); len(list.Matches) != 1 {
			t.Fatalf("%s sees %d matches after a reciprocal like", viewer, len(list.Matches))
		}
	}
}

func TestListMatchesShowsOnlyOwnMatchesWithBooleanInstagram(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya, sam := insertUser(t, "alex"), insertUser(t, "priya"), insertUser(t, "sam")
	insertProfile(t, alex, "Alex", "alex.plays")
	insertProfile(t, priya, "Priya", "priya.plays")
	insertProfile(t, sam, "Sam", "")
	insertApprovedPhoto(t, priya, "photos/priya/1", 1)

	react(t, service, alex, priya, "like")
	react(t, service, priya, alex, "like")

	response, decoded := listMatches(t, service, alex)
	if response.Code != http.StatusOK {
		t.Fatalf("list returned %d: %s", response.Code, response.Body.String())
	}
	if len(decoded.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(decoded.Matches))
	}
	row := decoded.Matches[0]
	if row.DisplayName != "Priya" {
		t.Fatalf("display name = %q", row.DisplayName)
	}
	if !row.HasInstagram {
		t.Fatal("has_instagram should be true for a matched player with a handle")
	}
	if len(row.PhotoURLs) != 1 || !strings.HasPrefix(row.PhotoURLs[0], "https://") {
		t.Fatalf("photo URLs = %v", row.PhotoURLs)
	}
	if strings.Contains(response.Body.String(), "priya.plays") {
		t.Fatal("match list leaked the Instagram handle")
	}

	other, otherList := listMatches(t, service, sam)
	if other.Code != http.StatusOK || len(otherList.Matches) != 0 {
		t.Fatalf("uninvolved user sees %d matches", len(otherList.Matches))
	}
}

func TestMatchDetailIsMemberOnly(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya, sam := insertUser(t, "alex"), insertUser(t, "priya"), insertUser(t, "sam")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")
	insertProfile(t, sam, "Sam", "")
	react(t, service, alex, priya, "like")
	_, matched := react(t, service, priya, alex, "like")

	for _, viewer := range []string{alex, priya} {
		response, detail := matchDetail(t, service, viewer, *matched.MatchID)
		if response.Code != http.StatusOK {
			t.Fatalf("member %s detail returned %d", viewer, response.Code)
		}
		if detail.DisplayName == "" {
			t.Fatal("detail missing display name")
		}
	}
	outsider, _ := matchDetail(t, service, sam, *matched.MatchID)
	if outsider.Code != http.StatusNotFound {
		t.Fatalf("outsider detail returned %d, want 404", outsider.Code)
	}
}

func TestInstagramDisclosureIsMatchScopedAndOnDemand(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya, sam := insertUser(t, "alex"), insertUser(t, "priya"), insertUser(t, "sam")
	insertProfile(t, alex, "Alex", "alex.plays")
	insertProfile(t, priya, "Priya", "priya.plays")
	insertProfile(t, sam, "Sam", "")

	// Before any match the handle must not be readable.
	requestBefore := httptest.NewRequest(http.MethodGet, "/api/v1/matches/00000000-0000-4000-8000-000000000001/instagram", nil)
	requestBefore.SetPathValue("id", "00000000-0000-4000-8000-000000000001")
	before := httptest.NewRecorder()
	service.Instagram(before, requestBefore, alex)
	if before.Code != http.StatusNotFound {
		t.Fatalf("pre-match disclosure returned %d, want 404", before.Code)
	}

	react(t, service, alex, priya, "like")
	_, matched := react(t, service, priya, alex, "like")

	response, decoded := discloseHandle(t, service, alex, *matched.MatchID)
	if response.Code != http.StatusOK {
		t.Fatalf("disclosure returned %d: %s", response.Code, response.Body.String())
	}
	if decoded.InstagramHandle != "priya.plays" {
		t.Fatalf("handle = %q", decoded.InstagramHandle)
	}

	_, otherView := discloseHandle(t, service, priya, *matched.MatchID)
	if otherView.InstagramHandle != "alex.plays" {
		t.Fatalf("reverse handle = %q", otherView.InstagramHandle)
	}

	outsider, _ := discloseHandle(t, service, sam, *matched.MatchID)
	if outsider.Code != http.StatusNotFound {
		t.Fatalf("outsider disclosure returned %d, want 404", outsider.Code)
	}

	// The detail payload never carries the handle.
	_, detail := matchDetail(t, service, alex, *matched.MatchID)
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	if bytes.Contains(raw, []byte("priya.plays")) {
		t.Fatal("match detail leaked the Instagram handle")
	}
}

func TestNoInstagramMatchReturnsEmptyHandle(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	react(t, service, alex, priya, "like")
	_, matched := react(t, service, priya, alex, "like")

	response, decoded := discloseHandle(t, service, alex, *matched.MatchID)
	if response.Code != http.StatusOK {
		t.Fatalf("disclosure returned %d", response.Code)
	}
	if decoded.InstagramHandle != "" {
		t.Fatalf("handle = %q, want empty", decoded.InstagramHandle)
	}
}

func TestBlockClosesMatchAndRevokesBothDirections(t *testing.T) {
	testutil.ResetTables(t)
	service, presigner := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "alex.plays")
	insertProfile(t, priya, "Priya", "priya.plays")
	insertApprovedPhoto(t, priya, "photos/priya/1", 1)
	insertApprovedPhoto(t, alex, "photos/alex/1", 1)

	react(t, service, alex, priya, "like")
	_, matched := react(t, service, priya, alex, "like")
	if _, before := listMatches(t, service, alex); len(before.Matches) != 1 || len(before.Matches[0].PhotoURLs) == 0 {
		t.Fatalf("pre-block list = %+v, want one match with photo URLs", before.Matches)
	}

	if response := block(t, service, alex, priya); response.Code != http.StatusNoContent {
		t.Fatalf("block returned %d: %s", response.Code, response.Body.String())
	}

	// Both directions lose the match, the detail, the handle and the photos.
	for _, viewer := range []string{alex, priya} {
		_, list := listMatches(t, service, viewer)
		if len(list.Matches) != 0 {
			t.Fatalf("%s still sees %d matches after a block", viewer, len(list.Matches))
		}
		if response, _ := matchDetail(t, service, viewer, *matched.MatchID); response.Code != http.StatusNotFound {
			t.Fatalf("%s detail after block returned %d, want 404", viewer, response.Code)
		}
		if response, _ := discloseHandle(t, service, viewer, *matched.MatchID); response.Code != http.StatusNotFound {
			t.Fatalf("%s disclosure after block returned %d, want 404", viewer, response.Code)
		}
	}
	if len(presigner.recorded()) == 0 {
		t.Fatal("test did not exercise photo URL issuance before the block")
	}
}

func TestBlockIsIdempotentAndRejectsSelf(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	if response := block(t, service, alex, priya); response.Code != http.StatusNoContent {
		t.Fatalf("first block returned %d", response.Code)
	}
	if response := block(t, service, alex, priya); response.Code != http.StatusNoContent {
		t.Fatalf("repeat block returned %d, want idempotent 204", response.Code)
	}
	body := fmt.Sprintf(`{"target_id":%q}`, alex)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/blocks", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.Block(response, request, alex)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("self block returned %d, want 400", response.Code)
	}
}

func TestBlockedPairCannotCreateNewMatch(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	if response := block(t, service, alex, priya); response.Code != http.StatusNoContent {
		t.Fatalf("block returned %d", response.Code)
	}
	if response, _ := react(t, service, alex, priya, "like"); response.Code != http.StatusForbidden {
		t.Fatalf("blocked reaction returned %d, want 403", response.Code)
	}
	if response, _ := react(t, service, priya, alex, "like"); response.Code != http.StatusForbidden {
		t.Fatalf("reverse blocked reaction returned %d, want 403", response.Code)
	}
}

func TestMatchDetailIssuesOnlyApprovedPhotos(t *testing.T) {
	testutil.ResetTables(t)
	service, presigner := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")
	insertApprovedPhoto(t, priya, "photos/priya/1", 1)
	if _, err := testutil.DB.Exec(`
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
		VALUES ($1, 'photos/priya/pending', 2, 'pending')`, priya); err != nil {
		t.Fatalf("insert pending photo: %v", err)
	}

	react(t, service, alex, priya, "like")
	_, matched := react(t, service, priya, alex, "like")

	response, detail := matchDetail(t, service, alex, *matched.MatchID)
	if response.Code != http.StatusOK {
		t.Fatalf("detail returned %d", response.Code)
	}
	if len(detail.PhotoURLs) != 1 {
		t.Fatalf("photo URLs = %v, want only the approved photo", detail.PhotoURLs)
	}
	for _, key := range presigner.recorded() {
		if strings.Contains(key, "pending") {
			t.Fatalf("presigned a pending photo: %q", key)
		}
	}
}

func TestMatchResponseNeverCaches(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex := insertUser(t, "alex")
	insertProfile(t, alex, "Alex", "")

	response, _ := listMatches(t, service, alex)
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestReactionRequiresJSONContentType(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	body := fmt.Sprintf(`{"target_id":%q,"action":"like"}`, priya)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reactions", strings.NewReader(body))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	service.CreateReaction(response, request, alex)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestReactionRequiresAllowedOrigin(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	body := fmt.Sprintf(`{"target_id":%q,"action":"like"}`, priya)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reactions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	service.CreateReaction(response, request, alex)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM reactions").Scan(&count); err != nil {
		t.Fatalf("count reactions: %v", err)
	}
	if count != 0 {
		t.Fatalf("reactions = %d, want 0 after a rejected origin", count)
	}
}

func TestMatchDetailRejectsMalformedID(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex := insertUser(t, "alex")
	insertProfile(t, alex, "Alex", "")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches/not-a-uuid", nil)
	request.SetPathValue("id", "not-a-uuid")
	response := httptest.NewRecorder()
	service.Detail(response, request, alex)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestMatchListOrdersNewestFirst(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya, sam := insertUser(t, "alex"), insertUser(t, "priya"), insertUser(t, "sam")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")
	insertProfile(t, sam, "Sam", "")

	react(t, service, alex, priya, "like")
	react(t, service, priya, alex, "like")
	react(t, service, alex, sam, "like")
	react(t, service, sam, alex, "like")

	_, list := listMatches(t, service, alex)
	if len(list.Matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(list.Matches))
	}
	if list.Matches[0].DisplayName != "Sam" {
		t.Fatalf("first row = %q, want the newest match Sam", list.Matches[0].DisplayName)
	}
}

func TestReactionAcceptsMixedCaseTarget(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")

	// A client may echo the id back in any case; both sides must still land in
	// the same canonical pair or the check constraint turns into a 503.
	upper := strings.ToUpper(priya)
	response, _ := react(t, service, alex, upper, "like")
	if response.Code != http.StatusOK {
		t.Fatalf("upper-case target returned %d: %s", response.Code, response.Body.String())
	}
	second, matched := react(t, service, priya, alex, "like")
	if second.Code != http.StatusOK || !matched.Matched {
		t.Fatalf("reciprocal like after a mixed-case like: %d %+v", second.Code, matched)
	}
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM matches").Scan(&count); err != nil {
		t.Fatalf("count matches: %v", err)
	}
	if count != 1 {
		t.Fatalf("matches = %d, want 1", count)
	}
}

func TestReactionRejectsMixedCaseSelf(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex := insertUser(t, "alex")
	insertProfile(t, alex, "Alex", "")

	response, _ := react(t, service, alex, strings.ToUpper(alex), "like")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("upper-case self reaction returned %d, want 400", response.Code)
	}
}

func TestBlockRejectsUnknownAndDeletedTarget(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "")
	if _, err := testutil.DB.Exec("UPDATE users SET deleted_at = now() WHERE id = $1", priya); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	unknown := httptest.NewRecorder()
	body := `{"target_id":"00000000-0000-4000-8000-000000000001"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/blocks", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	service.Block(unknown, request, alex)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown target block returned %d, want 404", unknown.Code)
	}

	deleted := block(t, service, alex, priya)
	if deleted.Code != http.StatusNotFound {
		t.Fatalf("deleted target block returned %d, want 404", deleted.Code)
	}
}

func TestMatchReadsHideSoftDeletedAccounts(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)
	alex, priya := insertUser(t, "alex"), insertUser(t, "priya")
	insertProfile(t, alex, "Alex", "")
	insertProfile(t, priya, "Priya", "priya.plays")
	insertApprovedPhoto(t, priya, "photos/priya/1", 1)

	react(t, service, alex, priya, "like")
	_, matched := react(t, service, priya, alex, "like")

	_, before := listMatches(t, service, alex)
	if len(before.Matches) != 1 {
		t.Fatalf("pre-delete matches = %d, want 1", len(before.Matches))
	}

	if _, err := testutil.DB.Exec("UPDATE users SET deleted_at = now() WHERE id = $1", priya); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if _, after := listMatches(t, service, alex); len(after.Matches) != 0 {
		t.Fatalf("match list still shows a deleted account: %+v", after.Matches)
	}
	if response, _ := matchDetail(t, service, alex, *matched.MatchID); response.Code != http.StatusNotFound {
		t.Fatalf("detail for a deleted account returned %d, want 404", response.Code)
	}
	if response, _ := discloseHandle(t, service, alex, *matched.MatchID); response.Code != http.StatusNotFound {
		t.Fatalf("handle for a deleted account returned %d, want 404", response.Code)
	}
}

func TestInstagramHandleSchemaMatchesGoValidator(t *testing.T) {
	testutil.ResetTables(t)
	userID := insertUser(t, "alex")
	valid := []string{"priya.plays", "a_b.c9", "_ok", "a", "player.2026"}
	if _, err := testutil.DB.Exec(
		"INSERT INTO profiles (user_id, display_name, gender, skill_level, gender_preference, min_age, max_age) VALUES ($1,$2,'woman','intermediate','all',18,120)",
		userID, "Alex"); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	for _, handle := range valid {
		if _, err := testutil.DB.Exec(
			"UPDATE profiles SET instagram_handle = $1 WHERE user_id = $2", handle, userID); err != nil {
			t.Fatalf("schema rejected valid handle %q: %v", handle, err)
		}
	}
	invalid := []string{".priya", "priya.", "priya..plays", "Priya.Plays", "priya plays", strings.Repeat("a", 31)}
	for _, handle := range invalid {
		if _, err := testutil.DB.Exec(
			"UPDATE profiles SET instagram_handle = $1 WHERE user_id = $2", handle, userID); err == nil {
			t.Fatalf("schema accepted invalid handle %q", handle)
		}
	}
}

func TestBlockRacingAReactionCannotCreateAMatch(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newService(t)

	// Repeat the race in-process: a single attempt can pass by luck of
	// scheduling, and the invariant is about the interleaving, not one run.
	for attempt := range 15 {
		testutil.ResetTables(t)
		alex, priya := insertUser(t, fmt.Sprintf("alex-%d", attempt)), insertUser(t, fmt.Sprintf("priya-%d", attempt))
		insertProfile(t, alex, "Alex", "")
		insertProfile(t, priya, "Priya", "")

		// Priya likes first, so Alex's like would complete the pair.
		if response, _ := react(t, service, priya, alex, "like"); response.Code != http.StatusOK {
			t.Fatalf("attempt %d primer like returned %d", attempt, response.Code)
		}

		var wait sync.WaitGroup
		start := make(chan struct{})
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			block(t, service, alex, priya)
		}()
		go func() {
			defer wait.Done()
			<-start
			react(t, service, alex, priya, "like")
		}()
		close(start)
		wait.Wait()

		var count int
		if err := testutil.DB.QueryRow(`
			SELECT count(*) FROM matches
			WHERE (user_low = $1 AND user_high = $2) OR (user_low = $2 AND user_high = $1)`,
			alex, priya).Scan(&count); err != nil {
			t.Fatalf("attempt %d count matches: %v", attempt, err)
		}
		if count != 0 {
			t.Fatalf("attempt %d: a blocked pair ended up with %d match rows", attempt, count)
		}
	}
}
