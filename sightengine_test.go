package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func floatPointer(value float64) *float64 { return &value }

func stringPointerValue(value string) *string { return &value }

func validModerationResponse() sightengineResponse {
	var response sightengineResponse
	response.Status = "success"
	response.Request.ID = "req-1"
	response.Nudity = &sightengineNudity{
		SexualActivity: floatPointer(0.01),
		SexualDisplay:  floatPointer(0.01),
		Erotica:        floatPointer(0.01),
		VerySuggestive: floatPointer(0.01),
	}
	response.Offensive = &sightengineOffensive{
		Nazi:          floatPointer(0.01),
		AsianSwastika: floatPointer(0.01),
		Confederate:   floatPointer(0.01),
		Supremacist:   floatPointer(0.01),
		Terrorist:     floatPointer(0.01),
		MiddleFinger:  floatPointer(0.01),
	}
	response.Faces = &[]sightengineFace{{
		X1: floatPointer(0.3), Y1: floatPointer(0.3),
		X2: floatPointer(0.7), Y2: floatPointer(0.7),
		Attributes: &sightengineFaceAttributes{Obstruction: &sightengineObstruction{None: floatPointer(0.9)}},
	}}
	response.Text = &sightengineText{Content: stringPointerValue("")}
	return response
}

func TestModerationPolicyApprovesCleanPortrait(t *testing.T) {
	verdict, err := moderationVerdictFor(validModerationResponse())
	if err != nil {
		t.Fatalf("verdict error: %v", err)
	}
	if verdict.status != photoModerationApproved {
		t.Fatalf("status = %q, want approved", verdict.status)
	}
	if verdict.reference != "req-1" {
		t.Fatalf("reference = %q", verdict.reference)
	}

	// Swimwear/vest photos score high on suggestive classes the vendor documents
	// as everyday clothing; only the safety keys decide.
	swimwear := validModerationResponse()
	swimwear.Text.Content = stringPointerValue("")
	if verdict, err := moderationVerdictFor(swimwear); err != nil || verdict.status != photoModerationApproved {
		t.Fatalf("swimwear verdict = %q, %v", verdict.status, err)
	}
}

func TestModerationPolicyRejects(t *testing.T) {
	tests := []struct {
		name   string
		change func(*sightengineResponse)
	}{
		{name: "sexual activity", change: func(r *sightengineResponse) { r.Nudity.SexualActivity = floatPointer(0.5) }},
		{name: "sexual display", change: func(r *sightengineResponse) { r.Nudity.SexualDisplay = floatPointer(0.5) }},
		{name: "erotica", change: func(r *sightengineResponse) { r.Nudity.Erotica = floatPointer(0.5) }},
		{name: "very suggestive", change: func(r *sightengineResponse) { r.Nudity.VerySuggestive = floatPointer(0.5) }},
		{name: "nazi", change: func(r *sightengineResponse) { r.Offensive.Nazi = floatPointer(0.5) }},
		{name: "asian swastika", change: func(r *sightengineResponse) { r.Offensive.AsianSwastika = floatPointer(0.5) }},
		{name: "confederate", change: func(r *sightengineResponse) { r.Offensive.Confederate = floatPointer(0.5) }},
		{name: "supremacist", change: func(r *sightengineResponse) { r.Offensive.Supremacist = floatPointer(0.5) }},
		{name: "terrorist", change: func(r *sightengineResponse) { r.Offensive.Terrorist = floatPointer(0.5) }},
		{name: "middle finger", change: func(r *sightengineResponse) { r.Offensive.MiddleFinger = floatPointer(0.5) }},
		{name: "no faces", change: func(r *sightengineResponse) { r.Faces = &[]sightengineFace{} }},
		{name: "artificial face only", change: func(r *sightengineResponse) {
			r.Faces = &[]sightengineFace{}
			// A real-face list without attributes models an artificial-only response.
		}},
		{name: "tiny face", change: func(r *sightengineResponse) {
			(*r.Faces)[0].X2 = floatPointer(0.35)
			(*r.Faces)[0].Y2 = floatPointer(0.35)
		}},
		{name: "obstructed face", change: func(r *sightengineResponse) {
			(*r.Faces)[0].Attributes.Obstruction.None = floatPointer(0.2)
		}},
		{name: "phone number text", change: func(r *sightengineResponse) { r.Text.Content = stringPointerValue("+44 7700 900123") }},
		{name: "email text", change: func(r *sightengineResponse) { r.Text.Content = stringPointerValue("me@example.com") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := validModerationResponse()
			test.change(&response)
			verdict, err := moderationVerdictFor(response)
			if err != nil {
				t.Fatalf("verdict error: %v", err)
			}
			if verdict.status != photoModerationRejected {
				t.Fatalf("status = %q, want rejected", verdict.status)
			}
		})
	}
}

