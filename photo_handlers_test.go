package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image/jpeg"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakePhotoStorage struct {
	mu                 sync.Mutex
	objects            map[string][]byte
	presignedKeys      []string
	presignExpiries    []time.Duration
	deletedKeys        []string
	getCalls           int
	getHook            func(call int)
	getError           error
	putError           error
	deleteError        error
	presignError       error
	presignGetError    error
	blockGet           bool
	presignedGetKeys   []string
	presignGetExpiries []time.Duration
}

func (f *fakePhotoStorage) storage() photoStorage {
	return photoStorage{
		presignPut: func(_ context.Context, key string, expires time.Duration) (string, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.presignError != nil {
				return "", f.presignError
			}
			f.presignedKeys = append(f.presignedKeys, key)
			f.presignExpiries = append(f.presignExpiries, expires)
			return "https://storage.example.test/" + key + "?X-Amz-Signature=signature-secret", nil
		},
		get: func(ctx context.Context, key string) ([]byte, error) {
			f.mu.Lock()
			f.getCalls++
			call := f.getCalls
			hook := f.getHook
			block := f.blockGet
			f.mu.Unlock()
			if hook != nil {
				hook(call)
			}
			if block {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.getError != nil {
				return nil, f.getError
			}
			data, ok := f.objects[key]
			if !ok {
				return nil, errPhotoObjectMissing
			}
			return append([]byte(nil), data...), nil
		},
		presignGet: func(_ context.Context, key string, expires time.Duration) (string, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.presignGetError != nil {
				return "", f.presignGetError
			}
			f.presignedGetKeys = append(f.presignedGetKeys, key)
			f.presignGetExpiries = append(f.presignGetExpiries, expires)
			return "https://storage.example.test/" + key + "?X-Amz-Signature=get-secret", nil
		},
		put: func(_ context.Context, key string, data []byte) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.putError != nil {
				return f.putError
			}
			f.objects[key] = append([]byte(nil), data...)
			return nil
		},
		delete: func(_ context.Context, key string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.deleteError != nil {
				return f.deleteError
			}
			f.deletedKeys = append(f.deletedKeys, key)
			delete(f.objects, key)
			return nil
		},
	}
}

func (f *fakePhotoStorage) objectCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.objects)
}

func newPhotoTestService(t *testing.T) (*photoService, string, *authService, *fakePhotoStorage) {
	t.Helper()
	auth, _ := newTestAuth(t)
	userID := insertTestUser(t, auth, "photographer@example.com")
	fake := &fakePhotoStorage{objects: make(map[string][]byte)}
	service := &photoService{db: testDB, appBaseURL: "http://localhost:8080", storage: fake.storage(), moderate: approveModeration}
	return service, userID, auth, fake
}

func uploadPhotoRequest(service *photoService, userID string) *httptest.ResponseRecorder {
	return uploadPhotoRequestContext(context.Background(), service, userID, "")
}

func uploadPhotoRequestContext(ctx context.Context, service *photoService, userID, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", nil).WithContext(ctx)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	service.uploadURL(response, request, userID)
	return response
}

func processPhotoRequest(service *photoService, userID, photoID string) *httptest.ResponseRecorder {
	return processPhotoRequestContext(context.Background(), service, userID, photoID, "")
}

func processPhotoRequestContext(ctx context.Context, service *photoService, userID, photoID, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/photos/"+photoID+"/process", nil).WithContext(ctx)
	request.SetPathValue("id", photoID)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	service.process(response, request, userID)
	return response
}

func getPhotoURLRequest(service *photoService, userID, photoID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/photos/"+photoID+"/url", nil)
	request.SetPathValue("id", photoID)
	response := httptest.NewRecorder()
	service.photoURL(response, request, userID)
	return response
}

func getPhotoListRequest(service *photoService, userID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/photos", nil)
	response := httptest.NewRecorder()
	service.photoList(response, request, userID)
	return response
}

func insertPhotoRow(t *testing.T, ownerID, key string, displayOrder int, status photoModerationStatus, age time.Duration) string {
	t.Helper()
	var photoID string
	err := testDB.QueryRow(`
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status, updated_at)
		VALUES ($1, $2, $3, $4, now() - make_interval(secs => $5))
		RETURNING id`, ownerID, key, displayOrder, string(status), age.Seconds()).Scan(&photoID)
	if err != nil {
		t.Fatalf("insert photo row: %v", err)
	}
	return photoID
}

func countPhotos(t *testing.T) int {
	t.Helper()
	var count int
	if err := testDB.QueryRow("SELECT count(*) FROM photos").Scan(&count); err != nil {
		t.Fatalf("count photos: %v", err)
	}
	return count
}

func storedPhotoKey(t *testing.T, photoID string) string {
	t.Helper()
	var key string
	if err := testDB.QueryRow("SELECT object_key FROM photos WHERE id = $1", photoID).Scan(&key); err != nil {
		t.Fatalf("read photo key: %v", err)
	}
	return key
}

