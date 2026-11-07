// Package location stores a player's coarse home area and optional travel
// destination. Exact coordinates never leave the server: requests accept a
// browser position or an area name, responses carry only a neighbourhood label.
package location

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tennisconnect/internal/httpapi"
	"tennisconnect/internal/saferead"
)

const (
	mapboxEndpoint = "https://api.mapbox.com/search/geocode/v6"
	requestTimeout = 4 * time.Second
	responseLimit  = 64 << 10
	labelLimit     = 80
	defaultDays    = 7
	maxDays        = 7
)

var (
	// ErrOutsideUK reports a geocode result outside Great Britain.
	ErrOutsideUK = errors.New("area outside the UK")
	// ErrNoResult reports an input with no usable UK settlement.
	ErrNoResult = errors.New("area not found")
	// ErrProvider reports a geocoding provider or transport failure.
	ErrProvider = errors.New("geocoding provider failed")
)

// Point is a WGS84 position used between the service and its geocoder.
type Point struct {
	Latitude  float64
	Longitude float64
}

// Geocoder resolves UK areas and browser coordinates to a coarse label.
type Geocoder interface {
	Forward(ctx context.Context, area string) (Point, string, error)
	Reverse(ctx context.Context, latitude, longitude float64) (string, error)
}

// Service owns location persistence and its HTTP handlers.
type Service struct {
	DB         *sql.DB
	Geocoder   Geocoder
	AppBaseURL string
}

type homeInput struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Area      *string  `json:"area"`
}

type travelInput struct {
	Area string `json:"area"`
	Days *int   `json:"days"`
}

type homeResponse struct {
	AreaLabel       string  `json:"area_label"`
	TravelAreaLabel *string `json:"travel_area_label,omitempty"`
	TravelExpiresAt *string `json:"travel_expires_at,omitempty"`
}

type travelResponse struct {
	AreaLabel string `json:"area_label"`
	ExpiresAt string `json:"expires_at"`
}

// Put sets the authenticated player's home area from browser coordinates or an
// area name. Exactly one input form is accepted.
func (s *Service) Put(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.Geocoder == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var input homeInput
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	hasCoordinates := input.Latitude != nil || input.Longitude != nil
	hasArea := input.Area != nil
	if hasCoordinates == hasArea {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var point Point
	var label string
	var err error
	if hasCoordinates {
		if input.Latitude == nil || input.Longitude == nil ||
			!validLatitude(*input.Latitude) || !validLongitude(*input.Longitude) {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
			return
		}
		point = Point{Latitude: *input.Latitude, Longitude: *input.Longitude}
		label, err = s.Geocoder.Reverse(ctx, point.Latitude, point.Longitude)
	} else {
		area := strings.TrimSpace(*input.Area)
		if area == "" || utf8.RuneCountInString(area) > labelLimit {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
			return
		}
		point, label, err = s.Geocoder.Forward(ctx, area)
	}
	if !writeGeocodeError(w, err) {
		return
	}
	label, ok := cleanLabel(label)
	if !ok {
		httpapi.WriteError(w, http.StatusUnprocessableEntity, "unsupported area")
		return
	}

	_, err = s.DB.ExecContext(ctx, `
		INSERT INTO locations (user_id, home, home_area_label)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			home = EXCLUDED.home,
			home_area_label = EXCLUDED.home_area_label,
			updated_at = now()`,
		userID, point.Longitude, point.Latitude, label)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, homeResponse{AreaLabel: label})
}

