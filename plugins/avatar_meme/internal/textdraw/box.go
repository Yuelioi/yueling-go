package textdraw

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type BoxStyle struct {
	MaxFontSize int
	MinFontSize int
	LineGap     int
	Color       color.Color
}

type HorizontalAlign uint8

const (
	AlignCenter HorizontalAlign = iota
	AlignLeft
	AlignRight
)

type BoxLayout struct {
	face        font.Face
	lines       []string
	widths      []int
	lineAdvance int
	blockHeight int
}

func FitBox(parsed *opentype.Font, text string, bounds image.Rectangle, style BoxStyle) (*BoxLayout, error) {
	text = strings.TrimSpace(text)
	if parsed == nil || text == "" || bounds.Empty() || style.MaxFontSize <= 0 || style.MinFontSize <= 0 || style.MinFontSize > style.MaxFontSize {
		return nil, errors.New("文字布局参数无效")
	}
	for size := style.MaxFontSize; size >= style.MinFontSize; size-- {
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
			Size: float64(size), DPI: 72, Hinting: font.HintingFull,
		})
		if err != nil {
			return nil, errors.New("文字字体加载失败")
		}
		lines, widths, ok := wrapText(face, text, bounds.Dx())
		metrics := face.Metrics()
		lineAdvance := metrics.Height.Ceil() + max(0, style.LineGap)
		blockHeight := metrics.Ascent.Ceil() + metrics.Descent.Ceil()
		if len(lines) > 1 {
			blockHeight += (len(lines) - 1) * lineAdvance
		}
		if ok && blockHeight <= bounds.Dy() {
			return &BoxLayout{face: face, lines: lines, widths: widths, lineAdvance: lineAdvance, blockHeight: blockHeight}, nil
		}
		closeFace(face)
	}
	return nil, errors.New("文字过长，请缩短后重试")
}

func DrawCenteredBox(dst draw.Image, parsed *opentype.Font, text string, bounds image.Rectangle, style BoxStyle) error {
	layout, err := FitBox(parsed, text, bounds, style)
	if err != nil {
		return err
	}
	defer layout.Close()
	layout.Draw(dst, bounds, style.Color)
	return nil
}

func (l *BoxLayout) Draw(dst draw.Image, bounds image.Rectangle, ink color.Color) {
	l.DrawAligned(dst, bounds, ink, AlignCenter)
}

func (l *BoxLayout) DrawAligned(dst draw.Image, bounds image.Rectangle, ink color.Color, align HorizontalAlign) {
	if l == nil || l.face == nil || dst == nil {
		return
	}
	if ink == nil {
		ink = color.NRGBA{R: 24, G: 24, B: 27, A: 255}
	}
	metrics := l.face.Metrics()
	baseline := bounds.Min.Y + (bounds.Dy()-l.blockHeight)/2 + metrics.Ascent.Ceil()
	for i, line := range l.lines {
		var x int
		switch align {
		case AlignLeft:
			x = bounds.Min.X
		case AlignRight:
			x = bounds.Max.X - l.widths[i]
		default:
			x = bounds.Min.X + (bounds.Dx()-l.widths[i])/2
		}
		d := font.Drawer{
			Dst:  dst,
			Src:  image.NewUniform(ink),
			Face: l.face,
			Dot:  fixed.P(x, baseline+i*l.lineAdvance),
		}
		d.DrawString(line)
	}
}

func (l *BoxLayout) LineCount() int {
	if l == nil {
		return 0
	}
	return len(l.lines)
}

func (l *BoxLayout) Height() int {
	if l == nil {
		return 0
	}
	return l.blockHeight
}

func (l *BoxLayout) Close() {
	if l == nil {
		return
	}
	closeFace(l.face)
	l.face = nil
}

func wrapText(face font.Face, text string, maxWidth int) ([]string, []int, bool) {
	var lines []string
	var widths []int
	for _, paragraph := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		var line strings.Builder
		for _, r := range paragraph {
			candidate := line.String() + string(r)
			if font.MeasureString(face, candidate).Ceil() <= maxWidth {
				line.WriteRune(r)
				continue
			}
			current := strings.TrimSpace(line.String())
			if current == "" {
				return nil, nil, false
			}
			lines = append(lines, current)
			widths = append(widths, font.MeasureString(face, current).Ceil())
			line.Reset()
			if !unicode.IsSpace(r) {
				line.WriteRune(r)
			}
		}
		if current := strings.TrimSpace(line.String()); current != "" {
			lines = append(lines, current)
			widths = append(widths, font.MeasureString(face, current).Ceil())
		}
	}
	return lines, widths, len(lines) > 0
}

func closeFace(face font.Face) {
	if closer, ok := face.(interface{ Close() error }); ok {
		closer.Close()
	}
}
