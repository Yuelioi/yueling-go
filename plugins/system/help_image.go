package system

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/logx"
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
	hclrRow   = color.RGBA{255, 255, 255, 255}
	hclrCmd   = color.RGBA{5, 150, 105, 255}
	hclrUsage = color.RGBA{51, 65, 85, 255}
	hclrBadge = color.RGBA{199, 210, 254, 255}
)

// ── Font state ────────────────────────────────────────────────────────────────

var (
	hOnce   sync.Once
	hfont   *opentype.Font
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
		logx.Infof("[help] pre-rendering help image...")
		data, err := RenderHelpListImage()
		if err != nil {
			logx.Errorf("[help] render failed: %v", err)
			return
		}
		helpListMu.Lock()
		helpListCache = data
		helpListMu.Unlock()
		logx.Infof("[help] image ready (%dKB)", len(data)/1024)
	}()
}

// ── Font loading ──────────────────────────────────────────────────────────────

func initHelpFont() {
	hOnce.Do(func() {
		hfont = loadFirstOpenTypeFont(services.DataPath("fonts"))
		if hfont == nil {
			return
		}
		newFace := func(size float64) font.Face {
			face, err := opentype.NewFace(hfont, &opentype.FaceOptions{
				Size: size, DPI: helpDPI, Hinting: font.HintingNone,
			})
			if err != nil {
				return nil
			}
			return face
		}
		hfTitle = newFace(szTitle)
		hfHead = newFace(szHead)
		hfBody = newFace(szBody)
		hfSmall = newFace(szSmall)
		if hfTitle == nil || hfHead == nil || hfBody == nil || hfSmall == nil {
			return
		}
		hfReady = true
		logx.Infof("[help] font initialized")
	})
}

func loadFirstOpenTypeFont(dir string) *opentype.Font {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".ttf" && ext != ".otf" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		f, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		logx.Infof("[help] loaded font: %s", e.Name())
		return f
	}
	return nil
}

// ── Painter: one OpenType face per font size, shared glyph cache ─────────────

// painter holds one face per font size so glyph bitmaps are cached across the
// entire image instead of rebuilding a face for every string.
type painter struct {
	img   *image.RGBA
	faces map[float64]font.Face
}

func newPainter(img *image.RGBA) *painter {
	return &painter{img: img, faces: make(map[float64]font.Face)}
}

// put draws s at (x, baseline) in the given color and size, returns new x.
func (p *painter) put(size float64, clr color.Color, x, y int, s string) int {
	face, ok := p.faces[size]
	if !ok {
		var err error
		face, err = opentype.NewFace(hfont, &opentype.FaceOptions{
			Size: size, DPI: helpDPI, Hinting: font.HintingNone,
		})
		if err != nil {
			return x
		}
		p.faces[size] = face
	}
	drawer := font.Drawer{
		Dst:  p.img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	advance := drawer.MeasureString(s)
	drawer.DrawString(s)
	return x + advance.Ceil()
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
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hWrapFace(face font.Face, maxW int, text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if hMW(face, line) <= maxW {
			out = append(out, line)
			continue
		}
		runes := []rune(line)
		start := 0
		for start < len(runes) {
			end := start + 1
			for end < len(runes) && hMW(face, string(runes[start:end+1])) <= maxW {
				end++
			}
			out = append(out, string(runes[start:end]))
			start = end
		}
	}
	return out
}

func hWrap(maxW int, text string) []string {
	return hWrapFace(hfBody, maxW, text)
}

// ── Usage table model ────────────────────────────────────────────────────────

type helpUsageRow struct {
	Command     string
	Description string
	Note        string
	Spacer      bool
}

func parseHelpUsageRows(usage string) []helpUsageRow {
	lines := strings.Split(strings.Trim(usage, "\n"), "\n")
	rows := make([]helpUsageRow, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			if len(rows) > 0 && !rows[len(rows)-1].Spacer {
				rows = append(rows, helpUsageRow{Spacer: true})
			}
			continue
		}
		command, description, separated := splitHelpUsageColumns(line)
		if separated {
			rows = append(rows, helpUsageRow{Command: command, Description: description})
			continue
		}
		if isHelpUsageNote(line) {
			rows = append(rows, helpUsageRow{Note: line})
			continue
		}
		rows = append(rows, helpUsageRow{Command: line})
	}
	for len(rows) > 0 && rows[len(rows)-1].Spacer {
		rows = rows[:len(rows)-1]
	}
	return rows
}

func splitHelpUsageColumns(line string) (command, description string, separated bool) {
	runes := []rune(line)
	for i := 0; i < len(runes); {
		if !unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		start := i
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if i-start < 2 {
			continue
		}
		left := strings.TrimSpace(string(runes[:start]))
		right := strings.TrimSpace(string(runes[i:]))
		if left != "" && right != "" {
			return left, right, true
		}
	}
	return line, "", false
}