// Get returns only labels and dates so a client can restore its header after reload.
func (s *Service) Get(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var response homeResponse
	var travelLabel sql.NullString
	var travelExpires sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT home_area_label, travel_area_label,
			CASE WHEN travel_expires_at > now() THEN travel_expires_at END
		FROM locations WHERE user_id = $1`, userID).Scan(&response.AreaLabel, &travelLabel, &travelExpires)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "location not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if travelExpires.Valid {
		if travelLabel.Valid {
			label := travelLabel.String
			response.TravelAreaLabel = &label
		}
		expires := travelExpires.Time.UTC().Format(time.RFC3339)
		response.TravelExpiresAt = &expires
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

// PutTravel sets or replaces a temporary destination for one to seven days.
func (s *Service) PutTravel(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.Geocoder == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var input travelInput
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	area := strings.TrimSpace(input.Area)
	if area == "" || utf8.RuneCountInString(area) > labelLimit {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	days := defaultDays
	if input.Days != nil {
		days = *input.Days
	}
	if days < 1 || days > maxDays {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var hasHome bool
	if err := s.DB.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM locations WHERE user_id = $1)", userID).Scan(&hasHome); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !hasHome {
		httpapi.WriteError(w, http.StatusConflict, "set your area first")
		return
	}

	point, label, err := s.Geocoder.Forward(ctx, area)
	if !writeGeocodeError(w, err) {
		return
	}
	label, ok := cleanLabel(label)
	if !ok {
		httpapi.WriteError(w, http.StatusUnprocessableEntity, "unsupported area")
		return
	}

	var expiresAt time.Time
	err = s.DB.QueryRowContext(ctx, `
		INSERT INTO locations (user_id, home, home_area_label, travel, travel_area_label, travel_expires_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4,
			ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, now() + make_interval(days => $5))
		ON CONFLICT (user_id) DO UPDATE SET
			travel = EXCLUDED.travel,
			travel_area_label = EXCLUDED.travel_area_label,
			travel_expires_at = EXCLUDED.travel_expires_at,
			updated_at = now()
		RETURNING travel_expires_at`,
		userID, point.Longitude, point.Latitude, label, days).Scan(&expiresAt)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, travelResponse{
		AreaLabel: label,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

// DeleteTravel clears only the travel columns and keeps the home row. It is
// idempotent: 204 whether or not a destination was set.
func (s *Service) DeleteTravel(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := s.DB.ExecContext(ctx, `
		UPDATE locations
		SET travel = NULL, travel_area_label = NULL, travel_expires_at = NULL, updated_at = now()
		WHERE user_id = $1`, userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeGeocodeError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrOutsideUK), errors.Is(err, ErrNoResult):
		httpapi.WriteError(w, http.StatusUnprocessableEntity, "unsupported area")
		return false
	default:
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
}

func validLatitude(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -90 && value <= 90
}

func validLongitude(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -180 && value <= 180
}

type mapbox struct {
	token    string
	endpoint string
	client   *http.Client
}

// NewMapbox returns the stdlib Mapbox v6 geocoder client.
func NewMapbox(token string) Geocoder {
	return &mapbox{token: token, endpoint: mapboxEndpoint, client: newHTTPClient()}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		// A redirect could replay the token to another host.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (m *mapbox) Forward(ctx context.Context, area string) (Point, string, error) {
	values := url.Values{}
	values.Set("q", area)
	values.Set("country", "gb")
	values.Set("limit", "1")
	values.Set("permanent", "true")
	values.Set("types", forwardTypes(area))
	values.Set("access_token", m.token)
	feature, err := m.feature(ctx, "/forward", values)
	if err != nil {
		return Point{}, "", err
	}
	return feature.point()
}

func (m *mapbox) Reverse(ctx context.Context, latitude, longitude float64) (string, error) {
	values := url.Values{}
	values.Set("longitude", formatCoordinate(longitude))
	values.Set("latitude", formatCoordinate(latitude))
	values.Set("country", "gb")
	values.Set("limit", "1")
	values.Set("permanent", "true")
	values.Set("types", "neighborhood,locality,place")
	values.Set("access_token", m.token)
	feature, err := m.feature(ctx, "/reverse", values)
	if err != nil {
		return "", err
	}
	return feature.label()
}

func formatCoordinate(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func forwardTypes(area string) string {
	if postcodeShaped(area) {
		return "postcode,neighborhood,locality,place,district"
	}
	return "neighborhood,locality,place,district"
}

func (m *mapbox) feature(ctx context.Context, path string, values url.Values) (geojsonFeature, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.endpoint+path+"?"+values.Encode(), nil)
	if err != nil {
		return geojsonFeature{}, ErrProvider
	}
	response, err := m.client.Do(request)
	if err != nil {
		return geojsonFeature{}, ErrProvider
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return geojsonFeature{}, ErrProvider
	}
	payload, err := saferead.ReadBounded(response.Body, responseLimit)
	if err != nil {
		return geojsonFeature{}, ErrProvider
	}
	var decoded geojsonResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return geojsonFeature{}, ErrProvider
	}
	if len(decoded.Features) == 0 {
		return geojsonFeature{}, ErrNoResult
	}
	feature := decoded.Features[0]
	if !strings.EqualFold(feature.Properties.Context.Country.CountryCode, "GB") {
		return geojsonFeature{}, ErrOutsideUK
	}
	return feature, nil
}

type geojsonResponse struct {
	Features []geojsonFeature `json:"features"`
}

type geojsonFeature struct {
	Properties struct {
		Name        string `json:"name"`
		FeatureType string `json:"feature_type"`
		Coordinates struct {
			Longitude float64 `json:"longitude"`
			Latitude  float64 `json:"latitude"`
		} `json:"coordinates"`
		Context struct {
			Country struct {
				CountryCode string `json:"country_code"`
			} `json:"country"`
			Neighborhood struct {
				Name string `json:"name"`
			} `json:"neighborhood"`
			Locality struct {
				Name string `json:"name"`
			} `json:"locality"`
			Place struct {
				Name string `json:"name"`
			} `json:"place"`
		} `json:"context"`
	} `json:"properties"`
}

func (f geojsonFeature) point() (Point, string, error) {
	point := Point{Latitude: f.Properties.Coordinates.Latitude, Longitude: f.Properties.Coordinates.Longitude}
	if !validLatitude(point.Latitude) || !validLongitude(point.Longitude) {
		return Point{}, "", ErrNoResult
	}
	label, err := f.label()
	if err != nil {
		return Point{}, "", err
	}
	return point, label, nil
}

func (f geojsonFeature) label() (string, error) {
	// A settlement's own name is the best label; a postcode's own name is
	// never used because the coarse area lives in its context instead.
	switch f.Properties.FeatureType {
	case "neighborhood", "locality", "place":
		if label, ok := cleanLabel(f.Properties.Name); ok {
			return label, nil
		}
	}
	for _, raw := range []string{
		f.Properties.Context.Neighborhood.Name,
		f.Properties.Context.Locality.Name,
		f.Properties.Context.Place.Name,
	} {
		if label, ok := cleanLabel(raw); ok {
			return label, nil
		}
	}
	return "", ErrNoResult
}

// cleanLabel normalises a provider label and strips any postcode text. It
// rejects empty, over-long, and control-character labels.
func cleanLabel(raw string) (string, bool) {
	if strings.ContainsAny(raw, "\x00\r\n") {
		return "", false
	}
	fields := strings.Fields(raw)
	kept := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		if postcodeShaped(fields[i]) {
			continue
		}
		if i+1 < len(fields) && postcodeShaped(fields[i]+fields[i+1]) {
			i++
			continue
		}
		kept = append(kept, fields[i])
	}
	label := strings.Join(kept, " ")
	if label == "" || utf8.RuneCountInString(label) > labelLimit {
		return "", false
	}
	return label, true
}

// postcodeShaped reports whether value looks like a complete UK postcode, with
// or without the usual space. Partial outward codes do not match.
func postcodeShaped(value string) bool {
	compact := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	if len(compact) < 4 || len(compact) > 7 {
		return false
	}
	letters := 0
	for letters < len(compact) && compact[letters] >= 'A' && compact[letters] <= 'Z' {
		letters++
	}
	if letters < 1 || letters > 2 {
		return false
	}
	rest := compact[letters:]
	if len(rest) < 4 || len(rest) > 5 {
		return false
	}
	// outward: digit, optional alphanumeric; inward: digit, letter, letter.
	if rest[0] < '0' || rest[0] > '9' {
		return false
	}
	inward := rest[len(rest)-3:]
	if inward[0] < '0' || inward[0] > '9' ||
		inward[1] < 'A' || inward[1] > 'Z' ||
		inward[2] < 'A' || inward[2] > 'Z' {
		return false
	}
	outwardTail := rest[:len(rest)-3]
	if len(outwardTail) == 0 || len(outwardTail) > 2 {
		return false
	}
	for _, char := range outwardTail {
		if (char < '0' || char > '9') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}
