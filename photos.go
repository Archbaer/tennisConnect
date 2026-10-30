package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
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
