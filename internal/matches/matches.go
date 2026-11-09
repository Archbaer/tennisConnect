// Package matches records one-way reactions, promotes reciprocal likes into a
// single match per unordered pair, and enforces blocks. It is the only place
// where one player's Instagram handle is disclosed to another.
package matches

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"tennisconnect/internal/httpapi"
)

const (
	requestBudget = 5 * time.Second
	photoLimit    = 3
	photoExpiry   = 5 * time.Minute
)

// pgUniqueViolation is the SQLSTATE for a duplicate key.
const pgUniqueViolation = "23505"

// Presigner issues a short-lived URL for one approved photo object.
type Presigner interface {
	PresignGet(ctx context.Context, key string, duration time.Duration) (string, error)
}

// Service serves reaction, match and block endpoints.
type Service struct {
	DB         *sql.DB
	AppBaseURL string
	Presigner  Presigner
}

type reactionInput struct {
	TargetID string `json:"target_id"`
	Action   string `json:"action"`
}

type reactionResponse struct {
	Matched bool    `json:"matched"`
	MatchID *string `json:"match_id,omitempty"`
}

type blockInput struct {
	TargetID string `json:"target_id"`
}

type matchRow struct {
	MatchID      string
	OtherID      string
	DisplayName  string
	MatchedAt    time.Time
	HasInstagram bool
}

type matchListRow struct {
	MatchID      string    `json:"match_id"`
	DisplayName  string    `json:"display_name"`
	MatchedAt    time.Time `json:"matched_at"`
	HasInstagram bool      `json:"has_instagram"`
	PhotoURLs    []string  `json:"photo_urls"`
}

type matchListResponse struct {
	Matches []matchListRow `json:"matches"`
}

type matchDetailResponse struct {
	MatchID      string    `json:"match_id"`
	DisplayName  string    `json:"display_name"`
	MatchedAt    time.Time `json:"matched_at"`
	HasInstagram bool      `json:"has_instagram"`
	PhotoURLs    []string  `json:"photo_urls"`
}

type instagramResponse struct {
	InstagramHandle string `json:"instagram_handle"`
}

// CreateReaction serves POST /api/v1/reactions.
func (s *Service) CreateReaction(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input reactionInput
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	// Normalise before any comparison or write: uuid ordering in Go is a string
	// compare, so a mixed-case client value would sort differently from its
	// stored lower-case form and trip the canonical-pair check.
	input.TargetID = strings.ToLower(input.TargetID)
	if !validUUID(input.TargetID) || input.TargetID == userID {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid target")
		return
	}
	if input.Action != "like" && input.Action != "pass" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid action")
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

	// Serialise the two sides of one pair before reading anything. Without this
	// each concurrent reciprocal like reads the other's reaction as missing
	// (repeatable read is per statement, and both transactions are uncommitted)
	// and neither creates the match. The lock is transaction-scoped and keyed
	// only on the canonical pair, so unrelated pairs never contend.
	if _, err := tx.ExecContext(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", pairKey(userID, input.TargetID)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}

	var blocked bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM blocks
			WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)
		)`, userID, input.TargetID).Scan(&blocked); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if blocked {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	var targetExists bool
	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)",
		input.TargetID).Scan(&targetExists); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !targetExists {
		httpapi.WriteError(w, http.StatusNotFound, "player not found")
		return
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO reactions (actor_id, target_id, action) VALUES ($1, $2, $3)`,
		userID, input.TargetID, input.Action)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		httpapi.WriteError(w, http.StatusConflict, "already replied")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}

	response := reactionResponse{}
	if input.Action == "like" {
		matchID, matched, err := createMatchIfReciprocal(ctx, tx, userID, input.TargetID)
		if err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		if matched {
			response.Matched = true
			response.MatchID = &matchID
		}
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

// pairKey returns the canonical, order-independent key for one unordered pair.
// Both actors must derive the same key so the advisory lock serialises them.
func pairKey(first, second string) string {
	low, high := first, second
	if low > high {
		low, high = high, low
	}
	return low + ":" + high
}

// createMatchIfReciprocal inserts the canonical pair when the target already
// likes the actor. The unique constraint makes the second concurrent reciprocal
// like a no-op instead of a duplicate match.
func createMatchIfReciprocal(ctx context.Context, tx *sql.Tx, actorID, targetID string) (string, bool, error) {
	var reciprocal bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM reactions WHERE actor_id = $1 AND target_id = $2 AND action = 'like')`,
		targetID, actorID).Scan(&reciprocal); err != nil {
		return "", false, err
	}
	if !reciprocal {
		return "", false, nil
	}
	low, high := actorID, targetID
	if low > high {
		low, high = high, low
	}
	var matchID string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO matches (user_low, user_high) VALUES ($1, $2)
		ON CONFLICT (user_low, user_high) DO UPDATE SET user_low = EXCLUDED.user_low
		RETURNING id`, low, high).Scan(&matchID)
	if err != nil {
		return "", false, err
	}
	return matchID, true, nil
}

