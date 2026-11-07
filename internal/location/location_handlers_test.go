package location

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

type fakeGeocoder struct {
	mu            sync.Mutex
	forwardPoint  Point
	forwardLabel  string
	forwardErr    error
	forwardInputs []string
	reverseLabel  string
	reverseErr    error
	reverseInputs [][2]float64
}

func newFakeGeocoder() *fakeGeocoder {
	return &fakeGeocoder{
		forwardPoint: Point{Latitude: 51.4649, Longitude: -0.1611},
		forwardLabel: "Battersea",
		reverseLabel: "Battersea",
	}
}

func (f *fakeGeocoder) Forward(_ context.Context, area string) (Point, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forwardInputs = append(f.forwardInputs, area)
	return f.forwardPoint, f.forwardLabel, f.forwardErr
}

func (f *fakeGeocoder) Reverse(_ context.Context, latitude, longitude float64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reverseInputs = append(f.reverseInputs, [2]float64{latitude, longitude})
	return f.reverseLabel, f.reverseErr
}

func (f *fakeGeocoder) lastForwardInput(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.forwardInputs) == 0 {
		t.Fatal("geocoder received no forward request")
	}
	return f.forwardInputs[len(f.forwardInputs)-1]
}

func newTestLocationService(t *testing.T) (*Service, *fakeGeocoder, string) {
	t.Helper()
	testutil.ResetTables(t)
	geocoder := newFakeGeocoder()
	return &Service{DB: testutil.DB, Geocoder: geocoder}, geocoder, testutil.InsertUser(t, "player@example.com")
}

func locationRequest(service *Service, method, path, userID, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	switch path {
	case "/api/v1/location":
		if method == http.MethodPut {
			service.Put(response, request, userID)
		} else {
			service.Get(response, request, userID)
		}
	case "/api/v1/location/travel":
		if method == http.MethodPut {
			service.PutTravel(response, request, userID)
		} else {
			service.DeleteTravel(response, request, userID)
		}
	default:
		panic("unknown location path " + path)
	}
	return response
}

func marshalBody(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return string(body)
}

func putCoordinates(t *testing.T, service *Service, userID string, latitude, longitude float64) *httptest.ResponseRecorder {
	t.Helper()
	return locationRequest(service, http.MethodPut, "/api/v1/location", userID,
		marshalBody(t, map[string]any{"latitude": latitude, "longitude": longitude}))
}

func putArea(t *testing.T, service *Service, userID, area string) *httptest.ResponseRecorder {
	t.Helper()
	return locationRequest(service, http.MethodPut, "/api/v1/location", userID,
		marshalBody(t, map[string]any{"area": area}))
}

func putTravel(t *testing.T, service *Service, userID, area string, days any) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]any{"area": area}
	if days != nil {
		body["days"] = days
	}
	return locationRequest(service, http.MethodPut, "/api/v1/location/travel", userID, marshalBody(t, body))
}

func getLocation(t *testing.T, service *Service, userID string) *httptest.ResponseRecorder {
	t.Helper()
	return locationRequest(service, http.MethodGet, "/api/v1/location", userID, "")
}

func deleteTravel(t *testing.T, service *Service, userID string) *httptest.ResponseRecorder {
	t.Helper()
	return locationRequest(service, http.MethodDelete, "/api/v1/location/travel", userID, "")
}

type locationBody struct {
	AreaLabel       string  `json:"area_label"`
	TravelAreaLabel *string `json:"travel_area_label"`
	TravelExpiresAt *string `json:"travel_expires_at"`
}

type travelBody struct {
	AreaLabel string `json:"area_label"`
	ExpiresAt string `json:"expires_at"`
}

func decodeLocation(t *testing.T, response *httptest.ResponseRecorder) locationBody {
	t.Helper()
	var body locationBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode location response: %v", err)
	}
	return body
}

func storedHomeLabel(t *testing.T, userID string) string {
	t.Helper()
	var label string
	if err := testutil.DB.QueryRow("SELECT home_area_label FROM locations WHERE user_id = $1", userID).Scan(&label); err != nil {
		t.Fatalf("read stored home label: %v", err)
	}
	return label
}

func storedCoordinates(t *testing.T, userID string) (float64, float64) {
	t.Helper()
	var latitude, longitude float64
	if err := testutil.DB.QueryRow("SELECT ST_Y(home::geometry), ST_X(home::geometry) FROM locations WHERE user_id = $1", userID).Scan(&latitude, &longitude); err != nil {
		t.Fatalf("read stored home coordinates: %v", err)
	}
	return latitude, longitude
}

func locationRowCount(t *testing.T, userID string) int {
	t.Helper()
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM locations WHERE user_id = $1", userID).Scan(&count); err != nil {
		t.Fatalf("count location rows: %v", err)
	}
	return count
}

