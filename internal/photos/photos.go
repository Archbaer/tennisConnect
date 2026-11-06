package photos

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"tennisconnect/internal/httpapi"
	"tennisconnect/internal/moderation"
)

const (
	// MaxPhotoBytes is the upload and object size cap.
	MaxPhotoBytes     = 5 << 20
	maxPhotoPixels    = 20_000_000
	maxPhotoDimension = 1600
)

const (
	uploadURLExpiry   = 5 * time.Minute
	staleAfter        = time.Hour
	uploadAttempts    = 3
	moderationTimeout = 5 * time.Second
)

var (
	// ErrObjectMissing reports a photo object absent from storage.
	ErrObjectMissing = errors.New("photo object missing")
	// ErrObjectTooLarge reports a photo object above MaxPhotoBytes.
	ErrObjectTooLarge = errors.New("photo object too large")

	errInvalidImage       = errors.New("invalid image")
	errPhotoLimit         = errors.New("photo limit reached")
	errPhotoNotFound      = errors.New("photo not found")
	errPhotoNotPending    = errors.New("photo is not pending")
	errPhotoUploadExpired = errors.New("photo upload expired")
)

// Storage is the object store used for photo bytes.
type Storage interface {
	PresignPut(ctx context.Context, key string, expires time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, expires time.Duration) (string, error)
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
}

// Moderator checks sanitized photo bytes before they are published.
type Moderator interface {
	Moderate(ctx context.Context, image []byte) (moderation.Verdict, error)
}

type validatedImage struct {
	data []byte
}

type photoRecord struct {
	ID                string
	OwnerID           string
	ObjectKey         string
	DisplayOrder      int
	ModerationStatus  moderation.Status
	ProviderReference sql.NullString
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func newPhotoObjectKey() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("could not generate photo key")
	}
	return "photos/" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func validateImage(data []byte, declaredType string) (validatedImage, error) {
	if len(data) == 0 || len(data) > MaxPhotoBytes {
		return validatedImage{}, errInvalidImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") || declaredType != "image/"+format {
		return validatedImage{}, errInvalidImage
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxPhotoPixels/config.Height {
		return validatedImage{}, errInvalidImage
	}

	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return validatedImage{}, errInvalidImage
	}
	if format == "jpeg" {
		source = applyPhotoOrientation(source, jpegEXIFOrientation(data))
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > maxPhotoDimension || height > maxPhotoDimension {
		resizedWidth, resizedHeight := maxPhotoDimension, height*maxPhotoDimension/width
		if height > width {
			resizedWidth, resizedHeight = width*maxPhotoDimension/height, maxPhotoDimension
		}
		if resizedWidth < 1 {
			resizedWidth = 1
		}
		if resizedHeight < 1 {
			resizedHeight = 1
		}
		resized := image.NewRGBA(image.Rect(0, 0, resizedWidth, resizedHeight))
		xdraw.ApproxBiLinear.Scale(resized, resized.Bounds(), source, bounds, xdraw.Src, nil)
		source = resized
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, source, &jpeg.Options{Quality: 85}); err != nil {
		return validatedImage{}, errors.New("could not encode image")
	}
	return validatedImage{data: output.Bytes()}, nil
}

func jpegEXIFOrientation(data []byte) int {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset < len(data); {
		if data[offset] != 0xff {
			return 1
		}
		for offset < len(data) && data[offset] == 0xff {
			offset++
		}
		if offset >= len(data) {
			return 1
		}
		marker := data[offset]
		offset++
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if offset+2 > len(data) {
			return 1
		}
		segmentLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if segmentLength < 2 || offset+segmentLength > len(data) {
			return 1
		}
		payload := data[offset+2 : offset+segmentLength]
		if marker == 0xe1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			return parseEXIFOrientation(payload[6:])
		}
		offset += segmentLength
	}
	return 1
}

func parseEXIFOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 1
	}
	ifdOffset := uint64(order.Uint32(tiff[4:8]))
	if ifdOffset+2 > uint64(len(tiff)) {
		return 1
	}
	entryCount := uint64(order.Uint16(tiff[int(ifdOffset) : int(ifdOffset)+2]))
	if ifdOffset+2+entryCount*12 > uint64(len(tiff)) {
		return 1
	}
	for i := uint64(0); i < entryCount; i++ {
		entry := tiff[int(ifdOffset+2+i*12):]
		if order.Uint16(entry[:2]) != 0x0112 || order.Uint16(entry[2:4]) != 3 || order.Uint32(entry[4:8]) != 1 {
			continue
		}
		orientation := int(order.Uint16(entry[8:10]))
		if orientation >= 1 && orientation <= 8 {
			return orientation
		}
		return 1
	}
	return 1
}

