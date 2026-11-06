package location

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPostcodeShaped(t *testing.T) {
	for _, value := range []string{"SW11 4NJ", "sw114nj", "EC1A 1BB", " M1 1AE "} {
		if !postcodeShaped(value) {
			t.Fatalf("postcodeShaped(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"Battersea", "London", "Edinburgh", "SW11", ""} {
		if postcodeShaped(value) {
			t.Fatalf("postcodeShaped(%q) = true, want false", value)
		}
	}
}

func TestCleanLabel(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  string
		valid bool
	}{
		{name: "trims and collapses", raw: "  Battersea   Reach ", want: "Battersea Reach", valid: true},
		{name: "strips trailing postcode", raw: "Battersea SW11 4NJ", want: "Battersea", valid: true},
		{name: "rejects postcode only", raw: "SW11 4NJ", valid: false},
		{name: "rejects empty", raw: "   ", valid: false},
		{name: "rejects control characters", raw: "Battersea\x00", valid: false},
		{name: "rejects over eighty runes", raw: strings.Repeat("é", 81), valid: false},
		{name: "accepts eighty runes", raw: strings.Repeat("é", 80), want: strings.Repeat("é", 80), valid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := cleanLabel(test.raw)
			if ok != test.valid {
				t.Fatalf("cleanLabel(%q) ok = %t, want %t", test.raw, ok, test.valid)
			}
			if ok && got != test.want {
				t.Fatalf("cleanLabel(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func geojsonBody(features string) string {
	return `{"type":"FeatureCollection","features":[` + features + `]}`
}

func featureBody(featureType, name, neighborhood, locality, place, countryCode string) string {
	return `{
		"type":"Feature",
		"properties":{
			"name":"` + name + `",
			"feature_type":"` + featureType + `",
			"coordinates":{"longitude":-0.1611,"latitude":51.4649},
			"context":{
				"country":{"country_code":"` + countryCode + `"},
				"neighborhood":{"name":"` + neighborhood + `"},
				"locality":{"name":"` + locality + `"},
				"place":{"name":"` + place + `"}
			}
		},
		"geometry":{"type":"Point","coordinates":[-0.1611,51.4649]}
	}`
}

func newTestMapbox(t *testing.T, handler http.HandlerFunc) *mapbox {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &mapbox{
		token:    "pk.test-token",
		endpoint: server.URL,
		client:   newHTTPClient(),
	}
}

func TestMapboxForward(t *testing.T) {
	var request *http.Request
	client := newTestMapbox(t, func(w http.ResponseWriter, r *http.Request) {
		request = r
		_, _ = io.WriteString(w, geojsonBody(featureBody("neighborhood", "Battersea", "", "Wandsworth", "London", "GB")))
	})

	point, label, err := client.Forward(context.Background(), "Battersea")
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if label != "Battersea" || point.Latitude != 51.4649 || point.Longitude != -0.1611 {
		t.Fatalf("Forward() = %#v, %q", point, label)
	}
	query := request.URL.Query()
	if query.Get("q") != "Battersea" || query.Get("country") != "gb" {
		t.Fatalf("forward query = %v", query)
	}
	if query.Get("types") != "neighborhood,locality,place,district" {
		t.Fatalf("forward types = %q", query.Get("types"))
	}
	if query.Get("permanent") != "true" || query.Get("limit") != "1" || query.Get("access_token") != "pk.test-token" {
		t.Fatalf("forward parameters = %v", query)
	}
	if strings.Contains(request.URL.Path, "reverse") {
		t.Fatalf("forward path = %q", request.URL.Path)
	}
}

func TestMapboxForwardPostcodeRequestsPostcodeType(t *testing.T) {
	var types string
	client := newTestMapbox(t, func(w http.ResponseWriter, r *http.Request) {
		types = r.URL.Query().Get("types")
		_, _ = io.WriteString(w, geojsonBody(featureBody("postcode", "SW11 4NJ", "Battersea", "Wandsworth", "London", "GB")))
	})

	_, label, err := client.Forward(context.Background(), "SW11 4NJ")
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if !strings.Contains(types, "postcode") {
		t.Fatalf("postcode input types = %q, want postcode", types)
	}
	if label != "Battersea" {
		t.Fatalf("postcode label = %q, want context neighbourhood", label)
	}
}

func TestMapboxReverse(t *testing.T) {
	var request *http.Request
	client := newTestMapbox(t, func(w http.ResponseWriter, r *http.Request) {
		request = r
		_, _ = io.WriteString(w, geojsonBody(featureBody("neighborhood", "Battersea", "Battersea", "Wandsworth", "London", "GB")))
	})

	label, err := client.Reverse(context.Background(), 51.4649, -0.1611)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if label != "Battersea" {
		t.Fatalf("Reverse label = %q", label)
	}
	query := request.URL.Query()
	if query.Get("latitude") != "51.4649" || query.Get("longitude") != "-0.1611" {
		t.Fatalf("reverse coordinates = %v", query)
	}
	if query.Get("country") != "gb" || query.Get("permanent") != "true" || query.Get("limit") != "1" {
		t.Fatalf("reverse parameters = %v", query)
	}
	if query.Get("types") != "neighborhood,locality,place" {
		t.Fatalf("reverse types = %q", query.Get("types"))
	}
}

func TestMapboxRejectsNonUKAndEmptyResults(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want error
	}{
		{name: "non GB", body: geojsonBody(featureBody("place", "Paris", "", "", "Paris", "FR")), want: ErrOutsideUK},
		{name: "empty", body: geojsonBody(""), want: ErrNoResult},
		{name: "unknown country", body: geojsonBody(featureBody("place", "Paris", "", "", "Paris", "")), want: ErrOutsideUK},
		{name: "postcode only label", body: geojsonBody(featureBody("postcode", "SW11 4NJ", "", "", "", "GB")), want: ErrNoResult},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newTestMapbox(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, test.body)
			})
			if _, _, err := client.Forward(context.Background(), "somewhere"); !errors.Is(err, test.want) {
				t.Fatalf("Forward err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestMapboxProviderFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "server error", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}},
		{name: "rate limited", handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}},
		{name: "redirect", handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}},
		{name: "malformed", handler: func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"features":`)
		}},
		{name: "oversized", handler: func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("a", responseLimit+1))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestMapbox(t, test.handler)
			_, _, err := client.Forward(context.Background(), "Battersea")
			if !errors.Is(err, ErrProvider) {
				t.Fatalf("Forward err = %v, want ErrProvider", err)
			}
			if strings.Contains(err.Error(), "pk.test-token") || strings.Contains(err.Error(), "-0.1611") {
				t.Fatalf("error leaks provider details: %v", err)
			}
		})
	}
}

func TestMapboxTimeout(t *testing.T) {
	client := newTestMapbox(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, _, err := client.Forward(ctx, "Battersea"); !errors.Is(err, ErrProvider) {
		t.Fatalf("Forward err = %v, want ErrProvider", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout not bounded: %v", elapsed)
	}
}