func TestLocationUKBoundaries(t *testing.T) {
	service, geocoder, userID := newTestLocationService(t)
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	response := putCoordinates(t, service, userID, 51.4649, -0.1611)
	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", response.Code, response.Body.String())
	}
	if body := decodeLocation(t, response); body.AreaLabel != "Battersea" {
		t.Fatalf("area_label = %q", body.AreaLabel)
	}
	if label := storedHomeLabel(t, userID); label != "Battersea" {
		t.Fatalf("stored label = %q", label)
	}
	latitude, longitude := storedCoordinates(t, userID)
	if latitude != 51.4649 || longitude != -0.1611 {
		t.Fatalf("stored coordinates = %v,%v", latitude, longitude)
	}

	geocoder.reverseErr = ErrOutsideUK
	otherUser := testutil.InsertUser(t, "paris@example.com")
	if response := putCoordinates(t, service, otherUser, 48.8566, 2.3522); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-UK status = %d, body = %s", response.Code, response.Body.String())
	}
	if rows := locationRowCount(t, otherUser); rows != 0 {
		t.Fatalf("non-UK stored %d rows", rows)
	}

	thirdUser := testutil.InsertUser(t, "range@example.com")
	if response := putCoordinates(t, service, thirdUser, 91, 0); response.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range status = %d", response.Code)
	}
	if rows := locationRowCount(t, thirdUser); rows != 0 {
		t.Fatalf("out-of-range stored %d rows", rows)
	}

	for _, forbidden := range []string{"51.4649", "-0.1611", "pk.", "SW11"} {
		if strings.Contains(logs.String(), forbidden) || strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("location response or log leaked %q", forbidden)
		}
	}
}

func TestLocationManualArea(t *testing.T) {
	service, geocoder, userID := newTestLocationService(t)
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	response := putArea(t, service, userID, "SW11 4NJ")
	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", response.Code, response.Body.String())
	}
	if geocoder.lastForwardInput(t) != "SW11 4NJ" {
		t.Fatalf("forward input = %q", geocoder.lastForwardInput(t))
	}
	if body := decodeLocation(t, response); body.AreaLabel != "Battersea" {
		t.Fatalf("area_label = %q", body.AreaLabel)
	}
	if strings.Contains(response.Body.String(), "SW11") || strings.Contains(logs.String(), "SW11") {
		t.Fatal("postcode leaked into response or log")
	}

	geocoder.forwardLabel = "SW11 4NJ"
	postcodeUser := testutil.InsertUser(t, "postcode@example.com")
	if response := putArea(t, service, postcodeUser, "Battersea"); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("postcode label status = %d, body = %s", response.Code, response.Body.String())
	}
	if rows := locationRowCount(t, postcodeUser); rows != 0 {
		t.Fatalf("postcode label stored %d rows", rows)
	}

	geocoder.forwardLabel = strings.Repeat("x", 81)
	longUser := testutil.InsertUser(t, "long@example.com")
	if response := putArea(t, service, longUser, "Battersea"); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("over-long label status = %d", response.Code)
	}
	if rows := locationRowCount(t, longUser); rows != 0 {
		t.Fatalf("over-long label stored %d rows", rows)
	}

	geocoder.forwardLabel = ""
	emptyLabelUser := testutil.InsertUser(t, "emptylabel@example.com")
	if response := putArea(t, service, emptyLabelUser, "Battersea"); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty label status = %d", response.Code)
	}

	for _, request := range []struct {
		name string
		body string
	}{
		{name: "blank area", body: `{"area":"   "}`},
		{name: "over-long input", body: marshalBody(t, map[string]any{"area": strings.Repeat("x", 81)})},
		{name: "both", body: `{"area":"Battersea","latitude":51.4,"longitude":-0.1}`},
		{name: "latitude only", body: `{"latitude":51.4}`},
		{name: "empty object", body: `{}`},
	} {
		t.Run(request.name, func(t *testing.T) {
			if response := locationRequest(service, http.MethodPut, "/api/v1/location", userID, request.body); response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}

	if strings.Contains(logs.String(), "SW11") || strings.Contains(logs.String(), "51.4649") {
		t.Fatal("log leaked area input or coordinates")
	}
}

func TestLocationProviderFailures(t *testing.T) {
	service, geocoder, userID := newTestLocationService(t)
	if response := putArea(t, service, userID, "Battersea"); response.Code != http.StatusOK {
		t.Fatalf("initial PUT status = %d", response.Code)
	}

	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "timeout", err: context.DeadlineExceeded, want: http.StatusServiceUnavailable},
		{name: "provider", err: ErrProvider, want: http.StatusServiceUnavailable},
		{name: "transport", err: errors.New("connection refused"), want: http.StatusServiceUnavailable},
		{name: "no result", err: ErrNoResult, want: http.StatusUnprocessableEntity},
		{name: "outside UK", err: ErrOutsideUK, want: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			geocoder.forwardErr = test.err
			response := putArea(t, service, userID, "Paris")
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
			if want := "Battersea"; storedHomeLabel(t, userID) != want {
				t.Fatalf("provider failure overwrote stored label with %q", storedHomeLabel(t, userID))
			}
			for _, forbidden := range []string{"pk.", "Paris", "51.", "-0."} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatalf("provider failure response leaked %q", forbidden)
				}
			}
		})
	}
}