func applyPhotoOrientation(source image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return source
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	outputWidth, outputHeight := width, height
	if orientation >= 5 {
		outputWidth, outputHeight = height, width
	}
	output := image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	for y := 0; y < outputHeight; y++ {
		for x := 0; x < outputWidth; x++ {
			sourceX, sourceY := x, y
			switch orientation {
			case 2:
				sourceX = width - 1 - x
			case 3:
				sourceX, sourceY = width-1-x, height-1-y
			case 4:
				sourceY = height - 1 - y
			case 5:
				sourceX, sourceY = y, x
			case 6:
				sourceX, sourceY = y, height-1-x
			case 7:
				sourceX, sourceY = width-1-y, height-1-x
			case 8:
				sourceX, sourceY = width-1-y, x
			}
			output.Set(x, y, source.At(bounds.Min.X+sourceX, bounds.Min.Y+sourceY))
		}
	}
	return output
}

type Service struct {
	DB         *sql.DB
	AppBaseURL string
	Storage    Storage
	Moderator  Moderator
}

type photoUploadResponse struct {
	PhotoID          string `json:"photo_id"`
	UploadURL        string `json:"upload_url"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type photoProcessResponse struct {
	PhotoID string            `json:"photo_id"`
	Status  moderation.Status `json:"status"`
}

func validPhotoID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for index, character := range id {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		hex := (character >= '0' && character <= '9') ||
			(character >= 'a' && character <= 'f') ||
			(character >= 'A' && character <= 'F')
		if !hex {
			return false
		}
	}
	return true
}

func (s *Service) UploadURL(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.Storage == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "photo uploads unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	record, err := s.insertPendingPhoto(ctx, userID)
	if err != nil {
		if errors.Is(err, errPhotoLimit) {
			httpapi.WriteError(w, http.StatusConflict, "photo limit reached")
			return
		}
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	uploadURL, err := s.Storage.PresignPut(ctx, record.ObjectKey, uploadURLExpiry)
	if err != nil {
		s.deletePhotoRowBestEffort(ctx, record.ID, userID, record.ObjectKey)
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, photoUploadResponse{
		PhotoID:          record.ID,
		UploadURL:        uploadURL,
		ExpiresInSeconds: int(uploadURLExpiry.Seconds()),
	})
}

func (s *Service) Process(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if !httpapi.OriginAllowed(r, s.AppBaseURL) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.Storage == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "photo uploads unavailable")
		return
	}
	if s.Moderator == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "photo checks unavailable")
		return
	}
	photoID := r.PathValue("id")
	if !validPhotoID(photoID) {
		httpapi.WriteError(w, http.StatusNotFound, "photo not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	originalKey, err := claimPendingPhoto(ctx, s.DB, photoID, userID)
	if err != nil {
		switch {
		case errors.Is(err, errPhotoNotFound):
			httpapi.WriteError(w, http.StatusNotFound, "photo not found")
		case errors.Is(err, errPhotoNotPending), errors.Is(err, errPhotoUploadExpired):
			httpapi.WriteError(w, http.StatusConflict, "photo cannot be processed")
		default:
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		}
		return
	}

	data, getErr := s.Storage.Get(ctx, originalKey)
	if getErr != nil && !errors.Is(getErr, ErrObjectMissing) && !errors.Is(getErr, ErrObjectTooLarge) {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	var sanitized []byte
	if getErr == nil {
		if validated, validErr := validateImage(data, http.DetectContentType(data)); validErr == nil {
			sanitized = validated.data
		}
	}
	if sanitized == nil {
		s.discardInvalidPhoto(ctx, w, photoID, userID, originalKey)
		return
	}

	newKey, err := newPhotoObjectKey()
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if err := s.Storage.Put(ctx, newKey, sanitized); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	swapped, err := swapPhotoObjectKey(ctx, s.DB, photoID, userID, originalKey, newKey)
	if err != nil {
		s.deleteObjectBestEffort(ctx, newKey)
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !swapped {
		s.deleteObjectBestEffort(ctx, newKey)
		httpapi.WriteError(w, http.StatusConflict, "photo already processed")
		return
	}
	s.deleteObjectBestEffort(ctx, originalKey)

	moderationCtx, stopModeration := context.WithTimeout(ctx, moderationTimeout)
	verdict, moderationErr := s.Moderator.Moderate(moderationCtx, sanitized)
	stopModeration()
	if moderationErr != nil {
		// The sanitized object stays addressable so the owner can retry.
		httpapi.WriteError(w, http.StatusServiceUnavailable, "photo checks unavailable")
		return
	}
	if verdict.Status == moderation.StatusPending {
		httpapi.WriteJSON(w, http.StatusOK, photoProcessResponse{PhotoID: photoID, Status: moderation.StatusPending})
		return
	}
	applied, applyErr := applyModeration(ctx, s.DB, photoID, userID, newKey, verdict)
	if applyErr != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !applied {
		s.deleteObjectBestEffort(ctx, newKey)
		httpapi.WriteError(w, http.StatusConflict, "photo already processed")
		return
	}
	if verdict.Status == moderation.StatusRejected {
		s.deleteObjectBestEffort(ctx, newKey)
	}
	httpapi.WriteJSON(w, http.StatusOK, photoProcessResponse{PhotoID: photoID, Status: verdict.Status})
}

func (s *Service) discardInvalidPhoto(ctx context.Context, w http.ResponseWriter, photoID, userID, objectKey string) {
	deleted, err := deletePhotoRowCAS(ctx, s.DB, photoID, userID, objectKey)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if !deleted {
		httpapi.WriteError(w, http.StatusConflict, "photo already processed")
		return
	}
	s.deleteObjectBestEffort(ctx, objectKey)
	httpapi.WriteError(w, http.StatusBadRequest, "invalid photo")
}

func applyModeration(ctx context.Context, db *sql.DB, photoID, ownerID, objectKey string, verdict moderation.Verdict) (bool, error) {
	var reference sql.NullString
	if verdict.Reference != "" {
		reference = sql.NullString{String: verdict.Reference, Valid: true}
	}
	result, err := db.ExecContext(ctx, `
		UPDATE photos SET moderation_status = $1, provider_reference = $2, updated_at = now()
		WHERE id = $3 AND owner_id = $4 AND object_key = $5 AND moderation_status = 'pending'`,
		string(verdict.Status), reference, photoID, ownerID, objectKey)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func photoApprovedForDiscovery(ctx context.Context, db *sql.DB, ownerID string) (bool, error) {
	var approved bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM photos WHERE owner_id = $1 AND moderation_status = 'approved')`,
		ownerID).Scan(&approved)
	return approved, err
}

