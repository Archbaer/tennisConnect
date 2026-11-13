// Package account implements the player's appeal, deletion, recovery and
// export rights. Exports are owner-scoped and omit other players' private data.
package account

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"tennisconnect/internal/httpapi"
)

const (
	requestBudget = 10 * time.Second
	recoveryDays  = 30
	appealDays    = 14
)

// Service serves account lifecycle endpoints.
type Service struct {
	DB         *sql.DB
	AppBaseURL string
}

// Delete serves DELETE /api/v1/account.
func (s *Service) Delete(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	var deletedAt time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE users SET deleted_at = COALESCE(deleted_at, now())
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING deleted_at`, userID).Scan(&deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusConflict, "account already scheduled for deletion")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{
		"recover_until": deletedAt.Add(recoveryDays * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
}

// CleanupExpired hard-deletes accounts whose recovery window has passed.
// Database cascades remove player data; the documented S3 lifecycle rule
// removes photo objects that no longer have rows.
func (s *Service) CleanupExpired(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
		DELETE FROM users
		WHERE deleted_at IS NOT NULL AND deleted_at < now() - interval '30 days'`)
	return err
}

// AppealStatus serves GET /api/v1/account/appeal. It is reachable only with a
// banned account's session.
func (s *Service) AppealStatus(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	var bannedAt time.Time
	err := s.DB.QueryRowContext(ctx, "SELECT banned_at FROM users WHERE id = $1 AND banned_at IS NOT NULL", userID).Scan(&bannedAt)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "no suspension")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var createdAt time.Time
	var body string
	err = s.DB.QueryRowContext(ctx, "SELECT created_at, body FROM appeals WHERE user_id = $1", userID).Scan(&createdAt, &body)
	submitted := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	response := map[string]any{
		"deadline":  bannedAt.Add(appealDays * 24 * time.Hour).UTC().Format(time.RFC3339),
		"submitted": submitted,
		"open":      time.Now().Before(bannedAt.Add(appealDays * 24 * time.Hour)),
	}
	if submitted {
		response["submitted_at"] = createdAt.UTC().Format(time.RFC3339)
		response["body"] = body
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

// SubmitAppeal serves POST /api/v1/account/appeal. One appeal only.
func (s *Service) SubmitAppeal(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input.Body = strings.TrimSpace(input.Body)
	if utf8.RuneCountInString(input.Body) < 1 || utf8.RuneCountInString(input.Body) > 2000 ||
		strings.ContainsRune(input.Body, '\x00') {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid appeal")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()
	var bannedAt time.Time
	var reportID sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT banned_at, ban_report_id FROM users
		WHERE id = $1 AND banned_at IS NOT NULL FOR UPDATE`, userID).Scan(&bannedAt, &reportID)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusForbidden, "no suspension")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if time.Now().After(bannedAt.Add(appealDays * 24 * time.Hour)) {
		httpapi.WriteError(w, http.StatusForbidden, "appeal window closed")
		return
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO appeals (user_id, report_id, body) VALUES ($1, $2, $3)`,
		userID, reportID, input.Body)
	if err != nil {
		if isUniqueViolation(err) {
			httpapi.WriteError(w, http.StatusConflict, "appeal already submitted")
			return
		}
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO report_events (report_id, actor_kind, actor_id, action)
		SELECT $1, 'user', $2, 'appeal_submitted'
		WHERE $1::uuid IS NOT NULL`, reportID, userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]string{"status": "submitted"})
}

type exportAvailability struct {
	Weekday int    `json:"weekday"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

type exportPhoto struct {
	PhotoID      string    `json:"photo_id"`
	DisplayOrder int       `json:"display_order"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type exportReaction struct {
	DisplayName string    `json:"display_name"`
	Action      string    `json:"action"`
	CreatedAt   time.Time `json:"created_at"`
}

type exportMatch struct {
	MatchID      string    `json:"match_id"`
	DisplayName  string    `json:"display_name"`
	MatchedAt    time.Time `json:"matched_at"`
	HasInstagram bool      `json:"has_instagram"`
}

type exportReport struct {
	ReportID    string    `json:"report_id"`
	DisplayName string    `json:"display_name"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Export serves GET /api/v1/account/export as an authenticated JSON download.
func (s *Service) Export(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()

	export := map[string]any{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
	}
	var profile map[string]any
	var displayName, gender, skill, bio, preference string
	var ranking, instagram sql.NullString
	var minAge, maxAge, tolerance int
	err = tx.QueryRowContext(ctx, `
		SELECT display_name, gender, skill_level, ranking, bio, gender_preference,
			min_age, max_age, skill_tolerance, instagram_handle
		FROM profiles WHERE user_id = $1`, userID).Scan(&displayName, &gender, &skill, &ranking,
		&bio, &preference, &minAge, &maxAge, &tolerance, &instagram)
	if err == nil {
		profile = map[string]any{
			"display_name": displayName, "gender": gender, "skill_level": skill,
			"bio": bio, "gender_preference": preference, "min_age": minAge,
			"max_age": maxAge, "skill_tolerance": tolerance,
		}
		if ranking.Valid {
			profile["ranking"] = ranking.String
		}
		if instagram.Valid {
			profile["instagram_handle"] = instagram.String
		}
		export["profile"] = profile
	} else if !errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}

	availability := make([]exportAvailability, 0)
	rows, err := tx.QueryContext(ctx, `
		SELECT weekday, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI')
		FROM availability WHERE profile_id = $1 ORDER BY weekday, start_time`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	for rows.Next() {
		var slot exportAvailability
		if err := rows.Scan(&slot.Weekday, &slot.Start, &slot.End); err != nil {
			rows.Close()
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		availability = append(availability, slot)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	export["availability"] = availability

	photos := make([]exportPhoto, 0)
	rows, err = tx.QueryContext(ctx, `
		SELECT id, display_order, moderation_status, created_at
		FROM photos WHERE owner_id = $1 ORDER BY display_order`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	for rows.Next() {
		var photo exportPhoto
		if err := rows.Scan(&photo.PhotoID, &photo.DisplayOrder, &photo.Status, &photo.CreatedAt); err != nil {
			rows.Close()
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		photos = append(photos, photo)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	export["photos"] = photos

	reactions := make([]exportReaction, 0)
	rows, err = tx.QueryContext(ctx, `
		SELECT p.display_name, r.action, r.created_at
		FROM reactions r JOIN profiles p ON p.user_id = r.target_id
		WHERE r.actor_id = $1 ORDER BY r.created_at`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	for rows.Next() {
		var reaction exportReaction
		if err := rows.Scan(&reaction.DisplayName, &reaction.Action, &reaction.CreatedAt); err != nil {
			rows.Close()
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		reactions = append(reactions, reaction)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	export["reactions"] = reactions

	matches := make([]exportMatch, 0)
	rows, err = tx.QueryContext(ctx, `
		SELECT m.id, p.display_name, m.matched_at, p.instagram_handle IS NOT NULL
		FROM matches m
		JOIN profiles p ON p.user_id = CASE WHEN m.user_low = $1 THEN m.user_high ELSE m.user_low END
		WHERE m.user_low = $1 OR m.user_high = $1
		ORDER BY m.matched_at`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	for rows.Next() {
		var match exportMatch
		if err := rows.Scan(&match.MatchID, &match.DisplayName, &match.MatchedAt, &match.HasInstagram); err != nil {
			rows.Close()
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		matches = append(matches, match)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	export["matches"] = matches

	reports := make([]exportReport, 0)
	rows, err = tx.QueryContext(ctx, `
		SELECT r.id, p.display_name, r.category, r.status, r.created_at
		FROM reports r JOIN profiles p ON p.user_id = r.target_id
		WHERE r.reporter_id = $1 ORDER BY r.created_at`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	for rows.Next() {
		var report exportReport
		if err := rows.Scan(&report.ReportID, &report.DisplayName, &report.Category, &report.Status, &report.CreatedAt); err != nil {
			rows.Close()
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		reports = append(reports, report)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	export["reports_made"] = reports

	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="tennis-connect-data.json"`)
	httpapi.WriteJSON(w, http.StatusOK, export)
}

func isUniqueViolation(err error) bool {
	var sqlState interface{ SQLState() string }
	return errors.As(err, &sqlState) && sqlState.SQLState() == "23505"
}
