package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestValidateImageAcceptsSupportedFormats(t *testing.T) {
	webpData, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		data         []byte
		declaredType string
		width        int
		height       int
	}{
		{name: "JPEG", data: photoTestJPEG(t, 4, 3), declaredType: "image/jpeg", width: 4, height: 3},
		{name: "PNG", data: photoTestPNG(t, 4, 3), declaredType: "image/png", width: 4, height: 3},
		{name: "WebP", data: webpData, declaredType: "image/webp", width: 1, height: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateImage(test.data, test.declaredType)
			if err != nil {
				t.Fatalf("validateImage() returned error: %v", err)
			}
			config, format, err := image.DecodeConfig(bytes.NewReader(got.data))
			if err != nil {
				t.Fatalf("decode validated image: %v", err)
			}
			if format != "jpeg" {
				t.Fatalf("validated format = %q, want jpeg", format)
			}
			if config.Width != test.width || config.Height != test.height {
				t.Fatalf("validated dimensions = %dx%d, want %dx%d", config.Width, config.Height, test.width, test.height)
			}
		})
	}
}

func TestValidateImageResizesLongestSide(t *testing.T) {
	got, err := validateImage(photoTestPNG(t, 2000, 1000), "image/png")
	if err != nil {
		t.Fatalf("validateImage() returned error: %v", err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(got.data))
	if err != nil {
		t.Fatalf("decode validated image: %v", err)
	}
	if config.Width != 1600 || config.Height != 800 {
		t.Fatalf("validated dimensions = %dx%d, want 1600x800", config.Width, config.Height)
	}
}

func TestValidateImageRejectsInvalidInputs(t *testing.T) {
	oversized := photoTestJPEG(t, 4, 3)
	oversized = append(oversized, make([]byte, 5<<20+1-len(oversized))...)
	truncated := photoTestPNG(t, 64, 64)
	truncated = truncated[:len(truncated)/2]
	tests := []struct {
		name         string
		data         []byte
		declaredType string
	}{
		{name: "declared actual mismatch", data: photoTestJPEG(t, 4, 3), declaredType: "image/png"},
		{name: "unsupported format", data: []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"), declaredType: "image/gif"},
		{name: "over five megabytes", data: oversized, declaredType: "image/jpeg"},
		{name: "over twenty megapixels", data: photoTestPNG(t, 5000, 4001), declaredType: "image/png"},
		{name: "truncated image", data: truncated, declaredType: "image/png"},
		{name: "malformed data", data: []byte("not an image"), declaredType: "image/jpeg"},
		{name: "empty input", data: nil, declaredType: "image/jpeg"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := validateImage(test.data, test.declaredType); err == nil {
				t.Fatal("validateImage() accepted invalid input")
			}
		})
	}
}

func TestValidateImageStripsEXIF(t *testing.T) {
	input := photoTestJPEGWithEXIF(t)
	got, err := validateImage(input, "image/jpeg")
	if err != nil {
		t.Fatalf("validateImage() returned error: %v", err)
	}
	if bytes.Contains(got.data, []byte("Exif")) || bytes.Contains(got.data, []byte("private-exif-marker")) {
		t.Fatal("validated JPEG retained EXIF metadata")
	}
}

func TestValidateImageDeterministicJPEG(t *testing.T) {
	input := photoTestPNG(t, 32, 16)
	first, err := validateImage(input, "image/png")
	if err != nil {
		t.Fatalf("first validateImage() returned error: %v", err)
	}
	second, err := validateImage(input, "image/png")
	if err != nil {
		t.Fatalf("second validateImage() returned error: %v", err)
	}
	if !bytes.Equal(first.data, second.data) {
		t.Fatal("identical input produced different JPEG bytes")
	}
}

func TestPhotoSchemaStoresValidMetadata(t *testing.T) {
	resetAuthTables(t)
	t.Cleanup(func() { resetAuthTables(t) })
	ownerID := photoTestUser(t, "valid-metadata")

	var record photoRecord
	err := testDB.QueryRow(`
		INSERT INTO photos (owner_id, object_key, display_order)
		VALUES ($1, $2, 1)
		RETURNING id, owner_id, object_key, display_order, moderation_status,
			provider_reference, created_at, updated_at`, ownerID, "photos/pending.jpg").Scan(
		&record.ID, &record.OwnerID, &record.ObjectKey, &record.DisplayOrder,
		&record.ModerationStatus, &record.ProviderReference, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		t.Fatalf("insert pending photo: %v", err)
	}
	if record.ID == "" || record.OwnerID != ownerID || record.ObjectKey != "photos/pending.jpg" || record.DisplayOrder != 1 {
		t.Fatalf("stored photo metadata is wrong: %#v", record)
	}
	if record.ModerationStatus != photoModerationPending {
		t.Fatalf("default moderation status = %q, want pending", record.ModerationStatus)
	}
	if record.ProviderReference.Valid {
		t.Fatal("provider reference defaults to non-NULL")
	}
	if record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		t.Fatal("photo timestamps were not populated")
	}

	for _, photo := range []struct {
		key    string
		order  int
		status photoModerationStatus
	}{
		{key: "photos/approved.jpg", order: 2, status: photoModerationApproved},
		{key: "photos/rejected.jpg", order: 3, status: photoModerationRejected},
	} {
		var id string
		if err := testDB.QueryRow(`
			INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
			VALUES ($1, $2, $3, $4) RETURNING id`, ownerID, photo.key, photo.order, photo.status).Scan(&id); err != nil {
			t.Fatalf("insert %s photo: %v", photo.status, err)
		}
		if id == record.ID {
			t.Fatal("generated photo IDs are not unique")
		}
	}
}

