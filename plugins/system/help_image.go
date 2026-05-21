package system

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"os"
	"strings"
	"sync"

	ft "github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"

	"github.com/Yuelioi/yueling-go/services"
)

// ── Canvas constants ──────────────────────────────────────────────────────────

const (
	helpW   = 720
	helpPad = 24
	helpDPI = 96.0

	szTitle = 22.0
	szHead  = 16.0
	szBody  = 14.0
	szSmall = 12.0
)

// ── Color palette ─────────────────────────────────────────────────────────────

var (
	hclrBg    = color.RGBA{248, 249, 252, 255}
	hclrTitle = color.RGBA{26, 27, 62, 255}
	hclrSub   = color.RGBA{107, 114, 128, 255}
	hclrGBg   = color.RGBA{224, 231, 255, 255}
	hclrGrp   = color.RGBA{55, 48, 163, 255}
	hclrID    = color.RGBA{124, 58, 237, 255}
	hclrName  = color.RGBA{30, 41, 59, 255}
	hclrDesc  = color.RGBA{100, 116, 139, 255}
	hclrSep   = color.RGBA{226, 232, 240, 255}
	hclrCBg   = color.RGBA{241, 245, 249, 255}
	hclrCmd   = color.RGBA{5, 150, 105, 255}
	hclrUsage = color.RGBA{51, 65, 85, 255}
	hclrBadge = color.RGBA{199, 210, 254, 255}
)

// ── Font state ────────────────────────────────────────────────────────────────

var (
	hOnce   sync.Once
	hfont   *truetype.Font
	hfTitle font.Face
	hfHead  font.Face
	hfBody  font.Face
	hfSmall font.Face
	hfReady bool
)

// ── Image cache ───────────────────────────────────────────────────────────────

var (
	helpListCache []byte
	helpListMu    sync.RWMutex
)

// PreRenderHelpImage renders the help list in background at startup.
func PreRenderHelpImage() {
	go func() {
		log.Println("[help] pre-rendering help image...")
		data, err := RenderHelpListImage()
		if err != nil {
			log.Printf("[help] render failed: %v", err)
			return
		}
		helpListMu.Lock()
		helpListCache = data
		helpListMu.Unlock()
		log.Printf("[help] image ready (%dKB)", len(data)/1024)
	}()
}

// ── Font loading ──────────────────────────────────────────────────────────────

func initHelpFont() {
	hOnce.Do(func() {
		hfont = loadFirstTTF(services.DataPath("fonts"))
		if hfont == nil {
			return
		}
		newFace := func(size float64) font.Face {
			return truetype.NewFace(hfont, &truetype.Options{
				Size: size, DPI: helpDPI, Hinting: font.HintingNone,
			})
		}
		hfTitle = newFace(szTitle)
		hfHead = newFace(szHead)
		hfBody = newFace(szBody)
		hfSmall = newFace(szSmall)
		hfReady = true
		log.Println("[help] font initialized")
	})
}

func loadFirstTTF(dir string) *truetype.Font {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".ttf") {
			continue
		}
		data, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		f, err := truetype.Parse(data)
		if err != nil {
			continue
		}
		log.Printf("[help] loaded font: %s", e.Name())
		return f
	}
	return nil
}

// ── Painter: one freetype Context per font size, shared glyph cache ───────────

// painter holds one ft.Context per font size so glyph bitmaps are cached
// across the entire image. Recreating a context throws away the cache and
// forces every glyph to be re-rasterized — that's what made rendering slow.
type painter struct {
	img  *image.RGBA
	ctxs map[float64]*ft.Context
}

func newPainter(img *image.RGBA) *painter {
	return &painter{img: img, ctxs: make(map[float64]*ft.Context)}
}

// put draws s at (x, baseline) in the given color and size, returns new x.
func (p *painter) put(size float64, clr color.Color, x, y int, s string) int {
	c, ok := p.ctxs[size]
	if !ok {
		c = ft.NewContext()
		c.SetDPI(helpDPI)
		c.SetFont(hfont)
		c.SetFontSize(size)
		c.SetClip(p.img.Bounds())
		c.SetDst(p.img)
		p.ctxs[size] = c
	}
	c.SetSrc(image.NewUniform(clr))
	pt, _ := c.DrawString(s, ft.Pt(x, y))
	return int(pt.X >> 6)
}

// hMW returns the pixel width of s at the given face.
func hMW(face font.Face, s string) int {
	return font.MeasureString(face, s).Ceil()
}

// hLH returns line height in pixels for a font size.
func hLH(size float64) int {
	return int(size*helpDPI/72*1.4 + 0.5)
}

// hAsc returns the ascent (baseline offset from top) for a face.
func hAsc(face font.Face) int {
	return face.Metrics().Ascent.Ceil()
}

func hFill(img *image.RGBA, x, y, w, h int, c color.Color) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(c), image.Point{}, draw.Src)
}

