// Package discovery returns paged, whole-mile discovery cards for an eligible
// viewer. Exact coordinates are used only inside PostgreSQL and never leave it.
package discovery

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"tennisconnect/internal/httpapi"
)

const (
	minRadius     = 5
	maxRadius     = 30
	defaultRadius = 10
	minLimit      = 20
	maxLimit      = 50
	defaultLimit  = 20
	metresPerMile = 1609.344
	photoLimit    = 3
	photoExpiry   = 5 * time.Minute
	requestBudget = 10 * time.Second
)

// Presigner issues a short-lived URL for one approved photo object. It is a
// narrow interface so discovery never depends on the storage implementation.
type Presigner interface {
	PresignGet(ctx context.Context, key string, duration time.Duration) (string, error)
}

// Service serves discovery pages.
type Service struct {
	DB        *sql.DB
	Presigner Presigner
	// afterCandidates is a test hook for repeatable-read snapshot checks.
	afterCandidates func()
}

type pageRequest struct {
	radius int
	limit  int
	cursor *pageCursor
}

type pageCursor struct {
	miles  int
	userID string
}

// The discovery page uses two mutually exclusive location branches so both
// GiST indexes are usable. A live travel destination wins; otherwise home.
// Task 6 will extend the WHERE clause with block, match and reaction exclusion
// predicates once those tables exist; Task 5 intentionally excludes only self.
const candidateQuery = `
WITH viewer AS (
	SELECT CASE
		WHEN travel IS NOT NULL AND travel_expires_at > now() THEN travel
		ELSE home
	END AS point
	FROM locations
	WHERE user_id = $1
),
viewer_profile AS MATERIALIZED (
	SELECT gender, skill_level, gender_preference, min_age, max_age, skill_tolerance
	FROM profiles WHERE user_id = $1
),
viewer_user AS MATERIALIZED (
	SELECT birth_date FROM users WHERE id = $1
),
viewer_slots AS MATERIALIZED (
	SELECT weekday, start_time, end_time FROM availability WHERE profile_id = $1
),
effective AS (
	SELECT l.user_id, l.travel AS point
	FROM locations l
	WHERE l.travel IS NOT NULL
		AND l.travel_expires_at > now()
		AND ST_DWithin(l.travel, (SELECT point FROM viewer), $2)
	UNION ALL
	SELECT l.user_id, l.home
	FROM locations l
	WHERE NOT (l.travel IS NOT NULL AND l.travel_expires_at > now())
		AND ST_DWithin(l.home, (SELECT point FROM viewer), $2)
),
scored AS (
	SELECT e.user_id,
		CASE
			WHEN ST_Distance(e.point, (SELECT point FROM viewer)) < 1609.344 THEN 0
			ELSE ceil(ST_Distance(e.point, (SELECT point FROM viewer)) / 1609.344)::int
		END AS display_miles
	FROM effective e
)
SELECT s.user_id, s.display_miles, p.display_name, p.gender, p.skill_level, p.bio, u.birth_date
FROM scored s
JOIN profiles p ON p.user_id = s.user_id
JOIN users u ON u.id = s.user_id
JOIN viewer_profile vp ON true
JOIN viewer_user vu ON true
WHERE s.user_id <> $1
	AND u.deleted_at IS NULL
	AND u.birth_date <= (current_date - interval '18 years')::date
	AND u.last_active_at > now() - interval '7 days'
	AND EXISTS (
		SELECT 1 FROM photos ph
		WHERE ph.owner_id = u.id AND ph.moderation_status = 'approved'
	)
	AND (vp.gender_preference = 'all'
		OR (vp.gender_preference = 'women' AND p.gender = 'woman')
		OR (vp.gender_preference = 'men' AND p.gender = 'man'))
	AND (p.gender_preference = 'all'
		OR (p.gender_preference = 'women' AND vp.gender = 'woman')
		OR (p.gender_preference = 'men' AND vp.gender = 'man'))
	AND u.birth_date <= (current_date - make_interval(years => vp.min_age))::date
	AND u.birth_date > (current_date - make_interval(years => vp.max_age + 1))::date
	AND vu.birth_date <= (current_date - make_interval(years => p.min_age))::date
	AND vu.birth_date > (current_date - make_interval(years => p.max_age + 1))::date
	AND abs(
		CASE p.skill_level
			WHEN 'beginner' THEN 0 WHEN 'improver' THEN 1 WHEN 'intermediate' THEN 2 ELSE 3
		END
		- CASE vp.skill_level
			WHEN 'beginner' THEN 0 WHEN 'improver' THEN 1 WHEN 'intermediate' THEN 2 ELSE 3
		END
	) <= GREATEST(p.skill_tolerance, vp.skill_tolerance)
	AND EXISTS (
		SELECT 1 FROM availability ca
		JOIN viewer_slots vs ON vs.weekday = ca.weekday
		WHERE ca.profile_id = u.id
			AND ca.start_time < vs.end_time
			AND vs.start_time < ca.end_time
	)
	AND ($3::int IS NULL OR s.display_miles > $3 OR (s.display_miles = $3 AND s.user_id > $4::uuid))
ORDER BY s.display_miles, s.user_id
LIMIT $5`