func TestPhotoSchemaRejectsInvalidMetadata(t *testing.T) {
	type photoRow struct {
		ownerID      string
		objectKey    string
		displayOrder int
		status       photoModerationStatus
	}
	tests := []struct {
		name             string
		useOriginalOwner bool
		mutate           func(*photoRow)
	}{
		{name: "display order zero", mutate: func(photo *photoRow) { photo.displayOrder = 0 }},
		{name: "display order four", mutate: func(photo *photoRow) { photo.displayOrder = 4 }},
		{name: "empty object key", mutate: func(photo *photoRow) { photo.objectKey = "" }},
		{name: "invalid moderation status", mutate: func(photo *photoRow) { photo.status = "visible" }},
		{name: "missing owner", mutate: func(photo *photoRow) { photo.ownerID = "00000000-0000-0000-0000-000000000001" }},
		{name: "duplicate owner order", useOriginalOwner: true, mutate: func(photo *photoRow) {}},
		{name: "duplicate object key", mutate: func(photo *photoRow) { photo.objectKey = "photos/original.jpg" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetAuthTables(t)
			t.Cleanup(func() { resetAuthTables(t) })
			ownerID := photoTestUser(t, "original-owner")
			otherOwnerID := photoTestUser(t, "other-owner")
			if _, err := testDB.Exec(`
				INSERT INTO photos (owner_id, object_key, display_order)
				VALUES ($1, 'photos/original.jpg', 1)`, ownerID); err != nil {
				t.Fatalf("insert original photo: %v", err)
			}
			photo := photoRow{ownerID: otherOwnerID, objectKey: "photos/other.jpg", displayOrder: 1, status: photoModerationPending}
			if test.useOriginalOwner {
				photo.ownerID = ownerID
			}
			test.mutate(&photo)
			if _, err := testDB.Exec(`
				INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
				VALUES ($1, $2, $3, $4)`, photo.ownerID, photo.objectKey, photo.displayOrder, photo.status); err == nil {
				t.Fatal("photo schema accepted invalid metadata")
			}
		})
	}
}

func photoTestUser(t *testing.T, label string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(label))
	var userID string
	if err := testDB.QueryRow(`
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date)
		VALUES ($1, $2, '1990-01-01') RETURNING id`, []byte{1}, digest[:]).Scan(&userID); err != nil {
		t.Fatalf("insert photo test user: %v", err)
	}
	return userID
}

func photoTestJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, image.NewGray(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatalf("encode JPEG fixture: %v", err)
	}
	return buffer.Bytes()
}

func photoTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode PNG fixture: %v", err)
	}
	return buffer.Bytes()
}

func photoTestJPEGWithEXIF(t *testing.T) []byte {
	t.Helper()
	input := photoTestJPEG(t, 4, 3)
	payload := []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00private-exif-marker")
	segmentLength := len(payload) + 2
	output := make([]byte, 0, len(input)+len(payload)+4)
	output = append(output, input[:2]...)
	output = append(output, 0xff, 0xe1, byte(segmentLength>>8), byte(segmentLength))
	output = append(output, payload...)
	output = append(output, input[2:]...)
	if !bytes.Contains(output, []byte("private-exif-marker")) {
		t.Fatal("JPEG fixture does not contain EXIF marker")
	}
	return output
}