func TestModerationPolicyAllowsMonthNameDatesInText(t *testing.T) {
	response := validModerationResponse()
	response.Text.Content = stringPointerValue("29 October 2026, courts 3-4")
	if verdict, err := moderationVerdictFor(response); err != nil || verdict.status != photoModerationApproved {
		t.Fatalf("verdict = %q, %v", verdict.status, err)
	}
}

func TestModerationPolicyBoundaries(t *testing.T) {
	tests := []struct {
		score float64
		want  photoModerationStatus
	}{
		{score: 0.399, want: photoModerationApproved},
		{score: 0.4, want: photoModerationPending},
		{score: 0.499, want: photoModerationPending},
		{score: 0.5, want: photoModerationRejected},
	}
	for _, test := range tests {
		response := validModerationResponse()
		response.Nudity.SexualDisplay = floatPointer(test.score)
		verdict, err := moderationVerdictFor(response)
		if err != nil {
			t.Fatalf("score %v: %v", test.score, err)
		}
		if verdict.status != test.want {
			t.Fatalf("score %v status = %q, want %q", test.score, verdict.status, test.want)
		}
	}
}

func TestModerationPolicyMalformedResponses(t *testing.T) {
	tests := []struct {
		name   string
		change func(*sightengineResponse)
	}{
		{name: "failure status", change: func(r *sightengineResponse) { r.Status = "failure" }},
		{name: "missing nudity", change: func(r *sightengineResponse) { r.Nudity = nil }},
		{name: "missing offensive", change: func(r *sightengineResponse) { r.Offensive = nil }},
		{name: "missing faces", change: func(r *sightengineResponse) { r.Faces = nil }},
		{name: "missing text", change: func(r *sightengineResponse) { r.Text = nil }},
		{name: "missing text content", change: func(r *sightengineResponse) { r.Text.Content = nil }},
		{name: "missing nudity key", change: func(r *sightengineResponse) { r.Nudity.Erotica = nil }},
		{name: "missing offensive key", change: func(r *sightengineResponse) { r.Offensive.Terrorist = nil }},
		{name: "missing face coordinates", change: func(r *sightengineResponse) { (*r.Faces)[0].X1 = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := validModerationResponse()
			test.change(&response)
			if _, err := moderationVerdictFor(response); err == nil {
				t.Fatal("malformed response was accepted")
			}
		})
	}
}

func TestSightengineClientSendsBoundedMultipartRequest(t *testing.T) {
	var receivedSecretQuery string
	var receivedModels string
	var receivedMedia []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/1.0/check.json" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		receivedSecretQuery = r.URL.RawQuery
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		receivedModels = r.FormValue("models")
		if r.FormValue("api_user") == "" || r.FormValue("api_secret") == "" {
			t.Error("credentials missing from multipart body")
		}
		file, _, err := r.FormFile("media")
		if err != nil {
			t.Errorf("media missing: %v", err)
		} else {
			defer file.Close()
			var buffer bytes.Buffer
			_, _ = buffer.ReadFrom(file)
			receivedMedia = buffer.Bytes()
		}
		response := validModerationResponse()
		response.Request.ID = strings.Repeat("r", 300)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := sightengineClient{
		apiUser:   "user",
		apiSecret: "secret",
		endpoint:  server.URL + "/1.0/check.json",
		client:    newSightengineHTTPClient(),
	}
	image := photoTestJPEG(t, 8, 8)
	verdict, err := client.moderate(context.Background(), image)
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if verdict.status != photoModerationApproved {
		t.Fatalf("status = %q", verdict.status)
	}
	if len(verdict.reference) != 200 {
		t.Fatalf("reference length = %d, want 200", len(verdict.reference))
	}
	if receivedSecretQuery != "" {
		t.Fatalf("credentials leaked into the query: %q", receivedSecretQuery)
	}
	if receivedModels != photoModerationModels {
		t.Fatalf("models = %q, want %q", receivedModels, photoModerationModels)
	}
	if !bytes.Equal(receivedMedia, image) {
		t.Fatal("uploaded media differs from the image")
	}
}

func TestSightengineClientRejectsRedirectsAndErrors(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	client := sightengineClient{
		apiUser:   "user",
		apiSecret: "secret",
		endpoint:  redirector.URL,
		client:    newSightengineHTTPClient(),
	}
	if _, err := client.moderate(context.Background(), photoTestJPEG(t, 8, 8)); err == nil {
		t.Fatal("redirect was followed")
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()
	client.endpoint = failing.URL
	if _, err := client.moderate(context.Background(), photoTestJPEG(t, 8, 8)); err == nil {
		t.Fatal("500 response was accepted")
	}

	oversized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","padding":"` + strings.Repeat("x", 70<<10) + `"}`))
	}))
	defer oversized.Close()
	client.endpoint = oversized.URL
	if _, err := client.moderate(context.Background(), photoTestJPEG(t, 8, 8)); !errors.Is(err, errPhotoObjectTooLarge) {
		t.Fatalf("oversized response err = %v", err)
	}
}
