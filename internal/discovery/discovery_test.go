package discovery

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

const (
	londonLatitude  = 51.5074
	londonLongitude = -0.1278
	mileMetres      = 1609.344
)

var emailCounter atomic.Int64

type slotFixture struct {
	weekday    int
	start, end string
}

type photoFixture struct {
	order  int
	status string
}

type viewerFixture struct {
	name         string
	gender       string
	genderPref   string
	skill        string
	minAge       int
	maxAge       int
	tolerance    int
	birthDate    string
	lastActive   string
	withProfile  bool
	withLocation bool
	photoStatus  string
	slots        []slotFixture
}

func defaultViewer(name string) viewerFixture {
	return viewerFixture{
		name:         name,
		gender:       "woman",
		genderPref:   "all",
		skill:        "intermediate",
		minAge:       18,
		maxAge:       120,
		tolerance:    1,
		birthDate:    "date '1990-01-01'",
		lastActive:   "now()",
		withProfile:  true,
		withLocation: true,
		photoStatus:  "approved",
		slots:        []slotFixture{{weekday: 2, start: "18:00", end: "21:00"}},
	}
}

type candidateFixture struct {
	name         string
	gender       string
	genderPref   string
	skill        string
	minAge       int
	maxAge       int
	tolerance    int
	birthDate    string
	lastActive   string
	deleted      bool
	withProfile  bool
	withLocation bool
	homeMetres   float64
	travelMetres *float64
	travelExpiry string
	photos       []photoFixture
	slots        []slotFixture
	bio          string
}

func defaultCandidate(name string) candidateFixture {
	return candidateFixture{
		name:         name,
		gender:       "woman",
		genderPref:   "all",
		skill:        "intermediate",
		minAge:       18,
		maxAge:       120,
		tolerance:    1,
		birthDate:    "date '1990-01-01'",
		lastActive:   "now()",
		withProfile:  true,
		withLocation: true,
		homeMetres:   0,
		photos:       []photoFixture{{order: 1, status: "approved"}},
		slots:        []slotFixture{{weekday: 2, start: "18:00", end: "21:00"}},
		bio:          "Hello",
	}
}

func mustExec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := testutil.DB.Exec(query, args...); err != nil {
		t.Fatalf("exec fixture: %v", err)
	}
}

func insertUserRow(t *testing.T, name, birthDate, lastActive string, deleted bool) string {
	t.Helper()
	email := fmt.Sprintf("%s-%d@example.com", name, emailCounter.Add(1))
	deletedAt := "NULL"
	if deleted {
		deletedAt = "now()"
	}
	var userID string
	query := fmt.Sprintf(`
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, last_active_at, deleted_at)
		VALUES ('\x01', decode(md5($1) || md5($2), 'hex'), %s, %s, %s)
		RETURNING id`, birthDate, lastActive, deletedAt)
	if err := testutil.DB.QueryRow(query, email, "salt:"+email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

func insertProfileRow(t *testing.T, userID, name, gender, skill, genderPref string, minAge, maxAge, tolerance int, slots []slotFixture) {
	t.Helper()
	mustExec(t, `
		INSERT INTO profiles (user_id, display_name, gender, skill_level, bio,
			gender_preference, min_age, max_age, skill_tolerance)
		VALUES ($1, $2, $3, $4, 'Hello', $5, $6, $7, $8)`,
		userID, name, gender, skill, genderPref, minAge, maxAge, tolerance)
	for _, slot := range slots {
		mustExec(t, `
			INSERT INTO availability (profile_id, weekday, start_time, end_time)
			VALUES ($1, $2, $3, $4)`, userID, slot.weekday, slot.start, slot.end)
	}
}

func insertHomeAt(t *testing.T, userID string, metres float64) {
	t.Helper()
	mustExec(t, `
		INSERT INTO locations (user_id, home, home_area_label)
		VALUES ($1, ST_Project(ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4::float8, 0::float8), 'Area')
		ON CONFLICT (user_id) DO UPDATE SET home = EXCLUDED.home`,
		userID, londonLongitude, londonLatitude, metres)
}

func insertTravelAt(t *testing.T, userID string, metres float64, expiry string) {
	t.Helper()
	query := fmt.Sprintf(`
		UPDATE locations
		SET travel = ST_Project(ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4::float8, 0::float8),
			travel_area_label = 'Travel',
			travel_expires_at = %s
		WHERE user_id = $1`, expiry)
	mustExec(t, query, userID, londonLongitude, londonLatitude, metres)
}

func insertPhotoRow(t *testing.T, userID string, order int, status string) {
	t.Helper()
	mustExec(t, `
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
		VALUES ($1, $2, $3, $4)`,
		userID, fmt.Sprintf("photos/%s/%d", userID, order), order, status)
}

func addViewer(t *testing.T, fixture viewerFixture) string {
	t.Helper()
	userID := insertUserRow(t, fixture.name, fixture.birthDate, fixture.lastActive, false)
	if fixture.withProfile {
		insertProfileRow(t, userID, fixture.name, fixture.gender, fixture.skill, fixture.genderPref,
			fixture.minAge, fixture.maxAge, fixture.tolerance, fixture.slots)
	}
	if fixture.withLocation {
		insertHomeAt(t, userID, 0)
	}
	if fixture.photoStatus != "" {
		insertPhotoRow(t, userID, 1, fixture.photoStatus)
	}
	return userID
}

func addCandidate(t *testing.T, fixture candidateFixture) string {
	t.Helper()
	userID := insertUserRow(t, fixture.name, fixture.birthDate, fixture.lastActive, fixture.deleted)
	if fixture.withProfile {
		insertProfileRow(t, userID, fixture.name, fixture.gender, fixture.skill, fixture.genderPref,
			fixture.minAge, fixture.maxAge, fixture.tolerance, fixture.slots)
	}
	if fixture.withLocation {
		insertHomeAt(t, userID, fixture.homeMetres)
		if fixture.travelMetres != nil {
			expiry := fixture.travelExpiry
			if expiry == "" {
				expiry = "now() + interval '3 days'"
			}
			insertTravelAt(t, userID, *fixture.travelMetres, expiry)
		}
	}
	for _, photo := range fixture.photos {
		insertPhotoRow(t, userID, photo.order, photo.status)
	}
	return userID
}

type presignCall struct {
	key      string
	duration time.Duration
}

type fakePresigner struct {
	mu      sync.Mutex
	calls   []presignCall
	fail    map[string]bool
	counter int
}

func newFakePresigner() *fakePresigner {
	return &fakePresigner{fail: map[string]bool{}}
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, duration time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, presignCall{key: key, duration: duration})
	if f.fail[key] {
		return "", errors.New("presign failed")
	}
	f.counter++
	return fmt.Sprintf("https://tennisconnect-photos.s3.eu-west-2.amazonaws.com/photos/photo-%d?X-Amz-Expires=300&X-Amz-Signature=sig", f.counter), nil
}

func (f *fakePresigner) recordedCalls() []presignCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]presignCall(nil), f.calls...)
}