func TestLocationWritesRequireGeocoder(t *testing.T) {
	testutil.ResetTables(t)
	service := &Service{DB: testutil.DB}
	userID := testutil.InsertUser(t, "player@example.com")

	if response := putArea(t, service, userID, "Battersea"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("PUT without geocoder status = %d", response.Code)
	}
	if response := putTravel(t, service, userID, "Edinburgh", nil); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("travel without geocoder status = %d", response.Code)
	}
	if rows := locationRowCount(t, userID); rows != 0 {
		t.Fatalf("writes without geocoder stored %d rows", rows)
	}
	if response := getLocation(t, service, userID); response.Code != http.StatusNotFound {
		t.Fatalf("GET without location status = %d", response.Code)
	}
	if response := deleteTravel(t, service, userID); response.Code != http.StatusNoContent {
		t.Fatalf("DELETE without location status = %d", response.Code)
	}
}

func TestTravelLifecycle(t *testing.T) {
	service, geocoder, userID := newTestLocationService(t)

	if response := putTravel(t, service, userID, "Edinburgh", nil); response.Code != http.StatusConflict {
		t.Fatalf("travel before home status = %d, body = %s", response.Code, response.Body.String())
	} else if !strings.Contains(response.Body.String(), "set your area first") {
		t.Fatalf("travel before home message = %s", response.Body.String())
	}

	if response := putArea(t, service, userID, "Battersea"); response.Code != http.StatusOK {
		t.Fatalf("home PUT status = %d", response.Code)
	}

	geocoder.forwardLabel = "Edinburgh"
	for _, days := range []int{1, 7} {
		response := putTravel(t, service, userID, "Edinburgh", days)
		if response.Code != http.StatusOK {
			t.Fatalf("travel days=%d status = %d, body = %s", days, response.Code, response.Body.String())
		}
		var body travelBody
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.AreaLabel != "Edinburgh" {
			t.Fatalf("travel label = %q", body.AreaLabel)
		}
		expires, err := time.Parse(time.RFC3339, body.ExpiresAt)
		if err != nil {
			t.Fatalf("parse expires_at: %v", err)
		}
		expected := time.Now().Add(time.Duration(days) * 24 * time.Hour)
		if difference := expires.Sub(expected); difference < -2*time.Minute || difference > 2*time.Minute {
			t.Fatalf("expires_at = %v, want about %v", expires, expected)
		}
	}
	for _, days := range []int{0, 8, -1} {
		if response := putTravel(t, service, userID, "Edinburgh", days); response.Code != http.StatusBadRequest {
			t.Fatalf("travel days=%d status = %d", days, response.Code)
		}
	}

	get := getLocation(t, service, userID)
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d", get.Code)
	}
	body := decodeLocation(t, get)
	if body.AreaLabel != "Battersea" || body.TravelAreaLabel == nil || *body.TravelAreaLabel != "Edinburgh" || body.TravelExpiresAt == nil {
		t.Fatalf("GET body = %#v", body)
	}

	geocoder.forwardLabel = "Glasgow"
	if response := putTravel(t, service, userID, "Glasgow", 3); response.Code != http.StatusOK {
		t.Fatalf("second travel status = %d", response.Code)
	}
	var rows int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM locations WHERE user_id = $1", userID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("travel rows = %d, want 1", rows)
	}

	geocoder.reverseLabel = "Battersea"
	if response := putCoordinates(t, service, userID, 51.4649, -0.1611); response.Code != http.StatusOK {
		t.Fatalf("home update status = %d", response.Code)
	}
	if body := decodeLocation(t, getLocation(t, service, userID)); body.TravelAreaLabel == nil || *body.TravelAreaLabel != "Glasgow" {
		t.Fatal("home update cleared live travel")
	}

	if _, err := testutil.DB.Exec("UPDATE locations SET travel_expires_at = now() - interval '1 second' WHERE user_id = $1", userID); err != nil {
		t.Fatal(err)
	}
	if body := decodeLocation(t, getLocation(t, service, userID)); body.TravelAreaLabel != nil || body.TravelExpiresAt != nil {
		t.Fatalf("expired travel still returned: %#v", body)
	}

	if response := deleteTravel(t, service, userID); response.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d", response.Code)
	}
	var travelLabel, travelExpiry *string
	var homeLabel string
	if err := testutil.DB.QueryRow("SELECT home_area_label, travel_area_label, travel_expires_at::text FROM locations WHERE user_id = $1", userID).Scan(&homeLabel, &travelLabel, &travelExpiry); err != nil {
		t.Fatal(err)
	}
	if homeLabel != "Battersea" || travelLabel != nil || travelExpiry != nil {
		t.Fatalf("DELETE left home=%q travel=%v expiry=%v", homeLabel, travelLabel, travelExpiry)
	}
	if response := deleteTravel(t, service, userID); response.Code != http.StatusNoContent {
		t.Fatalf("repeated DELETE status = %d", response.Code)
	}
	if rows := locationRowCount(t, userID); rows != 1 {
		t.Fatalf("DELETE removed the location row (%d rows)", rows)
	}
}