type photoUploadBody struct {
	PhotoID          string `json:"photo_id"`
	UploadURL        string `json:"upload_url"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

func TestUploadURLCreatesPendingPhotoForOwner(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	response := uploadPhotoRequest(service, userID)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body photoUploadBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PhotoID == "" || body.ExpiresInSeconds != 300 {
		t.Fatalf("body = %#v", body)
	}
	if !strings.HasPrefix(body.UploadURL, "https://storage.example.test/photos/") {
		t.Fatalf("upload_url = %q", body.UploadURL)
	}
	for _, forbidden := range []string{"object_key", "bucket"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("response leaks %q: %s", forbidden, response.Body.String())
		}
	}

	var ownerID string
	var order int
	var status photoModerationStatus
	if err := testDB.QueryRow("SELECT owner_id, display_order, moderation_status FROM photos WHERE id = $1", body.PhotoID).
		Scan(&ownerID, &order, &status); err != nil {
		t.Fatalf("read photo row: %v", err)
	}
	if ownerID != userID || order != 1 || status != photoModerationPending {
		t.Fatalf("row = owner %s order %d status %s", ownerID, order, status)
	}
	if got := storedPhotoKey(t, body.PhotoID); !strings.HasPrefix(got, "photos/") {
		t.Fatalf("stored object key = %q", got)
	}
	if fake.objectCount() != 0 {
		t.Fatalf("upload-url created stored objects: %d", fake.objectCount())
	}
}

func TestUploadURLRejectsCrossOrigin(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	response := uploadPhotoRequestContext(context.Background(), service, userID, "https://evil.example")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("cross-origin upload created %d rows", count)
	}
}

func TestUploadURLFillsOneToThreeThenConflicts(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		response := uploadPhotoRequest(service, userID)
		if response.Code != http.StatusCreated {
			t.Fatalf("upload %d status = %d, body = %s", i, response.Code, response.Body.String())
		}
	}
	rows, err := testDB.Query("SELECT display_order FROM photos ORDER BY display_order")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var orders []int
	for rows.Next() {
		var order int
		if err := rows.Scan(&order); err != nil {
			t.Fatal(err)
		}
		orders = append(orders, order)
	}
	if len(orders) != 3 || orders[0] != 1 || orders[1] != 2 || orders[2] != 3 {
		t.Fatalf("display orders = %v, want [1 2 3]", orders)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusConflict {
		t.Fatalf("fourth upload status = %d, want 409", response.Code)
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
}

func TestUploadURLPresignIsShortLivedAndScoped(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	response := uploadPhotoRequest(service, userID)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d", response.Code)
	}
	var body photoUploadBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(fake.presignedKeys) != 1 || fake.presignedKeys[0] != storedPhotoKey(t, body.PhotoID) {
		t.Fatalf("presigned keys = %v", fake.presignedKeys)
	}
	if fake.presignExpiries[0] != photoUploadURLExpiry {
		t.Fatalf("presign expiry = %v, want %v", fake.presignExpiries[0], photoUploadURLExpiry)
	}
	if !strings.Contains(body.UploadURL, storedPhotoKey(t, body.PhotoID)) {
		t.Fatalf("upload URL is not scoped to the photo key: %q", body.UploadURL)
	}
}

func TestConcurrentUploadURLsNeverExceedThree(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	const clients = 8
	codes := make([]int, clients)
	var wait sync.WaitGroup
	for i := 0; i < clients; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			codes[index] = uploadPhotoRequest(service, userID).Code
		}(i)
	}
	wait.Wait()
	counts := make(map[int]int)
	for _, code := range codes {
		counts[code]++
	}
	if counts[http.StatusCreated] != 3 || counts[http.StatusConflict] != 5 {
		t.Fatalf("status codes = %v, want 3x201 and 5x409", counts)
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
}

func TestProcessMovesSanitizedObjectToNewKey(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/original"
	fake.objects[originalKey] = photoTestJPEGWithEXIF(t)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		PhotoID string `json:"photo_id"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.PhotoID != photoID || body.Status != string(photoModerationApproved) {
		t.Fatalf("body = %#v", body)
	}

	key := storedPhotoKey(t, photoID)
	if key == originalKey {
		t.Fatal("process did not move the object to a new key")
	}
	if _, ok := fake.objects[originalKey]; ok {
		t.Fatal("process left the original object in place")
	}
	stored, ok := fake.objects[key]
	if !ok {
		t.Fatalf("sanitized object %q not stored", key)
	}
	if bytes.Contains(stored, []byte("Exif")) || bytes.Contains(stored, []byte("private-exif-marker")) {
		t.Fatal("sanitized object still contains EXIF")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(stored))
	if err != nil {
		t.Fatalf("sanitized object is not JPEG: %v", err)
	}
	if config.Width > maxPhotoDimension || config.Height > maxPhotoDimension {
		t.Fatalf("sanitized object size = %dx%d", config.Width, config.Height)
	}
}

