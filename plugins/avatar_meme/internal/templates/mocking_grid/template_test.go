package mocking_grid

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

func TestSpecRegistersMockingWithAvatarInput(t *testing.T) {
	spec := (&Template{}).Spec()
	if spec.Key != "mocking_grid" || len(spec.Keywords) != 1 || spec.Keywords[0] != "嘲笑" {
		t.Fatalf("unexpected template identity: key=%q keywords=%q", spec.Key, spec.Keywords)
	}
	if spec.MinImages != 1 || spec.MaxImages != 1 || !spec.AllowAvatarFallback {
		t.Fatalf("unexpected image contract: min=%d max=%d avatarFallback=%v", spec.MinImages, spec.MaxImages, spec.AllowAvatarFallback)
	}
}

func TestRenderReplacesCenterAndPreservesGrid(t *testing.T) {
	template := &Template{}
	inputFrame := solidFrame(400, 200, color.NRGBA{R: 255, B: 255, A: 255})
	result, err := template.Render(context.Background(), model.RenderRequest{
		Images: []*imaging.Animation{{Frames: []*image.NRGBA{inputFrame}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 1 || result.Frames[0].Bounds() != image.Rect(0, 0, 720, 720) {
		t.Fatalf("result frames=%d bounds=%v", result.FrameCount(), result.Frames[0].Bounds())
	}
	if got := result.Frames[0].NRGBAAt(centerRect.Min.X, centerRect.Min.Y); got != (color.NRGBA{R: 255, B: 255, A: 255}) {
		t.Fatalf("center was not replaced: %#v", got)
	}
	for _, point := range []image.Point{{X: 100, Y: 100}, {X: centerRect.Min.X - 1, Y: 360}, {X: centerRect.Max.X, Y: 360}} {
		if got, want := result.Frames[0].NRGBAAt(point.X, point.Y), template.source.NRGBAAt(point.X, point.Y); got != want {
			t.Fatalf("source pixel %v changed: got %#v want %#v", point, got, want)
		}
	}
}

func TestRenderPreservesAnimationTiming(t *testing.T) {
	template := &Template{}
	input := &imaging.Animation{
		Frames: []*image.NRGBA{
			solidFrame(20, 20, color.NRGBA{R: 255, A: 255}),
			solidFrame(20, 20, color.NRGBA{G: 255, A: 255}),
		},
		Delays:    []int{7, 11},
		LoopCount: 3,
	}
	result, err := template.Render(context.Background(), model.RenderRequest{Images: []*imaging.Animation{input}})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 2 || result.Delays[0] != 7 || result.Delays[1] != 11 || result.LoopCount != 3 {
		t.Fatalf("animation metadata = frames:%d delays:%v loop:%d", result.FrameCount(), result.Delays, result.LoopCount)
	}
	if got := result.Frames[0].NRGBAAt(360, 360); got != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("first frame center = %#v", got)
	}
	if got := result.Frames[1].NRGBAAt(360, 360); got != (color.NRGBA{G: 255, A: 255}) {
		t.Fatalf("second frame center = %#v", got)
	}
}

func TestRenderRequiresOneImage(t *testing.T) {
	if _, err := (&Template{}).Render(context.Background(), model.RenderRequest{}); err == nil {
		t.Fatal("Render accepted a request without an image")
	}
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
