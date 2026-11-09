package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"tennisconnect/internal/httpapi"
	"tennisconnect/internal/textguard"
)

type availabilityInput struct {
	Weekday int    `json:"weekday"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

type profileInput struct {
	DisplayName      string              `json:"display_name"`
	Gender           string              `json:"gender"`
	SkillLevel       string              `json:"skill_level"`
	Ranking          *string             `json:"ranking"`
	Bio              string              `json:"bio"`
	GenderPreference string              `json:"gender_preference"`
	MinAge           int                 `json:"min_age"`
	MaxAge           int                 `json:"max_age"`
	SkillTolerance   int                 `json:"skill_tolerance"`
	Availability     []availabilityInput `json:"availability"`
	// InstagramHandle is optional and disclosed only inside a match. It is
	// stored bare and lower case; a leading "@" is accepted and stripped.
	InstagramHandle *string `json:"instagram_handle"`
	// Age is server-computed from users.birth_date; a client value is ignored.
	Age int `json:"age,omitempty"`
}

func validateProfile(input profileInput) (profileInput, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Bio = strings.TrimSpace(input.Bio)
	if input.Ranking != nil {
		ranking := strings.TrimSpace(*input.Ranking)
		if ranking == "" {
			input.Ranking = nil
		} else {
			input.Ranking = &ranking
		}
	}

	if utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 50 ||
		strings.ContainsAny(input.DisplayName, "\x00\r\n") {
		return input, errors.New("invalid display name")
	}
	if input.Gender != "woman" && input.Gender != "man" {
		return input, errors.New("invalid gender")
	}
	switch input.SkillLevel {
	case "beginner", "improver", "intermediate", "advanced":
	default:
		return input, errors.New("invalid skill level")
	}
	if input.Ranking != nil && (utf8.RuneCountInString(*input.Ranking) > 50 || strings.ContainsRune(*input.Ranking, '\x00')) {
		return input, errors.New("invalid ranking")
	}
	if utf8.RuneCountInString(input.Bio) > 200 || strings.ContainsRune(input.Bio, '\x00') {
		return input, errors.New("invalid bio")
	}
	if textguard.ContainsContactDetails(input.DisplayName) || textguard.ContainsContactDetails(input.Bio) {
		return input, errors.New("contact details are not allowed")
	}
	if input.GenderPreference != "women" && input.GenderPreference != "men" && input.GenderPreference != "all" {
		return input, errors.New("invalid gender preference")
	}
	if input.MinAge < 18 || input.MaxAge > 120 || input.MinAge > input.MaxAge {
		return input, errors.New("invalid age range")
	}
	if input.SkillTolerance < 0 || input.SkillTolerance > 1 {
		return input, errors.New("invalid skill tolerance")
	}
	if input.InstagramHandle != nil {
		handle := strings.ToLower(strings.TrimSpace(*input.InstagramHandle))
		handle = strings.TrimPrefix(handle, "@")
		if handle == "" {
			input.InstagramHandle = nil
		} else if !validInstagramHandle(handle) {
			return input, errors.New("invalid instagram handle")
		} else {
			input.InstagramHandle = &handle
		}
	}
	if len(input.Availability) == 0 || len(input.Availability) > 20 {
		return input, errors.New("invalid availability count")
	}

	seen := make(map[availabilityInput]struct{}, len(input.Availability))
	for _, slot := range input.Availability {
		if slot.Weekday < 1 || slot.Weekday > 7 {
			return input, errors.New("invalid weekday")
		}
		start, err := time.Parse("15:04", slot.Start)
		if err != nil {
			return input, errors.New("invalid start time")
		}
		end, err := time.Parse("15:04", slot.End)
		if err != nil {
			return input, errors.New("invalid end time")
		}
		if !start.Before(end) {
			return input, errors.New("availability must stay within one day")
		}
		if _, exists := seen[slot]; exists {
			return input, errors.New("duplicate availability")
		}
		seen[slot] = struct{}{}
	}
	return input, nil
}

// ageFromBirthDate returns completed years, clamped to the contractual 18..120.
func ageFromBirthDate(birthDate, now time.Time) int {
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	// Accounts are adults by registration; never clamp a real age upward.
	if age > 120 {
		return 120
	}
	return age
}

type Service struct {
	DB         *sql.DB
	AppBaseURL string
}

// validInstagramHandle enforces the stored form: 1-30 of a-z, 0-9, dot and
// underscore, with no leading or trailing dot and no consecutive dots.
func validInstagramHandle(handle string) bool {
	if len(handle) < 1 || len(handle) > 30 {
		return false
	}
	if handle[0] == '.' || handle[len(handle)-1] == '.' || strings.Contains(handle, "..") {
		return false
	}
	for _, char := range handle {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '.' && char != '_' {
			return false
		}
	}
	return true
}

func (s *Service) Put(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input profileInput
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input, err := validateProfile(input)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	var birthDate time.Time
	if err := tx.QueryRowContext(ctx, "SELECT birth_date FROM users WHERE id = $1", userID).Scan(&birthDate); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	input.Age = ageFromBirthDate(birthDate, time.Now().UTC())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO profiles (
			user_id, display_name, gender, skill_level, ranking, bio,
			gender_preference, min_age, max_age, skill_tolerance, instagram_handle
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			gender = EXCLUDED.gender,
			skill_level = EXCLUDED.skill_level,
			ranking = EXCLUDED.ranking,
			bio = EXCLUDED.bio,
			gender_preference = EXCLUDED.gender_preference,
			min_age = EXCLUDED.min_age,
			max_age = EXCLUDED.max_age,
			skill_tolerance = EXCLUDED.skill_tolerance,
			instagram_handle = EXCLUDED.instagram_handle,
			updated_at = now()`, userID, input.DisplayName, input.Gender, input.SkillLevel, input.Ranking, input.Bio,
		input.GenderPreference, input.MinAge, input.MaxAge, input.SkillTolerance, input.InstagramHandle)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM availability WHERE profile_id = $1", userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	values := make([]string, 0, len(input.Availability))
	args := make([]any, 0, 4*len(input.Availability))
	for i, slot := range input.Availability {
		values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d)", 4*i+1, 4*i+2, 4*i+3, 4*i+4))
		args = append(args, userID, slot.Weekday, slot.Start, slot.End)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO availability (profile_id, weekday, start_time, end_time)
		VALUES `+strings.Join(values, ","), args...); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, input)
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	var input profileInput
	var ranking sql.NullString
	var instagram sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT display_name, gender, skill_level, ranking, bio,
			gender_preference, min_age, max_age, skill_tolerance, instagram_handle
		FROM profiles WHERE user_id = $1`, userID).Scan(
		&input.DisplayName, &input.Gender, &input.SkillLevel, &ranking, &input.Bio,
		&input.GenderPreference, &input.MinAge, &input.MaxAge, &input.SkillTolerance, &instagram)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if ranking.Valid {
		input.Ranking = &ranking.String
	}
	if instagram.Valid {
		input.InstagramHandle = &instagram.String
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT weekday, start_time, end_time FROM availability
		WHERE profile_id = $1 ORDER BY weekday, start_time, end_time`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var slot availabilityInput
		var start, end string
		if err := rows.Scan(&slot.Weekday, &start, &end); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		startTime, err := time.Parse("15:04:05", start)
		if err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		endTime, err := time.Parse("15:04:05", end)
		if err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		slot.Start = startTime.Format("15:04")
		slot.End = endTime.Format("15:04")
		input.Availability = append(input.Availability, slot)
	}
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := rows.Close(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var birthDate time.Time
	if err := tx.QueryRowContext(ctx, "SELECT birth_date FROM users WHERE id = $1", userID).Scan(&birthDate); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	input.Age = ageFromBirthDate(birthDate, time.Now().UTC())
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, input)
}