func TestProcessSecondCallAfterVerdictIs409(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/original"
	fake.objects[originalKey] = photoTestJPEG(t, 40, 30)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusOK {
		t.Fatalf("first process status = %d, body = %s", response.Code, response.Body.String())
	}
	firstKey := storedPhotoKey(t, photoID)
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusConflict {
		t.Fatalf("second process status = %d, want 409", response.Code)
	}
	if secondKey := storedPhotoKey(t, photoID); secondKey != firstKey {
		t.Fatalf("second process changed the key: %q -> %q", firstKey, secondKey)
	}
	if fake.objectCount() != 1 {
		t.Fatalf("stored object count = %d, want 1", fake.objectCount())
	}
	var status photoModerationStatus
	if err := testDB.QueryRow("SELECT moderation_status FROM photos WHERE id = $1", photoID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationApproved {
		t.Fatalf("status after second process = %q, want approved", status)
	}
}

func TestProcessDeletesOversizedObjectAndRow(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/oversized"
	fake.objects[key] = photoTestJPEG(t, 10, 10)
	fake.getError = errPhotoObjectTooLarge
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d, want 0", count)
	}
	if len(fake.deletedKeys) != 1 || fake.deletedKeys[0] != key {
		t.Fatalf("deleted keys = %v, want %q", fake.deletedKeys, key)
	}
	response = uploadPhotoRequest(service, userID)
	if response.Code != http.StatusCreated {
		t.Fatalf("slot was not freed, upload status = %d", response.Code)
	}
}

func TestProcessDeletesInvalidObjectAndRow(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/not-an-image"
	fake.objects[key] = []byte("not an image")
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d, want 0", count)
	}

	gifKey := "photos/gif"
	fake.objects[gifKey] = []byte("GIF87a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff\x21\xf9\x04\x01\x00\x00\x00\x00\x2c\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02\x44\x01\x00\x3b")
	gifID := insertPhotoRow(t, userID, gifKey, 1, photoModerationPending, 0)
	if response := processPhotoRequest(service, userID, gifID); response.Code != http.StatusBadRequest {
		t.Fatalf("GIF status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count after GIF = %d, want 0", count)
	}
}

func TestProcessRejectsMissingObject(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/missing"
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d, want 0", count)
	}
	if len(fake.deletedKeys) != 1 || fake.deletedKeys[0] != key {
		t.Fatalf("deleted keys = %v, want %q", fake.deletedKeys, key)
	}
}

func TestProcessRejectsForeignOwnerUnknownAndGarbageID(t *testing.T) {
	service, userID, auth, _ := newPhotoTestService(t)
	const key = "photos/owned"
	fake := &fakePhotoStorage{objects: map[string][]byte{key: photoTestJPEG(t, 10, 10)}}
	service.storage = fake.storage()
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)

	otherID := insertTestUser(t, auth, "other@example.com")
	if response := processPhotoRequest(service, otherID, photoID); response.Code != http.StatusNotFound {
		t.Fatalf("foreign owner status = %d, want 404", response.Code)
	}
	const unknownID = "00000000-0000-4000-8000-000000000000"
	if response := processPhotoRequest(service, userID, unknownID); response.Code != http.StatusNotFound {
		t.Fatalf("unknown id status = %d, want 404", response.Code)
	}
	if response := processPhotoRequest(service, userID, "not-a-uuid"); response.Code != http.StatusNotFound {
		t.Fatalf("garbage id status = %d, want 404", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
}

func TestProcessRefusesNonPendingPhoto(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/approved"
	fake.objects[key] = photoTestJPEG(t, 10, 10)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationApproved, 0)
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
}

func TestProcessRejectsCrossOrigin(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/csrf"
	fake.objects[key] = photoTestJPEG(t, 10, 10)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	response := processPhotoRequestContext(context.Background(), service, userID, photoID, "https://evil.example")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
}

func TestProcessConcurrentLoserKeepsWinner(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/race"
	fake.objects[originalKey] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)

	var barrierOpen sync.Once
	barrier := make(chan struct{})
	fake.getHook = func(call int) {
		if call == 1 {
			<-barrier
			return
		}
		if call == 2 {
			barrierOpen.Do(func() { close(barrier) })
		}
	}

	var codes [2]int
	var wait sync.WaitGroup
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			codes[index] = processPhotoRequest(service, userID, photoID).Code
		}(i)
	}
	wait.Wait()
	if codes[0] == codes[1] {
		t.Fatalf("both process calls returned %d", codes[0])
	}
	winners := 0
	for _, code := range codes {
		if code == http.StatusOK {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("status codes = %v, want exactly one 200", codes)
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}
	key := storedPhotoKey(t, photoID)
	if key == originalKey {
		t.Fatal("row still points at the original key")
	}
	fake.mu.Lock()
	_, exists := fake.objects[key]
	fake.mu.Unlock()
	if !exists {
		t.Fatalf("winner object %q is missing", key)
	}
	if fake.objectCount() != 1 {
		t.Fatalf("stored object count = %d, want 1", fake.objectCount())
	}
}