func isHelpUsageNote(line string) bool {
	if strings.HasSuffix(line, "：") {
		return true
	}
	for _, prefix := range []string{
		"图片优先级：", "不提供图片", "需要图片", "无文字时", "支持同时",
		"所有命令", "每群最多", "直接发送", "可用主题目录", "递归展开",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type helpUsageLayoutRow struct {
	CommandLines     []string
	DescriptionLines []string
	NoteLines        []string
	Spacer           bool
	Height           int
}

func layoutHelpUsageRows(rows []helpUsageRow, commandWidth, descriptionWidth, noteWidth int) []helpUsageLayoutRow {
	const rowPadY = 7
	bodyLineH := hLH(szBody) + 2
	noteLineH := hLH(szSmall) + 1
	layouts := make([]helpUsageLayoutRow, 0, len(rows))
	for _, row := range rows {
		layout := helpUsageLayoutRow{Spacer: row.Spacer}
		switch {
		case row.Spacer:
			layout.Height = 8
		case row.Note != "":
			layout.NoteLines = hWrapFace(hfSmall, noteWidth, row.Note)
			layout.Height = len(layout.NoteLines)*noteLineH + rowPadY*2
		default:
			layout.CommandLines = hWrap(commandWidth, row.Command)
			if row.Description != "" {
				layout.DescriptionLines = hWrap(descriptionWidth, row.Description)
			}
			lineCount := max(1, len(layout.CommandLines), len(layout.DescriptionLines))
			layout.Height = lineCount*bodyLineH + rowPadY*2
		}
		layouts = append(layouts, layout)
	}
	return layouts
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

	const (
		tablePadX       = 12
		rowPadY         = 7
		tableCommandW   = 300
		tableHeaderPadY = 7
	)
	usable := helpW - helpPad*2
	commandTextW := tableCommandW - tablePadX*2
	descriptionTextW := usable - tableCommandW - tablePadX*2
	noteTextW := usable - tablePadX*2
	descriptionLines := hWrapFace(hfSmall, usable, p.Desc)
	rows := layoutHelpUsageRows(
		parseHelpUsageRows(p.Usage),
		commandTextW,
		descriptionTextW,
		noteTextW,
	)
	tableHeaderH := hLH(szSmall) + tableHeaderPadY*2

	h := helpPad
	h += hLH(szTitle) + 10
	if p.Desc != "" {
		h += len(descriptionLines)*(hLH(szSmall)+1) + 10
	}
	h += tableHeaderH
	for _, row := range rows {
		h += row.Height
	}
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

	if p.Desc != "" {
		for _, line := range descriptionLines {
			pa.put(szSmall, hclrSub, helpPad, y+hAsc(hfSmall), line)
			y += hLH(szSmall) + 1
		}
		y += 9
	}

	// A real table owns column positions and row heights. This avoids relying on
	// spaces whose visual width varies with the selected Chinese font.
	hFill(img, helpPad, y, usable, tableHeaderH, hclrGBg)
	pa.put(szSmall, hclrGrp, helpPad+tablePadX, y+tableHeaderPadY+hAsc(hfSmall), "命令")
	pa.put(szSmall, hclrGrp, helpPad+tableCommandW+tablePadX, y+tableHeaderPadY+hAsc(hfSmall), "说明")
	hFill(img, helpPad+tableCommandW, y, 1, tableHeaderH, hclrSep)
	hFill(img, helpPad, y+tableHeaderH-1, usable, 1, hclrSep)
	y += tableHeaderH

	visibleRow := 0
	for _, row := range rows {
		if row.Spacer {
			y += row.Height
			continue
		}
		if visibleRow%2 == 0 {
			hFill(img, helpPad, y, usable, row.Height, hclrRow)
		} else {
			hFill(img, helpPad, y, usable, row.Height, hclrCBg)
		}
		visibleRow++

		if len(row.NoteLines) > 0 {
			lineY := y + rowPadY
			for _, line := range row.NoteLines {
				pa.put(szSmall, hclrSub, helpPad+tablePadX, lineY+hAsc(hfSmall), line)
				lineY += hLH(szSmall) + 1
			}
		} else {
			hFill(img, helpPad+tableCommandW, y, 1, row.Height, hclrSep)
			lineY := y + rowPadY
			for _, line := range row.CommandLines {
				pa.put(szBody, hclrCmd, helpPad+tablePadX, lineY+hAsc(hfBody), line)
				lineY += hLH(szBody) + 2
			}
			lineY = y + rowPadY
			for _, line := range row.DescriptionLines {
				pa.put(szBody, hclrUsage, helpPad+tableCommandW+tablePadX, lineY+hAsc(hfBody), line)
				lineY += hLH(szBody) + 2
			}
		}
		hFill(img, helpPad, y+row.Height-1, usable, 1, hclrSep)
		y += row.Height
	}

	return hEncode(img)
}
