package textdraw

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

func TestFitBoxWrapsAndRejectsOverflow(t *testing.T) {
	parsed := testFont(t)
	bounds := image.Rect(20, 20, 300, 220)
	style := BoxStyle{MaxFontSize: 34, MinFontSize: 16, LineGap: 5}
	layout, err := FitBox(parsed, "A LONG CAPTION THAT MUST WRAP ACROSS MULTIPLE LINES", bounds, style)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Close()
	if layout.LineCount() < 2 || layout.Height() > bounds.Dy() {
		t.Fatalf("layout lines=%d height=%d", layout.LineCount(), layout.Height())
	}
	if _, err := FitBox(parsed, strings.Repeat("W", 2000), bounds, style); err == nil {
		t.Fatal("FitBox accepted overflowing text")
	}
}

func TestDrawCenteredBoxStaysInsideBounds(t *testing.T) {
	parsed := testFont(t)
	dst := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	bounds := image.Rect(20, 20, 300, 220)
	err := DrawCenteredBox(dst, parsed, "CENTERED TEXT THAT WRAPS", bounds, BoxStyle{
		MaxFontSize: 34, MinFontSize: 16, LineGap: 5, Color: color.Black,
	})
	if err != nil {
		t.Fatal(err)
	}
	var changed int
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			if dst.NRGBAAt(x, y).A == 0 {
				continue
			}
			changed++
			if !image.Pt(x, y).In(bounds) {
				t.Fatalf("text escaped bounds at %v", image.Pt(x, y))
			}
		}
	}
	if changed == 0 {
		t.Fatal("DrawCenteredBox drew no pixels")
	}
}

func testFont(t *testing.T) *opentype.Font {
	t.Helper()
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