func TestUploadURLReusesStaleSlot(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		key := "photos/stale-" + string(rune('0'+i))
		fake.objects[key] = []byte("stale")
		insertPhotoRow(t, userID, key, i, photoModerationPending, 2*time.Hour)
	}
	response := uploadPhotoRequest(service, userID)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}
	if len(fake.deletedKeys) != 3 {
		t.Fatalf("stale objects deleted = %d, want 3", len(fake.deletedKeys))
	}
}

func TestUploadURLKeepsFreshRows(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		key := "photos/fresh-" + string(rune('0'+i))
		fake.objects[key] = []byte("fresh")
		insertPhotoRow(t, userID, key, i, photoModerationPending, 0)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
	if len(fake.deletedKeys) != 0 {
		t.Fatalf("fresh rows deleted: %v", fake.deletedKeys)
	}
}

func TestProcessClaimSurvivesStaleCleanup(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/claim"
	fake.objects[key] = photoTestJPEG(t, 10, 10)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 59*time.Minute)

	ctx := context.Background()
	claimedKey, err := claimPendingPhoto(ctx, testDB, photoID, userID)
	if err != nil || claimedKey != key {
		t.Fatalf("claim = %q, %v", claimedKey, err)
	}
	keys, err := cleanupStalePending(ctx, testDB, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("cleanup removed a claimed row: %v", keys)
	}
	var count int
	if err := testDB.QueryRow("SELECT count(*) FROM photos").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}

	const expiredKey = "photos/expired"
	fake.objects[expiredKey] = photoTestJPEG(t, 10, 10)
	expiredID := insertPhotoRow(t, userID, expiredKey, 2, photoModerationPending, 2*time.Hour)
	if _, err := claimPendingPhoto(ctx, testDB, expiredID, userID); !errors.Is(err, errPhotoUploadExpired) {
		t.Fatalf("claim of expired row err = %v, want errPhotoUploadExpired", err)
	}
	if calls := fake.getCalls; calls != 0 {
		t.Fatalf("storage get calls = %d, want 0", calls)
	}
	keys, err = cleanupStalePending(ctx, testDB, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != expiredKey {
		t.Fatalf("cleanup keys = %v, want [%q]", keys, expiredKey)
	}

	// The handler path for an expired upload returns 409 without touching storage.
	service.storage = fake.storage()
	expiredID2 := insertPhotoRow(t, userID, "photos/expired-2", 3, photoModerationPending, 2*time.Hour)
	if response := processPhotoRequest(service, userID, expiredID2); response.Code != http.StatusConflict {
		t.Fatalf("expired upload status = %d, want 409", response.Code)
	}
	if calls := fake.getCalls; calls != 0 {
		t.Fatalf("storage get calls = %d, want 0", calls)
	}
}

func TestPhotoEndpointsFailClosedWithoutStorage(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	service.storage = photoStorage{}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("upload status = %d, want 503", response.Code)
	}
	if response := processPhotoRequest(service, userID, "00000000-0000-4000-8000-000000000000"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("process status = %d, want 503", response.Code)
	}
}

func TestProcessStorageErrorsAreBoundedAndGeneric(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/leak"
	fake.getError = errors.New("bucket=private-bucket key=" + key + " signature=signature-secret")
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)

	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	for _, leaked := range []string{"signature-secret", "private-bucket", key} {
		if strings.Contains(response.Body.String(), leaked) {
			t.Fatalf("response leaks %q: %s", leaked, response.Body.String())
		}
		if strings.Contains(logged.String(), leaked) {
			t.Fatalf("log leaks %q: %s", leaked, logged.String())
		}
	}

	blocking := &fakePhotoStorage{objects: map[string][]byte{key: photoTestJPEG(t, 10, 10)}, blockGet: true}
	service.storage = blocking.storage()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	response = processPhotoRequestContext(ctx, service, userID, photoID, "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("timeout status = %d, want 503", response.Code)
	}
	if strings.Contains(response.Body.String(), "signature-secret") {
		t.Fatal("timeout response leaks storage details")
	}
}

