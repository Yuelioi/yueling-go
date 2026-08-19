package yugioh_card

import (
	"context"
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

func TestSpecRegistersYugiohCard(t *testing.T) {
	spec := (&Template{}).Spec()
	if spec.Key != "yugioh_card" || len(spec.Keywords) != 1 || spec.Keywords[0] != "游戏王" {
		t.Fatalf("unexpected identity: key=%q keywords=%q", spec.Key, spec.Keywords)
	}
	if spec.MinImages != 1 || spec.MaxImages != 1 || spec.MinTexts != 2 || spec.MaxTexts != 2 || spec.AllowAvatarFallback {
		t.Fatalf("unexpected contract: %#v", spec)
	}
}

func TestRenderFillsPictureAndKeepsContentInsideCardFields(t *testing.T) {
	template := templateWithTestFont(t)
	input := solidFrame(500, 300, color.NRGBA{R: 255, B: 255, A: 255})
	result, err := template.Render(context.Background(), model.RenderRequest{
		Images: []*imaging.Animation{{Frames: []*image.NRGBA{input}}},
		Texts:  []string{"CARD TITLE", "A DESCRIPTION THAT WRAPS INSIDE THE LOWER FIELD"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 1 || result.Frames[0].Bounds() != image.Rect(0, 0, canvasWidth, canvasHeight) {
		t.Fatalf("result frames=%d bounds=%v", result.FrameCount(), result.Frames[0].Bounds())
	}
	if got := result.Frames[0].NRGBAAt(pictureBox.Min.X, pictureBox.Min.Y); got != (color.NRGBA{R: 255, B: 255, A: 255}) {
		t.Fatalf("picture was not inserted: %#v", got)
	}
	for _, point := range []image.Point{{X: 20, Y: 20}, {X: pictureBox.Min.X - 1, Y: 300}, {X: pictureBox.Max.X, Y: 300}} {
		if got, want := result.Frames[0].NRGBAAt(point.X, point.Y), template.source.NRGBAAt(point.X, point.Y); got != want {
			t.Fatalf("card frame pixel %v changed: got %#v want %#v", point, got, want)
		}
	}
	if changedPixels(result.Frames[0], template.source, titleBox) == 0 || changedPixels(result.Frames[0], template.source, descriptionBox) == 0 {
		t.Fatal("title or description was not drawn")
	}
	assertChangesStayInsideCardFields(t, result.Frames[0], template.source)
}

func TestRenderPreservesAnimatedImageTiming(t *testing.T) {
	template := templateWithTestFont(t)
	input := &imaging.Animation{
		Frames: []*image.NRGBA{
			solidFrame(20, 20, color.NRGBA{R: 255, A: 255}),
			solidFrame(20, 20, color.NRGBA{G: 255, A: 255}),
		},
		Delays:    []int{7, 11},
		LoopCount: 2,
	}
	result, err := template.Render(context.Background(), model.RenderRequest{
		Images: []*imaging.Animation{input},
		Texts:  []string{"TITLE", "DESCRIPTION"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 2 || result.Delays[0] != 7 || result.Delays[1] != 11 || result.LoopCount != 2 {
		t.Fatalf("animation metadata = frames:%d delays:%v loop:%d", result.FrameCount(), result.Delays, result.LoopCount)
	}
	if got := result.Frames[0].NRGBAAt(pictureBox.Min.X+10, pictureBox.Min.Y+10); got.R != 255 || got.G != 0 {
		t.Fatalf("first frame picture = %#v", got)
	}
	if got := result.Frames[1].NRGBAAt(pictureBox.Min.X+10, pictureBox.Min.Y+10); got.G != 255 || got.R != 0 {
		t.Fatalf("second frame picture = %#v", got)
	}
}

func TestRenderRequiresImageAndTwoTexts(t *testing.T) {
	template := templateWithTestFont(t)
	if _, err := template.Render(context.Background(), model.RenderRequest{Texts: []string{"TITLE", "DESCRIPTION"}}); err == nil {
		t.Fatal("Render accepted a request without an image")
	}
	if _, err := template.Render(context.Background(), model.RenderRequest{
		Images: []*imaging.Animation{{Frames: []*image.NRGBA{solidFrame(20, 20, color.NRGBA{A: 255})}}},
		Texts:  []string{"TITLE"},
	}); err == nil {
		t.Fatal("Render accepted one text")
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

func solidFrame(width, height int, fill color.NRGBA) *image.NRGBA {
	frame := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			frame.SetNRGBA(x, y, fill)
		}
	}
	return frame
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

func assertChangesStayInsideCardFields(t *testing.T, got, want *image.NRGBA) {
	t.Helper()
	for y := 0; y < canvasHeight; y++ {
		for x := 0; x < canvasWidth; x++ {
			point := image.Pt(x, y)
			inside := point.In(titleBox) || point.In(pictureBox) || point.In(descriptionBox)
			if !inside && got.NRGBAAt(x, y) != want.NRGBAAt(x, y) {
				t.Fatalf("pixel outside card fields changed at %v", point)
			}
		}
	}
}