type card struct {
	Player struct {
		DisplayName   string   `json:"display_name"`
		Age           int      `json:"age"`
		Gender        string   `json:"gender"`
		SkillLevel    string   `json:"skill_level"`
		Bio           string   `json:"bio"`
		PlaySummary   string   `json:"play_summary"`
		PhotoURLs     []string `json:"photo_urls"`
		DistanceMiles int      `json:"distance_miles"`
	} `json:"player"`
}

type discoveryPage struct {
	Cards      []card  `json:"cards"`
	NextCursor *string `json:"next_cursor"`
}

func newDiscoveryService(t *testing.T) (*Service, *fakePresigner) {
	t.Helper()
	presigner := newFakePresigner()
	return &Service{DB: testutil.DB, Presigner: presigner}, presigner
}

func fetchPage(t *testing.T, service *Service, viewerID, query string) (*httptest.ResponseRecorder, discoveryPage) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/discovery"+query, nil)
	response := httptest.NewRecorder()
	service.Get(response, request, viewerID)
	var page discoveryPage
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode discovery page: %v (body %s)", err, response.Body.String())
		}
	}
	return response, page
}

func cardNames(page discoveryPage) []string {
	names := make([]string, 0, len(page.Cards))
	for _, entry := range page.Cards {
		names = append(names, entry.Player.DisplayName)
	}
	return names
}

func cardByName(t *testing.T, page discoveryPage, name string) card {
	t.Helper()
	for _, entry := range page.Cards {
		if entry.Player.DisplayName == name {
			return entry
		}
	}
	t.Fatalf("card %q not found in %v", name, cardNames(page))
	return card{}
}

func contains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func effectiveMiles(t *testing.T, viewerID, candidateID string) int {
	t.Helper()
	var miles int
	err := testutil.DB.QueryRow(`
		SELECT CASE WHEN d < 1609.344 THEN 0 ELSE ceil(d / 1609.344)::int END
		FROM (
			SELECT ST_Distance(
				CASE WHEN c.travel IS NOT NULL AND c.travel_expires_at > now() THEN c.travel ELSE c.home END,
				v.home) AS d
			FROM locations c, locations v
			WHERE c.user_id = $1 AND v.user_id = $2
		) distances`, candidateID, viewerID).Scan(&miles)
	if err != nil {
		t.Fatalf("compute expected miles: %v", err)
	}
	return miles
}

func TestDiscoveryViewerEligibility(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)

	noProfile := defaultViewer("no-profile")
	noProfile.withProfile = false
	noProfileID := addViewer(t, noProfile)
	if response, _ := fetchPage(t, service, noProfileID, ""); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "complete your profile") {
		t.Fatalf("no profile = %d %s", response.Code, response.Body.String())
	}

	noArea := defaultViewer("no-area")
	noArea.withLocation = false
	noAreaID := addViewer(t, noArea)
	if response, _ := fetchPage(t, service, noAreaID, ""); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "set your area") {
		t.Fatalf("no area = %d %s", response.Code, response.Body.String())
	}

	noPhoto := defaultViewer("no-photo")
	noPhoto.photoStatus = ""
	noPhotoID := addViewer(t, noPhoto)
	if response, _ := fetchPage(t, service, noPhotoID, ""); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "one photo must be approved") {
		t.Fatalf("no photo = %d %s", response.Code, response.Body.String())
	}

	pendingPhoto := defaultViewer("pending-photo")
	pendingPhoto.photoStatus = "pending"
	pendingPhotoID := addViewer(t, pendingPhoto)
	if response, _ := fetchPage(t, service, pendingPhotoID, ""); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "one photo must be approved") {
		t.Fatalf("pending photo = %d %s", response.Code, response.Body.String())
	}

	idle := defaultViewer("idle-viewer")
	idle.lastActive = "now() - interval '30 days'"
	idleID := addViewer(t, idle)
	if response, _ := fetchPage(t, service, idleID, ""); response.Code != http.StatusOK {
		t.Fatalf("idle viewer = %d %s", response.Code, response.Body.String())
	}
}