func TestTravelRejectsNonUKAndBadLabels(t *testing.T) {
	service, geocoder, userID := newTestLocationService(t)
	if response := putArea(t, service, userID, "Battersea"); response.Code != http.StatusOK {
		t.Fatalf("home PUT status = %d", response.Code)
	}

	for _, test := range []struct {
		name  string
		err   error
		label string
		want  int
	}{
		{name: "outside UK", err: ErrOutsideUK, want: http.StatusUnprocessableEntity},
		{name: "no result", err: ErrNoResult, want: http.StatusUnprocessableEntity},
		{name: "empty label", label: "", want: http.StatusUnprocessableEntity},
		{name: "over-long label", label: strings.Repeat("x", 81), want: http.StatusUnprocessableEntity},
		{name: "postcode label", label: "SW11 4NJ", want: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			geocoder.forwardErr = test.err
			geocoder.forwardLabel = test.label
			response := putTravel(t, service, userID, "Paris", nil)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
			var travelLabel *string
			if err := testutil.DB.QueryRow("SELECT travel_area_label FROM locations WHERE user_id = $1", userID).Scan(&travelLabel); err != nil {
				t.Fatal(err)
			}
			if travelLabel != nil {
				t.Fatalf("failed travel wrote %q", *travelLabel)
			}
		})
	}
}

func TestTravelConcurrentUpsert(t *testing.T) {
	service, geocoder, userID := newTestLocationService(t)
	if response := putArea(t, service, userID, "Battersea"); response.Code != http.StatusOK {
		t.Fatalf("home PUT status = %d", response.Code)
	}
	geocoder.forwardLabel = "Edinburgh"

	const writers = 8
	var wait sync.WaitGroup
	statuses := make(chan int, writers)
	for range writers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := putTravel(t, service, userID, "Edinburgh", 7)
			statuses <- response.Code
		}()
	}
	wait.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("concurrent travel status = %d", status)
		}
	}

	var rows int
	var homeLabel, travelLabel string
	if err := testutil.DB.QueryRow("SELECT count(*), min(home_area_label), min(travel_area_label) FROM locations WHERE user_id = $1", userID).Scan(&rows, &homeLabel, &travelLabel); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || homeLabel != "Battersea" || travelLabel != "Edinburgh" {
		t.Fatalf("concurrent upsert rows=%d home=%q travel=%q", rows, homeLabel, travelLabel)
	}
	var expiresAt time.Time
	if err := testutil.DB.QueryRow("SELECT travel_expires_at FROM locations WHERE user_id = $1", userID).Scan(&expiresAt); err != nil {
		t.Fatal(err)
	}
	if time.Until(expiresAt) < 6*24*time.Hour {
		t.Fatalf("travel expires too soon: %v", expiresAt)
	}
}

func TestLocationGetContract(t *testing.T) {
	testutil.ResetTables(t)
	service := &Service{DB: testutil.DB, Geocoder: newFakeGeocoder()}
	userID := testutil.InsertUser(t, "player@example.com")

	response := getLocation(t, service, userID)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing location status = %d", response.Code)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("GET location is cacheable")
	}
	if strings.Contains(response.Body.String(), "latitude") || strings.Contains(response.Body.String(), "longitude") {
		t.Fatalf("GET missing body leaked coordinates: %s", response.Body.String())
	}

	if response := putArea(t, service, userID, "Battersea"); response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d", response.Code)
	}
	response = getLocation(t, service, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d", response.Code)
	}
	if body := decodeLocation(t, response); body.AreaLabel != "Battersea" || body.TravelAreaLabel != nil || body.TravelExpiresAt != nil {
		t.Fatalf("GET body = %#v", body)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("GET headers = %v", response.Header())
	}
}