func TestNoPublicRouteServesPhotoObjects(t *testing.T) {
	service, _, auth, _ := newPhotoTestService(t)
	handler := newMux(auth, service.storage, approveModeration, testDB.PingContext)
	for _, path := range []string{"/api/v1/photos/00000000-0000-4000-8000-000000000000", "/api/v1/photos/00000000-0000-4000-8000-000000000000/process"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s status = %d", path, response.Code)
		}
		if response.Body.Len() > 0 && bytes.HasPrefix(response.Body.Bytes(), []byte{0xff, 0xd8}) {
			t.Fatalf("GET %s served image bytes", path)
		}
	}
}

func TestUploadURLPresignFailureFreesSlot(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	fake.presignError = errors.New("provider down signature=signature-secret")
	response := uploadPhotoRequest(service, userID)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "signature-secret") {
		t.Fatalf("response leaks the provider error: %s", response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d, want 0", count)
	}
	fake.mu.Lock()
	fake.presignError = nil
	fake.mu.Unlock()
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("slot was not freed, upload status = %d", response.Code)
	}
}

func TestProcessSanitizedPutFailureKeepsOriginal(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/put-failure"
	fake.objects[originalKey] = photoTestJPEG(t, 20, 20)
	fake.putError = errors.New("put failed")
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", response.Code, response.Body.String())
	}
	if key := storedPhotoKey(t, photoID); key != originalKey {
		t.Fatalf("row key = %q, want %q", key, originalKey)
	}
	if _, ok := fake.objects[originalKey]; !ok {
		t.Fatal("retryable failure deleted the original object")
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}
}

func TestUploadURLCleanupDeleteFailureIsGeneric(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		key := "photos/cleanup-" + string(rune('0'+i))
		fake.objects[key] = []byte("stale")
		insertPhotoRow(t, userID, key, i, photoModerationPending, 2*time.Hour)
	}
	fake.deleteError = errors.New("delete failed bucket=private-bucket key=photos/cleanup-1")

	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	response := uploadPhotoRequest(service, userID)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}
	for _, leaked := range []string{"private-bucket", "photos/cleanup-1"} {
		if strings.Contains(logged.String(), leaked) {
			t.Fatalf("log leaks %q: %s", leaked, logged.String())
		}
	}
}

type fakeModeration struct {
	mu      sync.Mutex
	calls   int
	verdict moderationVerdict
	err     error
	images  [][]byte
	hook    func(call int)
}

func (f *fakeModeration) moderate(_ context.Context, image []byte) (moderationVerdict, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	verdict, err, hook := f.verdict, f.err, f.hook
	f.images = append(f.images, append([]byte(nil), image...))
	f.mu.Unlock()
	if hook != nil {
		hook(call)
	}
	return verdict, err
}

func (f *fakeModeration) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func approveModeration(context.Context, []byte) (moderationVerdict, error) {
	return moderationVerdict{status: photoModerationApproved, reference: "op-test"}, nil
}

type photoURLBody struct {
	PhotoID          string                `json:"photo_id"`
	Status           photoModerationStatus `json:"status"`
	URL              string                `json:"url"`
	ExpiresInSeconds int                   `json:"expires_in_seconds"`
}

type photoListBody struct {
	Photos []struct {
		PhotoID      string                `json:"photo_id"`
		DisplayOrder int                   `json:"display_order"`
		Status       photoModerationStatus `json:"status"`
	} `json:"photos"`
}

func TestProcessApprovesCleanPhoto(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/approve"
	fake.objects[originalKey] = photoTestJPEGWithEXIF(t)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)
	moderation := &fakeModeration{verdict: moderationVerdict{status: photoModerationApproved, reference: "op-approved"}}
	service.moderate = moderation.moderate

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		PhotoID string                `json:"photo_id"`
		Status  photoModerationStatus `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != photoModerationApproved {
		t.Fatalf("body status = %q, want approved", body.Status)
	}
	var status photoModerationStatus
	var reference sql.NullString
	if err := testDB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationApproved || reference.String != "op-approved" {
		t.Fatalf("row = %q / %q", status, reference.String)
	}
	if moderation.callCount() != 1 {
		t.Fatalf("moderation calls = %d", moderation.callCount())
	}
	image := moderation.images[0]
	if bytes.Contains(image, []byte("Exif")) || bytes.Contains(image, []byte("private-exif-marker")) {
		t.Fatal("moderator received raw EXIF bytes")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(image))
	if err != nil || config.Width > maxPhotoDimension || config.Height > maxPhotoDimension {
		t.Fatalf("moderator image = %dx%d, %v", config.Width, config.Height, err)
	}
	eligible, err := photoApprovedForDiscovery(context.Background(), testDB, userID)
	if err != nil || !eligible {
		t.Fatalf("discovery eligible = %v, %v", eligible, err)
	}
}

func TestProcessInconclusiveStaysPending(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/inconclusive"
	fake.objects[originalKey] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)
	moderation := &fakeModeration{verdict: moderationVerdict{status: photoModerationPending}}
	service.moderate = moderation.moderate

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Status photoModerationStatus `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != photoModerationPending {
		t.Fatalf("body status = %q, want pending", body.Status)
	}
	var status photoModerationStatus
	var reference sql.NullString
	if err := testDB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationPending || reference.Valid {
		t.Fatalf("row = %q %#v", status, reference)
	}

	response = processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("repeat status = %d, body = %s", response.Code, response.Body.String())
	}
	if moderation.callCount() != 2 {
		t.Fatalf("moderation calls = %d, want 2", moderation.callCount())
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d", count)
	}
}