func TestDiscoveryCandidateEligibility(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	addCandidate(t, defaultCandidate("eligible"))
	pendingOnly := defaultCandidate("pending-only")
	pendingOnly.photos = []photoFixture{{order: 1, status: "pending"}}
	addCandidate(t, pendingOnly)
	noLocation := defaultCandidate("no-location")
	noLocation.withLocation = false
	addCandidate(t, noLocation)
	noAvailability := defaultCandidate("no-availability")
	noAvailability.slots = nil
	addCandidate(t, noAvailability)
	deleted := defaultCandidate("deleted")
	deleted.deleted = true
	addCandidate(t, deleted)
	idle := defaultCandidate("idle")
	idle.lastActive = "now() - interval '7 days'"
	addCandidate(t, idle)
	underage := defaultCandidate("underage")
	underage.birthDate = "(current_date - interval '17 years')::date"
	addCandidate(t, underage)
	birthday := defaultCandidate("birthday")
	birthday.birthDate = "(current_date - interval '18 years')::date"
	addCandidate(t, birthday)
	approvedPlusPending := defaultCandidate("approved-plus-pending")
	approvedPlusPending.photos = []photoFixture{{order: 1, status: "approved"}, {order: 2, status: "pending"}}
	addCandidate(t, approvedPlusPending)
	noProfile := defaultCandidate("no-profile-candidate")
	noProfile.withProfile = false
	addCandidate(t, noProfile)

	travelling := defaultCandidate("travelling")
	travelling.homeMetres = 0.5 * mileMetres
	travelMetres := 2.4 * mileMetres
	travelling.travelMetres = &travelMetres
	travellingID := addCandidate(t, travelling)

	expiredTravel := defaultCandidate("expired-travel")
	expiredTravel.homeMetres = 3 * mileMetres
	expiredMetres := 0.5 * mileMetres
	expiredTravel.travelMetres = &expiredMetres
	expiredTravel.travelExpiry = "now() - interval '1 second'"
	expiredID := addCandidate(t, expiredTravel)

	exactExpiry := defaultCandidate("exact-expiry")
	exactExpiry.homeMetres = 3 * mileMetres
	exactMetres := 0.5 * mileMetres
	exactExpiry.travelMetres = &exactMetres
	exactExpiry.travelExpiry = "now()"
	exactID := addCandidate(t, exactExpiry)

	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	names := cardNames(page)
	for _, expected := range []string{"eligible", "birthday", "approved-plus-pending", "travelling", "expired-travel", "exact-expiry"} {
		if !contains(names, expected) {
			t.Fatalf("expected %q in %v", expected, names)
		}
	}
	for _, excluded := range []string{"pending-only", "no-location", "no-availability", "deleted", "idle", "underage", "no-profile-candidate"} {
		if contains(names, excluded) {
			t.Fatalf("excluded candidate %q was returned in %v", excluded, names)
		}
	}
	if contains(names, "viewer") {
		t.Fatal("viewer returned own card")
	}
	if got, want := cardByName(t, page, "travelling").Player.DistanceMiles, effectiveMiles(t, viewerID, travellingID); got != want {
		t.Fatalf("travel distance = %d, want %d", got, want)
	}
	if got, want := cardByName(t, page, "expired-travel").Player.DistanceMiles, effectiveMiles(t, viewerID, expiredID); got != want {
		t.Fatalf("expired travel distance = %d, want home %d", got, want)
	}
	if got, want := cardByName(t, page, "exact-expiry").Player.DistanceMiles, effectiveMiles(t, viewerID, exactID); got != want {
		t.Fatalf("exact expiry distance = %d, want home %d", got, want)
	}
	seen := map[string]bool{}
	for _, entry := range page.Cards {
		if seen[entry.Player.DisplayName] {
			t.Fatalf("duplicate card %q", entry.Player.DisplayName)
		}
		seen[entry.Player.DisplayName] = true
	}
	if got, want := cardByName(t, page, "birthday").Player.Age, 18; got != want {
		t.Fatalf("birthday age = %d, want %d", got, want)
	}
}

