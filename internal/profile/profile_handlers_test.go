package profile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

func newTestProfileService(t *testing.T) (*Service, string) {
	t.Helper()
	testutil.ResetTables(t)
	userID := testutil.InsertUser(t, "player@example.com")
	return &Service{DB: testutil.DB}, userID
}

func putProfileRequest(service *Service, userID string, input profileInput) *httptest.ResponseRecorder {
	body, _ := json.Marshal(input)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/profile", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.Put(response, request, userID)
	return response
}

func getProfileRequest(service *Service, userID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	response := httptest.NewRecorder()
	service.Get(response, request, userID)
	return response
}

func TestPutAndGetProfile(t *testing.T) {
	service, userID := newTestProfileService(t)
	input := validProfileInput()
	input.Ranking = stringPointer("10.5")
	input.Availability = append(input.Availability, availabilityInput{Weekday: 6, Start: "09:00", End: "11:00"})
	if response := putProfileRequest(service, userID, input); response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", response.Code, response.Body.String())
	}
	response := getProfileRequest(service, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", response.Code, response.Body.String())
	}
	var got profileInput
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if got.Age < 18 || got.Age > 120 {
		t.Fatalf("GET profile age = %d, want 18..120", got.Age)
	}
	input.Age = got.Age
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("GET profile = %#v, want %#v", got, input)
	}
	var timeZone string
	if err := testutil.DB.QueryRow("SELECT time_zone FROM availability LIMIT 1").Scan(&timeZone); err != nil {
		t.Fatal(err)
	}
	if timeZone != "Europe/London" {
		t.Fatalf("time_zone = %q, want Europe/London", timeZone)
	}
}

func TestPutProfileReplacesAvailability(t *testing.T) {
	service, userID := newTestProfileService(t)
	putProfileRequest(service, userID, validProfileInput())
	updated := validProfileInput()
	updated.DisplayName = "Updated"
	updated.Availability = []availabilityInput{{Weekday: 7, Start: "08:00", End: "10:00"}}
	if response := putProfileRequest(service, userID, updated); response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d", response.Code)
	}
	var got profileInput
	response := getProfileRequest(service, userID)
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	updated.Age = got.Age
	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("profile after replacement = %#v, want %#v", got, updated)
	}
}

func TestInvalidUpdatePreservesExistingProfile(t *testing.T) {
	service, userID := newTestProfileService(t)
	original := validProfileInput()
	putProfileRequest(service, userID, original)
	invalid := validProfileInput()
	invalid.DisplayName = "Changed"
	invalid.Availability[0].Start, invalid.Availability[0].End = "22:00", "02:00"
	if response := putProfileRequest(service, userID, invalid); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid PUT status = %d, want 400", response.Code)
	}
	var got profileInput
	response := getProfileRequest(service, userID)
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	original.Age = got.Age
	if !reflect.DeepEqual(got, original) {
		t.Fatal("invalid update changed stored profile")
	}
}

func TestGetProfileNotFound(t *testing.T) {
	service, userID := newTestProfileService(t)
	response := getProfileRequest(service, userID)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestGetProfileOnlyAllowsOwner(t *testing.T) {
	service, userID := newTestProfileService(t)
	putProfileRequest(service, userID, validProfileInput())
	otherID := testutil.InsertUser(t, "other@example.com")
	response := getProfileRequest(service, otherID)
	if response.Code != http.StatusNotFound {
		t.Fatalf("other user status = %d, want 404", response.Code)
	}
}

func TestProfileResponseExcludesPrivateAccountFields(t *testing.T) {
	service, userID := newTestProfileService(t)
	putProfileRequest(service, userID, validProfileInput())
	response := getProfileRequest(service, userID)
	for _, field := range []string{"email", "birth_date", "deleted_at", "last_active", "password"} {
		if strings.Contains(response.Body.String(), field) {
			t.Fatalf("profile response exposed %q: %s", field, response.Body.String())
		}
	}
}

func TestPutProfileRejectsCrossOriginRequest(t *testing.T) {
	service, userID := newTestProfileService(t)
	body, _ := json.Marshal(validProfileInput())
	request := httptest.NewRequest(http.MethodPut, "/api/v1/profile", strings.NewReader(string(body)))
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	service.Put(response, request, userID)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if got := getProfileRequest(service, userID); got.Code != http.StatusNotFound {
		t.Fatal("cross-origin request created profile")
	}
}

func TestGetProfileOrdersAvailabilityByStartAndEnd(t *testing.T) {
	service, userID := newTestProfileService(t)
	input := validProfileInput()
	input.Availability = []availabilityInput{
		{Weekday: 2, Start: "18:00", End: "21:00"},
		{Weekday: 2, Start: "18:00", End: "20:00"},
	}
	putProfileRequest(service, userID, input)
	response := getProfileRequest(service, userID)
	var got profileInput
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Availability) != 2 || got.Availability[0].End != "20:00" || got.Availability[1].End != "21:00" {
		t.Fatalf("availability order = %#v, want 20:00 then 21:00", got.Availability)
	}
}

