package funny

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/Yuelioi/yueling-go/services"
)

const (
	chatCloudWidth  = 960
	chatCloudHeight = 640
	chatCloudDPI    = 96
)

var (
	chatCloudFontOnce sync.Once
	chatCloudFont     *truetype.Font
	chatCloudFontErr  error
)

var chatCloudPalette = []color.RGBA{
	{113, 232, 255, 255},
	{139, 130, 255, 255},
	{248, 151, 231, 255},
	{255, 200, 110, 255},
	{127, 237, 188, 255},
	{207, 222, 255, 255},
}

type chatCloudRect struct{ x0, y0, x1, y1 int }

func renderChatWordCloud(analysis chatAnalysis) ([]byte, error) {
	parsed, err := loadChatCloudFont()
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, chatCloudWidth, chatCloudHeight))
	drawChatCloudBackground(img)

	seed := int64(chatCloudSeed(analysis))
	rng := rand.New(rand.NewSource(seed))
	occupied := []chatCloudRect{{0, 0, chatCloudWidth, 112}, {0, chatCloudHeight - 54, chatCloudWidth, chatCloudHeight}}
	maxCount := max(1, analysis.Words[0].Count)
	minCount := analysis.Words[len(analysis.Words)-1].Count

	for index, word := range analysis.Words {
		ratio := 1.0
		if maxCount != minCount {
			ratio = math.Log(float64(word.Count-minCount)+1) / math.Log(float64(maxCount-minCount)+1)
		}
		size := 24 + ratio*43
		if index == 0 {
			size = max(size, 62)
		}
		placeChatCloudWord(img, parsed, word.Text, size, index, occupied, rng, func(rect chatCloudRect) {
			occupied = append(occupied, rect)
		})
	}

	drawChatCloudHeader(img, parsed, analysis)
	return encodeChatCloud(img)
}

func loadChatCloudFont() (*truetype.Font, error) {
	chatCloudFontOnce.Do(func() {
		searchDirs := []string{
			services.DataPath("fonts"),
			"/usr/share/fonts/opentype/noto",
			"/usr/share/fonts/truetype/noto",
			"/System/Library/Fonts",
		}
		for _, dir := range searchDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			sort.SliceStable(entries, func(i, j int) bool {
				a, b := strings.ToLower(entries[i].Name()), strings.ToLower(entries[j].Name())
				return strings.Contains(a, "noto") && !strings.Contains(b, "noto")
			})
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".ttf") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err != nil {
					continue
				}
				parsed, err := truetype.Parse(data)
				if err == nil {
					chatCloudFont = parsed
					return
				}
			}
		}
		chatCloudFontErr = fmt.Errorf("未找到可用中文 TTF 字体，请放入 %s", services.DataPath("fonts"))
	})
	return chatCloudFont, chatCloudFontErr
}

