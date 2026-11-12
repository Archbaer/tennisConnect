// Package safety accepts player reports and serves the reviewer queue that
// dismisses reports or permanently bans the reported account.
package safety

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tennisconnect/internal/auth"
	"tennisconnect/internal/httpapi"
)

const (
	requestBudget = 5 * time.Second
	reportLimit   = 5
)

var categories = map[string]struct{}{
	"fake_profile":         {},
	"inappropriate_photos": {},
	"harassment":           {},
	"contact_details":      {},
	"underage":             {},
	"other":                {},
}

// Service serves report submission and reviewer decisions.
type Service struct {
	DB         *sql.DB
	AppBaseURL string
	HMACKey    []byte
}

type reportInput struct {
	TargetID string `json:"target_id"`
	Category string `json:"category"`
	Details  string `json:"details"`
}

type reportItem struct {
	ReportID    string     `json:"report_id"`
	TargetID    string     `json:"target_id"`
	DisplayName string     `json:"display_name"`
	Category    string     `json:"category"`
	Details     string     `json:"details"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ReviewDueAt time.Time  `json:"review_due_at"`
	Overdue     bool       `json:"overdue"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
}

type reportList struct {
	Reports []reportItem `json:"reports"`
}

// Report serves POST /api/v1/reports.
func (s *Service) Report(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input reportInput
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input.TargetID = strings.ToLower(input.TargetID)
	input.Category = strings.TrimSpace(input.Category)
	input.Details = strings.TrimSpace(input.Details)
	if !validUUID(input.TargetID) || input.TargetID == userID {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid target")
		return
	}
	if _, ok := categories[input.Category]; !ok {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid category")
		return
	}
	if utf8.RuneCountInString(input.Details) > 500 || strings.ContainsRune(input.Details, '\x00') {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid details")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	if !s.allowReport(ctx, w, userID) {
		return
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()

	var targetExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE id = $1 AND deleted_at IS NULL AND banned_at IS NULL
		)`, input.TargetID).Scan(&targetExists); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !targetExists {
		httpapi.WriteError(w, http.StatusNotFound, "player not found")
		return
	}
	var reportID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO reports (reporter_id, target_id, category, details)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, userID, input.TargetID, input.Category, input.Details).Scan(&reportID)
	if err != nil {
		if isUniqueViolation(err) {
			httpapi.WriteError(w, http.StatusConflict, "already reported")
			return
		}
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO report_events (report_id, actor_kind, actor_id, action)
		VALUES ($1, 'user', $2, 'submitted')`, reportID, userID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]string{"report_id": reportID, "status": "open"})
}