func TestDiscoveryFilters(t *testing.T) {
	genderCases := []struct {
		name         string
		viewerGender string
		viewerPref   string
		candGender   string
		candPref     string
		included     bool
	}{
		{name: "woman wants women woman", viewerGender: "woman", viewerPref: "women", candGender: "woman", included: true},
		{name: "woman wants women man", viewerGender: "woman", viewerPref: "women", candGender: "man", included: false},
		{name: "woman wants men woman", viewerGender: "woman", viewerPref: "men", candGender: "woman", included: false},
		{name: "woman wants men man", viewerGender: "woman", viewerPref: "men", candGender: "man", included: true},
		{name: "woman wants all woman", viewerGender: "woman", viewerPref: "all", candGender: "woman", included: true},
		{name: "woman wants all man", viewerGender: "woman", viewerPref: "all", candGender: "man", included: true},
		{name: "man wants women woman", viewerGender: "man", viewerPref: "women", candGender: "woman", included: true},
		{name: "man wants women man", viewerGender: "man", viewerPref: "women", candGender: "man", included: false},
		{name: "man wants men woman", viewerGender: "man", viewerPref: "men", candGender: "woman", included: false},
		{name: "man wants men man", viewerGender: "man", viewerPref: "men", candGender: "man", included: true},
		{name: "man wants all woman", viewerGender: "man", viewerPref: "all", candGender: "woman", included: true},
		{name: "man wants all man", viewerGender: "man", viewerPref: "all", candGender: "man", included: true},
		{name: "candidate wants women viewer woman", viewerGender: "woman", viewerPref: "all", candGender: "woman", candPref: "women", included: true},
		{name: "candidate wants women viewer man", viewerGender: "man", viewerPref: "all", candGender: "woman", candPref: "women", included: false},
		{name: "candidate wants men viewer woman", viewerGender: "woman", viewerPref: "all", candGender: "man", candPref: "men", included: false},
		{name: "candidate wants men viewer man", viewerGender: "man", viewerPref: "all", candGender: "man", candPref: "men", included: true},
	}
	for _, test := range genderCases {
		t.Run(test.name, func(t *testing.T) {
			testutil.ResetTables(t)
			service, _ := newDiscoveryService(t)
			viewer := defaultViewer("viewer")
			viewer.gender = test.viewerGender
			viewer.genderPref = test.viewerPref
			viewerID := addViewer(t, viewer)
			candidate := defaultCandidate("candidate")
			candidate.gender = test.candGender
			candidate.genderPref = test.candPref
			if candidate.genderPref == "" {
				candidate.genderPref = "all"
			}
			addCandidate(t, candidate)
			response, page := fetchPage(t, service, viewerID, "")
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if got := contains(cardNames(page), "candidate"); got != test.included {
				t.Fatalf("included = %t, want %t (cards %v)", got, test.included, cardNames(page))
			}
		})
	}

	t.Run("age ranges both ways", func(t *testing.T) {
		testutil.ResetTables(t)
		service, _ := newDiscoveryService(t)
		viewer := defaultViewer("viewer")
		viewer.birthDate = "(current_date - interval '30 years')::date"
		viewer.minAge = 25
		viewer.maxAge = 35
		viewerID := addViewer(t, viewer)

		inRange := defaultCandidate("in-range")
		inRange.birthDate = "(current_date - interval '30 years')::date"
		addCandidate(t, inRange)
		tooOld := defaultCandidate("too-old")
		tooOld.birthDate = "(current_date - interval '40 years')::date"
		addCandidate(t, tooOld)

		response, page := fetchPage(t, service, viewerID, "")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		if names := cardNames(page); !contains(names, "in-range") || contains(names, "too-old") {
			t.Fatalf("age filter cards = %v", names)
		}

		// Candidate's own range excludes the viewer.
		narrow := defaultCandidate("narrow-range")
		narrow.minAge = 40
		narrow.maxAge = 50
		addCandidate(t, narrow)
		_, page = fetchPage(t, service, viewerID, "")
		if contains(cardNames(page), "narrow-range") {
			t.Fatal("candidate range that excludes the viewer was ignored")
		}
	})

	t.Run("leap day and 120 boundary", func(t *testing.T) {
		testutil.ResetTables(t)
		service, _ := newDiscoveryService(t)
		viewer := defaultViewer("viewer")
		viewer.birthDate = "(current_date - interval '30 years')::date"
		viewerID := addViewer(t, viewer)

		leap := defaultCandidate("leap-day")
		leap.birthDate = "date '2000-02-29'"
		addCandidate(t, leap)
		oldest := defaultCandidate("oldest")
		oldest.birthDate = "(current_date - interval '120 years')::date"
		addCandidate(t, oldest)
		tooOld := defaultCandidate("too-old-121")
		tooOld.birthDate = "(current_date - interval '121 years')::date"
		addCandidate(t, tooOld)

		response, page := fetchPage(t, service, viewerID, "")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		names := cardNames(page)
		if !contains(names, "leap-day") || !contains(names, "oldest") || contains(names, "too-old-121") {
			t.Fatalf("age boundary cards = %v", names)
		}

		var age int
		if err := testutil.DB.QueryRow("SELECT EXTRACT(YEAR FROM age(current_date, date '2000-02-29'))::int").Scan(&age); err != nil {
			t.Fatal(err)
		}
		mustExec(t, "UPDATE profiles SET min_age = $1 WHERE user_id = $2", age+1, viewerID)
		_, page = fetchPage(t, service, viewerID, "")
		if contains(cardNames(page), "leap-day") {
			t.Fatal("leap-day candidate ignored the viewer minimum age")
		}
	})

	skillCases := []struct {
		name       string
		viewerTol  int
		candTol    int
		wantByRank map[int]bool // rank distance -> included
	}{
		{name: "both exact", viewerTol: 0, candTol: 0, wantByRank: map[int]bool{0: true, 1: false, 2: false, 3: false}},
		{name: "viewer exact candidate adjacent", viewerTol: 0, candTol: 1, wantByRank: map[int]bool{0: true, 1: true, 2: false, 3: false}},
		{name: "viewer adjacent candidate exact", viewerTol: 1, candTol: 0, wantByRank: map[int]bool{0: true, 1: true, 2: false, 3: false}},
		{name: "both adjacent", viewerTol: 1, candTol: 1, wantByRank: map[int]bool{0: true, 1: true, 2: false, 3: false}},
	}
	for _, test := range skillCases {
		t.Run("skill "+test.name, func(t *testing.T) {
			testutil.ResetTables(t)
			service, _ := newDiscoveryService(t)
			viewer := defaultViewer("viewer")
			viewer.skill = "intermediate"
			viewer.tolerance = test.viewerTol
			viewerID := addViewer(t, viewer)
			skills := []string{"beginner", "improver", "intermediate", "advanced"}
			names := make([]string, len(skills))
			for rank, skill := range skills {
				candidate := defaultCandidate(skill)
				candidate.skill = skill
				candidate.tolerance = test.candTol
				addCandidate(t, candidate)
				names[rank] = skill
			}
			response, page := fetchPage(t, service, viewerID, "")
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			for rank := range skills {
				distance := rank - 2
				if distance < 0 {
					distance = -distance
				}
				if got := contains(cardNames(page), names[rank]); got != test.wantByRank[distance] {
					t.Fatalf("skill %s rank distance %d included = %t, want %t", skills[rank], distance, got, test.wantByRank[distance])
				}
			}
		})
	}

	availabilityCases := []struct {
		name     string
		slots    []slotFixture
		included bool
	}{
		{name: "overlapping", slots: []slotFixture{{weekday: 2, start: "19:00", end: "20:00"}}, included: true},
		{name: "touching endpoint", slots: []slotFixture{{weekday: 2, start: "21:00", end: "22:00"}}, included: false},
		{name: "different weekday", slots: []slotFixture{{weekday: 3, start: "18:00", end: "21:00"}}, included: false},
		{name: "one of several weekdays", slots: []slotFixture{{weekday: 1, start: "08:00", end: "09:00"}, {weekday: 2, start: "20:00", end: "21:00"}}, included: true},
	}
	for _, test := range availabilityCases {
		t.Run("availability "+test.name, func(t *testing.T) {
			testutil.ResetTables(t)
			service, _ := newDiscoveryService(t)
			viewerID := addViewer(t, defaultViewer("viewer"))
			candidate := defaultCandidate("candidate")
			candidate.slots = test.slots
			addCandidate(t, candidate)
			response, page := fetchPage(t, service, viewerID, "")
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			if got := contains(cardNames(page), "candidate"); got != test.included {
				t.Fatalf("included = %t, want %t", got, test.included)
			}
		})
	}
}

