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
	chatCloudWidth          = 960
	chatCloudHeight         = 640
	chatCloudDPI            = 96
	chatCloudMinFontSize    = 16
	chatCloudMaxFontSize    = 67
	chatCloudCollisionGap   = 5
	chatCloudPlacementTries = 6500
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

var chatCloudWordArea = image.Rect(34, 110, chatCloudWidth-34, chatCloudHeight-56)

type chatCloudWordSprite struct {
	ink       *image.Alpha
	collision *image.Alpha
}

type chatCloudPlacement struct {
	text   string
	at     image.Point
	color  color.RGBA
	sprite *chatCloudWordSprite
}

type chatCloudOccupancy struct {
	width    int
	height   int
	pixels   []uint8
	integral []int
}

func renderChatWordCloud(analysis chatAnalysis) ([]byte, error) {
	if len(analysis.Words) == 0 {
		return nil, fmt.Errorf("没有可生成词云的词语")
	}
	parsed, err := loadChatCloudFont()
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, chatCloudWidth, chatCloudHeight))
	drawChatCloudBackground(img)

	seed := int64(chatCloudSeed(analysis))
	rng := rand.New(rand.NewSource(seed))
	placements := layoutChatCloudWords(parsed, analysis.Words, rng)
	if len(placements) == 0 {
		return nil, fmt.Errorf("没有词语能放入词云画布")
	}
	for _, placement := range placements {
		rect := placement.sprite.ink.Bounds().Add(placement.at)
		draw.DrawMask(img, rect, image.NewUniform(placement.color), image.Point{}, placement.sprite.ink, image.Point{}, draw.Over)
	}

	drawChatCloudHeader(img, parsed, analysis)
	return encodeChatCloud(img)
}

func loadChatCloudFont() (*truetype.Font, error) {
	chatCloudFontOnce.Do(func() {
		searchDirs := []string{
			services.DataPath("fonts"),
			services.DataPath("fortune", "fonts"),
			`C:\Windows\Fonts`,
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
				preferred := func(name string) bool {
					return strings.Contains(name, "noto") || strings.Contains(name, "sourcehan") ||
						strings.Contains(name, "sakura") || strings.Contains(name, "simhei") || strings.Contains(name, "msyh")
				}
				return preferred(a) && !preferred(b)
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
				if err == nil && chatCloudFontSupportsCJK(parsed) {
					chatCloudFont = parsed
					return
				}
			}
		}
		chatCloudFontErr = fmt.Errorf("未找到可用中文 TTF 字体，请放入 %s", services.DataPath("fonts"))
	})
	return chatCloudFont, chatCloudFontErr
}

