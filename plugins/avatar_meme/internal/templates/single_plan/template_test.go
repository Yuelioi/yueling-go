package single_plan

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

func TestRenderPreservesTopAndReplacesBottom(t *testing.T) {
	template := &Template{}
	inputFrame := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			inputFrame.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
		}
	}
	result, err := template.Render(context.Background(), model.RenderRequest{
		Images: []*imaging.Animation{{Frames: []*image.NRGBA{inputFrame}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 1 || result.Frames[0].Bounds().Size() != (image.Point{X: 1220, Y: 1574}) {
		t.Fatalf("result frames=%d size=%v", result.FrameCount(), result.Frames[0].Bounds().Size())
	}
	if got, want := result.Frames[0].NRGBAAt(20, 20), template.source.NRGBAAt(20, 20); got != want {
		t.Fatalf("top pixel changed: got %#v want %#v", got, want)
	}
	if got := result.Frames[0].NRGBAAt(20, splitY+40); got.B < 250 || got.A != 255 {
		t.Fatalf("bottom image was not replaced: %#v", got)
	}
}

func TestExtractOutlinedCaptionHasTransparentBackground(t *testing.T) {
	template := &Template{}
	template.once.Do(template.loadAssets)
	if template.err != nil {
		t.Fatal(template.err)
	}
	if got := template.caption.NRGBAAt(0, 0).A; got != 0 {
		t.Fatalf("caption corner alpha=%d want 0", got)
	}
	var opaque int
	for y := 0; y < template.caption.Bounds().Dy(); y++ {
		for x := 0; x < template.caption.Bounds().Dx(); x++ {
			if template.caption.NRGBAAt(x, y).A != 0 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Fatal("caption contains no visible pixels")
	}
}