const availabilityQuery = `
SELECT profile_id, weekday, to_char(start_time, 'HH24:MI')
FROM availability
WHERE profile_id = ANY($1::uuid[])
ORDER BY profile_id, weekday, start_time, end_time`

const photoQuery = `
SELECT owner_id, object_key
FROM photos
WHERE owner_id = ANY($1::uuid[]) AND moderation_status = 'approved'
ORDER BY owner_id, display_order`

type slot struct {
	weekday int
	start   string
	end     string
}

type candidateRow struct {
	userID       string
	displayMiles int
	displayName  string
	gender       string
	skillLevel   string
	bio          string
	birthDate    time.Time
}

type photoRow struct {
	ownerID   string
	objectKey string
}

type playerCard struct {
	DisplayName   string   `json:"display_name"`
	Age           int      `json:"age"`
	Gender        string   `json:"gender"`
	SkillLevel    string   `json:"skill_level"`
	Bio           string   `json:"bio"`
	PlaySummary   string   `json:"play_summary"`
	PhotoURLs     []string `json:"photo_urls"`
	DistanceMiles int      `json:"distance_miles"`
}

type cardResponse struct {
	Player playerCard `json:"player"`
}

type pageResponse struct {
	Cards      []cardResponse `json:"cards"`
	NextCursor *string        `json:"next_cursor"`
}

