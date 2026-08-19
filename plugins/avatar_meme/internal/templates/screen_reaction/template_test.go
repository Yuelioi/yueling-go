package screen_reaction

import (
	"context"
	"image"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
)

func TestSpecRegistersTwoSubtitleTemplate(t *testing.T) {
	spec := (&Template{}).Spec()
	if spec.Key != "screen_reaction" || len(spec.Keywords) != 1 || spec.Keywords[0] != "老人微笑" {
		t.Fatalf("unexpected identity: key=%q keywords=%q", spec.Key, spec.Keywords)
	}
	if spec.MinImages != 0 || spec.MaxImages != 0 || spec.MinTexts != 2 || spec.MaxTexts != 2 || len(spec.DefaultTexts) != 2 {
		t.Fatalf("unexpected contract: %#v", spec)
	}
	if &spec.DefaultTexts[0] == &defaultTexts[0] {
		t.Fatal("Spec returned the mutable default text backing array")
	}
}

func TestRenderAddsSubtitlesAtBothPanelBottoms(t *testing.T) {
	template := templateWithTestFont(t)
	result, err := template.Render(context.Background(), model.RenderRequest{Texts: []string{"FIRST CAPTION", "SECOND CAPTION"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 1 || result.Frames[0].Bounds() != image.Rect(0, 0, canvasWidth, canvasHeight) {
		t.Fatalf("result frames=%d bounds=%v", result.FrameCount(), result.Frames[0].Bounds())
	}
	for _, area := range []image.Rectangle{
		image.Rect(0, upperPanelBottom-subtitleFadeHeight, canvasWidth, upperPanelBottom),
		image.Rect(0, canvasHeight-subtitleFadeHeight, canvasWidth, canvasHeight),
	} {
		if changedPixels(result.Frames[0], template.source, area) == 0 {
			t.Fatalf("subtitle area %v did not change", area)
		}
	}
	if got, want := result.Frames[0].NRGBAAt(100, 100), template.source.NRGBAAt(100, 100); got != want {
		t.Fatalf("pixel outside subtitle areas changed: got %#v want %#v", got, want)
	}
}

func TestRenderRejectsWrongTextCountAndOverlongText(t *testing.T) {
	template := templateWithTestFont(t)
	if _, err := template.Render(context.Background(), model.RenderRequest{Texts: []string{"ONLY ONE"}}); err == nil {
		t.Fatal("Render accepted only one subtitle")
	}
	if _, err := template.Render(context.Background(), model.RenderRequest{Texts: []string{
		"THIS CAPTION IS FAR TOO LONG TO FIT ON A SINGLE SUBTITLE LINE EVEN AT THE MINIMUM FONT SIZE",
		"SECOND",
	}}); err == nil {
		t.Fatal("Render accepted an overlong subtitle")
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
