package funny

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
)

func fortuneDir() string { return services.DataPath("fortune") }

// Matching Python original: font sizes and layout constants.
const (
	fortuneTitleSize   = 45
	fortuneTextSize    = 25
	fortuneLineH       = fortuneTextSize + 4 // 29px — matches Python's (font_size + 4) line step
	fortuneCardinality = 9                   // max chars per column

	fortuneTitleCX = 140 // image_font_center[0] for title
	fortuneTitleCY = 99  // image_font_center[1] for title
	fortuneTextCX  = 140 // image_font_center[0] for content
	fortuneTextCY  = 297 // image_font_center[1] for content
)

type copywritingFile struct {
	Copywriting []fortuneItem `json:"copywriting"`
}

type fortuneItem struct {
	GoodLuck string   `json:"good-luck"`
	Content  []string `json:"content"`
}

var (
	fortuneItems  []fortuneItem
	fortuneTitleF font.Face // Mamelon.otf 45pt
	fortuneTextF  font.Face // sakura.ttf 25pt
	fortuneReady  bool
)

func loadFortuneAssets() error {
	if fortuneReady {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(fortuneDir(), "copywriting.json"))
	if err != nil {
		return fmt.Errorf("copywriting.json: %w", err)
	}
	var cf copywritingFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	fortuneItems = cf.Copywriting

	fortuneTitleF, err = loadFortuneFace(filepath.Join(fortuneDir(), "fonts", "Mamelon.otf"), fortuneTitleSize)
	if err != nil {
		fortuneTitleF, err = loadFortuneFace(filepath.Join(fortuneDir(), "fonts", "sakura.ttf"), fortuneTitleSize)
		if err != nil {
			return fmt.Errorf("title font: %w", err)
		}
	}
	fortuneTextF, err = loadFortuneFace(filepath.Join(fortuneDir(), "fonts", "sakura.ttf"), fortuneTextSize)
	if err != nil {
		return fmt.Errorf("text font: %w", err)
	}

	os.MkdirAll(filepath.Join(fortuneDir(), "cache"), 0o755)
	fortuneReady = true
	return nil
}