func TestPlaySummary(t *testing.T) {
	tests := []struct {
		name  string
		slots []slotFixture
		want  string
	}{
		{name: "none", slots: nil, want: ""},
		{name: "single", slots: []slotFixture{{weekday: 2, start: "18:00", end: "21:00"}}, want: "Tue evenings"},
		{name: "dedupe same weekday and part", slots: []slotFixture{{weekday: 2, start: "18:00", end: "19:00"}, {weekday: 2, start: "19:00", end: "20:00"}}, want: "Tue evenings"},
		{name: "shared part joins weekdays", slots: []slotFixture{{weekday: 2, start: "18:00", end: "21:00"}, {weekday: 4, start: "17:00", end: "20:00"}}, want: "Tue, Thu evenings"},
		{name: "different parts", slots: []slotFixture{{weekday: 2, start: "18:00", end: "21:00"}, {weekday: 6, start: "09:00", end: "11:00"}}, want: "Tue evenings, Sat mornings"},
		{name: "parts of one day", slots: []slotFixture{{weekday: 2, start: "09:00", end: "11:00"}, {weekday: 2, start: "18:00", end: "21:00"}}, want: "Tue mornings, Tue evenings"},
		{name: "caps at two phrases", slots: []slotFixture{{weekday: 1, start: "09:00", end: "11:00"}, {weekday: 2, start: "12:00", end: "13:00"}, {weekday: 2, start: "18:00", end: "21:00"}}, want: "Mon mornings, Tue afternoons"},
		{name: "three weekdays share one part", slots: []slotFixture{{weekday: 1, start: "18:00", end: "19:00"}, {weekday: 2, start: "18:00", end: "19:00"}, {weekday: 3, start: "18:00", end: "19:00"}}, want: "Mon, Tue, Wed evenings"},
		{name: "three distinct parts cap phrases not weekdays", slots: []slotFixture{{weekday: 1, start: "09:00", end: "11:00"}, {weekday: 2, start: "12:00", end: "13:00"}, {weekday: 3, start: "18:00", end: "21:00"}}, want: "Mon mornings, Tue afternoons"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			slots := make([]slot, 0, len(test.slots))
			for _, fixture := range test.slots {
				slots = append(slots, slot(fixture))
			}
			if got := playSummary(slots); got != test.want {
				t.Fatalf("playSummary() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiscoveryRadiusAndDistance(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	distances := []struct {
		name   string
		metres float64
		miles  int
	}{
		{name: "under-one", metres: 0.99 * mileMetres, miles: 0},
		{name: "one", metres: 1609.344, miles: 1},
		{name: "just-over-one", metres: 1.01 * mileMetres, miles: 2},
		{name: "ten", metres: 10*mileMetres - 0.001, miles: 10},
		{name: "nine", metres: 9 * mileMetres, miles: 9},
	}
	for _, fixture := range distances {
		candidate := defaultCandidate(fixture.name)
		candidate.homeMetres = fixture.metres
		addCandidate(t, candidate)
	}
	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, fixture := range distances {
		if got := cardByName(t, page, fixture.name).Player.DistanceMiles; got != fixture.miles {
			t.Fatalf("%s distance = %d, want %d", fixture.name, got, fixture.miles)
		}
	}
	if strings.Contains(response.Body.String(), `"distance_miles":0.`) ||
		!regexp.MustCompile(`"distance_miles":\d+`).MatchString(response.Body.String()) {
		t.Fatalf("distance_miles is not an integer: %s", response.Body.String())
	}

	edge := defaultCandidate("edge")
	edge.homeMetres = 5 * mileMetres
	addCandidate(t, edge)
	outside := defaultCandidate("outside")
	outside.homeMetres = 5*mileMetres + 1
	addCandidate(t, outside)
	near := defaultCandidate("near")
	near.homeMetres = 4 * mileMetres
	addCandidate(t, near)
	far := defaultCandidate("far")
	far.homeMetres = 6 * mileMetres
	addCandidate(t, far)

	response, page = fetchPage(t, service, viewerID, "?radius=5&limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("radius 5 status = %d", response.Code)
	}
	names := cardNames(page)
	if !contains(names, "edge") || !contains(names, "near") {
		t.Fatalf("ST_DWithin boundary excluded an in-radius candidate: %v", names)
	}
	if contains(names, "outside") || contains(names, "far") {
		t.Fatalf("out-of-radius candidate returned: %v", names)
	}

	response, page = fetchPage(t, service, viewerID, "?limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("default radius status = %d", response.Code)
	}
	if !contains(cardNames(page), "nine") {
		t.Fatal("default radius of 10 miles excluded a 9-mile candidate")
	}

	for _, query := range []string{
		"?radius=4", "?radius=31", "?radius=0", "?radius=0.5", "?radius=abc",
		"?limit=19", "?limit=51", "?limit=none",
	} {
		if response, _ := fetchPage(t, service, viewerID, query); response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", query, response.Code)
		}
	}
	if response, _ := fetchPage(t, service, viewerID, "?radius=30&limit=50"); response.Code != http.StatusOK {
		t.Fatalf("radius 30 status = %d", response.Code)
	}
	if response, _ := fetchPage(t, service, viewerID, "?radius=5&limit=20"); response.Code != http.StatusOK {
		t.Fatalf("radius 5 status = %d", response.Code)
	}
}

func testCursor(miles int, userID string, radius int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s:%d", miles, userID, radius)))
}

func decodeCursor(t *testing.T, cursor string) (int, string, int) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 {
		t.Fatalf("cursor parts = %v", parts)
	}
	var miles, radius int
	if _, err := fmt.Sscanf(parts[0], "%d", &miles); err != nil {
		t.Fatalf("cursor miles: %v", err)
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &radius); err != nil {
		t.Fatalf("cursor radius: %v", err)
	}
	return miles, parts[1], radius
}

func TestDiscoveryPagination(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	idByName := make(map[string]string, 25)
	allIDs := make([]string, 0, 25)
	for index := range 25 {
		name := fmt.Sprintf("same-bucket-%02d", index)
		candidate := defaultCandidate(name)
		candidate.homeMetres = 0.4 * mileMetres
		id := addCandidate(t, candidate)
		idByName[name] = id
		allIDs = append(allIDs, id)
	}
	sort.Strings(allIDs)

	response, first := fetchPage(t, service, viewerID, "?limit=20")
	if response.Code != http.StatusOK {
		t.Fatalf("first page status = %d", response.Code)
	}
	if len(first.Cards) != 20 || first.NextCursor == nil {
		t.Fatalf("first page cards=%d next=%v", len(first.Cards), first.NextCursor)
	}
	response, second := fetchPage(t, service, viewerID, "?limit=20&cursor="+*first.NextCursor)
	if response.Code != http.StatusOK {
		t.Fatalf("second page status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(second.Cards) != 5 || second.NextCursor != nil {
		t.Fatalf("second page cards=%d next=%v", len(second.Cards), second.NextCursor)
	}
	seen := map[string]bool{}
	ordered := make([]string, 0, 25)
	orderedIDs := make([]string, 0, 25)
	for _, page := range []discoveryPage{first, second} {
		for _, entry := range page.Cards {
			if seen[entry.Player.DisplayName] {
				t.Fatalf("duplicate card %q across pages", entry.Player.DisplayName)
			}
			seen[entry.Player.DisplayName] = true
			ordered = append(ordered, entry.Player.DisplayName)
			orderedIDs = append(orderedIDs, idByName[entry.Player.DisplayName])
		}
	}
	if len(ordered) != 25 {
		t.Fatalf("paged cards = %d, want 25", len(ordered))
	}
	for index := 1; index < len(orderedIDs); index++ {
		if orderedIDs[index-1] > orderedIDs[index] {
			t.Fatalf("cards not ordered by UUID within a mile bucket: %v", orderedIDs)
		}
	}

	// Keyset across a whole-mile boundary.
	testutil.ResetTables(t)
	service, _ = newDiscoveryService(t)
	viewerID = addViewer(t, defaultViewer("viewer"))
	for index := range 22 {
		candidate := defaultCandidate(fmt.Sprintf("bucket-three-%02d", index))
		candidate.homeMetres = 2.2 * mileMetres
		addCandidate(t, candidate)
	}
	for index := range 3 {
		candidate := defaultCandidate(fmt.Sprintf("bucket-four-%02d", index))
		candidate.homeMetres = 3.2 * mileMetres
		addCandidate(t, candidate)
	}
	response, page := fetchPage(t, service, viewerID, "?limit=20")
	if response.Code != http.StatusOK {
		t.Fatalf("boundary first page status = %d", response.Code)
	}
	if len(page.Cards) != 20 || page.NextCursor == nil {
		t.Fatalf("boundary first page cards=%d", len(page.Cards))
	}
	response, next := fetchPage(t, service, viewerID, "?limit=20&cursor="+*page.NextCursor)
	if response.Code != http.StatusOK {
		t.Fatalf("boundary second page status = %d", response.Code)
	}
	allNames := append(cardNames(page), cardNames(next)...)
	if len(allNames) != 25 || len(next.Cards) != 5 {
		t.Fatalf("boundary pages = %d cards (second %d)", len(allNames), len(next.Cards))
	}
	seen = map[string]bool{}
	for _, name := range allNames {
		if seen[name] {
			t.Fatalf("duplicate %q across a mile boundary", name)
		}
		seen[name] = true
	}

	// A forged cursor with a valid shape only moves the window.
	miles, _, radius := decodeCursor(t, *page.NextCursor)
	forged := testCursor(miles, "00000000-0000-4000-8000-000000000000", radius)
	if response, _ := fetchPage(t, service, viewerID, "?limit=20&cursor="+forged); response.Code != http.StatusOK {
		t.Fatalf("forged cursor status = %d", response.Code)
	}

	malformed := []string{
		"!!!", "not-base64", base64.RawURLEncoding.EncodeToString([]byte("1:only-two")),
		base64.RawURLEncoding.EncodeToString([]byte("x:00000000-0000-4000-8000-000000000000:10")),
		base64.RawURLEncoding.EncodeToString([]byte("-1:00000000-0000-4000-8000-000000000000:10")),
		base64.RawURLEncoding.EncodeToString([]byte("11:00000000-0000-4000-8000-000000000000:10")),
		base64.RawURLEncoding.EncodeToString([]byte("1:not-a-uuid:10")),
		base64.RawURLEncoding.EncodeToString([]byte("1:00000000-0000-4000-8000-000000000000:5")),
	}
	for _, cursor := range malformed {
		if response, _ := fetchPage(t, service, viewerID, "?limit=20&radius=10&cursor="+cursor); response.Code != http.StatusBadRequest {
			t.Fatalf("malformed cursor %q status = %d, want 400", cursor, response.Code)
		}
	}
}

func TestDiscoverySnapshotAndConcurrentEdits(t *testing.T) {
	testutil.ResetTables(t)
	service, presigner := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))
	firstID := addCandidate(t, defaultCandidate("first"))
	addCandidate(t, defaultCandidate("second"))

	var hookErr error
	service.afterCandidates = func() {
		if _, err := testutil.DB.Exec("UPDATE photos SET moderation_status = 'rejected' WHERE owner_id = $1", firstID); err != nil {
			hookErr = err
		}
		if _, err := testutil.DB.Exec("UPDATE profiles SET display_name = 'renamed' WHERE user_id = $1", firstID); err != nil {
			hookErr = err
		}
	}
	response, page := fetchPage(t, service, viewerID, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if hookErr != nil {
		t.Fatalf("hook: %v", hookErr)
	}
	if len(presigner.recordedCalls()) == 0 {
		t.Fatal("repeatable-read page did not presign a photo that changed after the candidate query")
	}
	card := cardByName(t, page, "first")
	if len(card.Player.PhotoURLs) == 0 {
		t.Fatal("page lost the photo that changed mid-page")
	}
	if contains(cardNames(page), "renamed") {
		t.Fatal("page mixed profile versions")
	}
	seen := map[string]bool{}
	for _, entry := range page.Cards {
		if seen[entry.Player.DisplayName] {
			t.Fatalf("duplicate card %q", entry.Player.DisplayName)
		}
		seen[entry.Player.DisplayName] = true
	}
}

func TestDiscoveryPhotoURLs(t *testing.T) {
	testutil.ResetTables(t)
	service, presigner := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))
	candidate := defaultCandidate("photographer")
	candidate.photos = []photoFixture{
		{order: 3, status: "approved"},
		{order: 1, status: "approved"},
		{order: 2, status: "approved"},
	}
	candidateID := addCandidate(t, candidate)

	response, page := fetchPage(t, service, viewerID, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	card := cardByName(t, page, "photographer")
	if len(card.Player.PhotoURLs) != 3 {
		t.Fatalf("photo URLs = %d, want 3", len(card.Player.PhotoURLs))
	}
	calls := presigner.recordedCalls()
	if len(calls) != 3 {
		t.Fatalf("presign calls = %d, want 3", len(calls))
	}
	for index, call := range calls {
		if call.duration != 5*time.Minute {
			t.Fatalf("presign duration = %v", call.duration)
		}
		if want := fmt.Sprintf("photos/%s/%d", candidateID, index+1); call.key != want {
			t.Fatalf("presign key[%d] = %q, want %q", index, call.key, want)
		}
	}
	for _, forbidden := range []string{candidateID, "private-bucket"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("response exposed %q: %s", forbidden, response.Body.String())
		}
	}

	// Pending and rejected rows never reach the presigner.
	testutil.ResetTables(t)
	service, presigner = newDiscoveryService(t)
	viewerID = addViewer(t, defaultViewer("viewer"))
	mixed := defaultCandidate("mixed")
	mixed.photos = []photoFixture{
		{order: 1, status: "approved"},
		{order: 2, status: "pending"},
		{order: 3, status: "rejected"},
	}
	mixedID := addCandidate(t, mixed)
	response, _ = fetchPage(t, service, viewerID, "")
	if response.Code != http.StatusOK {
		t.Fatalf("mixed status = %d", response.Code)
	}
	calls = presigner.recordedCalls()
	if len(calls) != 1 || calls[0].key != fmt.Sprintf("photos/%s/1", mixedID) {
		t.Fatalf("presigned non-approved photos: %v", calls)
	}

	// One presign failure omits only that URL and logs generically.
	testutil.ResetTables(t)
	service, presigner = newDiscoveryService(t)
	viewerID = addViewer(t, defaultViewer("viewer"))
	failing := defaultCandidate("failing")
	failing.photos = []photoFixture{{order: 1, status: "approved"}, {order: 2, status: "approved"}}
	failingID := addCandidate(t, failing)
	presigner.fail[fmt.Sprintf("photos/%s/1", failingID)] = true
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	response, page = fetchPage(t, service, viewerID, "")
	if response.Code != http.StatusOK {
		t.Fatalf("presign failure status = %d", response.Code)
	}
	if urls := cardByName(t, page, "failing").Player.PhotoURLs; len(urls) != 1 {
		t.Fatalf("presign failure URLs = %v", urls)
	}
	if logs.String() == "" {
		t.Fatal("presign failure not logged")
	}
	for _, forbidden := range []string{failingID, "private-bucket"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Fatalf("presign log leaked %q: %s", forbidden, logs.String())
		}
	}

	// No presigner means the endpoint fails closed.
	service.Presigner = nil
	if response, _ := fetchPage(t, service, viewerID, ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil presigner status = %d, want 503", response.Code)
	}
}

