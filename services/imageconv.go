package services

import (
	"bytes"
	"image"
	"image/jpeg"

	_ "image/gif"
	_ "image/png"
)

// ShrinkToJPEG re-encodes an oversized opaque image as JPEG to cut its size.
// It returns the original bytes unchanged when the image is below minBytes
// (minBytes <= 0 means always attempt), can't be decoded, is a GIF (would lose
// animation), has transparency, or wouldn't actually come out smaller.
// quality outside 1-100 falls back to 85.
func ShrinkToJPEG(data []byte, minBytes, quality int) []byte {
	if minBytes > 0 && len(data) < minBytes {
		return data
	}
	if quality < 1 || quality > 100 {
		quality = 85
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format == "gif" {
		return data
	}
	if op, ok := img.(interface{ Opaque() bool }); !ok || !op.Opaque() {
		return data
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return data
	}
	if buf.Len() >= len(data) {
		return data
	}
	return buf.Bytes()
}
