package single_plan

import (
	"context"
	"image"
	"image/color"
	"image/draw"
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

func TestSpecOnlyRegistersSingleKeyword(t *testing.T) {
	spec := (&Template{}).Spec()
	if spec.Key != "single_plan" || len(spec.Keywords) != 1 || spec.Keywords[0] != "单身" {
		t.Fatalf("unexpected template identity: key=%q keywords=%q", spec.Key, spec.Keywords)
	}
}

func TestRenderDrawsGeneratedCaptionOverReplacement(t *testing.T) {
	template := &Template{}
	inputFrame := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	draw.Draw(inputFrame, inputFrame.Bounds(), image.NewUniform(color.NRGBA{B: 255, A: 255}), image.Point{}, draw.Src)
	result, err := template.Render(context.Background(), model.RenderRequest{
		Images: []*imaging.Animation{{Frames: []*image.NRGBA{inputFrame}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	captionArea := image.Rect(0, result.Frames[0].Bounds().Dy()-captionBottomGap-captionBoxHeight-captionOutline, result.Frames[0].Bounds().Dx(), result.Frames[0].Bounds().Dy())
	var whitePixels, darkPixels int
	for y := captionArea.Min.Y; y < captionArea.Max.Y; y++ {
		for x := captionArea.Min.X; x < captionArea.Max.X; x++ {
			pixel := result.Frames[0].NRGBAAt(x, y)
			if pixel.R > 240 && pixel.G > 240 && pixel.B > 240 {
				whitePixels++
			}
			if pixel.R < 32 && pixel.G < 32 && pixel.B < 32 {
				darkPixels++
			}
		}
	}
	if whitePixels < 500 || darkPixels < 500 {
		t.Fatalf("generated caption pixels: white=%d dark=%d", whitePixels, darkPixels)
	}
}