func TestDiscoveryExcludesSelfAndDocumentsTask6Predicates(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))
	addCandidate(t, defaultCandidate("other"))

	response, page := fetchPage(t, service, viewerID, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if contains(cardNames(page), "viewer") {
		t.Fatal("viewer returned own card")
	}
	if !contains(cardNames(page), "other") {
		t.Fatal("eligible candidate missing")
	}

	// Task 6 landed the exclusion predicates, so the executable SQL must now
	// reference all three tables and the pre-Task-6 exposure gate must be gone.
	source, err := os.ReadFile("discovery.go")
	if err != nil {
		t.Fatalf("read discovery source: %v", err)
	}
	executable := stripComments(string(source))
	for _, required := range []string{"blocks", "matches", "reactions"} {
		if !regexp.MustCompile(`(?i)\b` + required + `\b`).MatchString(executable) {
			t.Fatalf("executable SQL is missing the Task 6 exclusion for %q", required)
		}
	}

	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if strings.Contains(string(readme), "internal-only until Task 6") {
		t.Fatal("README still carries the obsolete discovery internal-only gate")
	}
}

func stripComments(source string) string {
	block := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
	line := regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(block, "")
	return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(line, "")
}

func TestDiscoveryPrivacyScan(t *testing.T) {
	testutil.ResetTables(t)
	service, presigner := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))
	for index := range 21 {
		candidate := defaultCandidate(fmt.Sprintf("privacy-%02d", index))
		candidate.homeMetres = float64(index) * 0.1 * mileMetres
		candidate.bio = "Hello"
		addCandidate(t, candidate)
	}
	first := defaultCandidate("failing-photo")
	first.photos = []photoFixture{{order: 1, status: "approved"}}
	firstID := addCandidate(t, first)
	presigner.fail[fmt.Sprintf("photos/%s/1", firstID)] = true

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=20")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("discovery headers = %v", response.Header())
	}
	if page.NextCursor == nil {
		t.Fatal("page of 21 candidates has no cursor")
	}
	body := response.Body.String()
	for _, forbidden := range []string{
		"latitude", "longitude", "Latitude", "Longitude", "metres", "meters",
		"private-bucket", "pk.", "51.5074", "-0.1278", "ranking",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("discovery response leaked %q: %s", forbidden, body)
		}
	}
	if regexp.MustCompile(`\d+\.\d+`).MatchString(body) {
		t.Fatalf("discovery response contains a decimal value: %s", body)
	}
	for _, forbidden := range []string{"private-bucket", "51.5074", "-0.1278", "pk."} {
		if strings.Contains(logs.String(), forbidden) {
			t.Fatalf("discovery log leaked %q: %s", forbidden, logs.String())
		}
	}
	miles, userID, radius := decodeCursor(t, *page.NextCursor)
	if miles < 0 || miles > radius || radius != 10 || len(userID) != 36 {
		t.Fatalf("cursor contents miles=%d user=%q radius=%d", miles, userID, radius)
	}
}