type photoURLResponse struct {
	PhotoID          string            `json:"photo_id"`
	Status           moderation.Status `json:"status"`
	URL              string            `json:"url"`
	ExpiresInSeconds int               `json:"expires_in_seconds"`
}

type photoListItem struct {
	PhotoID      string            `json:"photo_id"`
	DisplayOrder int               `json:"display_order"`
	Status       moderation.Status `json:"status"`
}

type photoListResponse struct {
	Photos []photoListItem `json:"photos"`
}

func (s *Service) URL(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	if s.Storage == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "photo uploads unavailable")
		return
	}
	photoID := r.PathValue("id")
	if !validPhotoID(photoID) {
		httpapi.WriteError(w, http.StatusNotFound, "photo not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var status moderation.Status
	var objectKey string
	err := s.DB.QueryRowContext(ctx, `
		SELECT moderation_status, object_key FROM photos WHERE id = $1 AND owner_id = $2`, photoID, userID).
		Scan(&status, &objectKey)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteError(w, http.StatusNotFound, "photo not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	if status != moderation.StatusPending && status != moderation.StatusApproved {
		httpapi.WriteError(w, http.StatusNotFound, "photo not found")
		return
	}
	ownerURL, err := s.Storage.PresignGet(ctx, objectKey, uploadURLExpiry)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, photoURLResponse{
		PhotoID:          photoID,
		Status:           status,
		URL:              ownerURL,
		ExpiresInSeconds: int(uploadURLExpiry.Seconds()),
	})
}