func TestProcessProviderFailureKeepsPending(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/provider-failure"
	fake.objects[originalKey] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)
	moderation := &fakeModeration{err: errors.New("provider down")}
	service.moderate = moderation.moderate

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	var status photoModerationStatus
	if err := testDB.QueryRow("SELECT moderation_status FROM photos WHERE id = $1", photoID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationPending {
		t.Fatalf("status = %q, want pending", status)
	}
	key := storedPhotoKey(t, photoID)
	if key == originalKey {
		t.Fatal("post-swap failure did not keep the sanitized object")
	}
	if _, ok := fake.objects[key]; !ok {
		t.Fatal("sanitized object was deleted on a retryable failure")
	}

	moderation.err = nil
	moderation.verdict = moderationVerdict{status: photoModerationApproved, reference: "op-retry"}
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestProcessUnconfiguredModerationFailsClosed(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/no-moderation"
	fake.objects[key] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	service.moderate = nil
	var before time.Time
	if err := testDB.QueryRow("SELECT updated_at FROM photos WHERE id = $1", photoID).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
	var after time.Time
	if err := testDB.QueryRow("SELECT updated_at FROM photos WHERE id = $1", photoID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Equal(before) {
		t.Fatal("fail-closed path claimed the row")
	}
}

func TestProcessRejectsInvalidImageBeforeModeration(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/invalid"
	fake.objects[key] = []byte("not an image")
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	moderation := &fakeModeration{verdict: moderationVerdict{status: photoModerationApproved}}
	service.moderate = moderation.moderate

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if moderation.callCount() != 0 {
		t.Fatalf("moderation calls = %d, want 0", moderation.callCount())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d", count)
	}
}

func TestProcessNonPendingIs409WithoutStorage(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/terminal"
	fake.objects[key] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationApproved, 0)
	moderation := &fakeModeration{verdict: moderationVerdict{status: photoModerationApproved}}
	service.moderate = moderation.moderate

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d", fake.getCalls)
	}
	if moderation.callCount() != 0 {
		t.Fatalf("moderation calls = %d", moderation.callCount())
	}
}

func TestModerationCasStaleKey(t *testing.T) {
	_, userID, _, _ := newPhotoTestService(t)
	photoID := insertPhotoRow(t, userID, "photos/cas", 1, photoModerationPending, 0)

	applied, err := applyModeration(context.Background(), testDB, photoID, userID, "photos/stale", moderationVerdict{status: photoModerationApproved, reference: "op-stale"})
	if err != nil {
		t.Fatalf("apply stale key: %v", err)
	}
	if applied {
		t.Fatal("stale key moderation was applied")
	}
	var status photoModerationStatus
	if err := testDB.QueryRow("SELECT moderation_status FROM photos WHERE id = $1", photoID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationPending {
		t.Fatalf("status = %q, want pending", status)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := applyModeration(cancelled, testDB, photoID, userID, "photos/cas", moderationVerdict{status: photoModerationApproved}); err == nil {
		t.Fatal("cancelled context did not surface a database error")
	}
}

func TestModerationForcedRaceKeepsWinnerVerdict(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/race-moderation"
	fake.objects[originalKey] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)

	firstInModeration := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls int32
	service.moderate = func(_ context.Context, _ []byte) (moderationVerdict, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			close(firstInModeration)
			<-releaseFirst
			return moderationVerdict{status: photoModerationRejected, reference: "loser"}, nil
		}
		return moderationVerdict{status: photoModerationApproved, reference: "winner"}, nil
	}

	firstDone := make(chan int, 1)
	go func() { firstDone <- processPhotoRequest(service, userID, photoID).Code }()
	<-firstInModeration

	secondDone := make(chan int, 1)
	go func() { secondDone <- processPhotoRequest(service, userID, photoID).Code }()
	secondCode := <-secondDone
	close(releaseFirst)
	firstCode := <-firstDone

	if secondCode != http.StatusOK {
		t.Fatalf("second process status = %d, want 200", secondCode)
	}
	if firstCode != http.StatusConflict {
		t.Fatalf("first process status = %d, want 409", firstCode)
	}
	var status photoModerationStatus
	var reference sql.NullString
	if err := testDB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationApproved || reference.String != "winner" {
		t.Fatalf("row = %q %q, want approved/winner", status, reference.String)
	}
	if fake.objectCount() != 1 {
		t.Fatalf("stored object count = %d, want 1", fake.objectCount())
	}
}