// insertReaction records a one-way reaction for the Task 6 exclusion tests.
func insertReaction(t *testing.T, actorID, targetID, action string) {
	t.Helper()
	mustExec(t, "INSERT INTO reactions (actor_id, target_id, action) VALUES ($1, $2, $3)",
		actorID, targetID, action)
}

func insertMatch(t *testing.T, first, second string) {
	t.Helper()
	low, high := first, second
	if low > high {
		low, high = high, low
	}
	mustExec(t, "INSERT INTO matches (user_low, user_high) VALUES ($1, $2)", low, high)
}

func insertBlock(t *testing.T, blockerID, blockedID string) {
	t.Helper()
	mustExec(t, "INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)", blockerID, blockedID)
}

func TestDiscoveryExcludesReactedTargets(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	liked := addCandidate(t, defaultCandidate("liked"))
	passed := addCandidate(t, defaultCandidate("passed"))
	addCandidate(t, defaultCandidate("untouched"))

	insertReaction(t, viewerID, liked, "like")
	insertReaction(t, viewerID, passed, "pass")

	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	names := cardNames(page)
	if contains(names, "liked") || contains(names, "passed") {
		t.Fatalf("already reacted targets still visible: %v", names)
	}
	if !contains(names, "untouched") {
		t.Fatalf("unreacted candidate missing: %v", names)
	}
}

