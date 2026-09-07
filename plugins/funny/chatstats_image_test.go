package funny

import (
	"image"
	"os"
	"testing"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func TestChatCloudWordSpritePreservesWholeText(t *testing.T) {
	data, err := os.ReadFile("../../data/fonts/NotoSansSC-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := truetype.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"抽签", "运动", "今日", "红女", "Steam史低"} {
		t.Run(text, func(t *testing.T) {
			const size = 67
			sprite := newChatCloudWordSprite(parsed, text, size)
			face := truetype.NewFace(parsed, &truetype.Options{Size: size, DPI: chatCloudDPI, Hinting: font.HintingNone})
			defer face.Close()
			canvas := image.NewAlpha(image.Rect(0, 0, 1200, 300))
			drawer := font.Drawer{Dst: canvas, Src: image.White, Face: face, Dot: fixed.P(100, 150)}
			drawer.DrawString(text)
			inkSum := func(img *image.Alpha) int {
				total := 0
				for _, a := range img.Pix {
					total += int(a)
				}
				return total
			}
			if sprite == nil {
				t.Fatal("missing sprite")
			}
			if got, want := inkSum(sprite.ink), inkSum(canvas); got != want {
				t.Fatalf("word sprite clips text: ink coverage %d, full text %d", got, want)
			}
		})
	}
}
