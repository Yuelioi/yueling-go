package cards

import (
	"image"
	"sync"
	"sync/atomic"
	"testing"
)

func TestArtworkDecodeIsLazyAndShared(t *testing.T) {
	var calls atomic.Int32
	a := &decodedAsset{decode: func([]byte) (image.Image, error) { calls.Add(1); return image.NewRGBA(image.Rect(0, 0, 4, 4)), nil }}
	if calls.Load() != 0 {
		t.Fatal("decoded before use")
	}
	var group sync.WaitGroup
	images := make([]image.Image, 16)
	for i := range images {
		group.Add(1)
		go func() { defer group.Done(); images[i] = a.image() }()
	}
	group.Wait()
	if calls.Load() != 1 {
		t.Fatal("concurrent requests decoded more than once")
	}
	for _, img := range images {
		if img != images[0] {
			t.Fatal("decoded image not shared")
		}
	}
}
