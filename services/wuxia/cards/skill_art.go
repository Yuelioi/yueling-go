package cards

import (
	"bytes"
	"embed"
	"image"
	"io/fs"
	"path"
	"strings"
	"sync"

	"golang.org/x/image/webp"
)

//go:embed assets/martial/*.webp
var martialFiles embed.FS
var martialArt struct {
	sync.Once
	images map[string]image.Image
	err    error
}

func loadMartialArt() error {
	martialArt.Do(func() {
		martialArt.images = map[string]image.Image{}
		var files []string
		files, martialArt.err = fs.Glob(martialFiles, "assets/martial/*.webp")
		if martialArt.err != nil {
			return
		}
		for _, file := range files {
			data, err := martialFiles.ReadFile(file)
			if err != nil {
				martialArt.err = err
				return
			}
			img, err := webp.Decode(bytes.NewReader(data))
			if err != nil {
				martialArt.err = err
				return
			}
			martialArt.images[strings.TrimSuffix(path.Base(file), ".webp")] = crop(img, martialSubjectBounds(img))
		}
	})
	return martialArt.err
}

// Transparent margins must not shrink an already tiny combat icon.
func martialSubjectBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	bounds := image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0x1000 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bounds.Empty() {
		return b
	}
	return bounds.Inset(-2).Intersect(b)
}

func (p *painter) paintMartial(r image.Rectangle, art image.Image) {
	b := art.Bounds()
	w, h := r.Dx(), r.Dy()
	if b.Dx()*h > b.Dy()*w {
		h = max(1, b.Dy()*w/b.Dx())
	} else {
		w = max(1, b.Dx()*h/b.Dy())
	}
	x, y := r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-h)/2
	scaleAligned(p.image, image.Rect(x, y, x+w, y+h), art, .5)
}