func loadFortuneFace(path string, size float64) (font.Face, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(b)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func pickThemeImage(theme string) (string, error) {
	themesDir := filepath.Join(fortuneDir(), "themes")
	if theme == "" {
		entries, err := os.ReadDir(themesDir)
		if err != nil || len(entries) == 0 {
			return "", fmt.Errorf("no themes")
		}
		theme = entries[rand.Intn(len(entries))].Name()
	}
	imgs, err := os.ReadDir(filepath.Join(themesDir, theme))
	if err != nil || len(imgs) == 0 {
		return "", fmt.Errorf("theme %q not found or empty", theme)
	}
	img := imgs[rand.Intn(len(imgs))]
	return filepath.Join(themesDir, theme, img.Name()), nil
}

// fortuneDecrement mirrors Python fortune's decrement(): splits text into columns
// of at most fortuneCardinality chars, padding with spaces to align columns.
func fortuneDecrement(text string) (int, [][]rune) {
	runes := []rune(text)
	length := len(runes)
	if length == 0 {
		return 1, [][]rune{{}}
	}

	colNum := 1
	remaining := length
	for remaining > fortuneCardinality {
		colNum++
		remaining -= fortuneCardinality
	}

	spaces := func(n int) []rune {
		s := make([]rune, n)
		for i := range s {
			s[i] = ' '
		}
		return s
	}

	if colNum == 2 {
		if length%2 == 0 {
			half := length / 2
			fill := spaces(fortuneCardinality - half)
			col0 := append(append([]rune{}, runes[:half]...), fill...)
			col1 := append(append([]rune{}, fill...), runes[half:]...)
			return 2, [][]rune{col0, col1}
		}
		halfUp := (length + 1) / 2
		fill := spaces(fortuneCardinality - halfUp)
		col0 := append(append([]rune{}, runes[:halfUp]...), fill...)
		col1 := append(append([]rune{}, fill...), ' ')
		col1 = append(col1, runes[halfUp:]...)
		return 2, [][]rune{col0, col1}
	}

	var cols [][]rune
	for i := 0; i < colNum; i++ {
		if i == colNum-1 {
			cols = append(cols, runes[i*fortuneCardinality:])
		} else {
			cols = append(cols, runes[i*fortuneCardinality:(i+1)*fortuneCardinality])
		}
	}
	return colNum, cols
}

// generateFortuneImage returns (isFirst, imageBytes, error).
// isFirst is false when the user already drew today (served from cache).
func generateFortuneImage(userID int64, theme string) (bool, []byte, error) {
	if err := loadFortuneAssets(); err != nil {
		return false, nil, err
	}

	date := bot.Today()
	cachePath := filepath.Join(fortuneDir(), "cache", fmt.Sprintf("%d-%s.png", userID, date))
	if data, err := os.ReadFile(cachePath); err == nil {
		return false, data, nil
	}

	bgPath, err := pickThemeImage(theme)
	if err != nil {
		return false, nil, err
	}
	f, err := os.Open(bgPath)
	if err != nil {
		return false, nil, err
	}
	bg, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return false, nil, err
	}

	item := fortuneItems[rand.Intn(len(fortuneItems))]
	content := item.Content[rand.Intn(len(item.Content))]

	// Draw directly on the background image — no overlay panel (matches Python original).
	canvas := image.NewRGBA(bg.Bounds())
	draw.Draw(canvas, canvas.Bounds(), bg, image.Point{}, draw.Src)

	// Title: Mamelon.otf 45pt, #F5F5F5, bounding-box-centered at (140, 99).
	drawBBoxCenter(canvas, fortuneTitleF, color.RGBA{245, 245, 245, 255}, item.GoodLuck, fortuneTitleCX, fortuneTitleCY)

	// Content: sakura.ttf 25pt, #323232, vertical right-to-left columns centered at (140, 297).
	textClr := color.RGBA{50, 50, 50, 255}
	numCols, cols := fortuneDecrement(content)
	for i, col := range cols {
		// Python: x = center_x + (slices-2)*font_size/2 + (slices-1)*4 - i*(font_size+4)
		colX := fortuneTextCX + (numCols-2)*fortuneTextSize/2 + (numCols-1)*4 - i*fortuneLineH
		colY := fortuneTextCY - len(col)*fortuneLineH/2
		for j, r := range col {
			if r != ' ' && r != '\n' {
				fortuneDrawChar(canvas, fortuneTextF, textClr, r, colX, colY+j*fortuneLineH)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return false, nil, err
	}
	data := buf.Bytes()
	os.WriteFile(cachePath, data, 0o644)
	return true, data, nil
}

// drawBBoxCenter renders text with its bounding box centered at (cx, cy),
// matching Python PIL's draw.text placement after bbox centering.
func drawBBoxCenter(img *image.RGBA, face font.Face, clr color.Color, text string, cx, cy int) {
	m := face.Metrics()
	lineH := m.Ascent.Ceil() + m.Descent.Ceil()
	// PIL places y at top of bbox; Go needs baseline = top + ascent.
	// center_y = top + lineH/2  →  baseline = center_y - lineH/2 + ascent
	baselineY := cy - lineH/2 + m.Ascent.Ceil()
	adv := font.MeasureString(face, text)
	x := cx - adv.Ceil()/2
	d := &font.Drawer{
		Dst: img, Src: image.NewUniform(clr), Face: face,
		Dot: fixed.P(x, baselineY),
	}
	d.DrawString(text)
}

// fortuneDrawChar draws a single rune at (x, y) where y is the top of the char cell.
func fortuneDrawChar(img *image.RGBA, face font.Face, clr color.Color, r rune, x, y int) {
	m := face.Metrics()
	d := &font.Drawer{
		Dst: img, Src: image.NewUniform(clr), Face: face,
		Dot: fixed.P(x, y+m.Ascent.Ceil()),
	}
	d.DrawString(string(r))
}

func RegisterFortune(b *bot.Bot) {
	b.OnCommand("今日运势", "运势", "抽签").Handle(func(ctx *bot.CommandContext) error {
		theme := ""
		if len(ctx.Args) > 0 {
			theme = strings.ToLower(ctx.Args[0])
		}
		isFirst, imgData, err := generateFortuneImage(ctx.UserID(), theme)
		if err != nil {
			return ctx.Reply("今日运势生成出错……")
		}
		prefix := "✨今日运势✨\n"
		if !isFirst {
			prefix = "你今天抽过签了，再给你看一次哦🤗\n"
		}
		_, err = ctx.SendGroupMsg(ctx.GroupID(),
			bot.Msg().Text(prefix).ImageBytes(imgData).At(ctx.UserID()).Build())
		return err
	})
}
