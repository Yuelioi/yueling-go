package spiderman_glasses

import (
	"context"
	"image"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/textdraw"
)

func TestSpecRegistersSpidermanGlasses(t *testing.T) {
	spec := (&Template{}).Spec()
	if spec.Key != "spiderman_glasses" || len(spec.Keywords) != 1 || spec.Keywords[0] != "蜘蛛人戴眼镜" {
		t.Fatalf("unexpected identity: key=%q keywords=%q", spec.Key, spec.Keywords)
	}
	if spec.MinImages != 0 || spec.MaxImages != 0 || spec.MinTexts != 2 || spec.MaxTexts != 2 || len(spec.DefaultTexts) != 0 {
		t.Fatalf("unexpected contract: %#v", spec)
	}
}

func TestRenderCentersTextInsideBothRightHandBoxes(t *testing.T) {
	template := templateWithTestFont(t)
	result, err := template.Render(context.Background(), model.RenderRequest{Texts: []string{
		"WITHOUT GLASSES THIS CAPTION WRAPS",
		"WITH GLASSES THIS CAPTION WRAPS TOO",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.FrameCount() != 1 || result.Frames[0].Bounds() != image.Rect(0, 0, canvasWidth, canvasHeight) {
		t.Fatalf("result frames=%d bounds=%v", result.FrameCount(), result.Frames[0].Bounds())
	}
	for _, bounds := range textBoxes {
		if changedPixels(result.Frames[0], template.source, bounds) == 0 {
			t.Fatalf("text box %v did not change", bounds)
		}
	}
	if got, want := result.Frames[0].NRGBAAt(100, 100), template.source.NRGBAAt(100, 100); got != want {
		t.Fatalf("left image changed: got %#v want %#v", got, want)
	}
	if got, want := result.Frames[0].NRGBAAt(400, 279), template.source.NRGBAAt(400, 279); got != want {
		t.Fatalf("center divider changed: got %#v want %#v", got, want)
	}
	assertChangesStayInsideTextBoxes(t, result.Frames[0], template.source)
}

func TestFitTextBoxWrapsAndRejectsOverflow(t *testing.T) {
	template := templateWithTestFont(t)
	template.once.Do(template.loadAssets)
	layout, err := textdraw.FitBox(template.font, "A LONG CAPTION THAT MUST WRAP ACROSS MULTIPLE LINES", textBoxes[0], textBoxStyle)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Close()
	if layout.LineCount() < 2 || layout.Height() > textBoxes[0].Dy() {
		t.Fatalf("layout lines=%d height=%d", layout.LineCount(), layout.Height())
	}
}

func TestRenderRequiresTwoTexts(t *testing.T) {
	if _, err := templateWithTestFont(t).Render(context.Background(), model.RenderRequest{Texts: []string{"ONLY ONE"}}); err == nil {
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

func assertChangesStayInsideTextBoxes(t *testing.T, got, want *image.NRGBA) {
	t.Helper()
	for y := 0; y < canvasHeight; y++ {
		for x := 0; x < canvasWidth; x++ {
			point := image.Pt(x, y)
			inside := point.In(textBoxes[0]) || point.In(textBoxes[1])
			if !inside && got.NRGBAAt(x, y) != want.NRGBAAt(x, y) {
				t.Fatalf("pixel outside text boxes changed at %v", point)
			}
		}
	}
}
