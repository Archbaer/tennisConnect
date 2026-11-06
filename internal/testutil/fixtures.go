package testutil

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func PhotoJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, image.NewGray(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatalf("encode JPEG fixture: %v", err)
	}
	return buffer.Bytes()
}

func PhotoPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode PNG fixture: %v", err)
	}
	return buffer.Bytes()
}

func PhotoJPEGWithOrientation(t *testing.T, orientation int) []byte {
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
	return addEXIF(encoded.Bytes(), payload)
}

func PhotoJPEGWithEXIF(t *testing.T) []byte {
	t.Helper()
	input := PhotoJPEG(t, 4, 3)
	payload := []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00private-exif-marker")
	output := addEXIF(input, payload)
	if !bytes.Contains(output, []byte("private-exif-marker")) {
		t.Fatal("JPEG fixture does not contain EXIF marker")
	}
	return output
}

func addEXIF(input, payload []byte) []byte {
	segmentLength := len(payload) + 2
	output := make([]byte, 0, len(input)+len(payload)+4)
	output = append(output, input[:2]...)
	output = append(output, 0xff, 0xe1, byte(segmentLength>>8), byte(segmentLength))
	output = append(output, payload...)
	output = append(output, input[2:]...)
	return output
}