// List serves GET /api/v1/matches.
func (s *Service) List(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, liveMatchesQuery, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	matches, err := scanMatches(rows)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.OtherID)
	}
	photos, err := loadApprovedPhotos(ctx, tx, ids)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}

	urls := s.signPhotos(ctx, photos)
	items := make([]matchListRow, 0, len(matches))
	for _, match := range matches {
		photoURLs := urls[match.OtherID]
		if photoURLs == nil {
			photoURLs = []string{}
		}
		items = append(items, matchListRow{
			MatchID:      match.MatchID,
			DisplayName:  match.DisplayName,
			MatchedAt:    match.MatchedAt,
			HasInstagram: match.HasInstagram,
			PhotoURLs:    photoURLs,
		})
	}
	httpapi.WriteJSON(w, http.StatusOK, matchListResponse{Matches: items})
}

// Detail serves GET /api/v1/matches/{id}. It never includes the handle.
func (s *Service) Detail(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	matchID := r.PathValue("id")
	if !validUUID(matchID) {
		httpapi.WriteError(w, http.StatusNotFound, "match not found")
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

	match, found, err := loadMatch(ctx, tx, userID, matchID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !found {
		httpapi.WriteError(w, http.StatusNotFound, "match not found")
		return
	}
	photos, err := loadApprovedPhotos(ctx, tx, []string{match.OtherID})
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	photoURLs := s.signPhotos(ctx, photos)[match.OtherID]
	if photoURLs == nil {
		photoURLs = []string{}
	}
	httpapi.WriteJSON(w, http.StatusOK, matchDetailResponse{
		MatchID:      match.MatchID,
		DisplayName:  match.DisplayName,
		MatchedAt:    match.MatchedAt,
		HasInstagram: match.HasInstagram,
		PhotoURLs:    photoURLs,
	})
}

// Instagram serves GET /api/v1/matches/{id}/instagram. The handle is fetched on
// demand and is only reachable while an unblocked match exists.
func (s *Service) Instagram(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	matchID := r.PathValue("id")
	if !validUUID(matchID) {
		httpapi.WriteError(w, http.StatusNotFound, "match not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	var handle sql.NullString
	err := s.DB.QueryRowContext(ctx, `
		SELECT p.instagram_handle
		FROM matches m
		JOIN profiles p ON p.user_id = CASE WHEN m.user_low = $2 THEN m.user_high ELSE m.user_low END
		JOIN users u ON u.id = p.user_id AND u.deleted_at IS NULL
		WHERE m.id = $1
			AND (m.user_low = $2 OR m.user_high = $2)
			AND NOT EXISTS (
				SELECT 1 FROM blocks
				WHERE (blocker_id = m.user_low AND blocked_id = m.user_high)
					OR (blocker_id = m.user_high AND blocked_id = m.user_low)
			)`, matchID, userID).Scan(&handle)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "match not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, instagramResponse{InstagramHandle: handle.String})
}

// Block serves POST /api/v1/blocks. The block hides the pair in both
// directions, closes any match, and revokes handle and photo access.
func (s *Service) Block(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	var input blockInput
	if !httpapi.DecodeJSON(w, r, &input) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input.TargetID = strings.ToLower(input.TargetID)
	if !validUUID(input.TargetID) || input.TargetID == userID {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid target")
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

	// Blocker 2: take the same pair lock the reaction path takes, so a reaction
	// already in flight cannot read the old block state and then create a match
	// after this block commits.
	if _, err := tx.ExecContext(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", pairKey(userID, input.TargetID)); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}

	var targetExists bool
	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)",
		input.TargetID).Scan(&targetExists); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !targetExists {
		httpapi.WriteError(w, http.StatusNotFound, "player not found")
		return
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, input.TargetID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	// Close the match by deleting it, not just by filtering reads. Under the
	// pair lock above this also covers the reverse interleaving, where a
	// reaction committed the match moments before the block committed.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM matches
		WHERE (user_low = $1 AND user_high = $2) OR (user_low = $2 AND user_high = $1)`,
		userID, input.TargetID); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const liveMatchesQuery = `
SELECT m.id, CASE WHEN m.user_low = $1 THEN m.user_high ELSE m.user_low END,
	p.display_name, m.matched_at, p.instagram_handle IS NOT NULL
FROM matches m
JOIN profiles p ON p.user_id = CASE WHEN m.user_low = $1 THEN m.user_high ELSE m.user_low END
JOIN users u ON u.id = p.user_id AND u.deleted_at IS NULL
WHERE (m.user_low = $1 OR m.user_high = $1)
	AND NOT EXISTS (
		SELECT 1 FROM blocks
		WHERE (blocker_id = m.user_low AND blocked_id = m.user_high)
			OR (blocker_id = m.user_high AND blocked_id = m.user_low)
	)
ORDER BY m.matched_at DESC, m.id`

const detailMatchQuery = `
SELECT m.id, CASE WHEN m.user_low = $2 THEN m.user_high ELSE m.user_low END,
	p.display_name, m.matched_at, p.instagram_handle IS NOT NULL
FROM matches m
JOIN profiles p ON p.user_id = CASE WHEN m.user_low = $2 THEN m.user_high ELSE m.user_low END
JOIN users u ON u.id = p.user_id AND u.deleted_at IS NULL
WHERE m.id = $1
	AND (m.user_low = $2 OR m.user_high = $2)
	AND NOT EXISTS (
		SELECT 1 FROM blocks
		WHERE (blocker_id = m.user_low AND blocked_id = m.user_high)
			OR (blocker_id = m.user_high AND blocked_id = m.user_low)
	)`

const approvedPhotoQuery = `
SELECT owner_id, object_key
FROM photos
WHERE owner_id = ANY($1::uuid[]) AND moderation_status = 'approved'
ORDER BY owner_id, display_order`

func loadMatch(ctx context.Context, tx *sql.Tx, userID, matchID string) (matchRow, bool, error) {
	var match matchRow
	err := tx.QueryRowContext(ctx, detailMatchQuery, matchID, userID).Scan(
		&match.MatchID, &match.OtherID, &match.DisplayName, &match.MatchedAt, &match.HasInstagram)
	if errors.Is(err, sql.ErrNoRows) {
		return matchRow{}, false, nil
	}
	if err != nil {
		return matchRow{}, false, err
	}
	return match, true, nil
}

func scanMatches(rows *sql.Rows) ([]matchRow, error) {
	defer rows.Close()
	matches := make([]matchRow, 0)
	for rows.Next() {
		var match matchRow
		if err := rows.Scan(&match.MatchID, &match.OtherID, &match.DisplayName,
			&match.MatchedAt, &match.HasInstagram); err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matches, rows.Err()
}

type photoRow struct {
	ownerID   string
	objectKey string
}

func loadApprovedPhotos(ctx context.Context, tx *sql.Tx, ids []string) ([]photoRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, approvedPhotoQuery, ids)
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

// signPhotos maps owner id to at most three signed URLs. A signing failure
// omits that URL and never fails the page.
func (s *Service) signPhotos(ctx context.Context, photos []photoRow) map[string][]string {
	urls := make(map[string][]string, len(photos))
	if s.Presigner == nil {
		return urls
	}
	for _, photo := range photos {
		if len(urls[photo.ownerID]) >= photoLimit {
			continue
		}
		signed, err := s.Presigner.PresignGet(ctx, photo.objectKey, photoExpiry)
		if err != nil {
			log.Printf("match photo URL unavailable")
			continue
		}
		urls[photo.ownerID] = append(urls[photo.ownerID], signed)
	}
	return urls
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
