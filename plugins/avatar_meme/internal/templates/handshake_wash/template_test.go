package handshake_wash

import (
	"context"
	"image"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
)

func TestSpecRegistersHandshakeWash(t *testing.T) {
	spec := (&Template{}).Spec()
	if spec.Key != "handshake_wash" || len(spec.Keywords) != 1 || spec.Keywords[0] != "握手洗手" {
		t.Fatalf("unexpected identity: key=%q keywords=%q", spec.Key, spec.Keywords)
	}
	if spec.MinImages != 0 || spec.MaxImages != 0 || spec.MinTexts != 3 || spec.MaxTexts != 3 {
		t.Fatalf("unexpected contract: %#v", spec)
	}
}

func TestRenderPlacesThreeOutlinedTextsWithoutTouchingDividers(t *testing.T) {
	template := templateWithTestFont(t)
	result, err := template.Render(context.Background(), model.RenderRequest{Texts: []string{
		"I AM A PLAYER",
		"ME TOO WHAT DO YOU PLAY",
		"LOVE AND DEEPSPACE",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 1 || result.Frames[0].Bounds() != image.Rect(0, 0, canvasWidth, canvasHeight) {
		t.Fatalf("result frames=%d bounds=%v", result.FrameCount(), result.Frames[0].Bounds())
	}
	for i, bounds := range textBoxes {
		if changedPixels(result.Frames[0], template.source, bounds.Inset(-outlineSize)) == 0 {
			t.Fatalf("text area %d did not change", i+1)
		}
	}
	for _, point := range []image.Point{{X: 250, Y: 394}, {X: 250, Y: 749}, {X: 250, Y: 900}} {
		if got, want := result.Frames[0].NRGBAAt(point.X, point.Y), template.source.NRGBAAt(point.X, point.Y); got != want {
			t.Fatalf("protected pixel %v changed: got %#v want %#v", point, got, want)
		}
	}
	assertChangesStayInsideTextAreas(t, result.Frames[0], template.source)
}

func TestRenderRequiresThreeTexts(t *testing.T) {
	if _, err := templateWithTestFont(t).Render(context.Background(), model.RenderRequest{Texts: []string{"ONE", "TWO"}}); err == nil {
		t.Fatal("Render accepted two texts")
	}
}

func templateWithTestFont(t *testing.T) *Template {
	t.Helper()
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return &Template{font: parsed}
}

func changedPixels(got, want *image.NRGBA, area image.Rectangle) int {
	changed := 0
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if got.NRGBAAt(x, y) != want.NRGBAAt(x, y) {
				changed++
			}
		}
	}
	return changed
}

func assertChangesStayInsideTextAreas(t *testing.T, got, want *image.NRGBA) {
	t.Helper()
	allowed := make([]image.Rectangle, len(textBoxes))
	for i, bounds := range textBoxes {
		allowed[i] = bounds.Inset(-outlineSize)
	}
	for y := 0; y < canvasHeight; y++ {
		for x := 0; x < canvasWidth; x++ {
			point := image.Pt(x, y)
			inside := false
			for _, bounds := range allowed {
				inside = inside || point.In(bounds)
			}
			if !inside && got.NRGBAAt(x, y) != want.NRGBAAt(x, y) {
				t.Fatalf("pixel outside text areas changed at %v", point)
			}
		}
	}
}
