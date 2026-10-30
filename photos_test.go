package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
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

func TestValidateImageAppliesEXIFOrientation(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	green := color.RGBA{G: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	tests := []struct {
		name        string
		orientation int
		width       int
		height      int
		colors      [4]color.RGBA
	}{
		{name: "identity", orientation: 1, width: 20, height: 30, colors: [4]color.RGBA{red, green, blue, white}},
		{name: "mirror horizontal", orientation: 2, width: 20, height: 30, colors: [4]color.RGBA{green, red, white, blue}},
		{name: "rotate 180", orientation: 3, width: 20, height: 30, colors: [4]color.RGBA{white, blue, green, red}},
		{name: "mirror vertical", orientation: 4, width: 20, height: 30, colors: [4]color.RGBA{blue, white, red, green}},
		{name: "transpose", orientation: 5, width: 30, height: 20, colors: [4]color.RGBA{red, blue, green, white}},
		{name: "rotate 90", orientation: 6, width: 30, height: 20, colors: [4]color.RGBA{blue, red, white, green}},
		{name: "transverse", orientation: 7, width: 30, height: 20, colors: [4]color.RGBA{white, green, blue, red}},
		{name: "rotate 270", orientation: 8, width: 30, height: 20, colors: [4]color.RGBA{green, white, red, blue}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateImage(photoTestJPEGWithOrientation(t, test.orientation), "image/jpeg")
			if err != nil {
				t.Fatalf("validateImage() returned error: %v", err)
			}
			decoded, err := jpeg.Decode(bytes.NewReader(got.data))
			if err != nil {
				t.Fatalf("decode validated JPEG: %v", err)
			}
			if decoded.Bounds().Dx() != test.width || decoded.Bounds().Dy() != test.height {
				t.Fatalf("oriented dimensions = %dx%d, want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), test.width, test.height)
			}
			points := [4]image.Point{{X: 2, Y: 2}, {X: test.width - 3, Y: 2}, {X: 2, Y: test.height - 3}, {X: test.width - 3, Y: test.height - 3}}
			for i, point := range points {
				photoTestAssertColor(t, decoded, point, test.colors[i])
			}
		})
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

func TestPhotoMigrationPreservesExistingUsers(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := testDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{"SET search_path TO public", "DROP SCHEMA IF EXISTS photo_upgrade CASCADE"} {
			if _, err := conn.ExecContext(ctx, query); err != nil {
				t.Errorf("photo upgrade cleanup %q: %v", query, err)
			}
		}
		if err := conn.Close(); err != nil {
			t.Errorf("close photo upgrade connection: %v", err)
		}
	})
	if _, err := conn.ExecContext(ctx, "DROP SCHEMA IF EXISTS photo_upgrade CASCADE; CREATE SCHEMA photo_upgrade; SET search_path TO photo_upgrade;"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000001_users.sql", "000002_auth.sql", "000003_profiles.sql"} {
		contents, err := os.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
			t.Fatalf("apply pre-photo migration %s: %v", name, err)
		}
	}

	digest := sha256.Sum256([]byte("upgrade-owner"))
	var ownerID string
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date)
		VALUES ($1, $2, '1990-01-01') RETURNING id`, []byte{1}, digest[:]).Scan(&ownerID); err != nil {
		t.Fatalf("seed pre-photo user: %v", err)
	}
	contents, err := os.ReadFile("migrations/000004_photos.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
		t.Fatalf("apply photo upgrade: %v", err)
	}

	var storedDigest []byte
	if err := conn.QueryRowContext(ctx, "SELECT email_lookup_hmac FROM users WHERE id = $1", ownerID).Scan(&storedDigest); err != nil {
		t.Fatalf("read preserved user: %v", err)
	}
	if !bytes.Equal(storedDigest, digest[:]) {
		t.Fatal("photo upgrade changed existing user data")
	}
	var status photoModerationStatus
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO photos (owner_id, object_key, display_order)
		VALUES ($1, 'photos/upgrade.jpg', 1) RETURNING moderation_status`, ownerID).Scan(&status); err != nil {
		t.Fatalf("insert upgraded user's photo: %v", err)
	}
	if status != photoModerationPending {
		t.Fatalf("upgraded photo status = %q, want pending", status)
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

func photoTestJPEGWithOrientation(t *testing.T, orientation int) []byte {
	t.Helper()
	input := image.NewRGBA(image.Rect(0, 0, 20, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 20; x++ {
			switch {
			case x < 10 && y < 15:
				input.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			case y < 15:
				input.SetRGBA(x, y, color.RGBA{G: 255, A: 255})
			case x < 10:
				input.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
			default:
				input.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, input, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("encode orientation fixture: %v", err)
	}

	tiff := make([]byte, 26)
	copy(tiff, "II")
	binary.LittleEndian.PutUint16(tiff[2:], 42)
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint16(tiff[8:], 1)
	binary.LittleEndian.PutUint16(tiff[10:], 0x0112)
	binary.LittleEndian.PutUint16(tiff[12:], 3)
	binary.LittleEndian.PutUint32(tiff[14:], 1)
	binary.LittleEndian.PutUint16(tiff[18:], uint16(orientation))
	payload := append([]byte("Exif\x00\x00"), tiff...)
	return photoTestAddEXIF(encoded.Bytes(), payload)
}

func photoTestAssertColor(t *testing.T, source image.Image, point image.Point, expected color.RGBA) {
	t.Helper()
	got := [4]uint32{}
	got[0], got[1], got[2], got[3] = source.At(point.X, point.Y).RGBA()
	want := [4]uint32{uint32(expected.R) * 0x101, uint32(expected.G) * 0x101, uint32(expected.B) * 0x101, uint32(expected.A) * 0x101}
	const tolerance = 32 * 0x101
	for i := range got {
		if got[i]+tolerance < want[i] || got[i] > want[i]+tolerance {
			t.Errorf("color at %v channel %d = %d, want %d +/- %d", point, i, got[i], want[i], tolerance)
		}
	}
}

func photoTestJPEGWithEXIF(t *testing.T) []byte {
	t.Helper()
	input := photoTestJPEG(t, 4, 3)
	payload := []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00private-exif-marker")
	output := photoTestAddEXIF(input, payload)
	if !bytes.Contains(output, []byte("private-exif-marker")) {
		t.Fatal("JPEG fixture does not contain EXIF marker")
	}
	return output
}

func photoTestAddEXIF(input, payload []byte) []byte {
	segmentLength := len(payload) + 2
	output := make([]byte, 0, len(input)+len(payload)+4)
	output = append(output, input[:2]...)
	output = append(output, 0xff, 0xe1, byte(segmentLength>>8), byte(segmentLength))
	output = append(output, payload...)
	output = append(output, input[2:]...)
	return output
}