func hEncode(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func base64Image(data []byte) string {
	return "base64://" + base64.StdEncoding.EncodeToString(data)
}

func hWrap(maxW int, text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if hMW(hfBody, line) <= maxW {
			out = append(out, line)
			continue
		}
		runes := []rune(line)
		start := 0
		for start < len(runes) {
			end := start + 1
			for end < len(runes) && hMW(hfBody, string(runes[start:end+1])) <= maxW {
				end++
			}
			out = append(out, string(runes[start:end]))
			start = end
		}
	}
	return out
}

// ── List image ────────────────────────────────────────────────────────────────

func RenderHelpListImage() ([]byte, error) {
	initHelpFont()
	if !hfReady {
		return nil, fmt.Errorf("font not loaded — put a TTF in %s", services.DataPath("fonts"))
	}

	usable := helpW - helpPad*2

	h := helpPad
	h += hLH(szTitle) + 6
	h += hLH(szSmall) + 10
	h += 1 + 14
	for _, grp := range groupOrder {
		if len(pluginGroups[grp]) == 0 {
			continue
		}
		h += hLH(szHead) + 14 + 8
		h += len(pluginGroups[grp]) * (hLH(szBody) + 6)
		h += 12
	}
	h += helpPad

	img := image.NewRGBA(image.Rect(0, 0, helpW, h))
	hFill(img, 0, 0, helpW, h, hclrBg)
	pa := newPainter(img)

	y := helpPad

	pa.put(szTitle, hclrTitle, helpPad, y+hAsc(hfTitle), "月灵插件清单")
	y += hLH(szTitle) + 6

	pa.put(szSmall, hclrSub, helpPad, y+hAsc(hfSmall), "帮助 <ID / 名称>  查看插件详情")
	y += hLH(szSmall) + 10

	hFill(img, helpPad, y, usable, 1, hclrSep)
	y += 14

	xName := helpPad + 52
	xDesc := xName + 158

	for _, grp := range groupOrder {
		entries := pluginGroups[grp]
		if len(entries) == 0 {
			continue
		}
		gh := hLH(szHead) + 14
		hFill(img, helpPad-8, y, usable+16, gh, hclrGBg)
		pa.put(szHead, hclrGrp, helpPad, y+7+hAsc(hfHead), "【"+grp+"】")
		y += gh + 6

		for _, e := range entries {
			bl := y + hAsc(hfBody)
			pa.put(szBody, hclrID, helpPad+4, bl, fmt.Sprintf("#%-2d", e.ID))
			pa.put(szBody, hclrName, xName, bl, e.Name)

			desc := e.Desc
			maxW := helpW - helpPad - xDesc
			if hMW(hfBody, desc) > maxW {
				rr := []rune(desc)
				lo, hi := 0, len(rr)
				for lo < hi {
					mid := (lo + hi + 1) / 2
					if hMW(hfBody, string(rr[:mid])+"…") <= maxW {
						lo = mid
					} else {
						hi = mid - 1
					}
				}
				desc = string(rr[:lo]) + "…"
			}
			pa.put(szBody, hclrDesc, xDesc, bl, desc)
			y += hLH(szBody) + 6
		}
		y += 12
	}

	return hEncode(img)
}

// ── Detail image ──────────────────────────────────────────────────────────────

func RenderHelpDetailImage(p *pluginEntry) ([]byte, error) {
	initHelpFont()
	if !hfReady {
		return nil, fmt.Errorf("font not loaded")
	}

	usable := helpW - helpPad*2
	lines := hWrap(usable-16, p.Usage)

	h := helpPad
	h += hLH(szTitle) + 10
	if len(p.Commands) > 0 {
		h += hLH(szSmall) + 10
	}
	h += 1 + 12
	h += len(lines)*(hLH(szBody)+2) + 16
	h += helpPad

	img := image.NewRGBA(image.Rect(0, 0, helpW, h))
	hFill(img, 0, 0, helpW, h, hclrBg)
	pa := newPainter(img)

	y := helpPad

	x := pa.put(szTitle, hclrTitle, helpPad, y+hAsc(hfTitle), p.Name)
	x += 10
	badge := " " + p.Group + " "
	bw := hMW(hfSmall, badge) + 4
	bh := hLH(szSmall) + 4
	by := y + (hLH(szTitle)-bh)/2
	hFill(img, x, by, bw, bh, hclrBadge)
	pa.put(szSmall, hclrGrp, x+2, by+2+hAsc(hfSmall), badge)
	x += bw + 8
	pa.put(szSmall, hclrID, x, y+(hLH(szTitle)-hLH(szSmall))/2+hAsc(hfSmall), fmt.Sprintf("#%d", p.ID))
	y += hLH(szTitle) + 10

	if len(p.Commands) > 0 {
		pa.put(szSmall, hclrCmd, helpPad, y+hAsc(hfSmall), strings.Join(p.Commands, "  /  "))
		y += hLH(szSmall) + 10
	}

	hFill(img, helpPad, y, usable, 1, hclrSep)
	y += 12

	blockH := len(lines)*(hLH(szBody)+2) + 12
	hFill(img, helpPad, y, usable, blockH, hclrCBg)
	y += 8
	for _, line := range lines {
		pa.put(szBody, hclrUsage, helpPad+8, y+hAsc(hfBody), line)
		y += hLH(szBody) + 2
	}

	return hEncode(img)
}