func TestModerationNeverLeaksImageOrSecrets(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/leak-moderation"
	fake.objects[key] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	service.moderate = func(context.Context, []byte) (moderationVerdict, error) {
		return moderationVerdict{}, errors.New("api_secret=secret-value key=" + key + " signature=signature-secret")
	}

	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	for _, leaked := range []string{"secret-value", "signature-secret", key} {
		if strings.Contains(response.Body.String(), leaked) {
			t.Fatalf("response leaks %q: %s", leaked, response.Body.String())
		}
		if strings.Contains(logged.String(), leaked) {
			t.Fatalf("log leaks %q: %s", leaked, logged.String())
		}
	}
	jpegMagic := []byte{0xff, 0xd8, 0xff}
	if bytes.Contains(response.Body.Bytes(), jpegMagic) || bytes.Contains(logged.Bytes(), jpegMagic) {
		t.Fatal("image bytes leaked into the response or log")
	}
}

func TestProcessRejectsAndDeletesObject(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const originalKey = "photos/rejected"
	fake.objects[originalKey] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, photoModerationPending, 0)
	service.moderate = func(context.Context, []byte) (moderationVerdict, error) {
		return moderationVerdict{status: photoModerationRejected, reference: "op-reject"}, nil
	}

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Status photoModerationStatus `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != photoModerationRejected {
		t.Fatalf("body status = %q, want rejected", body.Status)
	}
	var status photoModerationStatus
	var reference sql.NullString
	if err := testDB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationRejected || reference.String != "op-reject" {
		t.Fatalf("row = %q %q", status, reference.String)
	}
	if fake.objectCount() != 0 {
		t.Fatalf("rejected object was kept: %d", fake.objectCount())
	}
	if response := getPhotoURLRequest(service, userID, photoID); response.Code != http.StatusNotFound {
		t.Fatalf("rejected URL status = %d, want 404", response.Code)
	}
	if eligible, err := photoApprovedForDiscovery(context.Background(), testDB, userID); err != nil || eligible {
		t.Fatalf("discovery eligible = %v, %v", eligible, err)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("slot was not reusable, upload status = %d", response.Code)
	}
}

func TestDiscoveryEligibilityRequiresApprovedPhoto(t *testing.T) {
	_, userID, auth, _ := newPhotoTestService(t)
	ctx := context.Background()
	eligible, err := photoApprovedForDiscovery(ctx, testDB, userID)
	if err != nil || eligible {
		t.Fatalf("empty owner eligible = %v, %v", eligible, err)
	}
	insertPhotoRow(t, userID, "photos/pending-eligibility", 1, photoModerationPending, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testDB, userID); err != nil || eligible {
		t.Fatalf("pending owner eligible = %v, %v", eligible, err)
	}
	insertPhotoRow(t, userID, "photos/rejected-eligibility", 2, photoModerationRejected, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testDB, userID); err != nil || eligible {
		t.Fatalf("rejected owner eligible = %v, %v", eligible, err)
	}
	insertPhotoRow(t, userID, "photos/approved-eligibility", 3, photoModerationApproved, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testDB, userID); err != nil || !eligible {
		t.Fatalf("approved owner eligible = %v, %v", eligible, err)
	}
	otherID := insertTestUser(t, auth, "other-photos@example.com")
	insertPhotoRow(t, otherID, "photos/other-approved", 1, photoModerationApproved, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testDB, otherID); err != nil || !eligible {
		t.Fatalf("other owner eligibility = %v, %v", eligible, err)
	}
}

func TestUploadURLReclaimsRejectedSlot(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	for order := 1; order <= 2; order++ {
		insertPhotoRow(t, userID, "photos/approved-slot-"+string(rune('0'+order)), order, photoModerationApproved, 0)
	}
	insertPhotoRow(t, userID, "photos/rejected-slot", 3, photoModerationRejected, 0)

	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
	var rejected int
	if err := testDB.QueryRow("SELECT count(*) FROM photos WHERE moderation_status = 'rejected'").Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected != 0 {
		t.Fatalf("rejected rows = %d, want 0", rejected)
	}
}

func TestUploadURLReclaimsAllRejectedRows(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	for order := 1; order <= 3; order++ {
		insertPhotoRow(t, userID, "photos/rejected-all-"+string(rune('0'+order)), order, photoModerationRejected, 0)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}
	var order int
	if err := testDB.QueryRow("SELECT display_order FROM photos").Scan(&order); err != nil {
		t.Fatal(err)
	}
	if order != 1 {
		t.Fatalf("display_order = %d, want 1", order)
	}
}

func TestUploadURLKeepsApprovedRows(t *testing.T) {
	service, userID, _, _ := newPhotoTestService(t)
	for order := 1; order <= 3; order++ {
		insertPhotoRow(t, userID, "photos/approved-keep-"+string(rune('0'+order)), order, photoModerationApproved, 0)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
}

func TestPhotoURLIsOwnerOnlyShortLived(t *testing.T) {
	service, userID, auth, fake := newPhotoTestService(t)
	const pendingKey = "photos/url-pending"
	pendingID := insertPhotoRow(t, userID, pendingKey, 1, photoModerationPending, 0)

	response := getPhotoURLRequest(service, userID, pendingID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers = %v", response.Header())
	}
	var body photoURLBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != photoModerationPending || body.ExpiresInSeconds != 300 || body.URL == "" {
		t.Fatalf("body = %#v", body)
	}
	fake.mu.Lock()
	presignedKey := fake.presignedGetKeys[0]
	presignExpiry := fake.presignGetExpiries[0]
	fake.mu.Unlock()
	if presignedKey != pendingKey || presignExpiry != photoUploadURLExpiry {
		t.Fatalf("presign = %q %v", presignedKey, presignExpiry)
	}

	otherID := insertTestUser(t, auth, "other-url@example.com")
	foreign := getPhotoURLRequest(service, otherID, pendingID)
	unknown := getPhotoURLRequest(service, userID, "00000000-0000-4000-8000-000000000000")
	if foreign.Code != http.StatusNotFound || unknown.Code != http.StatusNotFound {
		t.Fatalf("foreign %d unknown %d, want 404", foreign.Code, unknown.Code)
	}
	if foreign.Body.String() != unknown.Body.String() {
		t.Fatal("foreign and unknown responses differ")
	}

	insertPhotoRow(t, userID, "photos/url-rejected", 2, photoModerationRejected, 0)
	var rejectedID string
	if err := testDB.QueryRow("SELECT id FROM photos WHERE object_key = 'photos/url-rejected'").Scan(&rejectedID); err != nil {
		t.Fatal(err)
	}
	if response := getPhotoURLRequest(service, userID, rejectedID); response.Code != http.StatusNotFound {
		t.Fatalf("rejected URL status = %d, want 404", response.Code)
	}
}

func TestPhotoListIsOwnerOnly(t *testing.T) {
	service, userID, auth, _ := newPhotoTestService(t)
	insertPhotoRow(t, userID, "photos/list-1", 1, photoModerationPending, 0)
	insertPhotoRow(t, userID, "photos/list-2", 2, photoModerationApproved, 0)
	insertPhotoRow(t, userID, "photos/list-3", 3, photoModerationRejected, 0)
	otherID := insertTestUser(t, auth, "other-list@example.com")
	insertPhotoRow(t, otherID, "photos/other-list", 1, photoModerationApproved, 0)

	response := getPhotoListRequest(service, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	var body photoListBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Photos) != 3 {
		t.Fatalf("photos = %d, want 3", len(body.Photos))
	}
	for index, item := range body.Photos {
		if item.DisplayOrder != index+1 || item.PhotoID == "" || item.Status == "" {
			t.Fatalf("item %d = %#v", index, item)
		}
	}
	for _, forbidden := range []string{"url", "object_key", "photos/list"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("list leaks %q: %s", forbidden, response.Body.String())
		}
	}

	var otherBody photoListBody
	ownList := getPhotoListRequest(service, otherID)
	if ownList.Code != http.StatusOK {
		t.Fatalf("other list status = %d", ownList.Code)
	}
	if err := json.Unmarshal(ownList.Body.Bytes(), &otherBody); err != nil {
		t.Fatal(err)
	}
	if len(otherBody.Photos) != 1 {
		t.Fatalf("other owner's list = %d photos, want 1", len(otherBody.Photos))
	}
	thirdID := insertTestUser(t, auth, "third-list@example.com")
	empty := getPhotoListRequest(service, thirdID)
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"photos":[]`) {
		t.Fatalf("empty list = %d %s", empty.Code, empty.Body.String())
	}
}

func TestProcessModerationTimeoutKeepsPending(t *testing.T) {
	service, userID, _, fake := newPhotoTestService(t)
	const key = "photos/moderation-timeout"
	fake.objects[key] = photoTestJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, photoModerationPending, 0)
	service.moderate = func(ctx context.Context, _ []byte) (moderationVerdict, error) {
		<-ctx.Done()
		return moderationVerdict{}, ctx.Err()
	}

	requestContext, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	response := processPhotoRequestContext(requestContext, service, userID, photoID, "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	var status photoModerationStatus
	var reference sql.NullString
	if err := testDB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != photoModerationPending || reference.Valid {
		t.Fatalf("row = %q %#v", status, reference)
	}
	storedKey := storedPhotoKey(t, photoID)
	if _, ok := fake.objects[storedKey]; !ok {
		t.Fatal("timeout deleted the sanitized object")
	}
}