func chatCloudFontSupportsCJK(parsed *truetype.Font) bool {
	if parsed == nil {
		return false
	}
	for _, r := range "月灵词云消息群聊" {
		if parsed.Index(r) == 0 {
			return false
		}
	}
	return true
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

func layoutChatCloudWords(parsed *truetype.Font, words []chatWord, rng *rand.Rand) []chatCloudPlacement {
	if len(words) == 0 {
		return nil
	}
	maxCount, minCount := words[0].Count, words[0].Count
	for _, word := range words[1:] {
		maxCount = max(maxCount, word.Count)
		minCount = min(minCount, word.Count)
	}

	occupied := newChatCloudOccupancy(chatCloudWidth, chatCloudHeight)
	placements := make([]chatCloudPlacement, 0, len(words))
	for index, word := range words {
		ratio := 1.0
		if maxCount != minCount {
			ratio = math.Log(float64(word.Count-minCount)+1) / math.Log(float64(maxCount-minCount)+1)
		}
		initialSize := float64(chatCloudMinFontSize+5) + ratio*float64(chatCloudMaxFontSize-chatCloudMinFontSize-5)
		if index == 0 {
			initialSize = max(initialSize, 62)
		}
		placement, ok := placeChatCloudWord(parsed, word.Text, initialSize, index, occupied, rng)
		if !ok {
			continue
		}
		placement.color = chatCloudPalette[(index+rng.Intn(len(chatCloudPalette)))%len(chatCloudPalette)]
		if index > 14 {
			placement.color.A = 225
		}
		placements = append(placements, placement)
	}
	return placements
}

func placeChatCloudWord(parsed *truetype.Font, text string, initialSize float64, index int, occupied *chatCloudOccupancy, rng *rand.Rand) (chatCloudPlacement, bool) {
	phase := float64(index)*0.71 + rng.Float64()*0.35
	for size := initialSize; size >= chatCloudMinFontSize; size -= 3 {
		sprite := newChatCloudWordSprite(parsed, text, size)
		if sprite == nil || sprite.collision.Bounds().Dx() > chatCloudWordArea.Dx() || sprite.collision.Bounds().Dy() > chatCloudWordArea.Dy() {
			continue
		}
		width, height := sprite.collision.Bounds().Dx(), sprite.collision.Bounds().Dy()
		xRadius := float64(chatCloudWordArea.Dx()-width) / 2
		yRadius := float64(chatCloudWordArea.Dy()-height) / 2
		center := chatCloudWordArea.Min.Add(image.Pt(chatCloudWordArea.Dx()/2, chatCloudWordArea.Dy()/2))
		for attempt := 0; attempt < chatCloudPlacementTries; attempt++ {
			progress := math.Sqrt(float64(attempt) / float64(chatCloudPlacementTries-1))
			angle := float64(attempt)*2.399963229728653 + phase
			cx := center.X + int(math.Round(math.Cos(angle)*xRadius*progress))
			cy := center.Y + int(math.Round(math.Sin(angle)*yRadius*progress))
			at := image.Pt(cx-width/2, cy-height/2)
			if !sprite.collision.Bounds().Add(at).In(chatCloudWordArea) || occupied.collides(sprite.collision, at) {
				continue
			}
			occupied.add(sprite.collision, at)
			return chatCloudPlacement{text: text, at: at, sprite: sprite}, true
		}
	}
	return chatCloudPlacement{}, false
}

func newChatCloudWordSprite(parsed *truetype.Font, text string, size float64) *chatCloudWordSprite {
	face := truetype.NewFace(parsed, &truetype.Options{Size: size, DPI: chatCloudDPI, Hinting: font.HintingFull})
	defer face.Close()
	bounds, _ := font.BoundString(face, text)
	minX, minY := bounds.Min.X.Floor(), bounds.Min.Y.Floor()
	maxX, maxY := bounds.Max.X.Ceil(), bounds.Max.Y.Ceil()
	if maxX <= minX || maxY <= minY {
		return nil
	}
	padding := chatCloudCollisionGap + 1
	ink := image.NewAlpha(image.Rect(0, 0, maxX-minX+2*padding, maxY-minY+2*padding))
	d := font.Drawer{
		Dst:  ink,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(padding-minX, padding-minY),
	}
	d.DrawString(text)
	return &chatCloudWordSprite{ink: ink, collision: dilateChatCloudMask(ink, chatCloudCollisionGap)}
}

func dilateChatCloudMask(src *image.Alpha, radius int) *image.Alpha {
	dst := image.NewAlpha(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			if src.AlphaAt(x, y).A == 0 {
				continue
			}
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					if dx*dx+dy*dy > radius*radius {
						continue
					}
					point := image.Pt(x+dx, y+dy)
					if point.In(dst.Bounds()) {
						dst.SetAlpha(point.X, point.Y, color.Alpha{A: 255})
					}
				}
			}
		}
	}
	return dst
}

func newChatCloudOccupancy(width, height int) *chatCloudOccupancy {
	return &chatCloudOccupancy{
		width:    width,
		height:   height,
		pixels:   make([]uint8, width*height),
		integral: make([]int, (width+1)*(height+1)),
	}
}

func (o *chatCloudOccupancy) collides(mask *image.Alpha, at image.Point) bool {
	rect := mask.Bounds().Add(at)
	if o.sum(rect) == 0 {
		return false
	}
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			if mask.AlphaAt(x, y).A != 0 && o.pixels[(at.Y+y)*o.width+at.X+x] != 0 {
				return true
			}
		}
	}
	return false
}

func (o *chatCloudOccupancy) add(mask *image.Alpha, at image.Point) {
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			if mask.AlphaAt(x, y).A != 0 {
				o.pixels[(at.Y+y)*o.width+at.X+x] = 1
			}
		}
	}
	o.rebuildIntegral()
}

func (o *chatCloudOccupancy) rebuildIntegral() {
	stride := o.width + 1
	for y := 1; y <= o.height; y++ {
		rowSum := 0
		for x := 1; x <= o.width; x++ {
			rowSum += int(o.pixels[(y-1)*o.width+x-1])
			o.integral[y*stride+x] = o.integral[(y-1)*stride+x] + rowSum
		}
	}
}

func (o *chatCloudOccupancy) sum(rect image.Rectangle) int {
	rect = rect.Intersect(image.Rect(0, 0, o.width, o.height))
	if rect.Empty() {
		return 0
	}
	stride := o.width + 1
	return o.integral[rect.Max.Y*stride+rect.Max.X] - o.integral[rect.Min.Y*stride+rect.Max.X] -
		o.integral[rect.Max.Y*stride+rect.Min.X] + o.integral[rect.Min.Y*stride+rect.Min.X]
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