func (s *Service) allowReport(ctx context.Context, w http.ResponseWriter, userID string) bool {
	now := time.Now().UTC()
	windowStart := now.Truncate(time.Hour)
	var attempts int
	key := auth.EmailLookupHMAC("report-rate-limit\x00"+userID, s.HMACKey)
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO safety_rate_limits (key_hash, window_start, attempts)
		VALUES ($1, $2, 1)
		ON CONFLICT (key_hash, window_start)
		DO UPDATE SET attempts = safety_rate_limits.attempts + 1
		RETURNING attempts`, key, windowStart).Scan(&attempts)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return false
	}
	if attempts > reportLimit {
		retry := int(windowStart.Add(time.Hour).Sub(now).Seconds())
		if retry < 1 {
			retry = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		httpapi.WriteError(w, http.StatusTooManyRequests, "too many requests")
		return false
	}
	return true
}

// List serves GET /api/v1/reviewer/reports. Only reviewers reach it.
func (s *Service) List(w http.ResponseWriter, r *http.Request, reviewerID string) {
	httpapi.PreventCaching(w)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}
	if status != "open" && status != "dismissed" && status != "banned" && status != "all" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid status")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	query := `
		SELECT r.id, r.target_id, p.display_name, r.category, r.details, r.status,
			r.created_at, r.review_due_at, r.reviewed_at
		FROM reports r
		JOIN profiles p ON p.user_id = r.target_id
		WHERE ($1 = 'all' OR r.status = $1)
		ORDER BY r.review_due_at, r.created_at, r.id
		LIMIT 100`
	rows, err := s.DB.QueryContext(ctx, query, status)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer rows.Close()
	items := make([]reportItem, 0)
	for rows.Next() {
		var item reportItem
		var reviewed sql.NullTime
		if err := rows.Scan(&item.ReportID, &item.TargetID, &item.DisplayName, &item.Category,
			&item.Details, &item.Status, &item.CreatedAt, &item.ReviewDueAt, &reviewed); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		if reviewed.Valid {
			item.ReviewedAt = &reviewed.Time
		}
		item.Overdue = item.Status == "open" && item.ReviewDueAt.Before(time.Now())
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, reportList{Reports: items})
}

// Detail serves GET /api/v1/reviewer/reports/{id}.
func (s *Service) Detail(w http.ResponseWriter, r *http.Request, reviewerID string) {
	httpapi.PreventCaching(w)
	reportID := r.PathValue("id")
	if !validUUID(reportID) {
		httpapi.WriteError(w, http.StatusNotFound, "report not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	var item reportItem
	var reviewed sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT r.id, r.target_id, p.display_name, r.category, r.details, r.status,
			r.created_at, r.review_due_at, r.reviewed_at
		FROM reports r
		JOIN profiles p ON p.user_id = r.target_id
		WHERE r.id = $1`, reportID).Scan(&item.ReportID, &item.TargetID, &item.DisplayName,
		&item.Category, &item.Details, &item.Status, &item.CreatedAt, &item.ReviewDueAt, &reviewed)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "report not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if reviewed.Valid {
		item.ReviewedAt = &reviewed.Time
	}
	item.Overdue = item.Status == "open" && item.ReviewDueAt.Before(time.Now())
	httpapi.WriteJSON(w, http.StatusOK, item)
}

// Dismiss serves POST /api/v1/reviewer/reports/{id}/dismiss.
func (s *Service) Dismiss(w http.ResponseWriter, r *http.Request, reviewerID string) {
	s.decide(w, r, reviewerID, false)
}

// Ban serves POST /api/v1/reviewer/reports/{id}/ban.
func (s *Service) Ban(w http.ResponseWriter, r *http.Request, reviewerID string) {
	s.decide(w, r, reviewerID, true)
}

func (s *Service) decide(w http.ResponseWriter, r *http.Request, reviewerID string, ban bool) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	reportID := r.PathValue("id")
	if !validUUID(reportID) {
		httpapi.WriteError(w, http.StatusNotFound, "report not found")
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

	var targetID, category, status string
	err = tx.QueryRowContext(ctx, `
		SELECT target_id, category, status FROM reports WHERE id = $1 FOR UPDATE`,
		reportID).Scan(&targetID, &category, &status)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "report not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if status != "open" {
		httpapi.WriteError(w, http.StatusConflict, "report already reviewed")
		return
	}
	newStatus := "dismissed"
	action := "dismissed"
	if ban {
		newStatus = "banned"
		action = "banned"
		if _, err := tx.ExecContext(ctx, `
			UPDATE users SET
				banned_at = COALESCE(banned_at, now()),
				ban_category = COALESCE(ban_category, $2),
				ban_report_id = COALESCE(ban_report_id, $1)
			WHERE id = $3`, reportID, category, targetID); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", targetID); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE reports SET status = $1, reviewer_id = $2, reviewed_at = now()
		WHERE id = $3`, newStatus, reviewerID, reportID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO report_events (report_id, actor_kind, actor_id, action)
		VALUES ($1, 'reviewer', $2, $3)`, reportID, reviewerID, action); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": newStatus})
}

// CleanupExpired removes old report rate-limit windows.
func (s *Service) CleanupExpired(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx,
		"DELETE FROM safety_rate_limits WHERE window_start < now() - interval '2 hours'")
	return err
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, ch := range value {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
				return false
			}
		}
	}
	return true
}

func isUniqueViolation(err error) bool {
	var sqlState interface{ SQLState() string }
	return errors.As(err, &sqlState) && sqlState.SQLState() == "23505"
}
