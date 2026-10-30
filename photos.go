package main

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	maxPhotoBytes     = 5 << 20
	maxPhotoPixels    = 20_000_000
	maxPhotoDimension = 1600
)

var errInvalidImage = errors.New("invalid image")

type validatedImage struct {
	data []byte
}

type photoModerationStatus string

const (
	photoModerationPending  photoModerationStatus = "pending"
	photoModerationApproved photoModerationStatus = "approved"
	photoModerationRejected photoModerationStatus = "rejected"
)

type photoRecord struct {
	ID                string
	OwnerID           string
	ObjectKey         string
	DisplayOrder      int
	ModerationStatus  photoModerationStatus
	ProviderReference sql.NullString
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func validateImage(data []byte, declaredType string) (validatedImage, error) {
	if len(data) == 0 || len(data) > maxPhotoBytes {
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