// Get serves GET /api/v1/discovery.
func (s *Service) Get(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if s.Presigner == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	request, ok := parsePageRequest(r.URL.Query())
	if !ok {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()

	if !s.viewerEligible(ctx, tx, w, userID) {
		return
	}

	var cursorMiles any
	var cursorUser any
	if request.cursor != nil {
		cursorMiles = request.cursor.miles
		cursorUser = request.cursor.userID
	}
	rows, err := tx.QueryContext(ctx, candidateQuery,
		userID, float64(request.radius)*metresPerMile, cursorMiles, cursorUser, request.limit+1)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	candidates, err := scanCandidates(rows)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if s.afterCandidates != nil {
		s.afterCandidates()
	}
	hasMore := len(candidates) > request.limit
	if hasMore {
		candidates = candidates[:request.limit]
	}
	ids := make([]string, len(candidates))
	for index, candidate := range candidates {
		ids[index] = candidate.userID
	}
	availability, err := loadAvailability(ctx, tx, ids)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	photos, err := loadPhotos(ctx, tx, ids)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}

	urls := make(map[string][]string, len(candidates))
	for _, photo := range photos {
		if len(urls[photo.ownerID]) >= photoLimit {
			continue
		}
		signed, err := s.Presigner.PresignGet(ctx, photo.objectKey, photoExpiry)
		if err != nil {
			// A single unusable URL must not fail the page; never log the key.
			log.Printf("discovery photo URL unavailable")
			continue
		}
		urls[photo.ownerID] = append(urls[photo.ownerID], signed)
	}

	cards := make([]cardResponse, 0, len(candidates))
	for _, candidate := range candidates {
		photoURLs := urls[candidate.userID]
		if photoURLs == nil {
			photoURLs = []string{}
		}
		cards = append(cards, cardResponse{Player: playerCard{
			DisplayName:   candidate.displayName,
			Age:           ageAt(candidate.birthDate, time.Now().UTC()),
			Gender:        candidate.gender,
			SkillLevel:    candidate.skillLevel,
			Bio:           candidate.bio,
			PlaySummary:   playSummary(availability[candidate.userID]),
			PhotoURLs:     photoURLs,
			DistanceMiles: candidate.displayMiles,
		}})
	}
	response := pageResponse{Cards: cards}
	if hasMore {
		last := candidates[len(candidates)-1]
		cursor := encodeCursor(last.displayMiles, last.userID, request.radius)
		response.NextCursor = &cursor
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (s *Service) viewerEligible(ctx context.Context, tx *sql.Tx, w http.ResponseWriter, userID string) bool {
	var birthDate time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT u.birth_date
		FROM profiles p JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1 AND u.deleted_at IS NULL`, userID).Scan(&birthDate)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusConflict, "complete your profile")
		return false
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
	if ageAt(birthDate, time.Now().UTC()) < 18 {
		httpapi.WriteError(w, http.StatusConflict, "complete your profile")
		return false
	}

	var hasLocation bool
	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM locations WHERE user_id = $1)", userID).Scan(&hasLocation); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
	if !hasLocation {
		httpapi.WriteError(w, http.StatusConflict, "set your area")
		return false
	}

	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM photos WHERE owner_id = $1 AND moderation_status = 'approved')",
		userID).Scan(&hasLocation); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
	if !hasLocation {
		httpapi.WriteError(w, http.StatusConflict, "one photo must be approved")
		return false
	}
	return true
}

func scanCandidates(rows *sql.Rows) ([]candidateRow, error) {
	defer rows.Close()
	candidates := make([]candidateRow, 0)
	for rows.Next() {
		var candidate candidateRow
		if err := rows.Scan(&candidate.userID, &candidate.displayMiles, &candidate.displayName,
			&candidate.gender, &candidate.skillLevel, &candidate.bio, &candidate.birthDate); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func loadAvailability(ctx context.Context, tx *sql.Tx, ids []string) (map[string][]slot, error) {
	availability := make(map[string][]slot, len(ids))
	if len(ids) == 0 {
		return availability, nil
	}
	rows, err := tx.QueryContext(ctx, availabilityQuery, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var profileID string
		var entry slot
		if err := rows.Scan(&profileID, &entry.weekday, &entry.start); err != nil {
			return nil, err
		}
		availability[profileID] = append(availability[profileID], entry)
	}
	return availability, rows.Err()
}

func loadPhotos(ctx context.Context, tx *sql.Tx, ids []string) ([]photoRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, photoQuery, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := make([]photoRow, 0, len(ids)*photoLimit)
	for rows.Next() {
		var photo photoRow
		if err := rows.Scan(&photo.ownerID, &photo.objectKey); err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	return photos, rows.Err()
}

func parsePageRequest(values url.Values) (pageRequest, bool) {
	radius, ok := intParam(values.Get("radius"), defaultRadius)
	if !ok || radius < minRadius || radius > maxRadius {
		return pageRequest{}, false
	}
	limit, ok := intParam(values.Get("limit"), defaultLimit)
	if !ok || limit < minLimit || limit > maxLimit {
		return pageRequest{}, false
	}
	request := pageRequest{radius: radius, limit: limit}
	if raw := values.Get("cursor"); raw != "" {
		cursor, ok := parseCursor(raw, radius)
		if !ok {
			return pageRequest{}, false
		}
		request.cursor = cursor
	}
	return request, true
}

func intParam(raw string, fallback int) (int, bool) {
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseCursor(raw string, radius int) (*pageCursor, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, false
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return nil, false
	}
	miles, err := strconv.Atoi(parts[0])
	if err != nil || miles < 0 || miles > radius {
		return nil, false
	}
	if !validUUID(parts[1]) {
		return nil, false
	}
	cursorRadius, err := strconv.Atoi(parts[2])
	if err != nil || cursorRadius != radius {
		return nil, false
	}
	return &pageCursor{miles: miles, userID: parts[1]}, true
}

func encodeCursor(miles int, userID string, radius int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s:%d", miles, userID, radius)))
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

// ageAt returns completed years from a date of birth.
func ageAt(birthDate, now time.Time) int {
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return age
}

// playSummary renders at most two weekday-plus-part-of-day phrases. Entries are
// de-duplicated; weekdays sharing a part of day are joined into one phrase
// ("Tue, Thu evenings") before phrases are capped.
func playSummary(slots []slot) string {
	type entry struct {
		weekday int
		period  int
	}
	seen := make(map[entry]struct{}, len(slots))
	entries := make([]entry, 0, len(slots))
	for _, item := range slots {
		candidate := entry{weekday: item.weekday, period: partOfDay(item.start)}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		entries = append(entries, candidate)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].weekday != entries[j].weekday {
			return entries[i].weekday < entries[j].weekday
		}
		return entries[i].period < entries[j].period
	})
	if len(entries) == 0 {
		return ""
	}

	type phrase struct {
		period int
		days   []int
	}
	phrases := make([]phrase, 0, 2)
	byPeriod := make(map[int]int, 2)
	for _, item := range entries {
		index, ok := byPeriod[item.period]
		if !ok {
			index = len(phrases)
			byPeriod[item.period] = index
			phrases = append(phrases, phrase{period: item.period})
		}
		phrases[index].days = append(phrases[index].days, item.weekday)
	}
	sort.Slice(phrases, func(i, j int) bool { return phrases[i].days[0] < phrases[j].days[0] })

	rendered := make([]string, 0, 2)
	for _, item := range phrases {
		names := make([]string, len(item.days))
		for index, day := range item.days {
			names[index] = weekdayName(day)
		}
		rendered = append(rendered, strings.Join(names, ", ")+" "+periodName(item.period))
		if len(rendered) == 2 {
			break
		}
	}
	return strings.Join(rendered, ", ")
}

func partOfDay(start string) int {
	hour, _ := strconv.Atoi(strings.SplitN(start, ":", 2)[0])
	switch {
	case hour < 12:
		return 0
	case hour < 17:
		return 1
	default:
		return 2
	}
}

func weekdayName(weekday int) string {
	names := [...]string{"", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	if weekday < 1 || weekday > 7 {
		return ""
	}
	return names[weekday]
}

func periodName(period int) string {
	switch period {
	case 0:
		return "mornings"
	case 1:
		return "afternoons"
	default:
		return "evenings"
	}
}
