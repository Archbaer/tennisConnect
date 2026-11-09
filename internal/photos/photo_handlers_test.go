package photos

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

	"tennisconnect/internal/moderation"
	"tennisconnect/internal/testutil"
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

func (f *fakePhotoStorage) PresignPut(_ context.Context, key string, expires time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.presignError != nil {
		return "", f.presignError
	}
	f.presignedKeys = append(f.presignedKeys, key)
	f.presignExpiries = append(f.presignExpiries, expires)
	return "https://storage.example.test/" + key + "?X-Amz-Signature=signature-secret", nil
}

func (f *fakePhotoStorage) PresignGet(_ context.Context, key string, expires time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.presignGetError != nil {
		return "", f.presignGetError
	}
	f.presignedGetKeys = append(f.presignedGetKeys, key)
	f.presignGetExpiries = append(f.presignGetExpiries, expires)
	return "https://storage.example.test/" + key + "?X-Amz-Signature=get-secret", nil
}

func (f *fakePhotoStorage) Get(ctx context.Context, key string) ([]byte, error) {
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
		return nil, ErrObjectMissing
	}
	return append([]byte(nil), data...), nil
}

func (f *fakePhotoStorage) Put(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putError != nil {
		return f.putError
	}
	f.objects[key] = append([]byte(nil), data...)
	return nil
}

func (f *fakePhotoStorage) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteError != nil {
		return f.deleteError
	}
	f.deletedKeys = append(f.deletedKeys, key)
	delete(f.objects, key)
	return nil
}

func (f *fakePhotoStorage) objectCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.objects)
}

func newPhotoTestService(t *testing.T) (*Service, string, *fakePhotoStorage) {
	t.Helper()
	testutil.ResetTables(t)
	userID := testutil.InsertUser(t, "photographer@example.com")
	fake := &fakePhotoStorage{objects: make(map[string][]byte)}
	service := &Service{
		DB:         testutil.DB,
		AppBaseURL: "http://localhost:8080",
		Storage:    fake,
		Moderator:  moderatorFunc(approveModeration),
	}
	return service, userID, fake
}

func uploadPhotoRequest(service *Service, userID string) *httptest.ResponseRecorder {
	return uploadPhotoRequestContext(context.Background(), service, userID, "")
}

func uploadPhotoRequestContext(ctx context.Context, service *Service, userID, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", nil).WithContext(ctx)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	service.UploadURL(response, request, userID)
	return response
}

func processPhotoRequest(service *Service, userID, photoID string) *httptest.ResponseRecorder {
	return processPhotoRequestContext(context.Background(), service, userID, photoID, "")
}

func processPhotoRequestContext(ctx context.Context, service *Service, userID, photoID, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/photos/"+photoID+"/process", nil).WithContext(ctx)
	request.SetPathValue("id", photoID)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	service.Process(response, request, userID)
	return response
}

func getPhotoURLRequest(service *Service, userID, photoID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/photos/"+photoID+"/url", nil)
	request.SetPathValue("id", photoID)
	response := httptest.NewRecorder()
	service.URL(response, request, userID)
	return response
}

func getPhotoListRequest(service *Service, userID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/photos", nil)
	response := httptest.NewRecorder()
	service.List(response, request, userID)
	return response
}