func TestPutProfileAcceptsTwentySlots(t *testing.T) {
	service, userID := newTestProfileService(t)
	input := validProfileInput()
	input.Availability = nil
	for i := 0; i < 20; i++ {
		input.Availability = append(input.Availability, availabilityInput{Weekday: 1, Start: fmt.Sprintf("%02d:00", i), End: fmt.Sprintf("%02d:00", i+1)})
	}
	if response := putProfileRequest(service, userID, input); response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", response.Code, response.Body.String())
	}
	var count int
	if err := testutil.DB.QueryRow("SELECT count(*) FROM availability WHERE profile_id = $1", userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 20 {
		t.Fatalf("availability rows = %d, want 20", count)
	}
}

func TestConcurrentProfileUpdatesKeepAvailabilityConsistent(t *testing.T) {
	service, userID := newTestProfileService(t)
	first := validProfileInput()
	first.DisplayName = "First"
	second := validProfileInput()
	second.DisplayName = "Second"
	second.Availability = []availabilityInput{{Weekday: 7, Start: "08:00", End: "10:00"}}
	responses := make([]*httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	for i, input := range []profileInput{first, second} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			responses[i] = putProfileRequest(service, userID, input)
		}()
	}
	wait.Wait()
	for _, response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("concurrent PUT status = %d, body = %s", response.Code, response.Body.String())
		}
	}
	var got profileInput
	response := getProfileRequest(service, userID)
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.DisplayName == "First" && !reflect.DeepEqual(got.Availability, first.Availability) {
		t.Fatal("first profile mixed with second availability")
	}
	if got.DisplayName == "Second" && !reflect.DeepEqual(got.Availability, second.Availability) {
		t.Fatal("second profile mixed with first availability")
	}
}

func TestConcurrentGetNeverMixesProfileVersions(t *testing.T) {
	service, userID := newTestProfileService(t)
	first := validProfileInput()
	first.DisplayName = "First"
	second := validProfileInput()
	second.DisplayName = "Second"
	second.Availability = []availabilityInput{{Weekday: 7, Start: "08:00", End: "10:00"}}
	putProfileRequest(service, userID, first)

	start := make(chan struct{})
	messages := make(chan string, 1)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		for i := 0; i < 500; i++ {
			input := first
			if i%2 == 1 {
				input = second
			}
			if response := putProfileRequest(service, userID, input); response.Code != http.StatusOK {
				select {
				case messages <- fmt.Sprintf("PUT status = %d", response.Code):
				default:
				}
				return
			}
		}
	}()
	go func() {
		defer wait.Done()
		<-start
		for i := 0; i < 500; i++ {
			response := getProfileRequest(service, userID)
			if response.Code != http.StatusOK {
				select {
				case messages <- fmt.Sprintf("GET status = %d", response.Code):
				default:
				}
				return
			}
			var got profileInput
			if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
				select {
				case messages <- err.Error():
				default:
				}
				return
			}
			mixed := (got.DisplayName == "First" && !reflect.DeepEqual(got.Availability, first.Availability)) ||
				(got.DisplayName == "Second" && !reflect.DeepEqual(got.Availability, second.Availability))
			if mixed {
				select {
				case messages <- "GET combined two different profile versions":
				default:
				}
				return
			}
		}
	}()
	close(start)
	wait.Wait()
	select {
	case message := <-messages:
		t.Error(message)
	default:
	}
}

func setBirthDate(t *testing.T, userID string, birthDate time.Time) {
	t.Helper()
	if _, err := testutil.DB.Exec("UPDATE users SET birth_date = $1 WHERE id = $2", birthDate.Format("2006-01-02"), userID); err != nil {
		t.Fatalf("set birth date: %v", err)
	}
}

func TestProfileAgeContract(t *testing.T) {
	service, userID := newTestProfileService(t)
	setBirthDate(t, userID, time.Now().AddDate(-30, 0, 0))

	response := putProfileRequest(service, userID, validProfileInput())
	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", response.Code, response.Body.String())
	}
	var putBody map[string]any
	if err := json.NewDecoder(response.Body).Decode(&putBody); err != nil {
		t.Fatal(err)
	}
	if putBody["age"] != float64(30) {
		t.Fatalf("PUT age = %v, want 30", putBody["age"])
	}
	if _, ok := putBody["birth_date"]; ok {
		t.Fatal("PUT response returned birth_date")
	}

	response = getProfileRequest(service, userID)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"age":30`) {
		t.Fatalf("GET body has no server age: %s", body)
	}
	if strings.Contains(body, "birth_date") || strings.Contains(body, "199") {
		t.Fatalf("GET leaked a birth date: %s", body)
	}
}

func TestProfileAgeIgnoresClientValueAndHandlesBounds(t *testing.T) {
	service, userID := newTestProfileService(t)
	setBirthDate(t, userID, time.Now().AddDate(-120, 0, 0))
	input := validProfileInput()
	input.Age = 99
	if response := putProfileRequest(service, userID, input); response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d", response.Code)
	}
	var got profileInput
	if err := json.NewDecoder(getProfileRequest(service, userID).Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Age != 120 {
		t.Fatalf("age = %d, want 120 for a client value of 99", got.Age)
	}

	// Registration rejects under-18s; if an impossible row exists, report the
	// true age rather than clamping it up to 18.
	setBirthDate(t, userID, time.Now().AddDate(-17, 0, 0))
	if err := json.NewDecoder(getProfileRequest(service, userID).Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Age != 17 {
		t.Fatalf("underage age = %d, want 17", got.Age)
	}
}