func TestDiscoveryExcludesBlockedBothDirections(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	blockedByViewer := addCandidate(t, defaultCandidate("blockedByViewer"))
	blockedViewer := addCandidate(t, defaultCandidate("blockedViewer"))
	addCandidate(t, defaultCandidate("visible"))

	insertBlock(t, viewerID, blockedByViewer)
	insertBlock(t, blockedViewer, viewerID)

	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	names := cardNames(page)
	if contains(names, "blockedByViewer") || contains(names, "blockedViewer") {
		t.Fatalf("blocked players still visible: %v", names)
	}
	if !contains(names, "visible") {
		t.Fatalf("unblocked candidate missing: %v", names)
	}
}

func TestDiscoveryExcludesExistingMatches(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	matched := addCandidate(t, defaultCandidate("matched"))
	addCandidate(t, defaultCandidate("available"))
	insertMatch(t, viewerID, matched)

	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=50")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	names := cardNames(page)
	if contains(names, "matched") {
		t.Fatalf("existing match still in discovery: %v", names)
	}
	if !contains(names, "available") {
		t.Fatalf("available candidate missing: %v", names)
	}
}

func TestDiscoveryExclusionSurvivesPagination(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))

	first := addCandidate(t, defaultCandidate("first"))
	second := addCandidate(t, defaultCandidate("second"))
	third := addCandidate(t, defaultCandidate("third"))
	insertReaction(t, viewerID, first, "pass")
	insertBlock(t, viewerID, second)
	insertMatch(t, viewerID, third)

	response, page := fetchPage(t, service, viewerID, "?radius=10&limit=20")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if len(page.Cards) != 0 || page.NextCursor != nil {
		t.Fatalf("expected an empty terminal page, got %d cards", len(page.Cards))
	}
}

func TestDiscoveryExclusionIsPerViewer(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerA := addViewer(t, defaultViewer("viewerA"))
	viewerB := addViewer(t, defaultViewer("viewerB"))

	target := addCandidate(t, defaultCandidate("target"))
	insertBlock(t, viewerA, target)

	_, pageA := fetchPage(t, service, viewerA, "?radius=10&limit=50")
	if contains(cardNames(pageA), "target") {
		t.Fatalf("blocker still sees the blocked player: %v", cardNames(pageA))
	}
	_, pageB := fetchPage(t, service, viewerB, "?radius=10&limit=50")
	if !contains(cardNames(pageB), "target") {
		t.Fatalf("unrelated viewer lost the player: %v", cardNames(pageB))
	}
}

func TestDiscoveryExclusionHoldsUnderConcurrentBlock(t *testing.T) {
	testutil.ResetTables(t)
	service, _ := newDiscoveryService(t)
	viewerID := addViewer(t, defaultViewer("viewer"))
	candidateID := addCandidate(t, defaultCandidate("candidate"))

	// Baseline: visible.
	if _, page := fetchPage(t, service, viewerID, "?radius=10&limit=50"); !contains(cardNames(page), "candidate") {
		t.Fatalf("candidate not visible before blocking: %v", cardNames(page))
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 5 {
			mustExec(t, "INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
				viewerID, candidateID)
		}
	}()
	for range 5 {
		_, page := fetchPage(t, service, viewerID, "?radius=10&limit=50")
		names := cardNames(page)
		// Either the candidate is gone or the block has not committed yet; the
		// card must never reappear after a successful block.
		if contains(names, "candidate") {
			continue
		}
	}
	<-done
	if _, page := fetchPage(t, service, viewerID, "?radius=10&limit=50"); contains(cardNames(page), "candidate") {
		t.Fatal("blocked candidate visible after the block committed")
	}
}