func drawChatCloudBackground(img *image.RGBA) {
	for y := 0; y < chatCloudHeight; y++ {
		t := float64(y) / chatCloudHeight
		for x := 0; x < chatCloudWidth; x++ {
			xGlow := math.Exp(-math.Pow((float64(x)-760)/360, 2))
			r := uint8(13 + 12*t + 12*xGlow)
			g := uint8(17 + 7*t + 4*xGlow)
			b := uint8(38 + 24*t + 20*xGlow)
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	// Quiet constellation texture keeps unused areas intentional without
	// competing with the words.
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 120; i++ {
		x := rng.Intn(chatCloudWidth)
		y := 100 + rng.Intn(chatCloudHeight-155)
		a := uint8(22 + rng.Intn(36))
		img.SetRGBA(x, y, color.RGBA{180, 201, 255, a})
	}

	drawRoundRect(img, image.Rect(28, 24, chatCloudWidth-28, 98), 24, color.RGBA{255, 255, 255, 12})
	drawRoundRect(img, image.Rect(28, chatCloudHeight-48, chatCloudWidth-28, chatCloudHeight-18), 14, color.RGBA{255, 255, 255, 9})
}

func drawChatCloudHeader(img *image.RGBA, parsed *truetype.Font, analysis chatAnalysis) {
	titleFace := truetype.NewFace(parsed, &truetype.Options{Size: 27, DPI: chatCloudDPI, Hinting: font.HintingFull})
	smallFace := truetype.NewFace(parsed, &truetype.Options{Size: 13, DPI: chatCloudDPI, Hinting: font.HintingFull})
	defer titleFace.Close()
	defer smallFace.Close()

	drawChatText(img, titleFace, color.RGBA{245, 248, 255, 255}, 52, 69, analysis.Label+"群聊词云")
	meta := fmt.Sprintf("%d 条消息  ·  %d 人参与  ·  本地统计，不调用 AI", analysis.Total, analysis.Participants)
	metaWidth := font.MeasureString(smallFace, meta).Ceil()
	drawChatText(img, smallFace, color.RGBA{178, 190, 224, 255}, chatCloudWidth-52-metaWidth, 64, meta)
	drawChatText(img, smallFace, color.RGBA{137, 151, 190, 255}, 48, chatCloudHeight-27, "月灵从聊天中捞出了这些高频词")
}

func placeChatCloudWord(img *image.RGBA, parsed *truetype.Font, text string, initialSize float64, index int, occupied []chatCloudRect, rng *rand.Rand, add func(chatCloudRect)) {
	for shrink := 0; shrink < 6; shrink++ {
		size := initialSize - float64(shrink*3)
		if size < 18 {
			return
		}
		face := truetype.NewFace(parsed, &truetype.Options{Size: size, DPI: chatCloudDPI, Hinting: font.HintingFull})
		width := font.MeasureString(face, text).Ceil()
		height := face.Metrics().Height.Ceil()
		if width > chatCloudWidth-80 {
			face.Close()
			continue
		}

		for attempt := 0; attempt < 420; attempt++ {
			angle := float64(attempt)*0.42 + float64(index)*0.71
			radius := 3.1 * math.Sqrt(float64(attempt)) * 3.1
			cx := chatCloudWidth/2 + int(math.Cos(angle)*radius)
			cy := 340 + int(math.Sin(angle)*radius*0.62)
			if index > 0 {
				cx += rng.Intn(9) - 4
				cy += rng.Intn(7) - 3
			}
			rect := chatCloudRect{
				x0: cx - width/2 - 6,
				y0: cy - height/2 - 4,
				x1: cx + (width+1)/2 + 6,
				y1: cy + (height+1)/2 + 4,
			}
			if rect.x0 < 34 || rect.y0 < 110 || rect.x1 > chatCloudWidth-34 || rect.y1 > chatCloudHeight-56 || overlapsChatCloud(rect, occupied) {
				continue
			}
			baseline := cy + (face.Metrics().Ascent.Ceil()-face.Metrics().Descent.Ceil())/2
			clr := chatCloudPalette[(index+rng.Intn(len(chatCloudPalette)))%len(chatCloudPalette)]
			if index > 14 {
				clr.A = 220
			}
			drawChatText(img, face, clr, cx-width/2, baseline, text)
			add(rect)
			face.Close()
			return
		}
		face.Close()
	}
}

func overlapsChatCloud(rect chatCloudRect, occupied []chatCloudRect) bool {
	for _, other := range occupied {
		if rect.x0 < other.x1 && rect.x1 > other.x0 && rect.y0 < other.y1 && rect.y1 > other.y0 {
			return true
		}
	}
	return false
}

func drawChatText(dst draw.Image, face font.Face, clr color.Color, x, baseline int, text string) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	d.DrawString(text)
}

func drawRoundRect(img *image.RGBA, rect image.Rectangle, radius int, clr color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := max(max(rect.Min.X+radius-x, 0), x-(rect.Max.X-radius-1))
			dy := max(max(rect.Min.Y+radius-y, 0), y-(rect.Max.Y-radius-1))
			if dx*dx+dy*dy <= radius*radius {
				blendChatCloudPixel(img, x, y, clr)
			}
		}
	}
}

func blendChatCloudPixel(img *image.RGBA, x, y int, over color.RGBA) {
	base := img.RGBAAt(x, y)
	a := uint16(over.A)
	inv := 255 - a
	img.SetRGBA(x, y, color.RGBA{
		R: uint8((uint16(over.R)*a + uint16(base.R)*inv) / 255),
		G: uint8((uint16(over.G)*a + uint16(base.G)*inv) / 255),
		B: uint8((uint16(over.B)*a + uint16(base.B)*inv) / 255),
		A: 255,
	})
}

func chatCloudSeed(analysis chatAnalysis) uint64 {
	h := fnv.New64a()
	h.Write([]byte(analysis.Label))
	for _, word := range analysis.Words {
		h.Write([]byte(word.Text))
		h.Write([]byte(fmt.Sprintf("%d", word.Count)))
	}
	return h.Sum64()
}

func encodeChatCloud(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