func insertPhotoRow(t *testing.T, ownerID, key string, displayOrder int, status moderation.Status, age time.Duration) string {
	t.Helper()
	var photoID string
	err := testutil.DB.QueryRow(`
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
	if err := testutil.DB.QueryRow("SELECT count(*) FROM photos").Scan(&count); err != nil {
		t.Fatalf("count photos: %v", err)
	}
	return count
}

func storedPhotoKey(t *testing.T, photoID string) string {
	t.Helper()
	var key string
	if err := testutil.DB.QueryRow("SELECT object_key FROM photos WHERE id = $1", photoID).Scan(&key); err != nil {
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
	service, userID, fake := newPhotoTestService(t)
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
	var status moderation.Status
	if err := testutil.DB.QueryRow("SELECT owner_id, display_order, moderation_status FROM photos WHERE id = $1", body.PhotoID).
		Scan(&ownerID, &order, &status); err != nil {
		t.Fatalf("read photo row: %v", err)
	}
	if ownerID != userID || order != 1 || status != moderation.StatusPending {
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
	service, userID, _ := newPhotoTestService(t)
	response := uploadPhotoRequestContext(context.Background(), service, userID, "https://evil.example")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("cross-origin upload created %d rows", count)
	}
}

func TestUploadURLFillsOneToThreeThenConflicts(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		response := uploadPhotoRequest(service, userID)
		if response.Code != http.StatusCreated {
			t.Fatalf("upload %d status = %d, body = %s", i, response.Code, response.Body.String())
		}
	}
	rows, err := testutil.DB.Query("SELECT display_order FROM photos ORDER BY display_order")
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
	service, userID, fake := newPhotoTestService(t)
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
	if fake.presignExpiries[0] != uploadURLExpiry {
		t.Fatalf("presign expiry = %v, want %v", fake.presignExpiries[0], uploadURLExpiry)
	}
	if !strings.Contains(body.UploadURL, storedPhotoKey(t, body.PhotoID)) {
		t.Fatalf("upload URL is not scoped to the photo key: %q", body.UploadURL)
	}
}

func TestConcurrentUploadURLsNeverExceedThree(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
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
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/original"
	fake.objects[originalKey] = testutil.PhotoJPEGWithEXIF(t)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)

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
	if body.PhotoID != photoID || body.Status != string(moderation.StatusApproved) {
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
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/original"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 40, 30)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)

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
	var status moderation.Status
	if err := testutil.DB.QueryRow("SELECT moderation_status FROM photos WHERE id = $1", photoID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusApproved {
		t.Fatalf("status after second process = %q, want approved", status)
	}
}

func TestProcessDeletesOversizedObjectAndRow(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/oversized"
	fake.objects[key] = testutil.PhotoJPEG(t, 10, 10)
	fake.getError = ErrObjectTooLarge
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)

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
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/not-an-image"
	fake.objects[key] = []byte("not an image")
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d, want 0", count)
	}

	gifKey := "photos/gif"
	fake.objects[gifKey] = []byte("GIF87a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff\x21\xf9\x04\x01\x00\x00\x00\x00\x2c\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02\x44\x01\x00\x3b")
	gifID := insertPhotoRow(t, userID, gifKey, 1, moderation.StatusPending, 0)
	if response := processPhotoRequest(service, userID, gifID); response.Code != http.StatusBadRequest {
		t.Fatalf("GIF status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count after GIF = %d, want 0", count)
	}
}

func TestProcessRejectsMissingObject(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/missing"
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
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
	service, userID, _ := newPhotoTestService(t)
	const key = "photos/owned"
	fake := &fakePhotoStorage{objects: map[string][]byte{key: testutil.PhotoJPEG(t, 10, 10)}}
	service.Storage = fake
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)

	otherID := testutil.InsertUser(t, "other@example.com")
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
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/approved"
	fake.objects[key] = testutil.PhotoJPEG(t, 10, 10)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusApproved, 0)
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
}

func TestProcessRejectsCrossOrigin(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/csrf"
	fake.objects[key] = testutil.PhotoJPEG(t, 10, 10)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
	response := processPhotoRequestContext(context.Background(), service, userID, photoID, "https://evil.example")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
}

func TestProcessConcurrentLoserKeepsWinner(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/race"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)

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
	service, userID, fake := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		key := "photos/stale-" + string(rune('0'+i))
		fake.objects[key] = []byte("stale")
		insertPhotoRow(t, userID, key, i, moderation.StatusPending, 2*time.Hour)
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
	service, userID, fake := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		key := "photos/fresh-" + string(rune('0'+i))
		fake.objects[key] = []byte("fresh")
		insertPhotoRow(t, userID, key, i, moderation.StatusPending, 0)
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
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/claim"
	fake.objects[key] = testutil.PhotoJPEG(t, 10, 10)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 59*time.Minute)

	ctx := context.Background()
	claimedKey, err := claimPendingPhoto(ctx, testutil.DB, photoID, userID)
	if err != nil || claimedKey != key {
		t.Fatalf("claim = %q, %v", claimedKey, err)
	}
	keys, err := cleanupStalePending(ctx, testutil.DB, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("cleanup removed a claimed row: %v", keys)
	}
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM photos").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}

	const expiredKey = "photos/expired"
	fake.objects[expiredKey] = testutil.PhotoJPEG(t, 10, 10)
	expiredID := insertPhotoRow(t, userID, expiredKey, 2, moderation.StatusPending, 2*time.Hour)
	if _, err := claimPendingPhoto(ctx, testutil.DB, expiredID, userID); !errors.Is(err, errPhotoUploadExpired) {
		t.Fatalf("claim of expired row err = %v, want errPhotoUploadExpired", err)
	}
	if calls := fake.getCalls; calls != 0 {
		t.Fatalf("storage get calls = %d, want 0", calls)
	}
	keys, err = cleanupStalePending(ctx, testutil.DB, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != expiredKey {
		t.Fatalf("cleanup keys = %v, want [%q]", keys, expiredKey)
	}

	// The handler path for an expired upload returns 409 without touching storage.
	service.Storage = fake
	expiredID2 := insertPhotoRow(t, userID, "photos/expired-2", 3, moderation.StatusPending, 2*time.Hour)
	if response := processPhotoRequest(service, userID, expiredID2); response.Code != http.StatusConflict {
		t.Fatalf("expired upload status = %d, want 409", response.Code)
	}
	if calls := fake.getCalls; calls != 0 {
		t.Fatalf("storage get calls = %d, want 0", calls)
	}
}

func TestPhotoEndpointsFailClosedWithoutStorage(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
	service.Storage = nil
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("upload status = %d, want 503", response.Code)
	}
	if response := processPhotoRequest(service, userID, "00000000-0000-4000-8000-000000000000"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("process status = %d, want 503", response.Code)
	}
}

func TestProcessStorageErrorsAreBoundedAndGeneric(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/leak"
	fake.getError = errors.New("bucket=private-bucket key=" + key + " signature=signature-secret")
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)

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

	blocking := &fakePhotoStorage{objects: map[string][]byte{key: testutil.PhotoJPEG(t, 10, 10)}, blockGet: true}
	service.Storage = blocking
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

func TestUploadURLPresignFailureFreesSlot(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
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
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/put-failure"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 20, 20)
	fake.putError = errors.New("put failed")
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)

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
	service, userID, fake := newPhotoTestService(t)
	for i := 1; i <= 3; i++ {
		key := "photos/cleanup-" + string(rune('0'+i))
		fake.objects[key] = []byte("stale")
		insertPhotoRow(t, userID, key, i, moderation.StatusPending, 2*time.Hour)
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
	verdict moderation.Verdict
	err     error
	images  [][]byte
	hook    func(call int)
}

func (f *fakeModeration) Moderate(_ context.Context, image []byte) (moderation.Verdict, error) {
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

func approveModeration(context.Context, []byte) (moderation.Verdict, error) {
	return moderation.Verdict{Status: moderation.StatusApproved, Reference: "op-test"}, nil
}

type moderatorFunc func(context.Context, []byte) (moderation.Verdict, error)

func (f moderatorFunc) Moderate(ctx context.Context, image []byte) (moderation.Verdict, error) {
	return f(ctx, image)
}

type photoURLBody struct {
	PhotoID          string            `json:"photo_id"`
	Status           moderation.Status `json:"status"`
	URL              string            `json:"url"`
	ExpiresInSeconds int               `json:"expires_in_seconds"`
}

type photoListBody struct {
	Photos []struct {
		PhotoID      string            `json:"photo_id"`
		DisplayOrder int               `json:"display_order"`
		Status       moderation.Status `json:"status"`
	} `json:"photos"`
}

func TestProcessApprovesCleanPhoto(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/approve"
	fake.objects[originalKey] = testutil.PhotoJPEGWithEXIF(t)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)
	checker := &fakeModeration{verdict: moderation.Verdict{Status: moderation.StatusApproved, Reference: "op-approved"}}
	service.Moderator = checker

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		PhotoID string            `json:"photo_id"`
		Status  moderation.Status `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != moderation.StatusApproved {
		t.Fatalf("body status = %q, want approved", body.Status)
	}
	var status moderation.Status
	var reference sql.NullString
	if err := testutil.DB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusApproved || reference.String != "op-approved" {
		t.Fatalf("row = %q / %q", status, reference.String)
	}
	if checker.callCount() != 1 {
		t.Fatalf("moderation calls = %d", checker.callCount())
	}
	image := checker.images[0]
	if bytes.Contains(image, []byte("Exif")) || bytes.Contains(image, []byte("private-exif-marker")) {
		t.Fatal("moderator received raw EXIF bytes")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(image))
	if err != nil || config.Width > maxPhotoDimension || config.Height > maxPhotoDimension {
		t.Fatalf("moderator image = %dx%d, %v", config.Width, config.Height, err)
	}
	eligible, err := photoApprovedForDiscovery(context.Background(), testutil.DB, userID)
	if err != nil || !eligible {
		t.Fatalf("discovery eligible = %v, %v", eligible, err)
	}
}

func TestProcessInconclusiveStaysPending(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/inconclusive"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)
	checker := &fakeModeration{verdict: moderation.Verdict{Status: moderation.StatusPending}}
	service.Moderator = checker

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Status moderation.Status `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != moderation.StatusPending {
		t.Fatalf("body status = %q, want pending", body.Status)
	}
	var status moderation.Status
	var reference sql.NullString
	if err := testutil.DB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusPending || reference.Valid {
		t.Fatalf("row = %q %#v", status, reference)
	}

	response = processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("repeat status = %d, body = %s", response.Code, response.Body.String())
	}
	if checker.callCount() != 2 {
		t.Fatalf("moderation calls = %d, want 2", checker.callCount())
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d", count)
	}
}

func TestProcessProviderFailureKeepsPending(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/provider-failure"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)
	checker := &fakeModeration{err: errors.New("provider down")}
	service.Moderator = checker

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	var status moderation.Status
	if err := testutil.DB.QueryRow("SELECT moderation_status FROM photos WHERE id = $1", photoID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusPending {
		t.Fatalf("status = %q, want pending", status)
	}
	key := storedPhotoKey(t, photoID)
	if key == originalKey {
		t.Fatal("post-swap failure did not keep the sanitized object")
	}
	if _, ok := fake.objects[key]; !ok {
		t.Fatal("sanitized object was deleted on a retryable failure")
	}

	checker.err = nil
	checker.verdict = moderation.Verdict{Status: moderation.StatusApproved, Reference: "op-retry"}
	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestProcessUnconfiguredModerationFailsClosed(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/no-moderation"
	fake.objects[key] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
	service.Moderator = nil
	var before time.Time
	if err := testutil.DB.QueryRow("SELECT updated_at FROM photos WHERE id = $1", photoID).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d, want 0", fake.getCalls)
	}
	var after time.Time
	if err := testutil.DB.QueryRow("SELECT updated_at FROM photos WHERE id = $1", photoID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Equal(before) {
		t.Fatal("fail-closed path claimed the row")
	}
}

func TestProcessRejectsInvalidImageBeforeModeration(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/invalid"
	fake.objects[key] = []byte("not an image")
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
	checker := &fakeModeration{verdict: moderation.Verdict{Status: moderation.StatusApproved}}
	service.Moderator = checker

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if checker.callCount() != 0 {
		t.Fatalf("moderation calls = %d, want 0", checker.callCount())
	}
	if count := countPhotos(t); count != 0 {
		t.Fatalf("photo count = %d", count)
	}
}

func TestProcessNonPendingIs409WithoutStorage(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/terminal"
	fake.objects[key] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusApproved, 0)
	checker := &fakeModeration{verdict: moderation.Verdict{Status: moderation.StatusApproved}}
	service.Moderator = checker

	if response := processPhotoRequest(service, userID, photoID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if fake.getCalls != 0 {
		t.Fatalf("storage get calls = %d", fake.getCalls)
	}
	if checker.callCount() != 0 {
		t.Fatalf("moderation calls = %d", checker.callCount())
	}
}

func TestModerationCasStaleKey(t *testing.T) {
	_, userID, _ := newPhotoTestService(t)
	photoID := insertPhotoRow(t, userID, "photos/cas", 1, moderation.StatusPending, 0)

	applied, err := applyModeration(context.Background(), testutil.DB, photoID, userID, "photos/stale", moderation.Verdict{Status: moderation.StatusApproved, Reference: "op-stale"})
	if err != nil {
		t.Fatalf("apply stale key: %v", err)
	}
	if applied {
		t.Fatal("stale key moderation was applied")
	}
	var status moderation.Status
	if err := testutil.DB.QueryRow("SELECT moderation_status FROM photos WHERE id = $1", photoID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusPending {
		t.Fatalf("status = %q, want pending", status)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := applyModeration(cancelled, testutil.DB, photoID, userID, "photos/cas", moderation.Verdict{Status: moderation.StatusApproved}); err == nil {
		t.Fatal("cancelled context did not surface a database error")
	}
}

func TestModerationForcedRaceKeepsWinnerVerdict(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/race-moderation"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)

	firstInModeration := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls int32
	service.Moderator = moderatorFunc(func(_ context.Context, _ []byte) (moderation.Verdict, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			close(firstInModeration)
			<-releaseFirst
			return moderation.Verdict{Status: moderation.StatusRejected, Reference: "loser"}, nil
		}
		return moderation.Verdict{Status: moderation.StatusApproved, Reference: "winner"}, nil
	})

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
	var status moderation.Status
	var reference sql.NullString
	if err := testutil.DB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusApproved || reference.String != "winner" {
		t.Fatalf("row = %q %q, want approved/winner", status, reference.String)
	}
	if fake.objectCount() != 1 {
		t.Fatalf("stored object count = %d, want 1", fake.objectCount())
	}
}

func TestModerationNeverLeaksImageOrSecrets(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/leak-moderation"
	fake.objects[key] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
	service.Moderator = moderatorFunc(func(context.Context, []byte) (moderation.Verdict, error) {
		return moderation.Verdict{}, errors.New("api_secret=secret-value key=" + key + " signature=signature-secret")
	})

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
	service, userID, fake := newPhotoTestService(t)
	const originalKey = "photos/rejected"
	fake.objects[originalKey] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, originalKey, 1, moderation.StatusPending, 0)
	service.Moderator = moderatorFunc(func(context.Context, []byte) (moderation.Verdict, error) {
		return moderation.Verdict{Status: moderation.StatusRejected, Reference: "op-reject"}, nil
	})

	response := processPhotoRequest(service, userID, photoID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Status moderation.Status `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != moderation.StatusRejected {
		t.Fatalf("body status = %q, want rejected", body.Status)
	}
	var status moderation.Status
	var reference sql.NullString
	if err := testutil.DB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusRejected || reference.String != "op-reject" {
		t.Fatalf("row = %q %q", status, reference.String)
	}
	if fake.objectCount() != 0 {
		t.Fatalf("rejected object was kept: %d", fake.objectCount())
	}
	if response := getPhotoURLRequest(service, userID, photoID); response.Code != http.StatusNotFound {
		t.Fatalf("rejected URL status = %d, want 404", response.Code)
	}
	if eligible, err := photoApprovedForDiscovery(context.Background(), testutil.DB, userID); err != nil || eligible {
		t.Fatalf("discovery eligible = %v, %v", eligible, err)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("slot was not reusable, upload status = %d", response.Code)
	}
}

func TestDiscoveryEligibilityRequiresApprovedPhoto(t *testing.T) {
	_, userID, _ := newPhotoTestService(t)
	ctx := context.Background()
	eligible, err := photoApprovedForDiscovery(ctx, testutil.DB, userID)
	if err != nil || eligible {
		t.Fatalf("empty owner eligible = %v, %v", eligible, err)
	}
	insertPhotoRow(t, userID, "photos/pending-eligibility", 1, moderation.StatusPending, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testutil.DB, userID); err != nil || eligible {
		t.Fatalf("pending owner eligible = %v, %v", eligible, err)
	}
	insertPhotoRow(t, userID, "photos/rejected-eligibility", 2, moderation.StatusRejected, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testutil.DB, userID); err != nil || eligible {
		t.Fatalf("rejected owner eligible = %v, %v", eligible, err)
	}
	insertPhotoRow(t, userID, "photos/approved-eligibility", 3, moderation.StatusApproved, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testutil.DB, userID); err != nil || !eligible {
		t.Fatalf("approved owner eligible = %v, %v", eligible, err)
	}
	otherID := testutil.InsertUser(t, "other-photos@example.com")
	insertPhotoRow(t, otherID, "photos/other-approved", 1, moderation.StatusApproved, 0)
	if eligible, err := photoApprovedForDiscovery(ctx, testutil.DB, otherID); err != nil || !eligible {
		t.Fatalf("other owner eligibility = %v, %v", eligible, err)
	}
}

func TestUploadURLReclaimsRejectedSlot(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
	for order := 1; order <= 2; order++ {
		insertPhotoRow(t, userID, "photos/approved-slot-"+string(rune('0'+order)), order, moderation.StatusApproved, 0)
	}
	insertPhotoRow(t, userID, "photos/rejected-slot", 3, moderation.StatusRejected, 0)

	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
	var rejected int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM photos WHERE moderation_status = 'rejected'").Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected != 0 {
		t.Fatalf("rejected rows = %d, want 0", rejected)
	}
}