func (s *Service) List(w http.ResponseWriter, r *http.Request, userID string) {
	httpapi.PreventCaching(w)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, display_order, moderation_status FROM photos
		WHERE owner_id = $1 ORDER BY display_order`, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	defer rows.Close()
	items := []photoListItem{}
	for rows.Next() {
		var item photoListItem
		if err := rows.Scan(&item.PhotoID, &item.DisplayOrder, &item.Status); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "temporarily unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, photoListResponse{Photos: items})
}

func (s *Service) insertPendingPhoto(ctx context.Context, ownerID string) (photoRecord, error) {
	cleaned := false
	for attempt := 0; attempt < uploadAttempts; attempt++ {
		key, err := newPhotoObjectKey()
		if err != nil {
			return photoRecord{}, err
		}
		record, err := insertPhotoWithKey(ctx, s.DB, ownerID, key)
		if err == nil {
			return record, nil
		}
		if errors.Is(err, errPhotoLimit) {
			if cleaned {
				return photoRecord{}, errPhotoLimit
			}
			cleaned = true
			staleKeys, cleanupErr := cleanupStalePending(ctx, s.DB, ownerID)
			if cleanupErr != nil {
				return photoRecord{}, cleanupErr
			}
			for _, staleKey := range staleKeys {
				s.deleteObjectBestEffort(ctx, staleKey)
			}
			if len(staleKeys) > 0 {
				continue
			}
			removed, rejectedErr := deleteRejectedPhotos(ctx, s.DB, ownerID)
			if rejectedErr != nil {
				return photoRecord{}, rejectedErr
			}
			if removed == 0 {
				return photoRecord{}, errPhotoLimit
			}
			continue
		}
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" {
			continue
		}
		return photoRecord{}, err
	}
	return photoRecord{}, errPhotoLimit
}

func (s *Service) deleteObjectBestEffort(ctx context.Context, key string) {
	if err := s.Storage.Delete(ctx, key); err != nil {
		log.Print("photo storage cleanup failed")
	}
}

func (s *Service) deletePhotoRowBestEffort(ctx context.Context, photoID, ownerID, objectKey string) {
	if _, err := deletePhotoRowCAS(ctx, s.DB, photoID, ownerID, objectKey); err != nil {
		log.Print("photo row cleanup failed")
	}
}

func insertPhotoWithKey(ctx context.Context, db *sql.DB, ownerID, objectKey string) (photoRecord, error) {
	var record photoRecord
	err := db.QueryRowContext(ctx, `
		INSERT INTO photos (owner_id, object_key, display_order)
		SELECT $1::uuid, $2, slot
		FROM generate_series(1, 3) AS slot
		WHERE NOT EXISTS (
			SELECT 1 FROM photos WHERE owner_id = $1::uuid AND display_order = slot
		)
		ORDER BY slot
		LIMIT 1
		RETURNING id, owner_id, object_key, display_order, moderation_status,
			provider_reference, created_at, updated_at`, ownerID, objectKey).Scan(
		&record.ID, &record.OwnerID, &record.ObjectKey, &record.DisplayOrder,
		&record.ModerationStatus, &record.ProviderReference, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return photoRecord{}, errPhotoLimit
	}
	return record, err
}

func cleanupStalePending(ctx context.Context, db *sql.DB, ownerID string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		DELETE FROM photos
		WHERE owner_id = $1 AND moderation_status = 'pending'
			AND updated_at < now() - make_interval(secs => $2)
		RETURNING object_key`, ownerID, staleAfter.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func deleteRejectedPhotos(ctx context.Context, db *sql.DB, ownerID string) (int, error) {
	result, err := db.ExecContext(ctx, `DELETE FROM photos WHERE owner_id = $1 AND moderation_status = 'rejected'`, ownerID)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func claimPendingPhoto(ctx context.Context, db *sql.DB, photoID, ownerID string) (string, error) {
	var key string
	err := db.QueryRowContext(ctx, `
		UPDATE photos SET updated_at = now()
		WHERE id = $1 AND owner_id = $2 AND moderation_status = 'pending'
			AND updated_at > now() - make_interval(secs => $3)
		RETURNING object_key`, photoID, ownerID, staleAfter.Seconds()).Scan(&key)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "22P02" {
			return "", errPhotoNotFound
		}
		return "", err
	}

	var status moderation.Status
	var updatedAt time.Time
	err = db.QueryRowContext(ctx, `
		SELECT moderation_status, updated_at FROM photos WHERE id = $1 AND owner_id = $2`, photoID, ownerID).
		Scan(&status, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errPhotoNotFound
	}
	if err != nil {
		return "", err
	}
	if status != moderation.StatusPending {
		return "", errPhotoNotPending
	}
	return "", errPhotoUploadExpired
}

func swapPhotoObjectKey(ctx context.Context, db *sql.DB, photoID, ownerID, originalKey, newKey string) (bool, error) {
	result, err := db.ExecContext(ctx, `
		UPDATE photos SET object_key = $1, updated_at = now()
		WHERE id = $2 AND owner_id = $3 AND object_key = $4 AND moderation_status = 'pending'`,
		newKey, photoID, ownerID, originalKey)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func deletePhotoRowCAS(ctx context.Context, db *sql.DB, photoID, ownerID, objectKey string) (bool, error) {
	result, err := db.ExecContext(ctx, `
		DELETE FROM photos WHERE id = $1 AND owner_id = $2 AND object_key = $3`,
		photoID, ownerID, objectKey)
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "22P02" {
			return false, nil
		}
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}
