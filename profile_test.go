package main

import (
	"fmt"
	"strings"
	"testing"
)

func validProfileInput() profileInput {
	return profileInput{
		DisplayName:      "Alex",
		Gender:           "woman",
		SkillLevel:       "intermediate",
		Bio:              "Tuesday evening rallies.",
		GenderPreference: "all",
		MinAge:           25,
		MaxAge:           45,
		SkillTolerance:   1,
		Availability: []availabilityInput{
			{Weekday: 2, Start: "18:00", End: "21:00"},
		},
	}
}

func TestValidateProfileAcceptsValidInput(t *testing.T) {
	input := validProfileInput()
	input.DisplayName = " Alex "
	input.Ranking = stringPointer(" 10.5 ")
	got, err := validateProfile(input)
	if err != nil {
		t.Fatalf("validateProfile() returned error: %v", err)
	}
	if got.DisplayName != "Alex" || got.Ranking == nil || *got.Ranking != "10.5" {
		t.Fatalf("profile was not normalized: %#v", got)
	}
}

func TestValidateProfileRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*profileInput)
	}{
		{name: "empty name", mutate: func(p *profileInput) { p.DisplayName = " " }},
		{name: "long name", mutate: func(p *profileInput) { p.DisplayName = strings.Repeat("a", 51) }},
		{name: "name newline", mutate: func(p *profileInput) { p.DisplayName = "Alex\nSmith" }},
		{name: "gender", mutate: func(p *profileInput) { p.Gender = "other" }},
		{name: "skill", mutate: func(p *profileInput) { p.SkillLevel = "expert" }},
		{name: "long bio", mutate: func(p *profileInput) { p.Bio = strings.Repeat("🎾", 201) }},
		{name: "long ranking", mutate: func(p *profileInput) { p.Ranking = stringPointer(strings.Repeat("1", 51)) }},
		{name: "NUL bio", mutate: func(p *profileInput) { p.Bio = "bad\x00text" }},
		{name: "gender preference", mutate: func(p *profileInput) { p.GenderPreference = "women only" }},
		{name: "minimum age too low", mutate: func(p *profileInput) { p.MinAge = 17 }},
		{name: "maximum age too high", mutate: func(p *profileInput) { p.MaxAge = 121 }},
		{name: "reversed ages", mutate: func(p *profileInput) { p.MinAge, p.MaxAge = 50, 40 }},
		{name: "skill tolerance", mutate: func(p *profileInput) { p.SkillTolerance = 2 }},
		{name: "weekday too low", mutate: func(p *profileInput) { p.Availability[0].Weekday = 0 }},
		{name: "weekday too high", mutate: func(p *profileInput) { p.Availability[0].Weekday = 8 }},
		{name: "overnight", mutate: func(p *profileInput) { p.Availability[0].Start, p.Availability[0].End = "22:00", "02:00" }},
		{name: "equal times", mutate: func(p *profileInput) { p.Availability[0].Start, p.Availability[0].End = "18:00", "18:00" }},
		{name: "invalid time", mutate: func(p *profileInput) { p.Availability[0].Start = "25:00" }},
		{name: "no availability", mutate: func(p *profileInput) { p.Availability = nil }},
		{name: "duplicate slots", mutate: func(p *profileInput) { p.Availability = append(p.Availability, p.Availability[0]) }},
		{name: "too many slots", mutate: func(p *profileInput) {
			p.Availability = nil
			for i := 0; i < 21; i++ {
				p.Availability = append(p.Availability, availabilityInput{Weekday: 1, Start: fmt.Sprintf("%02d:00", i), End: fmt.Sprintf("%02d:00", i+1)})
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validProfileInput()
			test.mutate(&input)
			if _, err := validateProfile(input); err == nil {
				t.Fatal("validateProfile() accepted invalid input")
			}
		})
	}
}

func TestValidateProfileBlocksContactDetails(t *testing.T) {
	for _, text := range []string{
		"player@example.com",
		"https://example.com/player",
		"www.example.com/player",
		"instagram.com/player",
		"t.me/player",
		"discord.gg/player",
		"example.com/player",
		"example.dev",
		"DM @player",
		"DM:@player",
		"Call +44 7700 900123",
		"Call 07700 900123",
		"Call 1234 5678",
		"Call 123 456 789",
		"Call 29-10-2026",
		"Call 2026-10-29",
		"Reach me on 29-10-2026",
		"Reach me from 2026-10-29",
	} {
		t.Run(text, func(t *testing.T) {
			input := validProfileInput()
			input.Bio = text
			if _, err := validateProfile(input); err == nil {
				t.Fatalf("validateProfile() allowed contact details %q", text)
			}
			input = validProfileInput()
			input.DisplayName = text
			if _, err := validateProfile(input); err == nil {
				t.Fatalf("validateProfile() allowed name contact details %q", text)
			}
		})
	}
}

func TestValidateProfileAllowsNonContactText(t *testing.T) {
	for _, text := range []string{
		"I play Monday 18:00-21:00. My ranking is 10.5.",
		"Available 2026-10-29",
		"Played 2026.10.29",
		"Available 29-10-2026",
		"Played 29/10/2026",
		"Played 2026.10.29. Phone died after match",
	} {
		t.Run(text, func(t *testing.T) {
			input := validProfileInput()
			input.Bio = text
			if _, err := validateProfile(input); err != nil {
				t.Fatalf("validateProfile() rejected ordinary text: %v", err)
			}
		})
	}
}

func TestValidateProfileAllowsUnicodeBioAtLimit(t *testing.T) {
	input := validProfileInput()
	input.Bio = strings.Repeat("🎾", 200)
	if _, err := validateProfile(input); err != nil {
		t.Fatalf("validateProfile() rejected 200 characters: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