func TestUploadURLReclaimsAllRejectedRows(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
	for order := 1; order <= 3; order++ {
		insertPhotoRow(t, userID, "photos/rejected-all-"+string(rune('0'+order)), order, moderation.StatusRejected, 0)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if count := countPhotos(t); count != 1 {
		t.Fatalf("photo count = %d, want 1", count)
	}
	var order int
	if err := testutil.DB.QueryRow("SELECT display_order FROM photos").Scan(&order); err != nil {
		t.Fatal(err)
	}
	if order != 1 {
		t.Fatalf("display_order = %d, want 1", order)
	}
}

func TestUploadURLKeepsApprovedRows(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
	for order := 1; order <= 3; order++ {
		insertPhotoRow(t, userID, "photos/approved-keep-"+string(rune('0'+order)), order, moderation.StatusApproved, 0)
	}
	if response := uploadPhotoRequest(service, userID); response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if count := countPhotos(t); count != 3 {
		t.Fatalf("photo count = %d, want 3", count)
	}
}

func TestPhotoURLIsOwnerOnlyShortLived(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const pendingKey = "photos/url-pending"
	pendingID := insertPhotoRow(t, userID, pendingKey, 1, moderation.StatusPending, 0)

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
	if body.Status != moderation.StatusPending || body.ExpiresInSeconds != 300 || body.URL == "" {
		t.Fatalf("body = %#v", body)
	}
	fake.mu.Lock()
	presignedKey := fake.presignedGetKeys[0]
	presignExpiry := fake.presignGetExpiries[0]
	fake.mu.Unlock()
	if presignedKey != pendingKey || presignExpiry != uploadURLExpiry {
		t.Fatalf("presign = %q %v", presignedKey, presignExpiry)
	}

	otherID := testutil.InsertUser(t, "other-url@example.com")
	foreign := getPhotoURLRequest(service, otherID, pendingID)
	unknown := getPhotoURLRequest(service, userID, "00000000-0000-4000-8000-000000000000")
	if foreign.Code != http.StatusNotFound || unknown.Code != http.StatusNotFound {
		t.Fatalf("foreign %d unknown %d, want 404", foreign.Code, unknown.Code)
	}
	if foreign.Body.String() != unknown.Body.String() {
		t.Fatal("foreign and unknown responses differ")
	}

	insertPhotoRow(t, userID, "photos/url-rejected", 2, moderation.StatusRejected, 0)
	var rejectedID string
	if err := testutil.DB.QueryRow("SELECT id FROM photos WHERE object_key = 'photos/url-rejected'").Scan(&rejectedID); err != nil {
		t.Fatal(err)
	}
	if response := getPhotoURLRequest(service, userID, rejectedID); response.Code != http.StatusNotFound {
		t.Fatalf("rejected URL status = %d, want 404", response.Code)
	}
}

func TestPhotoListIsOwnerOnly(t *testing.T) {
	service, userID, _ := newPhotoTestService(t)
	insertPhotoRow(t, userID, "photos/list-1", 1, moderation.StatusPending, 0)
	insertPhotoRow(t, userID, "photos/list-2", 2, moderation.StatusApproved, 0)
	insertPhotoRow(t, userID, "photos/list-3", 3, moderation.StatusRejected, 0)
	otherID := testutil.InsertUser(t, "other-list@example.com")
	insertPhotoRow(t, otherID, "photos/other-list", 1, moderation.StatusApproved, 0)

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
	thirdID := testutil.InsertUser(t, "third-list@example.com")
	empty := getPhotoListRequest(service, thirdID)
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"photos":[]`) {
		t.Fatalf("empty list = %d %s", empty.Code, empty.Body.String())
	}
}

func TestProcessModerationTimeoutKeepsPending(t *testing.T) {
	service, userID, fake := newPhotoTestService(t)
	const key = "photos/moderation-timeout"
	fake.objects[key] = testutil.PhotoJPEG(t, 20, 20)
	photoID := insertPhotoRow(t, userID, key, 1, moderation.StatusPending, 0)
	service.Moderator = moderatorFunc(func(ctx context.Context, _ []byte) (moderation.Verdict, error) {
		<-ctx.Done()
		return moderation.Verdict{}, ctx.Err()
	})

	requestContext, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	response := processPhotoRequestContext(requestContext, service, userID, photoID, "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	var status moderation.Status
	var reference sql.NullString
	if err := testutil.DB.QueryRow("SELECT moderation_status, provider_reference FROM photos WHERE id = $1", photoID).
		Scan(&status, &reference); err != nil {
		t.Fatal(err)
	}
	if status != moderation.StatusPending || reference.Valid {
		t.Fatalf("row = %q %#v", status, reference)
	}
	storedKey := storedPhotoKey(t, photoID)
	if _, ok := fake.objects[storedKey]; !ok {
		t.Fatal("timeout deleted the sanitized object")
	}
}
