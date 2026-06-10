package services

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func noisyImage(withAlpha bool) *image.NRGBA {
	const size = 800
	rng := rand.New(rand.NewSource(1))
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			a := uint8(255)
			if withAlpha && x < 10 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), a})
		}
	}
	return img
}

func TestShrinkToJPEG(t *testing.T) {
	const minBytes = 1 << 20

	small := []byte("under the threshold, left alone")
	if got := ShrinkToJPEG(small, minBytes, 85); !bytes.Equal(got, small) {
		t.Fatalf("sub-threshold data must pass through untouched")
	}

	opaque := encodePNG(t, noisyImage(false))
	if len(opaque) <= minBytes {
		t.Fatalf("opaque fixture too small: %d bytes", len(opaque))
	}
	if got := ShrinkToJPEG(opaque, minBytes, 85); len(got) >= len(opaque) {
		t.Fatalf("opaque oversized png should shrink: %d -> %d", len(opaque), len(got))
	}

	transparent := encodePNG(t, noisyImage(true))
	if len(transparent) <= minBytes {
		t.Fatalf("transparent fixture too small: %d bytes", len(transparent))
	}
	if got := ShrinkToJPEG(transparent, minBytes, 85); !bytes.Equal(got, transparent) {
		t.Fatalf("transparent png must be left untouched")
	}

	// minBytes <= 0 means always attempt, even for small opaque images.
	tiny := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 64, 64)))
	if got := ShrinkToJPEG(tiny, 0, 85); len(got) == 0 {
		t.Fatalf("minBytes=0 path returned empty data")
	}
}
